#!/usr/bin/env sh
# Standalone bootstrapper and installed lifecycle implementation.
set -eu
umask 077

# ROOT is useful for package builders and isolated lifecycle tests; normal installs use /.
ROOT="${THREE_M_UI_ROOT:-}"
BASE="$ROOT/usr/local/lib/3m-ui"
ENTRY="$ROOT/usr/local/bin/3m-ui"
CONFIG_DIR="$ROOT/etc/3m-ui"
DATA_DIR="${THREE_M_UI_DATA_DIR:-}"
if [ -z "$DATA_DIR" ] && [ -s "$BASE/DATA_DIRECTORY" ]; then DATA_DIR="$(cat "$BASE/DATA_DIRECTORY")"; fi
DATA_DIR="${DATA_DIR:-$ROOT/var/lib/3m-ui}"
LOG_DIR="$ROOT/var/log/3m-ui"
CONFIG_FILE="$CONFIG_DIR/config.yaml"
APP_BIN="$BASE/3m-ui-bin"
MIHOMO_BIN="$BASE/mihomo"
SERVICE_NAME=3m-ui
REPO="${THREE_M_UI_REPO:-}"
CHANNEL="${THREE_M_UI_CHANNEL:-}"
REQUESTED_VERSION=""
INSTALL_MIHOMO=1
OPERATION=install
YES=0
SNAPSHOT=""
TRANSACTION=0
STOPPED=0
WAS_RUNNING=0
WORK=""

say(){ printf '%s\n' "$*"; }
err(){ say "Error: $*" >&2; exit 1; }
command_exists(){ command -v "$1" >/dev/null 2>&1; }
usage(){ cat <<USAGE
Usage: install.sh [VERSION] [--yes] [--no-mihomo]
       3m-ui update [VERSION] [--yes] [--no-mihomo]
       3m-ui backup
       3m-ui restore /absolute/path/to/snapshot.tar.gz [--yes]

The default channel is stable. Complete panel + Mihomo bundles support amd64
and arm64. Other release architectures require --no-mihomo and a custom core.

Environment:
  THREE_M_UI_REPO=owner/repo       Release source (saved for future upgrades)
  THREE_M_UI_CHANNEL=stable|pre   Update channel (saved for future upgrades)
  THREE_M_UI_VERIFY_COSIGN=1      Require release provenance verification
  PANEL_PORT=8080                 Initial panel port (existing config is kept)
  THREE_M_UI_MIHOMO_BINARY=PATH   Explicit external core for a new installation

A version selects that release for this operation; 'pre' selects the pre channel.
Updates stop the service only after downloads pass checksum validation, keep a
complete local snapshot, and restore it if initialization or health checks fail.
USAGE
}
for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES=1;;
    --no-mihomo) INSTALL_MIHOMO=0;;
    --static|--dynamic) :;; # Compatibility: official panel artifacts are static.
    --update) OPERATION=update;;
    --backup) OPERATION=backup;;
    --restore) OPERATION=restore;;
    -h|--help) usage; exit 0;;
    v[0-9]*|manual-[0-9]*|test-[0-9a-zA-Z._-]*|pre)
      [ -z "$REQUESTED_VERSION" ] || err 'Only one version may be specified.'
      REQUESTED_VERSION="$arg";;
    /*) [ "$OPERATION" = restore ] || err "Unknown argument: $arg"; SNAPSHOT="$arg";;
    *) err "Unknown argument: $arg";;
  esac
done

init_system(){
  if [ -d "$ROOT/run/systemd/system" ] && command_exists systemctl; then echo systemd
  elif command_exists rc-service; then echo openrc
  else echo unsupported; fi
}
service_action(){
  case "$INIT" in
    systemd) systemctl "$1" "$SERVICE_NAME";;
    openrc) rc-service "$SERVICE_NAME" "$1";;
    *) return 1;;
  esac
}
service_active(){
  case "$INIT" in
    systemd) systemctl is-active --quiet "$SERVICE_NAME";;
    openrc) rc-service "$SERVICE_NAME" status >/dev/null 2>&1;;
    *) return 1;;
  esac
}
reload_service(){ [ "$INIT" != systemd ] || systemctl daemon-reload; }
stop_existing(){
  if [ -e "$UNIT" ] || service_active; then service_action stop; fi
  STOPPED=1
}
arch(){
  case "$(uname -m)" in
    x86_64|amd64) echo amd64;; aarch64|arm64) echo arm64;;
    armv7l|armv7*) echo armv7;; armv6l|armv6*) echo armv6;;
    i386|i486|i586|i686|x86) echo 386;; riscv64) echo riscv64;;
    loongarch64|loong64) echo loong64;; ppc64le) echo ppc64le;; s390x) echo s390x;;
    *) err "Unsupported architecture: $(uname -m)";;
  esac
}
install_deps(){
  if command_exists curl && command_exists tar && command_exists gzip &&
     { command_exists sha256sum || command_exists shasum || command_exists openssl; }; then return; fi
  if command_exists apk; then apk add --no-cache curl ca-certificates tar gzip coreutils
  elif command_exists apt-get; then apt-get update && apt-get install -y curl ca-certificates tar gzip coreutils
  elif command_exists dnf; then dnf install -y curl ca-certificates tar gzip coreutils
  elif command_exists yum; then yum install -y curl ca-certificates tar gzip coreutils
  elif command_exists pacman; then pacman -Sy --noconfirm curl ca-certificates tar gzip coreutils
  elif command_exists zypper; then zypper --non-interactive install curl ca-certificates tar gzip coreutils
  else err 'Install curl, CA certificates, tar, gzip and sha256sum before continuing.'; fi
}
download(){ curl -fLsS --retry 3 --retry-delay 1 --connect-timeout 10 --max-time 300 "$1" -o "$2"; }
file_sha256(){
  if command_exists sha256sum; then sha256sum "$1" | awk '{print $1}'
  elif command_exists shasum; then shasum -a 256 "$1" | awk '{print $1}'
  else openssl dgst -sha256 "$1" | awk '{print $NF}'; fi
}
source_settings(){
  [ -n "$REPO" ] || { [ ! -s "$BASE/REPOSITORY" ] || REPO="$(cat "$BASE/REPOSITORY")"; }
  REPO="${REPO:-kazeyukiro/3m-ui}"
  case "$REPO" in *[!a-zA-Z0-9._/-]*|/*|*/|*/*/*|*/../*|../*) err 'Invalid THREE_M_UI_REPO; expected owner/repository.';; esac
  case "$REPO" in */*) :;; *) err 'Invalid THREE_M_UI_REPO; expected owner/repository.';; esac
  [ -n "$CHANNEL" ] || { [ ! -s "$BASE/CHANNEL" ] || CHANNEL="$(cat "$BASE/CHANNEL")"; }
  CHANNEL="${CHANNEL:-stable}"
  [ "$REQUESTED_VERSION" != pre ] || CHANNEL=pre
  [ "$CHANNEL" != prerelease ] || CHANNEL=pre
  case "$CHANNEL" in stable|pre) :;; *) err 'THREE_M_UI_CHANNEL must be stable or pre.';; esac
  if [ -n "$REQUESTED_VERSION" ]; then TAG="$REQUESTED_VERSION"
  elif [ "$CHANNEL" = pre ]; then TAG=pre
  else
    # GitHub latest excludes prereleases. Never silently fall back to a development build.
    location="$(curl -fLsSI --retry 3 --connect-timeout 10 --max-time 60 -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")" || err 'No stable release available; choose a version or explicitly select THREE_M_UI_CHANNEL=pre.'
    TAG="${location##*/}"
    case "$TAG" in v[0-9]*) :;; *) err 'Unable to resolve a stable release; choose an explicit version.';; esac
  fi
  case "$TAG" in *[!a-zA-Z0-9._+-]*) err 'Invalid release tag.';; esac
  RELEASE_URL="https://github.com/$REPO/releases/download/$TAG"
}
verify_asset(){
  expected="$(awk -v a="$1" '{ n=$2; sub(/^\*/,"",n); sub(/^\.\//,"",n); if(n==a) {print $1; exit} }' "$WORK/SHA256SUMS")"
  [ -n "$expected" ] || err "$1 is missing from SHA256SUMS."
  actual="$(file_sha256 "$2")"
  [ "$actual" = "$expected" ] || err "Checksum mismatch for $1."
  say "Checksum OK: $1"
}
verify_provenance(){
  [ "${THREE_M_UI_VERIFY_COSIGN:-0}" = 1 ] || return 0
  command_exists cosign || err 'THREE_M_UI_VERIFY_COSIGN=1 requires cosign.'
  download "$RELEASE_URL/SHA256SUMS.pem" "$WORK/SHA256SUMS.pem"
  download "$RELEASE_URL/SHA256SUMS.sig" "$WORK/SHA256SUMS.sig"
  workflow=release.yml
  [ "$TAG" != pre ] || workflow=release-pre.yml
  for ref in "refs/tags/$TAG" refs/heads/main refs/heads/test; do
    if cosign verify-blob --certificate "$WORK/SHA256SUMS.pem" --signature "$WORK/SHA256SUMS.sig" \
      --certificate-identity "https://github.com/$REPO/.github/workflows/$workflow@$ref" \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com "$WORK/SHA256SUMS" >/dev/null 2>&1; then return; fi
  done
  err 'Release signature verification failed.'
}
prepare_release(){
  mkdir -p "$WORK/release"
  download "$RELEASE_URL/SHA256SUMS" "$WORK/SHA256SUMS"
  verify_provenance
  cpu="$(arch)"
  if [ "$INSTALL_MIHOMO" = 1 ]; then
    case "$cpu" in amd64|arm64) :;; *) err "Complete bundles are available for amd64/arm64; use --no-mihomo for $cpu.";; esac
    asset="3m-ui-bundle-linux-$cpu.tar.gz"
    say "Downloading $REPO $TAG ($asset)..."
    download "$RELEASE_URL/$asset" "$WORK/bundle.tar.gz"
    verify_asset "$asset" "$WORK/bundle.tar.gz"
    # Bundles contain only these flat, regular files. Reject unexpected names/links.
    tar -tzf "$WORK/bundle.tar.gz" > "$WORK/members"
    while IFS= read -r member; do
      case "${member#./}" in 3m-ui-bin|mihomo|MIHOMO_VERSION|VERSION|REPOSITORY|mihomo.env|LICENSE|MIHOMO_LICENSE|MIHOMO_SOURCE|install.sh|update.sh|uninstall.sh|3m-ui.sh|3m-ui) :;; *) err "Unexpected bundle member: $member";; esac
    done < "$WORK/members"
    tar -tvzf "$WORK/bundle.tar.gz" | awk 'substr($0,1,1)!="-" { bad=1 } END {exit bad}' || err 'Bundle contains non-regular files.'
    tar -xzf "$WORK/bundle.tar.gz" -C "$WORK/release"
    for member in 3m-ui-bin mihomo MIHOMO_VERSION VERSION REPOSITORY install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui; do
      [ -s "$WORK/release/$member" ] || err "Incomplete bundle: $member missing."
    done
    [ "$(cat "$WORK/release/REPOSITORY")" = "$REPO" ] || err 'Bundle repository does not match the selected release source.'
  else
    asset="3m-ui-linux-$cpu"
    download "$RELEASE_URL/$asset" "$WORK/release/3m-ui-bin"
    verify_asset "$asset" "$WORK/release/3m-ui-bin"
    for member in install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui; do
      download "$RELEASE_URL/$member" "$WORK/release/$member"
      verify_asset "$member" "$WORK/release/$member"
    done
  fi
  for member in install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui; do sh -n "$WORK/release/$member"; done
  chmod 0755 "$WORK/release/3m-ui-bin"
  "$WORK/release/3m-ui-bin" --version >/dev/null
  if [ "$INSTALL_MIHOMO" = 1 ]; then chmod 0755 "$WORK/release/mihomo"; "$WORK/release/mihomo" -v >/dev/null; fi
}

validate_storage(){
  [ -f "$CONFIG_FILE" ] || return 0
  # Use the new binary to inspect old settings read-only, before any downtime or
  # migration. Refuse to claim a complete backup of shared external directories.
  inspection_bin="$1"
  (cd "$DATA_DIR" && THREE_M_UI_CONFIG="$CONFIG_FILE" "$inspection_bin" storage-paths) > "$WORK/storage-paths"
  while IFS= read -r path; do
    case "$path" in "$CONFIG_DIR"|"$CONFIG_DIR"/*|"$DATA_DIR"|"$DATA_DIR"/*) :;;
      *) err "Storage outside the managed configuration/data directories: $path. Move it into $CONFIG_DIR or $DATA_DIR before using automatic install/update/backup.";;
    esac
    case "$path" in "$DATA_DIR/backups"|"$DATA_DIR/backups"/*) err "Active storage cannot live in the backup directory: $path";; esac
  done < "$WORK/storage-paths"
}

# Snapshots cover configuration, encryption keys, SQLite/WAL, certificates, all
# app data, managed binaries/helpers, the entrypoint and service definition.
create_snapshot(){
  mkdir -p "$WORK/snapshot" "$DATA_DIR/backups"
  for spec in "base:$BASE" "config:$CONFIG_DIR" "entry:$ENTRY" "service:$UNIT"; do
    name="${spec%%:*}"; path="${spec#*:}"
    if [ -e "$path" ] || [ -L "$path" ]; then cp -a "$path" "$WORK/snapshot/$name"; fi
  done
  if [ -d "$DATA_DIR" ]; then
    mkdir -p "$WORK/snapshot/data"
    # Intermediate archive propagates tar errors rather than hiding a failing pipeline.
    tar -cf "$WORK/data.tar" --exclude='./backups' -C "$DATA_DIR" .
    tar -xf "$WORK/data.tar" -C "$WORK/snapshot/data"
  fi
  printf '%s\n' '3m-ui-snapshot-v1' > "$WORK/snapshot/FORMAT"
  printf '%s\n' "$BASE" "$CONFIG_DIR" "$DATA_DIR" "$ENTRY" "$UNIT" > "$WORK/snapshot/PATHS"
  printf '%s\n' "$WAS_RUNNING" > "$WORK/snapshot/RUNNING"
  enabled=0
  case "$INIT" in
    systemd) systemctl is-enabled --quiet "$SERVICE_NAME" && enabled=1 || true;;
    openrc) [ ! -e "$ROOT/etc/runlevels/default/$SERVICE_NAME" ] || enabled=1;;
  esac
  printf '%s\n' "$enabled" > "$WORK/snapshot/ENABLED"
  SNAPSHOT="$DATA_DIR/backups/$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gz"
  tar -czf "$SNAPSHOT.tmp" -C "$WORK/snapshot" .
  mv "$SNAPSHOT.tmp" "$SNAPSHOT"
  say "Snapshot: $SNAPSHOT"
}
restore_files(){
  restore_dir="$1"
  # Undo enablement created by a failed first installation before removing its unit.
  if [ "$(cat "$restore_dir/ENABLED")" = 0 ]; then
    case "$INIT" in
      systemd) systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true;;
      openrc) rc-update del "$SERVICE_NAME" default >/dev/null 2>&1 || true;;
    esac
  fi
  # Keep backup archives available across rollback and manual restores.
  mkdir -p "$DATA_DIR" || return 1
  for path in "$DATA_DIR"/* "$DATA_DIR"/.[!.]* "$DATA_DIR"/..?*; do
    [ -e "$path" ] || [ -L "$path" ] || continue
    [ "$path" = "$DATA_DIR/backups" ] || rm -rf "$path" || return 1
  done
  [ ! -d "$restore_dir/data" ] || cp -a "$restore_dir/data/." "$DATA_DIR/" || return 1
  for spec in "base:$BASE" "config:$CONFIG_DIR" "entry:$ENTRY" "service:$UNIT"; do
    name="${spec%%:*}"; path="${spec#*:}"
    rm -rf "$path" || return 1
    if [ -e "$restore_dir/$name" ] || [ -L "$restore_dir/$name" ]; then
      mkdir -p "$(dirname "$path")" || return 1
      cp -a "$restore_dir/$name" "$path" || return 1
    fi
  done
  reload_service || return 1
  if [ "$(cat "$restore_dir/ENABLED")" = 1 ]; then
    case "$INIT" in
      systemd) systemctl enable "$SERVICE_NAME" >/dev/null;;
      openrc) rc-update add "$SERVICE_NAME" default >/dev/null;;
    esac
  fi
}
cleanup(){
  result=$?
  trap - EXIT HUP INT TERM
  if [ "$TRANSACTION" = 1 ]; then
    say 'Operation failed; restoring the previous installation and data.' >&2
    service_action stop >/dev/null 2>&1 || true
    if [ -d "$WORK/snapshot" ] && restore_files "$WORK/snapshot"; then
      if [ "$WAS_RUNNING" = 1 ]; then service_action start || say 'Could not restart the restored service; inspect 3m-ui logs.' >&2; fi
    else
      say "Automatic restore failed. Recovery snapshot: $SNAPSHOT" >&2
    fi
    [ "$result" != 0 ] || result=1
  fi
  if [ "$TRANSACTION" = 0 ] && [ "$STOPPED" = 1 ] && [ "$WAS_RUNNING" = 1 ]; then
    service_action start || say 'Could not restart the previous service; inspect 3m-ui logs.' >&2
  fi
  [ -z "$WORK" ] || rm -rf "$WORK"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM

write_service(){
  # Keep operator overrides, capabilities and enablement on an existing install.
  # Older systemd syscall filters cannot run the current pure-Go SQLite build.
  if [ -f "$WORK/snapshot/service" ]; then
    if [ "$INIT" = systemd ]; then
      if grep -q '^SystemCallFilter=' "$UNIT"; then
        sed '/^SystemCallFilter=/d; /^PrivateDevices=/d; /^MemoryDenyWriteExecute=/d; /^RestrictNamespaces=/d' "$UNIT" > "$WORK/service"
        install -m 0644 "$WORK/service" "$UNIT"
      fi
      systemctl daemon-reload
    fi
    return 0
  fi
  mkdir -p "$(dirname "$UNIT")"
  if [ "$INIT" = systemd ]; then
    cat > "$UNIT" <<UNITFILE
[Unit]
Description=3m-ui panel and Mihomo Core
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
ExecStart=$APP_BIN
Environment=THREE_M_UI_CONFIG=$CONFIG_FILE
WorkingDirectory=$DATA_DIR
Restart=always
RestartSec=5
KillMode=control-group
UMask=0077
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
ReadWritePaths=$CONFIG_DIR $DATA_DIR $LOG_DIR
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
SystemCallArchitectures=native
NoNewPrivileges=true
LimitNOFILE=65535
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
[Install]
WantedBy=multi-user.target
UNITFILE
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME" >/dev/null
  else
    cat > "$UNIT" <<UNITFILE
#!/sbin/openrc-run
description="3m-ui panel and Mihomo Core"
command="$APP_BIN"
command_background="yes"
pidfile="/run/$SERVICE_NAME.pid"
directory="$DATA_DIR"
export THREE_M_UI_CONFIG="$CONFIG_FILE"
output_log="$LOG_DIR/$SERVICE_NAME.log"
error_log="$LOG_DIR/$SERVICE_NAME.log"
respawn_delay=5
supervisor=supervise-daemon
depend() { need net; after firewall; }
UNITFILE
    chmod 0755 "$UNIT"
    rc-update add "$SERVICE_NAME" default >/dev/null
  fi
}
wait_healthy(){
  attempt=0
  while [ "$attempt" -lt 30 ]; do
    if service_active && (cd "$DATA_DIR" && THREE_M_UI_CONFIG="$CONFIG_FILE" "$APP_BIN" healthcheck) >/dev/null 2>&1; then return 0; fi
    attempt=$((attempt + 1))
    sleep 1
  done
  err 'Service health check failed after 30 attempts.'
}
manual_restore(){
  [ -f "$SNAPSHOT" ] || err 'Usage: 3m-ui restore /absolute/path/to/snapshot.tar.gz [--yes]'
  mkdir -p "$WORK/restore"
  # Only archives produced by this installer are accepted; never source metadata.
  tar -tzf "$SNAPSHOT" > "$WORK/members"
  while IFS= read -r member; do
    case "$member" in /*|../*|*/../*|*/..) err 'Unsafe snapshot member.';; esac
    case "${member#./}" in ''|FORMAT|PATHS|RUNNING|ENABLED|base|base/*|config|config/*|data|data/*|entry|service) :;; *) err 'Unrecognized snapshot layout.';; esac
  done < "$WORK/members"
  tar -xzf "$SNAPSHOT" -C "$WORK/restore"
  [ "$(cat "$WORK/restore/FORMAT")" = '3m-ui-snapshot-v1' ] || err 'Unsupported snapshot format.'
  printf '%s\n' "$BASE" "$CONFIG_DIR" "$DATA_DIR" "$ENTRY" "$UNIT" > "$WORK/expected-paths"
  cmp -s "$WORK/expected-paths" "$WORK/restore/PATHS" || err 'Snapshot paths differ from this installation; restore on the original host paths.'
  if [ "$YES" != 1 ]; then
    [ -t 0 ] || err 'Non-interactive restore requires --yes.'
    printf 'Replace the installation and data with this snapshot? [y/N] '
    read -r answer
    case "$answer" in y|Y|yes|YES) :;; *) exit 0;; esac
  fi
  validate_storage "$APP_BIN"
  service_active && WAS_RUNNING=1 || true
  stop_existing
  # Keep a snapshot of the current installation so a failed restore is reversible.
  create_snapshot
  TRANSACTION=1
  restore_files "$WORK/restore"
  service_action start
  wait_healthy
  TRANSACTION=0
  STOPPED=0
  say 'Snapshot restored and service is healthy.'
}
main(){
  [ "$(id -u)" -eq 0 ] || err 'Please run as root.'
  [ "$(uname -s)" = Linux ] || err 'Native installation requires Linux.'
  # Avoid ambiguous paths in generated service definitions and snapshot metadata.
  for path in "$ROOT" "$BASE" "$CONFIG_DIR" "$DATA_DIR"; do
    case "$path" in *[[:space:]]*|*/../*|*/..|/|/etc|/usr|/var) err "Unsupported installation path: $path";; esac
  done
  case "$DATA_DIR" in /*) :;; *) err 'THREE_M_UI_DATA_DIR must be an absolute path.';; esac
  INIT="$(init_system)"
  [ "$INIT" != unsupported ] || err 'A systemd or OpenRC Linux host is required.'
  if [ "$INIT" = systemd ]; then UNIT="$ROOT/etc/systemd/system/$SERVICE_NAME.service"; else UNIT="$ROOT/etc/init.d/$SERVICE_NAME"; fi
  WORK="$(mktemp -d)"
  case "$OPERATION" in
    restore) manual_restore; return;;
    backup)
      [ -x "$APP_BIN" ] || err '3m-ui is not installed.'
      validate_storage "$APP_BIN"
      service_active && WAS_RUNNING=1 || true
      stop_existing
      # Cleanup restarts the previous service if creating the snapshot fails.
      create_snapshot
      if [ "$WAS_RUNNING" = 1 ]; then service_action start; fi
      STOPPED=0
      return;;
    update) [ -x "$APP_BIN" ] || err '3m-ui is not installed.';;
  esac
  install_deps
  source_settings
  prepare_release
  validate_storage "$WORK/release/3m-ui-bin"
  service_active && WAS_RUNNING=1 || true
  stop_existing
  create_snapshot
  TRANSACTION=1
  mkdir -p "$BASE" "$(dirname "$ENTRY")" "$CONFIG_DIR" "$DATA_DIR/mihomo" "$LOG_DIR"
  # Retain existing cores and custom files. Only replace release-owned files.
  for member in 3m-ui-bin install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui; do
    install -m 0755 "$WORK/release/$member" "$BASE/$member"
  done
  if [ "$INSTALL_MIHOMO" = 1 ]; then
    install -m 0755 "$WORK/release/mihomo" "$MIHOMO_BIN"
    cp "$WORK/release/MIHOMO_VERSION" "$BASE/MIHOMO_VERSION"
    for member in mihomo.env LICENSE MIHOMO_LICENSE MIHOMO_SOURCE; do
      if [ -f "$WORK/release/$member" ]; then install -m 0644 "$WORK/release/$member" "$BASE/$member"; fi
    done
  fi
  install -m 0755 "$WORK/release/3m-ui" "$ENTRY"
  printf '%s\n' "$TAG" > "$BASE/VERSION"
  printf '%s\n' static > "$BASE/BUILD_MODE"
  printf '%s\n' "$REPO" > "$BASE/REPOSITORY"
  printf '%s\n' "$CHANNEL" > "$BASE/CHANNEL"
  printf '%s\n' "$DATA_DIR" > "$BASE/DATA_DIRECTORY"
  # Existing YAML/DB settings are preserved by init, including external core paths.
  (cd "$DATA_DIR" && THREE_M_UI_CONFIG="$CONFIG_FILE" THREE_M_UI_DATA_DIR="$DATA_DIR" \
    THREE_M_UI_MIHOMO_BINARY="${THREE_M_UI_MIHOMO_BINARY:-$MIHOMO_BIN}" "$APP_BIN" init)
  write_service
  service_action start
  wait_healthy
  TRANSACTION=0
  STOPPED=0
  say "3m-ui $TAG is ready ($REPO, channel $CHANNEL)."
  say 'New installations print a one-time administrator password above. Use 3m-ui reset-admin if it is lost.'
  say "Configuration: $CONFIG_FILE"
  say "Management: 3m-ui   Backup: $SNAPSHOT"
}
main "$@"

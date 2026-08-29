#!/usr/bin/env sh
set -eu
umask 077

REPO="kazeyukiro/3m-ui"
BASE="/usr/local/lib/3m-ui"
APP_BIN="$BASE/3m-ui-bin"
VERSION_FILE="$BASE/VERSION"
MODE_FILE="$BASE/BUILD_MODE"
ENTRY="/usr/local/bin/3m-ui"
CONFIG_DIR="/etc/3m-ui"
DATA_DIR="/var/lib/3m-ui"
LOG_DIR="/var/log/3m-ui"
MIHOMO_BIN="/usr/local/bin/mihomo"
SERVICE_NAME="3m-ui"
YES=0
INSTALL_MIHOMO=1
# Official releases are pure-Go static binaries (portable on glibc + musl).
STATIC="${THREE_M_UI_STATIC:-1}"
REQUESTED_VERSION=""
RELEASE_TAG=""
TMP_FILES=""

say(){ printf '%s\n' "$*"; }
err(){ say "Error: $*" >&2; exit 1; }
command_exists(){ command -v "$1" >/dev/null 2>&1; }
need_root(){ [ "$(id -u)" -eq 0 ] || err "Please run this script as root."; }

cleanup_tmp(){
  # shellcheck disable=SC2086
  [ -n "$TMP_FILES" ] && rm -f $TMP_FILES 2>/dev/null || true
}
trap cleanup_tmp EXIT INT TERM

track_tmp(){
  TMP_FILES="$TMP_FILES $1"
}

usage(){ cat <<EOF
3m-ui installer

Usage: $0 [VERSION] [options]

Options:
  -y, --yes          Non-interactive installation
      --no-mihomo    Do not install Mihomo when it is missing
      --static       Prefer static build (default; official releases are static)
      --dynamic      Legacy no-op (releases are static pure-Go only)
  -h, --help         Show this help

Environment:
  THREE_M_UI_STATIC=1      Prefer static build (default)
  THREE_M_UI_INSECURE=1    Bypass SHA256SUMS verification (NOT recommended; supply-chain risk)

Supported CPU: x86_64, aarch64, armv7, armv6, i386/i686, riscv64, loongarch64, ppc64le, s390x
Supported OS: Alpine, Debian/Ubuntu, RHEL/Fedora/CentOS/Rocky/Alma, Arch, openSUSE, Gentoo, Void, …
EOF
}

for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES=1;;
    --no-mihomo) INSTALL_MIHOMO=0;;
    --static) STATIC=1;;
    --dynamic) STATIC=1;; # official artifacts are always static pure-Go
    -h|--help) usage; exit 0;;
    v[0-9]*|manual-[0-9]*) [ -z "$REQUESTED_VERSION" ] || err "Only one version may be specified."; REQUESTED_VERSION="$arg";;
    *) err "Unknown option: $arg";;
  esac
done

os_id(){
  if [ -r /etc/os-release ]; then . /etc/os-release; printf '%s' "${ID:-unknown}"; else printf '%s' unknown; fi
}

# Map uname -m → release artifact arch suffix (must match release.yml).
arch(){
  case "$(uname -m)" in
    x86_64|amd64) echo amd64;;
    aarch64|arm64) echo arm64;;
    armv7l|armv7*) echo armv7;;
    armv6l|armv6*) echo armv6;;
    i386|i486|i586|i686|x86) echo 386;;
    riscv64) echo riscv64;;
    loongarch64|loong64) echo loong64;;
    ppc64le) echo ppc64le;;
    s390x) echo s390x;;
    *) err "Unsupported CPU architecture: $(uname -m). Supported: x86_64 aarch64 armv7 armv6 i686 riscv64 loongarch64 ppc64le s390x";;
  esac
}

init_system(){ if [ -d /run/systemd/system ] && command_exists systemctl; then echo systemd; elif command_exists rc-service; then echo openrc; else echo unsupported; fi; }

install_deps(){
  missing=""
  for x in curl ca-certificates gzip tar sed; do command_exists "$x" || missing="$missing $x"; done
  [ -z "$missing" ] && return
  case "$(os_id)" in
    alpine) apk add --no-cache curl ca-certificates gzip tar sed coreutils;;
    debian|ubuntu|linuxmint|raspbian|pop|elementary|zorin|kali|parrot) apt-get update && apt-get install -y curl ca-certificates gzip tar sed coreutils;;
    fedora|rhel|centos|rocky|almalinux|oracle|amzn|ol) if command_exists dnf; then dnf install -y curl ca-certificates gzip tar sed coreutils; else yum install -y curl ca-certificates gzip tar sed coreutils; fi;;
    arch|manjaro|endeavouros|garuda|artix) pacman -Sy --noconfirm curl ca-certificates gzip tar sed coreutils;;
    opensuse*|sles) zypper --non-interactive install curl ca-certificates gzip tar sed coreutils;;
    void) xbps-install -Sy curl ca-certificates gzip tar sed coreutils;;
    gentoo) emerge --ask=n net-misc/curl app-arch/gzip app-arch/tar sys-apps/sed sys-apps/coreutils || err "emerge failed; install curl gzip tar sed coreutils manually";;
    *) err "Cannot install dependencies automatically on $(os_id):$missing. Install curl ca-certificates gzip tar sed, then re-run.";;
  esac
}

download(){ if command_exists curl; then curl -fL --retry 3 --retry-delay 1 --connect-timeout 10 --max-time 300 "$1" -o "$2"; else wget -qO "$2" "$1"; fi; }

latest_tag(){
  tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "${1}/releases/latest" 2>/dev/null | sed 's#.*/##; s/[[:space:][:cntrl:]]*$//')" || true
  case "$tag" in
    v[0-9]*|manual-[0-9]*) printf '%s' "$tag";;
    *)
      tag="$(curl -fsSL "${1}/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)" || true
      printf '%s' "$tag"
      ;;
  esac
}

random_hex(){ dd if=/dev/urandom bs=1 count="${1:-32}" 2>/dev/null | od -An -tx1 | tr -d ' \n'; }

# Compute SHA-256 of a file; empty string if no hasher available.
file_sha256(){
  f="$1"
  if command_exists sha256sum; then
    sha256sum "$f" | awk '{print $1}'
  elif command_exists shasum; then
    shasum -a 256 "$f" | awk '{print $1}'
  elif command_exists openssl; then
    openssl dgst -sha256 "$f" | awk '{print $NF}'
  else
    printf ''
  fi
}

# Verify file against release SHA256SUMS. Fail-closed (C-2): any integrity
# failure aborts the install. Set THREE_M_UI_INSECURE=1 to bypass verification
# when GitHub is unavailable (NOT recommended; prints a loud warning).
verify_release_sha256(){
  tag="$1"
  asset="$2"
  path="$3"
  # sha256 tooling is now a hard dependency for release verification.
  if ! command_exists sha256sum && ! command_exists shasum && ! command_exists openssl; then
    err "No sha256 tool available; cannot verify $asset. Install coreutils/sha256sum (or openssl) and re-run."
  fi
  sums_tmp="$(mktemp)"
  track_tmp "$sums_tmp"
  if ! download "https://github.com/$REPO/releases/download/${tag}/SHA256SUMS" "$sums_tmp" 2>/dev/null; then
    if [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
      say "WARNING: THREE_M_UI_INSECURE=1 set; SHA256SUMS not published for $tag; skipping checksum verification (NOT RECOMMENDED)."
      return 0
    fi
    err "SHA256SUMS not published for $tag; cannot verify $asset. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)."
  fi
  # SHA256SUMS may list "./3m-ui-linux-amd64" or "3m-ui-linux-amd64"
  expected="$(awk -v a="$asset" '{ n=$2; sub(/^\.\//,"",n); if (n==a) { print $1; exit } }' "$sums_tmp")"
  if [ -z "$expected" ]; then
    if [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
      say "WARNING: THREE_M_UI_INSECURE=1 set; $asset not listed in SHA256SUMS; skipping verification (NOT RECOMMENDED)."
      return 0
    fi
    err "$asset not listed in SHA256SUMS for $tag; cannot verify. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)."
  fi
  actual="$(file_sha256 "$path")"
  if [ "$actual" != "$expected" ]; then
    err "Checksum mismatch for $asset (expected $expected, got $actual)"
  fi
  say "Checksum OK: $asset"
}

# Verify the Mihomo .gz asset against the upstream SHA256SUMS published by
# MetaCubeX/mihomo (C-3). Best-effort: if Mihomo did not publish SHA256SUMS for
# a release, print a loud warning and return non-zero so the caller can decide
# (fail-closed unless THREE_M_UI_INSECURE=1).
verify_mihomo_sha256(){
  tag="$1"
  asset="$2"
  gzpath="$3"
  if ! command_exists sha256sum && ! command_exists shasum && ! command_exists openssl; then
    err "No sha256 tool available; cannot verify Mihomo $asset. Install coreutils/sha256sum (or openssl) and re-run."
  fi
  sums_tmp="$(mktemp)"; track_tmp "$sums_tmp"
  if ! download "https://github.com/MetaCubeX/mihomo/releases/download/${tag}/SHA256SUMS" "$sums_tmp" 2>/dev/null; then
    say "Warning: Mihomo SHA256SUMS not published for $tag; cannot verify checksum of $asset." >&2
    return 1
  fi
  expected="$(awk -v a="$asset" '{ n=$2; sub(/^\.\//,"",n); if (n==a) { print $1; exit } }' "$sums_tmp")"
  if [ -z "$expected" ]; then
    say "Warning: $asset not listed in Mihomo SHA256SUMS for $tag; cannot verify checksum." >&2
    return 1
  fi
  actual="$(file_sha256 "$gzpath")"
  if [ "$actual" != "$expected" ]; then
    err "Checksum mismatch for Mihomo $asset (expected $expected, got $actual)"
  fi
  say "Checksum OK: Mihomo $asset"
}

write_config(){
  mkdir -p "$CONFIG_DIR" "$DATA_DIR/mihomo" "$LOG_DIR"
  [ -f "$CONFIG_DIR/config.yaml" ] && { chmod 0600 "$CONFIG_DIR/config.yaml"; return; }
  # NAT-friendly: PANEL_PORT / THREE_M_UI_PORT (default 8080), optional PANEL_LISTEN / PUBLIC_URL
  panel_port="${PANEL_PORT:-${THREE_M_UI_PORT:-8080}}"
  case "$panel_port" in
    ''|*[!0-9]*) panel_port=8080;;
  esac
  if [ "$panel_port" -lt 1 ] || [ "$panel_port" -gt 65535 ]; then panel_port=8080; fi
  panel_listen="${PANEL_LISTEN:-${THREE_M_UI_LISTEN:-}}"
  public_url="${PUBLIC_URL:-${THREE_M_UI_PUBLIC_URL:-}}"
  cat > "$CONFIG_DIR/config.yaml" <<EOF
server:
  port: ${panel_port}
  listen: "${panel_listen}"
  public_url: "${public_url}"
  mode: release
database:
  path: "$DATA_DIR/3m-ui.db"
jwt:
  secret: "$(random_hex 32)"
security:
  credential_key: "$(random_hex 32)"
mihomo:
  binary: "$MIHOMO_BIN"
  config: "$DATA_DIR/mihomo/config.yaml"
EOF
  chmod 0600 "$CONFIG_DIR/config.yaml"
}

mihomo_asset(){
  case "$(uname -m)" in
    x86_64|amd64) echo mihomo-linux-amd64-compatible;;
    aarch64|arm64) echo mihomo-linux-arm64;;
    armv7l|armv7*) echo mihomo-linux-armv7;;
    armv6l|armv6*) echo mihomo-linux-armv6;;
    i386|i486|i586|i686|x86) echo mihomo-linux-386;;
    riscv64) echo mihomo-linux-riscv64;;
    loongarch64|loong64) echo mihomo-linux-loong64-abi2.0;;
    *) echo "";;
  esac
}

install_mihomo(){
  [ "$INSTALL_MIHOMO" -eq 1 ] || { say "Mihomo installation skipped."; return; }
  if [ -x "$MIHOMO_BIN" ]; then say "Existing Mihomo detected: $MIHOMO_BIN"; return; fi
  asset="$(mihomo_asset)"
  if [ -z "$asset" ]; then
    say "Warning: no official Mihomo binary for $(uname -m). Install Mihomo manually or re-run with --no-mihomo."
    return 0
  fi
  tag="$(latest_tag https://github.com/MetaCubeX/mihomo)"; [ -n "$tag" ] || err "Unable to determine latest Mihomo release."
  tmp="$(mktemp)"; gz="$tmp.gz"
  track_tmp "$tmp"; track_tmp "$gz"
  gzname="${asset}-${tag}.gz"
  url="https://github.com/MetaCubeX/mihomo/releases/download/${tag}/${gzname}"
  say "Downloading Mihomo $tag..."; download "$url" "$gz"
  # Verify the .gz checksum BEFORE decompressing (Mihomo sums cover the .gz asset) — C-3.
  if ! verify_mihomo_sha256 "$tag" "$gzname" "$gz"; then
    if [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
      say "WARNING: THREE_M_UI_INSECURE=1 set; continuing Mihomo install WITHOUT checksum verification (NOT RECOMMENDED)."
    else
      err "Mihomo checksum verification failed. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)."
    fi
  fi
  gzip -dc "$gz" > "$tmp"; chmod 0755 "$tmp"
  # Smoke-test the decompressed binary without executing it as root (C-5).
  if command_exists file; then
    file "$tmp" | grep -q "ELF" || err "Downloaded Mihomo is not a valid ELF binary."
  fi
  install -m 0755 "$tmp" "$MIHOMO_BIN"
  rm -f "$tmp" "$gz"
}

install_release_asset(){
  tag="$1"; file="$2"; destination="$3"
  tmp="$(mktemp)"; track_tmp "$tmp"
  # H-1: do NOT fall back to raw.githubusercontent.com — releases MUST contain
  # all helper scripts. A missing release asset indicates an incomplete release.
  if ! download "https://github.com/$REPO/releases/download/${tag}/${file}" "$tmp" 2>/dev/null; then
    err "Release $tag does not contain $file. Refusing to fall back to raw.githubusercontent.com for supply-chain safety; use a complete release."
  fi
  verify_release_sha256 "$tag" "$file" "$tmp"
  install -m 0755 "$tmp" "$destination"
  rm -f "$tmp"
}

install_helpers(){
  tag="$1"
  mkdir -p "$BASE"
  install_release_asset "$tag" 3m-ui.sh "$BASE/3m-ui.sh"
  install_release_asset "$tag" install.sh "$BASE/install.sh"
  install_release_asset "$tag" update.sh "$BASE/update.sh"
  install_release_asset "$tag" uninstall.sh "$BASE/uninstall.sh"
  install_release_asset "$tag" 3m-ui "$ENTRY"
}

install_panel(){
  RELEASE_TAG="${REQUESTED_VERSION:-$(latest_tag https://github.com/$REPO)}"
  [ -n "$RELEASE_TAG" ] || err "Unable to determine latest 3m-ui release."
  # Official releases are pure-Go static binaries named 3m-ui-linux-<arch>.
  # Fall back to legacy *-static name for older tags if needed.
  cpu="$(arch)"
  asset="3m-ui-linux-${cpu}"
  tmp="$(mktemp)"; track_tmp "$tmp"
  url="https://github.com/$REPO/releases/download/${RELEASE_TAG}/${asset}"
  say "Downloading 3m-ui $RELEASE_TAG ($asset)..."
  if ! download "$url" "$tmp" 2>/dev/null; then
    asset="3m-ui-linux-${cpu}-static"
    url="https://github.com/$REPO/releases/download/${RELEASE_TAG}/${asset}"
    say "Retrying legacy asset name: $asset"
    download "$url" "$tmp"
  fi
  verify_release_sha256 "$RELEASE_TAG" "$asset" "$tmp"
  chmod 0755 "$tmp"
  # Smoke-test the binary is a Linux ELF — do NOT execute it as root (C-5).
  # verify_release_sha256 already attests to integrity.
  if command_exists file; then
    file "$tmp" | grep -q "ELF" || err "Downloaded 3m-ui is not a valid ELF binary."
  fi
  install -m 0755 "$tmp" "$APP_BIN"
  printf '%s\n' "$RELEASE_TAG" > "$VERSION_FILE"
  printf '%s\n' "$([ "$STATIC" = "1" ] && echo static || echo dynamic)" > "$MODE_FILE"
  chmod 0600 "$VERSION_FILE" "$MODE_FILE"
  rm -f "$tmp"
}

write_systemd(){ cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
[Unit]
Description=3m-ui server panel and Mihomo Core
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$APP_BIN
Environment=THREE_M_UI_CONFIG=$CONFIG_DIR/config.yaml
WorkingDirectory=$DATA_DIR
Restart=always
RestartSec=5
KillMode=control-group
UMask=0077
# Sandbox hardening (H-2). The service still runs as root because Mihomo TUN
# mode requires CAP_NET_ADMIN, which is hard to delegate to a non-root user.
# A future revision should add a dedicated 3m-ui system user with ambient
# capabilities; for now we apply the full Protect*/Restrict*/SystemCallFilter
# set, which is a meaningful improvement even for a root process.
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
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
SystemCallFilter=@system-service
SystemCallFilter=~@privileged @resources
NoNewPrivileges=true
LimitNOFILE=65535
# Allow binding privileged ports (80/443) for optional panel ACME/SSL,
# and CAP_NET_ADMIN for Mihomo TUN mode.
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_ADMIN

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload; systemctl enable "$SERVICE_NAME" >/dev/null; systemctl restart "$SERVICE_NAME"; }

write_openrc(){ cat > /etc/init.d/$SERVICE_NAME <<EOF
#!/sbin/openrc-run
description="3m-ui server panel and Mihomo Core"
command="$APP_BIN"
command_background="yes"
pidfile="/run/$SERVICE_NAME.pid"
directory="$DATA_DIR"
export THREE_M_UI_CONFIG="$CONFIG_DIR/config.yaml"
output_log="$LOG_DIR/$SERVICE_NAME.log"
error_log="$LOG_DIR/$SERVICE_NAME.log"
respawn_delay=5
supervisor=supervise-daemon

depend() { need net; after firewall; }
EOF
  chmod 0755 /etc/init.d/$SERVICE_NAME; rc-update add "$SERVICE_NAME" default >/dev/null 2>&1 || true; rc-service "$SERVICE_NAME" restart; }

install_service(){ case "$(init_system)" in systemd) write_systemd;; openrc) write_openrc;; *) err "Unsupported init system. Supported: systemd and OpenRC.";; esac; }

main(){
  need_root; install_deps
  mkdir -p "$BASE" "$CONFIG_DIR" "$DATA_DIR/mihomo" "$LOG_DIR"
  if [ -t 0 ] && [ "$YES" -ne 1 ]; then
    say "3m-ui Installer"; say "OS: $(os_id)  Architecture: $(arch)  Init: $(init_system)"
    printf 'Install Mihomo Core too? [Y/n] '; read -r answer || true
    case "$answer" in n|N|no|NO) INSTALL_MIHOMO=0;; esac
  fi
  write_config
  if [ -x "$APP_BIN" ]; then
    case "$(init_system)" in systemd) systemctl stop "$SERVICE_NAME" 2>/dev/null || true;; openrc) rc-service "$SERVICE_NAME" stop 2>/dev/null || true;; esac
  fi
  install_panel
  install_mihomo
  install_helpers "$RELEASE_TAG"
  install_service
  say ""
  say "3m-ui installed successfully."
  say "Command: 3m-ui"
  panel_port="${PANEL_PORT:-${THREE_M_UI_PORT:-8080}}"
  say "Panel: http://SERVER_IP:${panel_port}/"
  say "Custom port: PANEL_PORT=8443 curl ... | bash   or edit server.port in $CONFIG_DIR/config.yaml"
  say "Initial administrator: admin / admin. You MUST change the password on first login."
}
main "$@"

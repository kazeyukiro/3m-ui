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
UPDATE_MIHOMO=1
REQUESTED_VERSION=""
BACKUP_KEEP=5

for arg in "$@"; do
  case "$arg" in
    -y|--yes) :;;
    --no-mihomo) UPDATE_MIHOMO=0;;
    -h|--help)
      printf '%s\n' 'Usage: update.sh [VERSION] [--yes] [--no-mihomo]'
      printf '%s\n' '  VERSION   optional tag such as v0.1.0 (default: latest)'
      exit 0
      ;;
    v[0-9]*|manual-[0-9]*)
      [ -z "$REQUESTED_VERSION" ] || { echo "Error: only one version may be specified." >&2; exit 1; }
      REQUESTED_VERSION="$arg"
      ;;
    *) printf '%s\n' "Unknown option: $arg" >&2; exit 2;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "Error: please run as root." >&2; exit 1; }
[ -x "$APP_BIN" ] || { echo "Error: 3m-ui application is not installed at $APP_BIN" >&2; exit 1; }
command_exists(){ command -v "$1" >/dev/null 2>&1; }
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
    *) echo "Error: unsupported architecture: $(uname -m)" >&2; exit 1;;
  esac
}
init_system(){ if [ -d /run/systemd/system ] && command_exists systemctl; then echo systemd; elif command_exists rc-service; then echo openrc; else echo unsupported; fi; }
download(){ if command_exists curl; then curl -fL --retry 3 --retry-delay 1 --connect-timeout 10 --max-time 300 "$1" -o "$2"; else wget -qO "$2" "$1"; fi; }
latest_tag(){
  tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$1/releases/latest" 2>/dev/null | sed 's#.*/##; s/[[:space:][:cntrl:]]*$//')" || true
  case "$tag" in
    v[0-9]*|manual-[0-9]*) printf '%s' "$tag";;
    *) curl -fsSL "$1/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1 || true;;
  esac
}

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
# failure aborts the update. Set THREE_M_UI_INSECURE=1 to bypass verification
# when GitHub is unavailable (NOT recommended; prints a loud warning).
verify_release_sha256(){
  tag="$1"
  asset="$2"
  path="$3"
  # sha256 tooling is now a hard dependency for release verification.
  if ! command_exists sha256sum && ! command_exists shasum && ! command_exists openssl; then
    echo "Error: no sha256 tool available; cannot verify $asset. Install coreutils/sha256sum (or openssl) and re-run." >&2
    exit 1
  fi
  sums_tmp="$(mktemp)"
  if ! download "https://github.com/$REPO/releases/download/${tag}/SHA256SUMS" "$sums_tmp" 2>/dev/null; then
    rm -f "$sums_tmp"
    if [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
      echo "WARNING: THREE_M_UI_INSECURE=1 set; SHA256SUMS not published for $tag; skipping checksum verification (NOT RECOMMENDED)." >&2
      return 0
    fi
    echo "Error: SHA256SUMS not published for $tag; cannot verify $asset. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)." >&2
    exit 1
  fi
  # SHA256SUMS may list "./3m-ui-linux-amd64" or "3m-ui-linux-amd64"
  expected="$(awk -v a="$asset" '{ n=$2; sub(/^\.\//,"",n); if (n==a) { print $1; exit } }' "$sums_tmp")"
  if [ -z "$expected" ]; then
    rm -f "$sums_tmp"
    if [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
      echo "WARNING: THREE_M_UI_INSECURE=1 set; $asset not listed in SHA256SUMS; skipping verification (NOT RECOMMENDED)." >&2
      return 0
    fi
    echo "Error: $asset not listed in SHA256SUMS for $tag; cannot verify. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)." >&2
    exit 1
  fi
  actual="$(file_sha256 "$path")"
  rm -f "$sums_tmp"
  if [ "$actual" != "$expected" ]; then
    echo "Error: checksum mismatch for $asset (expected $expected, got $actual)" >&2
    exit 1
  fi
  echo "Checksum OK: $asset"
}

# Verify the SHA256SUMS file itself with cosign keyless OIDC (authenticity).
# OPT-IN: only runs when THREE_M_UI_VERIFY_COSIGN=1 AND cosign is installed.
verify_release_cosign(){
  tag="$1"; repo="$2"
  if [ "${THREE_M_UI_VERIFY_COSIGN:-0}" != "1" ]; then
    return 0
  fi
  if ! command_exists cosign; then
    echo "Warning: THREE_M_UI_VERIFY_COSIGN=1 set but cosign not installed; skipping signature verification." >&2
    return 0
  fi
  pem_tmp="$(mktemp)"; sig_tmp="$(mktemp)"; sums_tmp="$(mktemp)"
  base="https://github.com/${repo}/releases/download/${tag}"
  if ! download "$base/SHA256SUMS" "$sums_tmp" 2>/dev/null; then
    echo "Warning: SHA256SUMS not available for $tag; cannot verify cosign signature." >&2
    return 0
  fi
  if ! download "$base/SHA256SUMS.sig" "$sig_tmp" 2>/dev/null || \
     ! download "$base/SHA256SUMS.pem" "$pem_tmp" 2>/dev/null; then
    echo "Warning: cosign signature artifacts (SHA256SUMS.sig / .pem) not published for $tag." >&2
    return 0
  fi
  identity="https://github.com/${repo}/.github/workflows/release.yml@refs/tags/${tag}"
  if cosign verify-blob \
       --certificate "$pem_tmp" \
       --signature "$sig_tmp" \
       --certificate-identity "$identity" \
       --certificate-oidc-issuer https://token.actions.githubusercontent.com \
       "$sums_tmp" >/dev/null 2>&1; then
    echo "Cosign signature OK: SHA256SUMS verified against $identity"
  else
    echo "Error: cosign signature verification FAILED for $tag. Refusing to continue (set THREE_M_UI_INSECURE=1 to bypass, NOT recommended)." >&2
    rm -f "$pem_tmp" "$sig_tmp" "$sums_tmp"
    exit 1
  fi
  rm -f "$pem_tmp" "$sig_tmp" "$sums_tmp"
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
    echo "Error: no sha256 tool available; cannot verify Mihomo $asset. Install coreutils/sha256sum (or openssl) and re-run." >&2
    exit 1
  fi
  sums_tmp="$(mktemp)"
  if ! download "https://github.com/MetaCubeX/mihomo/releases/download/${tag}/SHA256SUMS" "$sums_tmp" 2>/dev/null; then
    rm -f "$sums_tmp"
    echo "Warning: Mihomo SHA256SUMS not published for $tag; cannot verify $asset." >&2
    return 1
  fi
  expected="$(awk -v a="$asset" '{ n=$2; sub(/^\.\//,"",n); if (n==a) { print $1; exit } }' "$sums_tmp")"
  rm -f "$sums_tmp"
  if [ -z "$expected" ]; then
    echo "Warning: $asset not listed in Mihomo SHA256SUMS for $tag; cannot verify checksum." >&2
    return 1
  fi
  actual="$(file_sha256 "$gzpath")"
  if [ "$actual" != "$expected" ]; then
    echo "Error: checksum mismatch for Mihomo $asset (expected $expected, got $actual)" >&2
    exit 1
  fi
  echo "Checksum OK: Mihomo $asset"
}

prune_backups(){
  dir="$DATA_DIR/backups"
  [ -d "$dir" ] || return 0
  # Keep the newest BACKUP_KEEP directories; delete the rest (POSIX-ish).
  # shellcheck disable=SC2012
  ls -1dt "$dir"/* 2>/dev/null | tail -n +$((BACKUP_KEEP + 1)) | while read -r old; do
    rm -rf "$old"
  done || true
}

stop(){ case "$(init_system)" in systemd) systemctl stop "$SERVICE_NAME" 2>/dev/null || true;; openrc) rc-service "$SERVICE_NAME" stop 2>/dev/null || true;; esac; }
start(){ case "$(init_system)" in systemd) systemctl start "$SERVICE_NAME";; openrc) rc-service "$SERVICE_NAME" start;; *) return 1;; esac; }
ok(){ case "$(init_system)" in systemd) systemctl is-active --quiet "$SERVICE_NAME";; openrc) rc-service "$SERVICE_NAME" status >/dev/null 2>&1;; *) return 1;; esac; }

backup="$DATA_DIR/backups/$(date +%Y%m%d%H%M%S)"; mkdir -p "$backup"
cp -a "$BASE" "$backup/base"
[ -d "$CONFIG_DIR" ] && cp -a "$CONFIG_DIR" "$backup/config"
[ -f "$DATA_DIR/3m-ui.db" ] && cp -p "$DATA_DIR/3m-ui.db" "$backup/3m-ui.db"
[ -x "$MIHOMO_BIN" ] && cp -p "$MIHOMO_BIN" "$backup/mihomo" || true
prune_backups

panel_tmp="$(mktemp)"; manager_tmp="$(mktemp)"; installer_tmp="$(mktemp)"; updater_tmp="$(mktemp)"; uninstall_tmp="$(mktemp)"; entry_tmp="$(mktemp)"; mihomo_tmp=""; staging="$(mktemp -d)"
cleanup(){ rm -f "$panel_tmp" "$manager_tmp" "$installer_tmp" "$updater_tmp" "$uninstall_tmp" "$entry_tmp" "$mihomo_tmp" "${mihomo_tmp:+$mihomo_tmp.gz}"; rm -rf "$staging"; }
trap cleanup EXIT INT TERM

# Official releases are pure-Go static only (no -static suffix).
current_mode="static"
if [ -s "$MODE_FILE" ]; then
  current_mode="$(cat "$MODE_FILE")"
fi
cpu="$(arch)"
asset="3m-ui-linux-${cpu}"

if [ -n "$REQUESTED_VERSION" ]; then
  tag="$REQUESTED_VERSION"
else
  tag="$(latest_tag https://github.com/$REPO)"
fi
[ -n "$tag" ] || { echo "Error: unable to determine latest 3m-ui release." >&2; exit 1; }
echo "[1/5] Downloading 3m-ui $tag ($asset)..."
if ! download "https://github.com/$REPO/releases/download/${tag}/${asset}" "$panel_tmp" 2>/dev/null; then
  # Legacy tags used a -static suffix.
  asset="3m-ui-linux-${cpu}-static"
  echo "Retrying legacy asset: $asset"
  download "https://github.com/$REPO/releases/download/${tag}/${asset}" "$panel_tmp"
fi
verify_release_sha256 "$tag" "$asset" "$panel_tmp"
# Provenance: verify the release's SHA256SUMS was signed by this repo's
# release.yml workflow (cosign keyless). OPT-IN via THREE_M_UI_VERIFY_COSIGN=1.
verify_release_cosign "$tag" "$REPO"
chmod 0755 "$panel_tmp"
# Smoke-test the binary is a Linux ELF — do NOT execute it as root (C-5).
if command_exists file; then
  file "$panel_tmp" | grep -q "ELF" || { echo "Error: downloaded 3m-ui is not a valid ELF binary." >&2; exit 1; }
fi

download_release_script(){
  file="$1"; out="$2"
  # H-1: do NOT fall back to raw.githubusercontent.com — releases MUST contain all scripts.
  if ! download "https://github.com/$REPO/releases/download/${tag}/${file}" "$out" 2>/dev/null; then
    echo "Error: release $tag does not contain $file. Refusing to fall back to raw.githubusercontent.com for supply-chain safety; use a complete release." >&2
    exit 1
  fi
  verify_release_sha256 "$tag" "$file" "$out"
  chmod 0755 "$out"
}

for spec in "3m-ui.sh:$manager_tmp" "install.sh:$installer_tmp" "update.sh:$updater_tmp" "uninstall.sh:$uninstall_tmp" "3m-ui:$entry_tmp"; do
  file="${spec%%:*}"; out="${spec#*:}"
  download_release_script "$file" "$out"
done

if [ "$UPDATE_MIHOMO" -eq 1 ] && [ -x "$MIHOMO_BIN" ]; then
  mtag="$(latest_tag https://github.com/MetaCubeX/mihomo)" || true
  if [ -n "$mtag" ]; then
    mihomo_tmp="$(mktemp)"
    case "$(uname -m)" in
      x86_64|amd64) mihomo_asset="mihomo-linux-amd64-compatible";;
      aarch64|arm64) mihomo_asset="mihomo-linux-arm64";;
      armv7l|armv7*) mihomo_asset="mihomo-linux-armv7";;
      armv6l|armv6*) mihomo_asset="mihomo-linux-armv6";;
      i386|i486|i586|i686|x86) mihomo_asset="mihomo-linux-386";;
      riscv64) mihomo_asset="mihomo-linux-riscv64";;
      loongarch64|loong64) mihomo_asset="mihomo-linux-loong64-abi2.0";;
      *) mihomo_asset="";;
    esac
    mihomo_gzname="${mihomo_asset}-${mtag}.gz"
    if [ -n "$mihomo_asset" ] && download "https://github.com/MetaCubeX/mihomo/releases/download/${mtag}/${mihomo_gzname}" "$mihomo_tmp.gz"; then
      # Verify the .gz checksum BEFORE decompressing (C-3).
      if verify_mihomo_sha256 "$mtag" "$mihomo_gzname" "$mihomo_tmp.gz"; then
        : # verified
      elif [ "${THREE_M_UI_INSECURE:-0}" = "1" ]; then
        echo "WARNING: THREE_M_UI_INSECURE=1 set; continuing Mihomo update WITHOUT checksum verification (NOT recommended)." >&2
      else
        echo "Error: Mihomo checksum verification failed. Set THREE_M_UI_INSECURE=1 to bypass (NOT recommended)." >&2
        exit 1
      fi
      if gzip -dc "$mihomo_tmp.gz" > "$mihomo_tmp"; then
        chmod 0755 "$mihomo_tmp"
        # Smoke-test: confirm ELF without executing the downloaded binary as root (C-5).
        if command_exists file; then
          file "$mihomo_tmp" | grep -q "ELF" || { echo "Warning: downloaded Mihomo is not a valid ELF binary; skipping update." >&2; mihomo_tmp=""; }
        fi
      else
        mihomo_tmp=""
      fi
    else
      mihomo_tmp=""
    fi
  fi
fi

echo "[2/5] Validating management scripts..."
sh -n "$manager_tmp" "$installer_tmp" "$updater_tmp" "$uninstall_tmp" "$entry_tmp"

echo "[3/5] Installing release-consistent files..."
stop
mkdir -p "$staging"
install -m 0755 "$panel_tmp" "$staging/3m-ui-bin"
install -m 0755 "$manager_tmp" "$staging/3m-ui.sh"
install -m 0755 "$installer_tmp" "$staging/install.sh"
install -m 0755 "$updater_tmp" "$staging/update.sh"
install -m 0755 "$uninstall_tmp" "$staging/uninstall.sh"
install -m 0755 "$entry_tmp" "$staging/3m-ui"
printf '%s\n' "$tag" > "$staging/VERSION"
printf '%s\n' "$current_mode" > "$staging/BUILD_MODE"

rm -rf "$BASE"
mkdir -p "$BASE"
cp -a "$staging/." "$BASE/"
install -m 0755 "$BASE/3m-ui" "$ENTRY"
if [ -n "$mihomo_tmp" ] && [ -s "$mihomo_tmp" ]; then install -m 0755 "$mihomo_tmp" "$MIHOMO_BIN"; fi
chmod 0600 "$VERSION_FILE" "$MODE_FILE"

if ! start || ! ok; then
  echo "[ERROR] New version failed to start; restoring the complete previous installation." >&2
  stop
  rm -rf "$BASE"
  cp -a "$backup/base" "$BASE"
  [ -f "$backup/3m-ui.db" ] && cp -p "$backup/3m-ui.db" "$DATA_DIR/3m-ui.db" || true
  [ -f "$backup/mihomo" ] && install -m 0755 "$backup/mihomo" "$MIHOMO_BIN" || true
  start || true
  exit 1
fi

echo "[4/5] Verifying service health..."
sleep 1
if ! ok; then
  echo "[ERROR] Service stopped immediately after update; restoring backup." >&2
  stop
  rm -rf "$BASE"
  cp -a "$backup/base" "$BASE"
  [ -f "$backup/3m-ui.db" ] && cp -p "$backup/3m-ui.db" "$DATA_DIR/3m-ui.db" || true
  [ -f "$backup/mihomo" ] && install -m 0755 "$backup/mihomo" "$MIHOMO_BIN" || true
  start || true
  exit 1
fi

echo "[5/5] Update completed successfully."
echo "Version: $tag"
echo "Build mode: $current_mode"
echo "Backup: $backup (keeping last $BACKUP_KEEP)"
echo "Command: 3m-ui"

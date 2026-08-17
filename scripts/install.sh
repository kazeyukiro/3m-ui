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
STATIC="${THREE_M_UI_STATIC:-0}"
REQUESTED_VERSION=""
RELEASE_TAG=""

say(){ printf '%s\n' "$*"; }
err(){ say "Error: $*" >&2; exit 1; }
command_exists(){ command -v "$1" >/dev/null 2>&1; }
need_root(){ [ "$(id -u)" -eq 0 ] || err "Please run this script as root."; }

usage(){ cat <<EOF
3m-ui installer

Usage: $0 [VERSION] [options]

Options:
  -y, --yes          Non-interactive installation
      --no-mihomo    Do not install Mihomo when it is missing
      --static       Install the static 3m-ui build
      --dynamic      Install the CGO/dynamic 3m-ui build
  -h, --help         Show this help

Environment:
  THREE_M_UI_STATIC=1  Install the static build
EOF
}

for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES=1;;
    --no-mihomo) INSTALL_MIHOMO=0;;
    --static) STATIC=1;;
    --dynamic) STATIC=0;;
    -h|--help) usage; exit 0;;
    v[0-9]*|manual-[0-9]*) [ -z "$REQUESTED_VERSION" ] || err "Only one version may be specified."; REQUESTED_VERSION="$arg";;
    *) err "Unknown option: $arg";;
  esac
done

os_id(){
  if [ -r /etc/os-release ]; then . /etc/os-release; printf '%s' "${ID:-unknown}"; else printf '%s' unknown; fi
}
arch(){ case "$(uname -m)" in x86_64|amd64) echo amd64;; aarch64|arm64) echo arm64;; armv7l|armv7*) echo armv7;; *) err "Unsupported CPU architecture: $(uname -m)";; esac; }
init_system(){ if [ -d /run/systemd/system ] && command_exists systemctl; then echo systemd; elif command_exists rc-service; then echo openrc; else echo unsupported; fi; }

install_deps(){
  missing=""
  for x in curl ca-certificates gzip tar sed install; do command_exists "$x" || missing="$missing $x"; done
  [ -z "$missing" ] && return
  case "$(os_id)" in
    alpine) apk add --no-cache curl ca-certificates gzip tar sed;;
    debian|ubuntu|linuxmint|raspbian) apt-get update && apt-get install -y curl ca-certificates gzip tar sed;;
    fedora|rhel|centos|rocky|almalinux|oracle) if command_exists dnf; then dnf install -y curl ca-certificates gzip tar sed; else yum install -y curl ca-certificates gzip tar sed; fi;;
    arch|manjaro) pacman -Sy --noconfirm curl ca-certificates gzip tar sed;;
    opensuse*|sles) zypper --non-interactive install curl ca-certificates gzip tar sed;;
    *) err "Cannot install dependencies automatically on $(os_id):$missing";;
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

write_config(){
  mkdir -p "$CONFIG_DIR" "$DATA_DIR/mihomo" "$LOG_DIR"
  [ -f "$CONFIG_DIR/config.yaml" ] && { chmod 0600 "$CONFIG_DIR/config.yaml"; return; }
  cat > "$CONFIG_DIR/config.yaml" <<EOF
server:
  port: 8080
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

mihomo_asset(){ case "$(uname -m)" in x86_64|amd64) echo mihomo-linux-amd64-compatible;; aarch64|arm64) echo mihomo-linux-arm64;; armv7l|armv7*) echo mihomo-linux-armv7;; *) err "Unsupported CPU architecture for Mihomo";; esac; }

install_mihomo(){
  [ "$INSTALL_MIHOMO" -eq 1 ] || { say "Mihomo installation skipped."; return; }
  if [ -x "$MIHOMO_BIN" ] && "$MIHOMO_BIN" -v >/dev/null 2>&1; then say "Existing Mihomo detected: $MIHOMO_BIN"; return; fi
  tag="$(latest_tag https://github.com/MetaCubeX/mihomo)"; [ -n "$tag" ] || err "Unable to determine latest Mihomo release."
  tmp="$(mktemp)"; trap 'rm -f "$tmp" "$tmp.gz"' EXIT INT TERM
  url="https://github.com/MetaCubeX/mihomo/releases/download/${tag}/$(mihomo_asset)-${tag}.gz"
  say "Downloading Mihomo $tag..."; download "$url" "$tmp.gz"; gzip -dc "$tmp.gz" > "$tmp"; chmod 0755 "$tmp"
  "$tmp" -v >/dev/null 2>&1 || err "Downloaded Mihomo failed validation."
  install -m 0755 "$tmp" "$MIHOMO_BIN"
  rm -f "$tmp" "$tmp.gz"; trap - EXIT INT TERM
}

install_release_asset(){
  tag="$1"; file="$2"; destination="$3"
  tmp="$(mktemp)"
  if download "https://github.com/$REPO/releases/download/${tag}/${file}" "$tmp" 2>/dev/null; then
    :
  else
    say "Release $tag does not contain $file; using the matching main-branch management script."
    rm -f "$tmp"
    download "https://raw.githubusercontent.com/$REPO/main/scripts/$file" "$tmp"
  fi
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
  suffix=""; [ "$STATIC" = "1" ] && suffix="-static"
  asset="3m-ui-linux-$(arch)${suffix}"
  tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT INT TERM
  url="https://github.com/$REPO/releases/download/${RELEASE_TAG}/${asset}"
  say "Downloading 3m-ui $RELEASE_TAG ($asset)..."; download "$url" "$tmp"; chmod 0755 "$tmp"
  "$tmp" --version >/dev/null 2>&1 || err "Downloaded 3m-ui failed executable validation."
  install -m 0755 "$tmp" "$APP_BIN"
  printf '%s\n' "$RELEASE_TAG" > "$VERSION_FILE"
  printf '%s\n' "$([ "$STATIC" = "1" ] && echo static || echo dynamic)" > "$MODE_FILE"
  chmod 0600 "$VERSION_FILE" "$MODE_FILE"
  rm -f "$tmp"; trap - EXIT INT TERM
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
  say "Panel: http://SERVER_IP:8080/"
  say "Default administrator credentials are unchanged; first login requires a password change."
}
main "$@"

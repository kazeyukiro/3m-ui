#!/usr/bin/env sh
set -eu
umask 077

BASE="/usr/local/lib/3m-ui"
APP_BIN="$BASE/3m-ui-bin"
ENTRY="/usr/local/bin/3m-ui"
CONFIG_DIR="/etc/3m-ui"
DATA_DIR="/var/lib/3m-ui"
LOG_DIR="/var/log/3m-ui"
MIHOMO_BIN="/usr/local/bin/mihomo"
SERVICE_NAME="3m-ui"
PURGE=0
YES=0

for arg in "$@"; do
  case "$arg" in
    -y|--yes) YES=1;;
    --purge) PURGE=1;;
    -h|--help) printf '%s\n' 'Usage: uninstall.sh [--yes] [--purge]'; exit 0;;
    *) echo "Unknown option: $arg" >&2; exit 2;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "Error: please run as root." >&2; exit 1; }
init_system(){ if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then echo systemd; elif command -v rc-service >/dev/null 2>&1; then echo openrc; else echo unsupported; fi; }

if [ "$YES" -ne 1 ]; then
  [ -t 0 ] || { echo "Non-interactive uninstall requires --yes." >&2; exit 1; }
  echo "3m-ui uninstall"
  echo "  Command: $ENTRY"
  echo "  Application: $APP_BIN"
  echo "  Config: $CONFIG_DIR"
  if [ "$PURGE" -eq 1 ]; then
    echo "  Data: $DATA_DIR [WILL BE DELETED — irreversible]"
  else
    echo "  Data: $DATA_DIR [KEPT]"
  fi
  printf 'Continue? [y/N] '; read -r answer
  case "$answer" in y|Y|yes|YES) ;; *) echo "Aborted."; exit 0;; esac
  if [ "$PURGE" -eq 1 ]; then
    printf 'Type PURGE to confirm deleting all application data: '
    read -r confirm
    [ "$confirm" = "PURGE" ] || { echo "Aborted (confirmation mismatch)."; exit 0; }
  fi
fi

case "$(init_system)" in
  systemd)
    systemctl disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
    rm -f "/etc/systemd/system/$SERVICE_NAME.service"
    systemctl daemon-reload >/dev/null 2>&1 || true
    ;;
  openrc)
    rc-service "$SERVICE_NAME" stop >/dev/null 2>&1 || true
    rc-update del "$SERVICE_NAME" default >/dev/null 2>&1 || true
    rm -f "/etc/init.d/$SERVICE_NAME"
    ;;
esac

rm -f "$ENTRY"
rm -rf "$BASE" "$CONFIG_DIR" "$LOG_DIR"

if [ "$PURGE" -eq 1 ]; then
  rm -rf "$DATA_DIR"
  echo "3m-ui uninstalled and application data purged."
else
  echo "3m-ui uninstalled. Persistent data kept at: $DATA_DIR"
fi

echo "Mihomo was left untouched: $MIHOMO_BIN"
echo "Done."

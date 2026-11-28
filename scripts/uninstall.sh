#!/usr/bin/env sh
set -eu
umask 077

ROOT="${THREE_M_UI_ROOT:-}"
BASE="$ROOT/usr/local/lib/3m-ui"
APP_BIN="$BASE/3m-ui-bin"
ENTRY="$ROOT/usr/local/bin/3m-ui"
CONFIG_DIR="$ROOT/etc/3m-ui"
DATA_DIR="${THREE_M_UI_DATA_DIR:-}"
if [ -z "$DATA_DIR" ] && [ -s "$BASE/DATA_DIRECTORY" ]; then DATA_DIR="$(cat "$BASE/DATA_DIRECTORY")"; fi
DATA_DIR="${DATA_DIR:-$ROOT/var/lib/3m-ui}"
LOG_DIR="$ROOT/var/log/3m-ui"
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

for path in "$BASE" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"; do
  case "$path" in /|/etc|/usr|/var|*/../*|*/..|*[[:space:]]*) echo "Unsafe installation path: $path" >&2; exit 1;; esac
done
[ "$(id -u)" -eq 0 ] || { echo "Error: please run as root." >&2; exit 1; }
init_system(){ if [ -d "$ROOT/run/systemd/system" ] && command -v systemctl >/dev/null 2>&1; then echo systemd; elif command -v rc-service >/dev/null 2>&1; then echo openrc; else echo unsupported; fi; }

if [ "$YES" -ne 1 ]; then
  [ -t 0 ] || { echo "Non-interactive uninstall requires --yes." >&2; exit 1; }
  echo "3m-ui uninstall"
  echo "  Command: $ENTRY"
  echo "  Application: $APP_BIN"
  echo "  Config: $CONFIG_DIR"
  if [ "$PURGE" -eq 1 ]; then
    echo "  Config and data: $CONFIG_DIR, $DATA_DIR [WILL BE DELETED — irreversible]"
  else
    echo "  Config and data: $CONFIG_DIR, $DATA_DIR [KEPT]"
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
    if [ -f "$ROOT/etc/systemd/system/$SERVICE_NAME.service" ]; then systemctl stop "$SERVICE_NAME"; fi
    systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true
    rm -f "$ROOT/etc/systemd/system/$SERVICE_NAME.service"
    systemctl daemon-reload >/dev/null 2>&1 || true
    ;;
  openrc)
    if [ -f "$ROOT/etc/init.d/$SERVICE_NAME" ]; then rc-service "$SERVICE_NAME" stop; fi
    rc-update del "$SERVICE_NAME" default >/dev/null 2>&1 || true
    rm -f "$ROOT/etc/init.d/$SERVICE_NAME"
    ;;
esac

rm -f "$ENTRY"
rm -rf "$BASE" "$LOG_DIR"

if [ "$PURGE" -eq 1 ]; then
  rm -rf "$DATA_DIR" "$CONFIG_DIR"
  echo "3m-ui uninstalled and application data purged."
else
  echo "3m-ui uninstalled. Data kept at: $DATA_DIR; configuration, keys and certificates kept at: $CONFIG_DIR"
fi

echo "The managed Mihomo binary was removed with 3m-ui; external cores were left untouched."
echo "Done."

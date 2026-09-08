#!/bin/sh
# Download exactly the reviewed core. Never resolve a moving upstream latest.
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../distribution/mihomo.env"
ARCH=${1:?Usage: download-mihomo.sh amd64|arm64 OUTPUT_DIRECTORY}
DEST=${2:?Output directory is required}
case "$ARCH" in
  amd64) ASSET=$MIHOMO_AMD64_ASSET; SHA256=$MIHOMO_AMD64_SHA256 ;;
  arm64) ASSET=$MIHOMO_ARM64_ASSET; SHA256=$MIHOMO_ARM64_SHA256 ;;
  *) echo "No reviewed Mihomo bundle for architecture: $ARCH" >&2; exit 1 ;;
esac
mkdir -p "$DEST"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
curl --fail --location --silent --show-error --retry 3 --connect-timeout 15 --max-time 180 \
  "https://github.com/MetaCubeX/mihomo/releases/download/$MIHOMO_VERSION/$ASSET" \
  --output "$TMP/$ASSET"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMP/$ASSET" | cut -d ' ' -f 1)
else
  ACTUAL=$(shasum -a 256 "$TMP/$ASSET" | cut -d ' ' -f 1)
fi
[ "$ACTUAL" = "$SHA256" ] || { echo "Mihomo checksum mismatch: $ASSET" >&2; exit 1; }
gzip -dc "$TMP/$ASSET" > "$DEST/mihomo"
chmod 0755 "$DEST/mihomo"
printf '%s\n' "$MIHOMO_VERSION" > "$DEST/MIHOMO_VERSION"

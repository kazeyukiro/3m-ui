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

# Hash the bytes as they arrive rather than re-reading the asset afterwards.
# Verification still completes before gzip is allowed anywhere near the file —
# a pipe never skips that step, it only avoids a second pass over the storage.
hash_stream(){
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | cut -d ' ' -f 1
  else
    shasum -a 256 | cut -d ' ' -f 1
  fi
}

mkdir -p "$DEST"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
ACTUAL=$(curl --fail --location --silent --show-error --retry 3 --connect-timeout 15 --max-time 180 \
  "https://github.com/MetaCubeX/mihomo/releases/download/$MIHOMO_VERSION/$ASSET" |
  tee "$TMP/$ASSET" | hash_stream)
[ "$ACTUAL" = "$SHA256" ] || { echo "Mihomo checksum mismatch: $ASSET" >&2; exit 1; }
gzip -dc "$TMP/$ASSET" > "$DEST/mihomo"
chmod 0755 "$DEST/mihomo"
printf '%s\n' "$MIHOMO_VERSION" > "$DEST/MIHOMO_VERSION"

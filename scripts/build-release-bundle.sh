#!/bin/sh
# Package an already-built static panel with its reviewed core and lifecycle scripts.
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
ARCH=${1:?Usage: build-release-bundle.sh amd64|arm64 PANEL_BINARY OUTPUT_DIRECTORY}
PANEL=${2:?Panel binary is required}
DEST=${3:?Output directory is required}
: "${VERSION:?Set VERSION to the release version}"
: "${REPOSITORY:?Set REPOSITORY to the owner/repository that publishes this release}"
case "$ARCH" in amd64|arm64) ;; *) echo "Unsupported bundle architecture: $ARCH" >&2; exit 1 ;; esac
mkdir -p "$DEST"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
sh "$SCRIPT_DIR/download-mihomo.sh" "$ARCH" "$TMP"
cp "$PANEL" "$TMP/3m-ui-bin"
chmod 0755 "$TMP/3m-ui-bin"
for SCRIPT in install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui; do
  cp "$SCRIPT_DIR/$SCRIPT" "$TMP/$SCRIPT"
  chmod 0755 "$TMP/$SCRIPT"
done
printf '%s\n' "$VERSION" > "$TMP/VERSION"
printf '%s\n' "$REPOSITORY" > "$TMP/REPOSITORY"
cp "$ROOT/distribution/mihomo.env" "$TMP/mihomo.env"
cp "$ROOT/distribution/MIHOMO_LICENSE" "$TMP/MIHOMO_LICENSE"
cp "$ROOT/LICENSE" "$TMP/LICENSE"
printf 'Mihomo source and build scripts: https://github.com/MetaCubeX/mihomo/tree/%s\n' \
  "$(cat "$TMP/MIHOMO_VERSION")" > "$TMP/MIHOMO_SOURCE"
COPYFILE_DISABLE=1 tar -czf "$DEST/3m-ui-bundle-linux-$ARCH.tar.gz" -C "$TMP" \
  3m-ui-bin mihomo MIHOMO_VERSION VERSION REPOSITORY mihomo.env LICENSE MIHOMO_LICENSE MIHOMO_SOURCE \
  install.sh update.sh uninstall.sh 3m-ui.sh 3m-ui

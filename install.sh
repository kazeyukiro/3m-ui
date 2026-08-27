#!/bin/sh
# Thin entry (s-ui style): bash <(curl -fsSL .../install.sh) -s -- [VERSION] [-y]
# Delegates to scripts/install.sh on the same branch/ref when possible,
# otherwise downloads the latest scripts/install.sh from main.

set -eu

REPO="kazeyukiro/3m-ui"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/main"

# If this file lives in a full git checkout, prefer local scripts/install.sh.
self_dir=$(CDPATH= cd -- "$(dirname -- "$0")" 2>/dev/null && pwd || true)
if [ -n "$self_dir" ] && [ -f "$self_dir/scripts/install.sh" ]; then
  exec sh "$self_dir/scripts/install.sh" "$@"
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT INT TERM
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "${RAW_BASE}/scripts/install.sh" -o "$tmp"
else
  wget -qO "$tmp" "${RAW_BASE}/scripts/install.sh"
fi
chmod +x "$tmp"
exec sh "$tmp" "$@"

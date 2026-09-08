#!/usr/bin/env sh
set -eu
# Installation and updates share the same verified, transactional path.
BASE="${THREE_M_UI_ROOT:-}/usr/local/lib/3m-ui"
[ -x "$BASE/install.sh" ] || { echo "Installer missing: $BASE/install.sh" >&2; exit 1; }
exec "$BASE/install.sh" --update "$@"

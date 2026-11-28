#!/usr/bin/env sh
# 验证 Sub-Store goja 集成在所有发布架构上可交叉编译。
# goja 是纯 Go 实现，bundle 是平台无关的 JS，因此理论上全架构可用；
# 此脚本用真实编译来证明这一点。
set -eu

BACKEND_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../../backend" && pwd)"
cd "$BACKEND_DIR"

export CGO_ENABLED=0
export GOTOOLCHAIN=auto
export GOFLAGS="-tags=sqlite_modernc"

# GOARCH:GOARM:label
ARCHES="amd64::amd64 arm64::arm64 arm:7:armv7 arm:6:armv6 386::386 riscv64::riscv64 loong64::loong64 ppc64le::ppc64le s390x::s390x"

OUT="${TMPDIR:-/tmp}/3m-ui-crosscheck"
mkdir -p "$OUT"

for spec in $ARCHES; do
  GOARCH=$(echo "$spec" | cut -d: -f1)
  GOARM=$(echo "$spec" | cut -d: -f2)
  LABEL=$(echo "$spec" | cut -d: -f3)
  printf '%-10s ' "$LABEL"
  if [ -n "$GOARM" ]; then
    GOOS=linux GOARCH="$GOARCH" GOARM="$GOARM" go build -tags sqlite_modernc -o "$OUT/3m-ui-$LABEL" ./cmd/server >/dev/null 2>&1
  else
    GOOS=linux GOARCH="$GOARCH" go build -tags sqlite_modernc -o "$OUT/3m-ui-$LABEL" ./cmd/server >/dev/null 2>&1
  fi
  if [ $? -eq 0 ]; then
    printf 'OK  (%s)\n' "$(du -h "$OUT/3m-ui-$LABEL" | cut -f1)"
  else
    printf 'FAIL\n'
    exit 1
  fi
done

echo "All architectures cross-compiled successfully."

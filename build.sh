#!/usr/bin/env bash
# ============================================================================
# build.sh -- cross-compile the VOM single-file binaries into dist/.
#
#   ./build.sh [--no-test] [--version VERSION]
#
# The app is pure Go (no cgo), so every binary is a static, dependency-free
# executable.  Build output is never committed; it is a release asset.
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

DO_TEST=1
VERSION="${VOM_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

while [ $# -gt 0 ]; do
  case "$1" in
    --no-test) DO_TEST=0; shift ;;
    --version) VERSION="${2:?--version needs a value}"; shift 2 ;;
    -h|--help) sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "error: unknown option: $1" >&2; exit 1 ;;
  esac
done

mkdir -p dist
rm -f dist/vom-* dist/SHA256SUMS

if [ "$DO_TEST" = 1 ]; then
  go test ./...
  go vet ./...
fi

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "==> building $name ($goos/$goarch)"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -tags netgo -ldflags "-s -w -X main.version=$VERSION" \
    -o "dist/$name" .
}

build linux amd64 vom-linux-amd64
build darwin amd64 vom-darwin-amd64
build darwin arm64 vom-darwin-arm64
build windows amd64 vom-windows-amd64.exe

( cd dist && sha256sum vom-linux-amd64 vom-darwin-amd64 vom-darwin-arm64 vom-windows-amd64.exe > SHA256SUMS )

echo
echo "version: $VERSION"
ls -lh dist
echo
cat dist/SHA256SUMS

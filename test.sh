#!/usr/bin/env bash
# Unit tests, vet and a native build.  The end-to-end emulator test lives in
# scripts/integration.sh because it downloads the official emulator and image.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
echo "==> go test"
go test ./...
echo "==> go vet"
go vet ./...
echo "==> go build"
CGO_ENABLED=0 go build -trimpath -o /tmp/vom-test .
rm -f /tmp/vom-test
echo "PASS"

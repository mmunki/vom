#!/usr/bin/env bash
# End-to-end test: install the official emulator, download Android 7.0
# (API 24, AOSP, x86_64), boot it headless and verify the version.
#
# Downloads about 800 MiB.  Linux needs KVM for a reasonable runtime; the
# script enables /dev/kvm for the current user when it can.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ "$(uname -s)" != "Linux" ]; then
  echo "integration test: only Linux is automated here" >&2
  exit 0
fi
if [ -e /dev/kvm ] && [ ! -r /dev/kvm ]; then
  sudo chmod 666 /dev/kvm 2>/dev/null || true
fi

HOME_DIR="${VOM_TEST_HOME:-$(mktemp -d -t vom-integration-XXXXXX)}"
BIN="${VOM_TEST_BIN:-./dist/vom-linux-amd64}"
mkdir -p "$HOME_DIR"
echo "test home: $HOME_DIR"
echo "binary:    $BIN"

"$BIN" doctor --home "$HOME_DIR"
"$BIN" setup --home "$HOME_DIR"
"$BIN" versions --home "$HOME_DIR" | sed -n '1,12p'
"$BIN" install 24 --tag default --abi x86_64 --refresh --home "$HOME_DIR"
"$BIN" run 24 --name "VOM-CI-Android-7" --headless --wait --timeout 10m --home "$HOME_DIR"

ADB="$HOME_DIR/sdk/platform-tools/adb"
echo "==> booted properties"
"$ADB" devices
"$ADB" -s "$("$ADB" devices | awk '/^emulator-/{print $1; exit}')" shell getprop ro.build.version.release
"$ADB" -s "$("$ADB" devices | awk '/^emulator-/{print $1; exit}')" shell getprop ro.build.version.sdk
"$ADB" -s "$("$ADB" devices | awk '/^emulator-/{print $1; exit}')" shell getprop ro.product.cpu.abi
"$BIN" stop "VOM-CI-Android-7" --home "$HOME_DIR"
echo "INTEGRATION PASS"

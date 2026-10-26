#!/usr/bin/env bash
# ============================================================================
# android/build.sh -- build the VOM Mobile APK without Gradle.
# ----------------------------------------------------------------------------
# Requires JDK 17+ and an Android SDK with a platform android.jar, aapt2,
# d8, zipalign and apksigner.  Set ANDROID_HOME / ANDROID_SDK_ROOT, or let the
# script use the standard full SDK location.
#
#   ./android/build.sh
#   ANDROID_HOME=/opt/android-sdk ./android/build.sh
#
# Output: android/dist/vom-android.apk
#
# The signing key is generated once per build host under .keys/ and kept out
# of git.  Set VOM_ANDROID_KEYSTORE, VOM_ANDROID_KS_PASS and
# VOM_ANDROID_KEY_PASS to use a stable release key instead.
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

SDK="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
if [ -z "$SDK" ] && [ -d /usr/local/lib/android/sdk ]; then
  SDK=/usr/local/lib/android/sdk
fi
[ -n "$SDK" ] && [ -d "$SDK" ] || {
  echo "error: set ANDROID_HOME to an Android SDK" >&2
  exit 1
}

pick() { # pick <directory> <name>
  local dir="$1" name="$2" latest=""
  [ -d "$dir" ] || return 1
  latest="$(find "$dir" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -V | tail -1)"
  [ -n "$latest" ] || return 1
  printf '%s/%s' "$dir/$latest" "$name"
}

PLATFORM="${VOM_ANDROID_PLATFORM:-}"
if [ -z "$PLATFORM" ]; then
  PLATFORM="$(find "$SDK/platforms" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null \
    | sed '/-\(alpha\|beta\|rc\|canary\|preview\|dev\)[0-9]*$/d' | sort -V | tail -1)"
fi
[ -n "$PLATFORM" ] || { echo "error: no Android platform installed" >&2; exit 1; }
ANDROID_JAR="$SDK/platforms/$PLATFORM/android.jar"
[ -f "$ANDROID_JAR" ] || { echo "error: $ANDROID_JAR not found" >&2; exit 1; }

BT="${VOM_ANDROID_BUILD_TOOLS:-}"
if [ -z "$BT" ]; then
  BT="$(find "$SDK/build-tools" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null | sort -V | tail -1)"
fi
[ -n "$BT" ] || { echo "error: no build-tools installed" >&2; exit 1; }
BT_DIR="$SDK/build-tools/$BT"
AAPT2="$BT_DIR/aapt2"
ZIPALIGN="$BT_DIR/zipalign"
D8_JAR="$BT_DIR/lib/d8.jar"
APKSIGNER_JAR="$BT_DIR/lib/apksigner.jar"
for f in "$AAPT2" "$ZIPALIGN" "$D8_JAR" "$APKSIGNER_JAR"; do
  [ -f "$f" ] || { echo "error: $f not found" >&2; exit 1; }
done

command -v javac >/dev/null || { echo "error: javac not found (need JDK 17+)" >&2; exit 1; }
command -v keytool >/dev/null || { echo "error: keytool not found" >&2; exit 1; }
command -v zip >/dev/null || { echo "error: zip not found" >&2; exit 1; }

MIN_API="${VOM_ANDROID_MIN_API:-24}"
TARGET_API="${VOM_ANDROID_TARGET_API:-${PLATFORM#android-}}"
TARGET_API="${TARGET_API%%.*}"
VERSION_CODE="${VOM_ANDROID_VERSION_CODE:-200}"
VERSION_NAME="${VOM_ANDROID_VERSION_NAME:-0.2.0}"
OUT="$ROOT/dist/vom-android.apk"
WORK="$ROOT/.build"
rm -rf "$WORK"
mkdir -p "$WORK/compiled" "$WORK/gen" "$WORK/classes" "$WORK/dex" "$ROOT/dist"

echo "==> aapt2 compile (platform $PLATFORM, build-tools $BT)"
"$AAPT2" compile --dir res -o "$WORK/compiled/resources.zip"

echo "==> aapt2 link"
"$AAPT2" link \
  -o "$WORK/base.apk" \
  -I "$ANDROID_JAR" \
  --manifest AndroidManifest.xml \
  --java "$WORK/gen" \
  --min-sdk-version "$MIN_API" \
  --target-sdk-version "$TARGET_API" \
  --version-code "$VERSION_CODE" \
  --version-name "$VERSION_NAME" \
  "$WORK/compiled/resources.zip"

echo "==> javac"
mapfile -t SOURCES < <(find src "$WORK/gen" -name '*.java' -type f | sort)
javac -encoding UTF-8 -source 8 -target 8 -Xlint:-options \
  -bootclasspath "$ANDROID_JAR" -d "$WORK/classes" "${SOURCES[@]}"

echo "==> d8"
mapfile -t CLASSES < <(find "$WORK/classes" -name '*.class' -type f | sort)
java -cp "$D8_JAR" com.android.tools.r8.D8 \
  --min-api "$MIN_API" --lib "$ANDROID_JAR" --output "$WORK/dex" "${CLASSES[@]}"
( cd "$WORK/dex" && zip -q "$WORK/base.apk" classes.dex )

echo "==> zipalign"
"$ZIPALIGN" -p -f 4 "$WORK/base.apk" "$WORK/aligned.apk"

KEYSTORE="${VOM_ANDROID_KEYSTORE:-$ROOT/.keys/vom-debug.keystore}"
KS_PASS="${VOM_ANDROID_KS_PASS:-android}"
KEY_PASS="${VOM_ANDROID_KEY_PASS:-android}"
if [ ! -f "$KEYSTORE" ]; then
  mkdir -p "$(dirname "$KEYSTORE")"
  echo "==> generating signing key $KEYSTORE"
  keytool -genkeypair -keystore "$KEYSTORE" -storepass "$KS_PASS" -keypass "$KEY_PASS" \
    -alias vom -keyalg RSA -keysize 2048 -validity 10000 \
    -dname "CN=VOM Mobile,O=VOM,C=US" -noprompt >/dev/null
fi

echo "==> apksigner"
java -jar "$APKSIGNER_JAR" sign \
  --ks "$KEYSTORE" --ks-pass "pass:$KS_PASS" --key-pass "pass:$KEY_PASS" \
  --ks-key-alias vom --out "$OUT" "$WORK/aligned.apk"
java -jar "$APKSIGNER_JAR" verify --verbose "$OUT" >&2

"$AAPT2" dump badging "$OUT" | sed -n '1,5p'
echo
echo "APK: $OUT ($(wc -c <"$OUT") bytes)"
echo "SHA-256: $(sha256sum "$OUT" | cut -d' ' -f1)"

VOM 0.2.0 - Android 7.0 to 17 on the host, live on your phone.

- Static single-file host binaries for Linux x86_64, macOS x86_64/arm64 and
  Windows x86_64.
- Catalog of Android 7.0 (API 24) through Android 17 (API 37): AOSP, Google
  APIs and Google Play images, x86_64 / arm64-v8a / x86.
- Resumable downloads with size and SHA-1 verification against Google's
  repository index, traversal-safe zip extraction.
- Creates standard AVDs (config.ini + .ini) and launches the official emulator,
  windowed or headless. Live log, stop, wipe and delete.
- Material 3 web UI, light/dark theme, installable PWA, phone-browser control.
- **New: VOM Mobile (`vom-android.apk`)** - signed Android client with live
  screen streaming, tap/swipe, Back/Home/Recents/volume/power and text input.
- **New API** - `GET /api/avd/screen` returns a PNG plus the real screen size;
  `POST /api/avd/input` sends tap, swipe, key and text.
- Each AVD gets its own emulator console/adb port (5554, 5556, ...), so
  several devices can run at once, and the client shows the correct serial.

The APK is a remote control, not a nested hypervisor: a third-party Android app
cannot boot another Android image because KVM/AVF and the emulator's virtual
hardware are not exposed to apps. The virtual device still runs on the VOM host
where hardware virtualization is available.

### Test matrix

| Component | Build | Unit tests | End-to-end |
| --- | --- | --- | --- |
| Linux x86_64 host (KVM) | PASS | PASS | PASS - boots Android 7.0 API 24 x86_64 AOSP, `7.0` / `24` / `x86_64` read over adb |
| macOS x86_64 / arm64 host | cross-compiled | PASS | not run in CI (needs a Mac) |
| Windows x86_64 host | cross-compiled | PASS | not run in CI (needs Windows) |
| VOM Mobile APK | PASS - minSdk 24, targetSdk 37, v2/v3 signed | n/a | PASS - client on Android 7.0 x86_64, target on Android 7.0 x86_64: live screen, tap, text, key input |
| APK ABI coverage | one universal APK, no native libraries | n/a | x86_64 emulator verified; Java-only code has no ABI-specific build |

`./test.sh` runs unit tests, vet and a native host build.
`scripts/integration.sh` downloads the stable Google emulator, platform-tools
and the Android 7.0 AOSP x86_64 image, boots it headless and reads
`ro.build.version.release`, `ro.build.version.sdk` and `ro.product.cpu.abi`
through adb. `android/build.sh` builds and signs the APK without Gradle.

### Requirement

The host emulator needs hardware virtualization: KVM on Linux,
Hypervisor.framework on macOS, Windows Hypervisor Platform on Windows. A host
without it still downloads and manages images but boots very slowly.

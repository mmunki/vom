VOM - Android 7.0 to 17 in a Material 3 manager.

- Static single-file binaries for Linux x86_64, macOS x86_64/arm64 and Windows x86_64.
- Catalog of Android 7.0 (API 24) through Android 17 (API 37): AOSP, Google
  APIs and Google Play images, x86_64 / arm64-v8a / x86.
- Resumable downloads with size and SHA-1 verification against Google's
  repository index, traversal-safe zip extraction.
- Creates standard AVDs (config.ini + .ini) and launches the official emulator,
  windowed or headless. Live log, stop, wipe and delete.
- Material 3 web UI, light/dark theme, installable PWA, phone-browser control.

### Test matrix

| Host | Build | Unit tests | End-to-end boot |
| --- | --- | --- | --- |
| Linux x86_64 (KVM) | PASS | PASS | PASS - Android 7.0 API 24, x86_64 AOSP: booted, `7.0` / `24` / `x86_64` verified over adb |
| macOS x86_64 / arm64 | cross-compiled | PASS | not run in CI (needs a Mac) |
| Windows x86_64 | cross-compiled | PASS | not run in CI (needs Windows) |

`./test.sh` runs unit tests, vet and a native build. `scripts/integration.sh`
downloads the stable Google emulator, platform-tools and the Android 7.0 AOSP
x86_64 image, boots it headless and reads `ro.build.version.release`,
`ro.build.version.sdk` and `ro.product.cpu.abi` through adb.

### Requirement

The emulator needs hardware virtualization: KVM on Linux, Hypervisor.framework
on macOS, Windows Hypervisor Platform on Windows. A host without it still
downloads and manages images but boots very slowly.

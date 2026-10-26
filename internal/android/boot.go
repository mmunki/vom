package android

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mmunki/vom/internal/store"
)

func adbDevices(ctx context.Context, p *store.Paths) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.ADBBinary(), "devices")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	devices := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "emulator-") {
			devices[fields[0]] = fields[1]
		}
	}
	return devices, nil
}

func adbGetProp(ctx context.Context, p *store.Paths, serial, prop string) string {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p.ADBBinary(), "-s", serial, "shell", "getprop", prop).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// WaitForBoot waits until a newly started emulator reports sys.boot_completed=1.
func WaitForBoot(ctx context.Context, p *store.Paths, timeout time.Duration) (string, error) {
	if !p.HasPlatformTools() {
		return "", fmt.Errorf("platform-tools (adb) is not installed")
	}
	if timeout <= 0 {
		timeout = 6 * time.Minute
	}
	before, _ := adbDevices(ctx, p)
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if time.Since(start) > timeout {
			return "", fmt.Errorf("emulator did not finish booting within %s", timeout)
		}
		devices, err := adbDevices(ctx, p)
		if err == nil {
			for serial, state := range devices {
				if _, existed := before[serial]; existed || state != "device" {
					continue
				}
				if adbGetProp(ctx, p, serial, "sys.boot_completed") == "1" {
					return serial, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

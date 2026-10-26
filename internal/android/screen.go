package android

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Screen is one screenshot of a running AVD.
type Screen struct {
	PNG    []byte
	Width  int // width of the real device screen
	Height int // height of the real device screen
}

// Screenshot grabs the current framebuffer of an AVD through adb and scales
// it down to width when width is smaller than the device screen.  The
// returned Width/Height are always the real device dimensions, so a client can
// map touches back to the device after showing a scaled image.
func (m *Manager) Screenshot(ctx context.Context, name string, width int) (Screen, error) {
	avd, ok := m.Get(name)
	if !ok {
		return Screen{}, fmt.Errorf("unknown AVD %q", name)
	}
	raw, err := m.adb(ctx, avd.Serial, "exec-out", "screencap", "-p")
	if err != nil {
		return Screen{}, err
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return Screen{}, fmt.Errorf("decode screenshot: %w", err)
	}
	bounds := img.Bounds()
	screen := Screen{Width: bounds.Dx(), Height: bounds.Dy()}
	if width > 0 && width < bounds.Dx() {
		height := bounds.Dy() * width / bounds.Dx()
		if height < 1 {
			height = 1
		}
		img = scaleNearest(img, width, height)
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return Screen{}, fmt.Errorf("encode screenshot: %w", err)
	}
	screen.PNG = buf.Bytes()
	return screen, nil
}

// Tap sends a touch at pixel coordinates of the real device screen.
func (m *Manager) Tap(ctx context.Context, name string, x, y int) error {
	if x < 0 || y < 0 {
		return fmt.Errorf("tap coordinates must be positive")
	}
	_, err := m.adb(ctx, m.serial(name), "shell", "input", "tap", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

// Swipe sends a swipe on the real device screen.
func (m *Manager) Swipe(ctx context.Context, name string, x1, y1, x2, y2, durationMS int) error {
	if x1 < 0 || y1 < 0 || x2 < 0 || y2 < 0 {
		return fmt.Errorf("swipe coordinates must be positive")
	}
	if durationMS < 50 {
		durationMS = 50
	}
	if durationMS > 3000 {
		durationMS = 3000
	}
	_, err := m.adb(ctx, m.serial(name), "shell", "input", "swipe",
		strconv.Itoa(x1), strconv.Itoa(y1), strconv.Itoa(x2), strconv.Itoa(y2),
		strconv.Itoa(durationMS))
	return err
}

// AllowedKeyCodes is the small set of device keys the remote UI may send.
var AllowedKeyCodes = map[int]bool{
	3:   true, // HOME
	4:   true, // BACK
	24:  true, // VOLUME_UP
	25:  true, // VOLUME_DOWN
	26:  true, // POWER
	82:  true, // MENU
	187: true, // APP_SWITCH
	224: true, // WAKEUP
	223: true, // SLEEP
}

// Key sends an Android key code.
func (m *Manager) Key(ctx context.Context, name string, keyCode int) error {
	if !AllowedKeyCodes[keyCode] {
		return fmt.Errorf("key code %d is not allowed", keyCode)
	}
	_, err := m.adb(ctx, m.serial(name), "shell", "input", "keyevent", strconv.Itoa(keyCode))
	return err
}

var safeText = regexp.MustCompile(`^[A-Za-z0-9 .,_!?@#+=/:*()-]+$`)

// Text types printable ASCII into the focused field.  Spaces are sent as %s,
// which is what "input text" expects.
func (m *Manager) Text(ctx context.Context, name, text string) error {
	if text == "" {
		return fmt.Errorf("text is empty")
	}
	if len(text) > 512 {
		return fmt.Errorf("text is too long (max 512 characters)")
	}
	if !safeText.MatchString(text) {
		return fmt.Errorf("text contains unsupported characters; use letters, digits and basic punctuation")
	}
	escaped := strings.ReplaceAll(text, " ", "%s")
	_, err := m.adb(ctx, m.serial(name), "shell", "input", "text", escaped)
	return err
}

func (m *Manager) serial(name string) string {
	avd, ok := m.Get(name)
	if !ok {
		return ""
	}
	return avd.Serial
}

func (m *Manager) adb(ctx context.Context, serial string, args ...string) ([]byte, error) {
	if !m.Paths.HasPlatformTools() {
		return nil, errors.New("platform-tools (adb) is not installed; run the runtime setup first")
	}
	if serial == "" {
		return nil, errors.New("this AVD has no adb serial yet; start it from VOM and wait for Android to boot")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	full := append([]string{"-s", serial}, args...)
	cmd := exec.CommandContext(ctx, m.Paths.ADBBinary(), full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("adb: %s", msg)
	}
	return stdout.Bytes(), nil
}

// scaleNearest resizes an image with nearest-neighbour sampling.  It is good
// enough for a downscaled live view and needs no external dependencies.
func scaleNearest(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sy := bounds.Min.Y + y*bounds.Dy()/height
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

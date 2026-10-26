package android

import (
	"image"
	"image/color"
	"testing"
)

func TestScaleNearest(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.Set(0, 0, color.RGBA{R: 255, A: 255})
	src.Set(3, 1, color.RGBA{B: 255, A: 255})
	got := scaleNearest(src, 2, 1)
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 1 {
		t.Fatalf("bounds = %v, want 2x1", got.Bounds())
	}
	r, _, _, _ := got.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Fatalf("first pixel red = %d, want 255", r>>8)
	}
}

func TestKeyCodeWhitelist(t *testing.T) {
	for _, code := range []int{3, 4, 187} {
		if !AllowedKeyCodes[code] {
			t.Errorf("key %d should be allowed", code)
		}
	}
	if AllowedKeyCodes[1] {
		t.Error("key 1 should not be allowed")
	}
}

func TestFreeEmulatorPort(t *testing.T) {
	port, err := freeEmulatorPort()
	if err != nil {
		t.Fatal(err)
	}
	if port < 5554 || port > 5682 || port%2 != 0 {
		t.Fatalf("unexpected port %d", port)
	}
}

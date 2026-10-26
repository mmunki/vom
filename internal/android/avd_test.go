package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmunki/vom/internal/catalog"
	"github.com/mmunki/vom/internal/store"
)

func fakeImage(t *testing.T) (*store.Paths, catalog.Image) {
	t.Helper()
	p := store.New(t.TempDir())
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	img := catalog.Image{
		ID: "api33-google_apis-x86_64", API: 33, APILevel: "33",
		Version: "13", Codename: "Tiramisu", Tag: "google_apis", ABI: "x86_64",
	}
	dir := p.SystemDir(img)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system.img"), []byte("not a real disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, img
}

func TestCreateWritesAVDConfig(t *testing.T) {
	p, img := fakeImage(t)
	m := NewManager(p)
	avd, err := m.Create(img, CreateOptions{Name: "Test-Device", RAMMB: 3072, Cores: 4, Width: 1080, Height: 2400, DPI: 420})
	if err != nil {
		t.Fatal(err)
	}
	if avd.Name != "Test-Device" || avd.API != 33 || avd.ABI != "x86_64" {
		t.Fatalf("unexpected AVD: %+v", avd)
	}
	config, err := os.ReadFile(filepath.Join(avd.Dir, "config.ini"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(config)
	for _, want := range []string{
		"abi.type=x86_64",
		"hw.cpu.arch=x86_64",
		"hw.ramSize=3072",
		"hw.lcd.width=1080",
		"tag.id=google_apis",
		"image.sysdir.1=" + filepath.ToSlash(p.SystemDir(img)) + "/",
		"PlayStore.enabled=false",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("config.ini is missing %q:\n%s", want, text)
		}
	}
	ini, err := os.ReadFile(filepath.Join(p.AVD, "Test-Device.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ini), "target=android-33") {
		t.Fatalf("target missing from .ini:\n%s", ini)
	}
	list, err := m.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
}

func TestCreateRejectsBadNameAndDuplicate(t *testing.T) {
	p, img := fakeImage(t)
	m := NewManager(p)
	if _, err := m.Create(img, CreateOptions{Name: "../oops"}); err == nil {
		t.Fatal("bad name accepted")
	}
	if _, err := m.Create(img, CreateOptions{Name: "Test Device"}); err == nil {
		t.Fatal("name with spaces accepted")
	}
	if _, err := m.Create(img, CreateOptions{Name: "Same"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(img, CreateOptions{Name: "Same"}); err == nil {
		t.Fatal("duplicate name accepted")
	}
}

func TestCPUArch(t *testing.T) {
	if cpuArch("arm64-v8a") != "arm64" || cpuArch("x86_64") != "x86_64" {
		t.Fatal("unexpected cpu arch mapping")
	}
}

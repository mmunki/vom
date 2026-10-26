package download

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestExtractZip(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"x86_64/system.img": "disk",
		"x86_64/build.prop": "ro.build.version.sdk=24\n",
	})
	dest := t.TempDir()
	if err := ExtractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "x86_64", "system.img"))
	if err != nil || string(b) != "disk" {
		t.Fatalf("system.img = %q, %v", b, err)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	zipPath := writeZip(t, map[string]string{"../escape.txt": "no"})
	if err := ExtractZip(zipPath, t.TempDir()); err == nil {
		t.Fatal("path traversal was accepted")
	}
}

func TestVerifySHA1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(path, "a9993e364706816aba3e25717850c26c9cd0d89d", 3)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	ok, _ = Verify(path, "a9993e364706816aba3e25717850c26c9cd0d89e", 3)
	if ok {
		t.Fatal("bad checksum accepted")
	}
}

func TestWithin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if !within(root, filepath.Join(root, "a", "b")) {
		t.Fatal("child rejected")
	}
	if within(root, filepath.Join(root, "..", "b")) {
		t.Fatal("traversal accepted")
	}
}

// Package store owns the on-disk layout of VOM: downloaded SDK tools,
// system images, AVDs, logs and the catalog cache.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mmunki/vom/internal/catalog"
)

// Paths is the VOM data directory layout.
type Paths struct {
	Home          string
	SDK           string
	Emulator      string
	PlatformTools string
	SystemImages  string
	AVD           string
	Logs          string
	Downloads     string
	Cache         string
}

// DefaultHome follows the platform convention for large application data.
func DefaultHome() string {
	if v := os.Getenv("VOM_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vom"
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "VOM")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "VOM")
		}
		return filepath.Join(home, "AppData", "Local", "VOM")
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "vom")
		}
		return filepath.Join(home, ".local", "share", "vom")
	}
}

// New creates the layout for a home directory.
func New(home string) *Paths {
	abs, err := filepath.Abs(home)
	if err == nil {
		home = abs
	}
	p := &Paths{Home: home}
	p.SDK = filepath.Join(home, "sdk")
	p.Emulator = filepath.Join(p.SDK, "emulator")
	p.PlatformTools = filepath.Join(p.SDK, "platform-tools")
	p.SystemImages = filepath.Join(p.SDK, "system-images")
	p.AVD = filepath.Join(home, "avd")
	p.Logs = filepath.Join(home, "logs")
	p.Downloads = filepath.Join(home, "downloads")
	p.Cache = filepath.Join(home, "cache")
	return p
}

// Ensure creates the directories VOM needs.
func (p *Paths) Ensure() error {
	for _, dir := range []string{p.Home, p.SDK, p.SystemImages, p.AVD, p.Logs, p.Downloads, p.Cache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// EmulatorBinary returns the emulator executable path for this host.
func (p *Paths) EmulatorBinary() string {
	name := "emulator"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.Emulator, name)
}

// ADBBinary returns the adb executable path for this host.
func (p *Paths) ADBBinary() string {
	name := "adb"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.PlatformTools, name)
}

// HasEmulator reports whether the emulator executable is present.
func (p *Paths) HasEmulator() bool {
	st, err := os.Stat(p.EmulatorBinary())
	return err == nil && !st.IsDir()
}

// HasPlatformTools reports whether adb is present.
func (p *Paths) HasPlatformTools() bool {
	st, err := os.Stat(p.ADBBinary())
	return err == nil && !st.IsDir()
}

// ImageDir is the directory a system-image zip extracts into (contains <abi>/).
func (p *Paths) ImageDir(img catalog.Image) string {
	return filepath.Join(p.SystemImages, "android-"+img.APILevel, img.Tag)
}

// SystemDir is the directory that image.sysdir.1 must point at.
func (p *Paths) SystemDir(img catalog.Image) string {
	return filepath.Join(p.ImageDir(img), img.ABI)
}

// InstalledImage describes a system image present on disk.
type InstalledImage struct {
	catalog.Image
	Dir         string    `json:"dir"`
	InstalledAt time.Time `json:"installedAt"`
	Bytes       int64     `json:"bytes"`
}

func metaPath(p *Paths, img catalog.Image) string {
	return filepath.Join(p.ImageDir(img), "vom-image.json")
}

// SaveImageMeta records an installed image.
func (p *Paths) SaveImageMeta(img catalog.Image) error {
	rec := InstalledImage{Image: img, Dir: p.SystemDir(img), InstalledAt: time.Now().UTC()}
	rec.Bytes = DirSize(p.ImageDir(img))
	b, err := json.MarshalIndent(rec, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.ImageDir(img), 0o755); err != nil {
		return err
	}
	return os.WriteFile(metaPath(p, img), b, 0o644)
}

// Installed lists system images that have a metadata file and a system.img.
func (p *Paths) Installed() ([]InstalledImage, error) {
	var out []InstalledImage
	err := filepath.WalkDir(p.SystemImages, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "vom-image.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var rec InstalledImage
		if json.Unmarshal(b, &rec) != nil {
			return nil
		}
		if _, err := os.Stat(filepath.Join(rec.Dir, "system.img")); err != nil {
			return nil
		}
		rec.Bytes = DirSize(filepath.Dir(path))
		out = append(out, rec)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].API != out[j].API {
			return out[i].API < out[j].API
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// InstalledMap is a convenience index of installed image IDs.
func (p *Paths) InstalledMap() map[string]InstalledImage {
	out := map[string]InstalledImage{}
	list, _ := p.Installed()
	for _, rec := range list {
		out[rec.ID] = rec
	}
	return out
}

// RemoveImage deletes one installed image directory.
func (p *Paths) RemoveImage(img catalog.Image) error {
	dir := p.ImageDir(img)
	if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(p.SystemImages)+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to remove %s: outside the image directory", dir)
	}
	return os.RemoveAll(dir)
}

// DirSize sums the regular files in a directory.
func DirSize(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// HumanBytes formats a byte count for logs and the CLI.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

package android

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmunki/vom/internal/catalog"
	"github.com/mmunki/vom/internal/download"
	"github.com/mmunki/vom/internal/store"
)

// InstallImage downloads, verifies, extracts and records one system image.
func InstallImage(ctx context.Context, p *store.Paths, client *http.Client, img catalog.Image, r download.Reporter) error {
	if _, err := os.Stat(filepath.Join(p.SystemDir(img), "system.img")); err == nil {
		return fmt.Errorf("Android %s (%s, %s) is already installed", img.Version, catalog.TagName(img.Tag), img.ABI)
	}
	if err := download.EnsureSpace(p.Home, img.Size); err != nil {
		return err
	}
	if err := os.MkdirAll(p.Downloads, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(p.Downloads, filepath.Base(img.URL))
	if err := download.File(ctx, client, img.URL, dest, img.SHA1, img.Size, r); err != nil {
		return err
	}
	if r != nil {
		r("extracting", 0, 0)
	}
	root := filepath.Clean(p.SystemImages) + string(os.PathSeparator)
	dir := filepath.Clean(p.ImageDir(img))
	if !strings.HasPrefix(dir, root) {
		return fmt.Errorf("refusing to write outside %s", p.SystemImages)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := download.ExtractZip(dest, dir); err != nil {
		return fmt.Errorf("extract image: %w", err)
	}
	if _, err := os.Stat(filepath.Join(p.SystemDir(img), "system.img")); err != nil {
		return fmt.Errorf("image archive did not contain %s/system.img", img.ABI)
	}
	if err := p.SaveImageMeta(img); err != nil {
		return err
	}
	os.Remove(dest)
	if r != nil {
		r("installed", 1, 1)
	}
	return nil
}

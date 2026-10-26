// Package download implements resumable HTTP downloads with checksum
// verification and safe archive extraction.
package download

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Reporter receives download progress. It must be safe to call it often.
type Reporter func(stage string, done, total int64)

// Client is tuned for multi-gigabyte downloads: no overall timeout, but the
// server has to start responding within a minute.
func Client() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

func report(r Reporter, stage string, done, total int64) {
	if r != nil {
		r(stage, done, total)
	}
}

// File downloads url to dest, resumes a .part file when the server supports
// ranges, checks the SHA-1 and only then renames the file into place.
func File(ctx context.Context, client *http.Client, url, dest, wantSHA1 string, wantSize int64, r Reporter) error {
	if client == nil {
		client = Client()
	}
	if ok, _ := Verify(dest, wantSHA1, wantSize); ok {
		report(r, "already downloaded", wantSize, wantSize)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	part := dest + ".part"
	var offset int64
	if st, err := os.Stat(part); err == nil && st.Size() > 0 {
		offset = st.Size()
	}
	if wantSize > 0 && offset > wantSize {
		os.Remove(part)
		offset = 0
	}

	f, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha1.New()
	if offset > 0 {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "VOM/1.0")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if offset > 0 {
			// The server ignored the range, start over.
			if err := f.Truncate(0); err != nil {
				return err
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			h = sha1.New()
			offset = 0
		}
	case http.StatusPartialContent:
		if offset == 0 {
			return fmt.Errorf("download: server sent a partial response without a range")
		}
	default:
		return fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}

	total := wantSize
	if total <= 0 && resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	}
	report(r, "downloading", offset, total)

	buf := make([]byte, 256*1024)
	last := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			h.Write(buf[:n])
			offset += int64(n)
			if time.Since(last) > 150*time.Millisecond {
				report(r, "downloading", offset, total)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("download: %w", rerr)
		}
	}
	report(r, "downloading", offset, total)
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if wantSize > 0 && offset != wantSize {
		return fmt.Errorf("download: got %d bytes, expected %d", offset, wantSize)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if wantSHA1 != "" && !strings.EqualFold(got, wantSHA1) {
		os.Remove(part)
		return fmt.Errorf("download: SHA-1 mismatch (got %s, expected %s)", got, wantSHA1)
	}
	report(r, "verifying", offset, offset)
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	report(r, "downloaded", offset, offset)
	return nil
}

// Verify checks the size and SHA-1 of an existing file.
func Verify(path, wantSHA1 string, wantSize int64) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if st.IsDir() {
		return false, fmt.Errorf("%s is a directory", path)
	}
	if wantSize > 0 && st.Size() != wantSize {
		return false, nil
	}
	if wantSHA1 == "" {
		return true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), wantSHA1), nil
}

// SHA1File returns the hex SHA-1 of a file.
func SHA1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var h hash.Hash = sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ExtractZip unpacks a zip into dest. It rejects absolute and traversal paths,
// validates symlinks and keeps executable modes. Existing files are replaced.
func ExtractZip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	cleanDest := filepath.Clean(dest)
	if err := os.MkdirAll(cleanDest, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("zip entry has an absolute path: %s", f.Name)
		}
		target := filepath.Join(cleanDest, filepath.FromSlash(name))
		if !within(cleanDest, target) {
			return fmt.Errorf("zip entry escapes the destination: %s", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			link, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			linkTarget := filepath.Join(filepath.Dir(target), filepath.FromSlash(string(link)))
			if !within(cleanDest, linkTarget) {
				return fmt.Errorf("zip symlink escapes the destination: %s", f.Name)
			}
			os.Remove(target)
			if err := os.Symlink(string(link), target); err != nil {
				return err
			}
			continue
		}

		if err := extractFile(f, target, mode); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string, mode os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	os.Remove(target)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode(mode))
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, rc)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(target, fileMode(mode))
}

func fileMode(m os.FileMode) os.FileMode {
	perm := m.Perm()
	if perm == 0 {
		perm = 0o644
	}
	return perm
}

func within(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// EnsureSpace returns an error when the destination does not have enough free
// space for the download plus its extracted contents.
func EnsureSpace(dest string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	free, err := FreeSpace(dest)
	if err != nil || free == 0 {
		return nil
	}
	need := uint64(bytes) * 2
	if free < need {
		return fmt.Errorf("not enough free space in %s: %s available, about %s needed",
			dest, human(free), human(need))
	}
	return nil
}

func human(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

var ErrCanceled = errors.New("canceled")

// Package android installs the official emulator and creates/launches AVDs.
package android

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/mmunki/vom/internal/download"
	"github.com/mmunki/vom/internal/store"
)

const repositoryURL = "https://dl.google.com/android/repository/repository2-3.xml"

// ToolArchive is one downloadable SDK tool archive.
type ToolArchive struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	SHA1     string `json:"sha1"`
	Size     int64  `json:"size"`
	Revision string `json:"revision"`
	HostOS   string `json:"hostOs"`
	HostArch string `json:"hostArch"`
}

// ToolManifest records what Setup installed.
type ToolManifest struct {
	InstalledAt time.Time   `json:"installedAt"`
	Emulator    ToolArchive `json:"emulator"`
	Platform    ToolArchive `json:"platformTools"`
}

type sdkRepoXML struct {
	RemotePackages []sdkPackageXML `xml:"remotePackage"`
}

type sdkPackageXML struct {
	Path       string      `xml:"path,attr"`
	Revision   xmlRevision `xml:"revision"`
	ChannelRef struct {
		Ref string `xml:"ref,attr"`
	} `xml:"channelRef"`
	Archives struct {
		Archive []archiveXML `xml:"archive"`
	} `xml:"archives"`
}

type xmlRevision struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
	Micro int `xml:"micro"`
}

func (r xmlRevision) key() [3]int { return [3]int{r.Major, r.Minor, r.Micro} }

type archiveXML struct {
	Complete struct {
		Size     int64 `xml:"size"`
		Checksum struct {
			Value string `xml:",chardata"`
		} `xml:"checksum"`
		URL string `xml:"url"`
	} `xml:"complete"`
	HostOS   string `xml:"host-os"`
	HostArch string `xml:"host-arch"`
}

func hostOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macosx"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func hostArch() string {
	if runtime.GOARCH == "arm64" {
		return "aarch64"
	}
	return "x64"
}

func mapHostOS(osName string) string {
	switch osName {
	case "macosx":
		return "darwin"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func fetchRepo(ctx context.Context, client *http.Client) ([]sdkPackageXML, error) {
	if client == nil {
		client = download.Client()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repositoryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "VOM/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch SDK repository: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch SDK repository: HTTP %s", resp.Status)
	}
	var repo sdkRepoXML
	if err := xml.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("parse SDK repository: %w", err)
	}
	return repo.RemotePackages, nil
}

func pickArchive(pkgs []sdkPackageXML, path, wantOS, wantArch string, stableOnly bool) (ToolArchive, error) {
	var best *sdkPackageXML
	for i := range pkgs {
		p := &pkgs[i]
		if p.Path != path {
			continue
		}
		if stableOnly && p.ChannelRef.Ref != "" && p.ChannelRef.Ref != "channel-0" {
			continue
		}
		if best == nil || greater(p.Revision.key(), best.Revision.key()) {
			best = p
		}
	}
	if best == nil {
		return ToolArchive{}, fmt.Errorf("no %s package for this host", path)
	}
	for _, a := range best.Archives.Archive {
		if a.HostOS != "" && a.HostOS != wantOS {
			continue
		}
		if a.HostArch != "" && a.HostArch != wantArch {
			continue
		}
		if a.Complete.URL == "" {
			continue
		}
		rev := fmt.Sprintf("%d.%d.%d", best.Revision.Major, best.Revision.Minor, best.Revision.Micro)
		return ToolArchive{
			Name:     path,
			URL:      "https://dl.google.com/android/repository/" + a.Complete.URL,
			SHA1:     a.Complete.Checksum.Value,
			Size:     a.Complete.Size,
			Revision: rev,
			HostOS:   mapHostOS(a.HostOS),
			HostArch: a.HostArch,
		}, nil
	}
	return ToolArchive{}, fmt.Errorf("no %s archive for %s/%s", path, wantOS, wantArch)
}

func greater(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// ResolveTools looks up the current stable emulator and platform-tools archives.
func ResolveTools(ctx context.Context, client *http.Client) (emulator, platformTools ToolArchive, err error) {
	pkgs, err := fetchRepo(ctx, client)
	if err != nil {
		return ToolArchive{}, ToolArchive{}, err
	}
	emulator, err = pickArchive(pkgs, "emulator", hostOS(), hostArch(), true)
	if err != nil {
		return ToolArchive{}, ToolArchive{}, err
	}
	platformTools, err = pickArchive(pkgs, "platform-tools", hostOS(), "", false)
	if err != nil {
		return ToolArchive{}, ToolArchive{}, err
	}
	return emulator, platformTools, nil
}

func manifestPath(p *store.Paths) string { return filepath.Join(p.SDK, ".vom-tools.json") }

// ReadManifest returns the installed tool manifest, if any.
func ReadManifest(p *store.Paths) (ToolManifest, error) {
	var m ToolManifest
	b, err := os.ReadFile(manifestPath(p))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

// Setup downloads and installs the emulator and platform-tools.
func Setup(ctx context.Context, p *store.Paths, client *http.Client, force bool, r download.Reporter) (ToolManifest, error) {
	if err := p.Ensure(); err != nil {
		return ToolManifest{}, err
	}
	if !force && p.HasEmulator() && p.HasPlatformTools() {
		if m, err := ReadManifest(p); err == nil {
			return m, nil
		}
		return ToolManifest{InstalledAt: time.Now().UTC()}, nil
	}
	emulator, platformTools, err := ResolveTools(ctx, client)
	if err != nil {
		return ToolManifest{}, err
	}
	if err := installArchive(ctx, p, emulator, filepath.Join(p.SDK, "emulator"), r); err != nil {
		return ToolManifest{}, err
	}
	if err := installArchive(ctx, p, platformTools, filepath.Join(p.SDK, "platform-tools"), r); err != nil {
		return ToolManifest{}, err
	}
	manifest := ToolManifest{InstalledAt: time.Now().UTC(), Emulator: emulator, Platform: platformTools}
	b, _ := json.MarshalIndent(manifest, "", " ")
	if err := os.WriteFile(manifestPath(p), b, 0o644); err != nil {
		return ToolManifest{}, err
	}
	return manifest, nil
}

func installArchive(ctx context.Context, p *store.Paths, a ToolArchive, target string, r download.Reporter) error {
	if a.Size > 0 {
		if err := download.EnsureSpace(p.Downloads, a.Size); err != nil {
			return err
		}
	}
	dest := filepath.Join(p.Downloads, filepath.Base(a.URL))
	if err := download.File(ctx, nil, a.URL, dest, a.SHA1, a.Size, r); err != nil {
		return err
	}
	if r != nil {
		r("extracting", 0, 0)
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	// Emulator and platform-tools zips both carry one top-level directory.
	if err := download.ExtractZip(dest, p.SDK); err != nil {
		return err
	}
	os.Remove(dest)
	if st, err := os.Stat(p.EmulatorBinary()); err == nil && !st.IsDir() {
		os.Chmod(p.EmulatorBinary(), 0o755)
	}
	if st, err := os.Stat(p.ADBBinary()); err == nil && !st.IsDir() {
		os.Chmod(p.ADBBinary(), 0o755)
	}
	if r != nil {
		r("installed", 1, 1)
	}
	return nil
}

// SortedToolArchives is used by the CLI doctor output.
func SortedToolArchives(m ToolManifest) []ToolArchive {
	out := []ToolArchive{m.Emulator, m.Platform}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SetExecutable is a small helper used by tests and the installer.
func SetExecutable(path string) error {
	if err := os.Chmod(path, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

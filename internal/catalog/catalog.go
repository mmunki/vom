// Package catalog reads the official Google Android system-image repository
// metadata and turns it into the list of installable Android 7.0 - 17 images.
package catalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	RepoBase    = "https://dl.google.com/android/repository/"
	MinAPI      = 24 // Android 7.0
	MaxAPI      = 37 // Android 17
	cacheName   = "catalog.json"
	cacheMaxAge = 12 * time.Hour
)

// Image is one downloadable system-image package.
type Image struct {
	ID       string `json:"id"`
	API      int    `json:"api"`
	APILevel string `json:"apiLevel"`
	Version  string `json:"version"`
	Codename string `json:"codename"`
	Tag      string `json:"tag"`
	ABI      string `json:"abi"`
	Path     string `json:"path"`
	URL      string `json:"url"`
	SHA1     string `json:"sha1"`
	Size     int64  `json:"size"`
	Revision string `json:"revision"`
	Display  string `json:"display"`
	Source   string `json:"source"`
}

// Release groups the images of one Android version.
type Release struct {
	API      int     `json:"api"`
	Version  string  `json:"version"`
	Codename string  `json:"codename"`
	Images   []Image `json:"images"`
}

type cacheFile struct {
	Updated time.Time `json:"updated"`
	Images  []Image   `json:"images"`
}

//go:embed fallback.json
var fallbackJSON []byte

var sources = []struct {
	name string
	url  string
}{
	{"android", RepoBase + "sys-img/android/sys-img2-3.xml"},
	{"google_apis", RepoBase + "sys-img/google_apis/sys-img2-3.xml"},
	{"google_apis_playstore", RepoBase + "sys-img/google_apis_playstore/sys-img2-3.xml"},
}

var (
	allowedTags = map[string]bool{
		"default":               true,
		"google_apis":           true,
		"google_apis_playstore": true,
	}
	allowedABIs = map[string]bool{
		"x86_64":    true,
		"x86":       true,
		"arm64-v8a": true,
	}
	tagOrder = map[string]int{
		"google_apis_playstore": 0,
		"google_apis":           1,
		"default":               2,
	}
	abiOrder = map[string]int{"x86_64": 0, "arm64-v8a": 1, "x86": 2}
)

var versionNames = map[int][2]string{
	24: {"7.0", "Nougat"},
	25: {"7.1", "Nougat"},
	26: {"8.0", "Oreo"},
	27: {"8.1", "Oreo"},
	28: {"9", "Pie"},
	29: {"10", "Q"},
	30: {"11", "R"},
	31: {"12", "S"},
	32: {"12L", "S_V2"},
	33: {"13", "Tiramisu"},
	34: {"14", "Upside Down Cake"},
	35: {"15", "Vanilla Ice Cream"},
	36: {"16", "Baklava"},
	37: {"17", ""},
}

// VersionName returns the marketing version and the AOSP codename of an API.
func VersionName(api int) (string, string) {
	v, ok := versionNames[api]
	if !ok {
		return strconv.Itoa(api), ""
	}
	return v[0], v[1]
}

// ID builds the stable identifier used by the UI and the CLI.
func ID(apiLevel, tag, abi string) string {
	return "api" + apiLevel + "-" + tag + "-" + abi
}

// TagName is the short, human readable name of a system-image tag.
func TagName(tag string) string {
	switch tag {
	case "default":
		return "AOSP"
	case "google_apis":
		return "Google APIs"
	case "google_apis_playstore":
		return "Google Play"
	default:
		return tag
	}
}

// Fallback returns the catalog that is embedded in the binary.
func Fallback() ([]Image, time.Time, error) {
	var images []Image
	if err := json.Unmarshal(fallbackJSON, &images); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode embedded catalog: %w", err)
	}
	return images, time.Time{}, nil
}

func cachePath(home string) string { return filepath.Join(home, "cache", cacheName) }

// Load returns the cached catalog, or the embedded one when there is no cache.
func Load(home string) (images []Image, updated time.Time, source string, err error) {
	b, err := os.ReadFile(cachePath(home))
	if err == nil {
		var c cacheFile
		if json.Unmarshal(b, &c) == nil && len(c.Images) > 0 {
			return c.Images, c.Updated, "cache", nil
		}
	}
	images, updated, err = Fallback()
	if err != nil {
		return nil, time.Time{}, "", err
	}
	return images, updated, "embedded", nil
}

// NeedsRefresh reports whether the cached catalog is older than cacheMaxAge.
func NeedsRefresh(updated time.Time) bool {
	return time.Since(updated) > cacheMaxAge
}

type xmlRepo struct {
	RemotePackages []xmlPackage `xml:"remotePackage"`
}

type xmlPackage struct {
	Path        string      `xml:"path,attr"`
	TypeDetails xmlDetails  `xml:"type-details"`
	Revision    xmlRevision `xml:"revision"`
	DisplayName string      `xml:"display-name"`
	Archives    xmlArchives `xml:"archives"`
}

type xmlDetails struct {
	APILevel string `xml:"api-level"`
	Tag      xmlTag `xml:"tag"`
	ABI      string `xml:"abi"`
}

type xmlTag struct {
	ID      string `xml:"id"`
	Display string `xml:"display"`
}

type xmlRevision struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
	Micro int `xml:"micro"`
}

func (r xmlRevision) String() string {
	return fmt.Sprintf("%d.%d.%d", r.Major, r.Minor, r.Micro)
}

type xmlArchives struct {
	Archive []xmlArchive `xml:"archive"`
}

type xmlArchive struct {
	Complete xmlComplete `xml:"complete"`
	HostOS   string      `xml:"host-os"`
	HostArch string      `xml:"host-arch"`
}

type xmlComplete struct {
	Size     int64       `xml:"size"`
	Checksum xmlChecksum `xml:"checksum"`
	URL      string      `xml:"url"`
}

type xmlChecksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

var majorAPI = regexp.MustCompile(`^(\d+)`)

func parseSource(name, baseURL string, r io.Reader) ([]Image, error) {
	var repo xmlRepo
	if err := xml.NewDecoder(r).Decode(&repo); err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	var out []Image
	for _, p := range repo.RemotePackages {
		if !strings.HasPrefix(p.Path, "system-images;") {
			continue
		}
		parts := strings.Split(p.Path, ";")
		if len(parts) < 4 {
			continue
		}
		m := majorAPI.FindStringSubmatch(strings.TrimSpace(p.TypeDetails.APILevel))
		if m == nil {
			continue
		}
		api, _ := strconv.Atoi(m[1])
		if api < MinAPI || api > MaxAPI {
			continue
		}
		tag := strings.TrimSpace(p.TypeDetails.Tag.ID)
		if tag == "" {
			tag = parts[2]
		}
		abi := strings.TrimSpace(p.TypeDetails.ABI)
		if abi == "" {
			abi = parts[len(parts)-1]
		}
		if !allowedTags[tag] || !allowedABIs[abi] {
			continue
		}
		low := strings.ToLower(p.Path)
		if strings.Contains(low, "beta") || strings.Contains(low, "-ext") ||
			strings.Contains(low, ";atd;") || strings.Contains(low, "ps16k") {
			continue
		}
		if len(p.Archives.Archive) == 0 {
			continue
		}
		a := p.Archives.Archive[0]
		if strings.TrimSpace(a.Complete.URL) == "" {
			continue
		}
		version, codename := VersionName(api)
		out = append(out, Image{
			ID:       ID(p.TypeDetails.APILevel, tag, abi),
			API:      api,
			APILevel: p.TypeDetails.APILevel,
			Version:  version,
			Codename: codename,
			Tag:      tag,
			ABI:      abi,
			Path:     p.Path,
			URL:      baseURL + strings.TrimSpace(a.Complete.URL),
			SHA1:     strings.ToLower(strings.TrimSpace(a.Complete.Checksum.Value)),
			Size:     a.Complete.Size,
			Revision: p.Revision.String(),
			Display:  p.DisplayName,
			Source:   name,
		})
	}
	return out, nil
}

func revisionKey(s string) [3]int {
	var out [3]int
	for i, p := range strings.Split(s, ".") {
		if i >= 3 {
			break
		}
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

func greaterRevision(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func dedupe(in []Image) []Image {
	best := map[string]Image{}
	for _, img := range in {
		key := fmt.Sprintf("%d|%s|%s", img.API, img.Tag, img.ABI)
		if old, ok := best[key]; !ok || greaterRevision(revisionKey(img.Revision), revisionKey(old.Revision)) {
			best[key] = img
		}
	}
	out := make([]Image, 0, len(best))
	for _, img := range best {
		out = append(out, img)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].API != out[j].API {
			return out[i].API < out[j].API
		}
		if tagOrder[out[i].Tag] != tagOrder[out[j].Tag] {
			return tagOrder[out[i].Tag] < tagOrder[out[j].Tag]
		}
		if abiOrder[out[i].ABI] != abiOrder[out[j].ABI] {
			return abiOrder[out[i].ABI] < abiOrder[out[j].ABI]
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Refresh downloads the three official repository indexes and writes the cache.
func Refresh(ctx context.Context, home string, client *http.Client) ([]Image, error) {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	var all []Image
	for _, src := range sources {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "VOM/1.0 catalog")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", src.name, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch %s: HTTP %s", src.name, resp.Status)
		}
		imgs, perr := parseSource(src.name, RepoBase+"sys-img/"+src.name+"/", resp.Body)
		resp.Body.Close()
		if perr != nil {
			return nil, perr
		}
		all = append(all, imgs...)
	}
	images := dedupe(all)
	if len(images) == 0 {
		return nil, fmt.Errorf("the repository returned no Android %d-%d images", MinAPI, MaxAPI)
	}
	c := cacheFile{Updated: time.Now().UTC(), Images: images}
	if err := os.MkdirAll(filepath.Dir(cachePath(home)), 0o755); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return nil, err
	}
	tmp := cachePath(home) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, cachePath(home)); err != nil {
		return nil, err
	}
	return images, nil
}

// Group turns a flat image list into Android releases.
func Group(images []Image) []Release {
	var releases []Release
	byAPI := map[int]int{}
	for _, img := range images {
		idx, ok := byAPI[img.API]
		if !ok {
			version, codename := VersionName(img.API)
			idx = len(releases)
			byAPI[img.API] = idx
			releases = append(releases, Release{API: img.API, Version: version, Codename: codename})
		}
		releases[idx].Images = append(releases[idx].Images, img)
	}
	sort.Slice(releases, func(i, j int) bool { return releases[i].API < releases[j].API })
	return releases
}

// Find returns the image with the given ID.
func Find(images []Image, id string) (Image, bool) {
	for _, img := range images {
		if img.ID == id {
			return img, true
		}
	}
	return Image{}, false
}

// Pick chooses the image for an Android API, tag and host ABI.
func Pick(images []Image, api int, tag, abi string) (Image, bool) {
	var candidates []Image
	for _, img := range images {
		if img.API != api {
			continue
		}
		if tag != "" && img.Tag != tag {
			continue
		}
		candidates = append(candidates, img)
	}
	if len(candidates) == 0 {
		return Image{}, false
	}
	preferredABIs := []string{abi, "x86_64", "arm64-v8a", "x86"}
	for _, want := range preferredABIs {
		for _, img := range candidates {
			if img.ABI == want {
				return img, true
			}
		}
	}
	return candidates[0], true
}

// RecommendedABI returns the ABI the official emulator can run best on this host.
func RecommendedABI(goos, goarch string) string {
	switch goarch {
	case "arm64":
		return "arm64-v8a"
	default:
		return "x86_64"
	}
}

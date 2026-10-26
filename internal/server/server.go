// Package server exposes the local VOM HTTP API and serves the Material 3 UI.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mmunki/vom/internal/android"
	"github.com/mmunki/vom/internal/catalog"
	"github.com/mmunki/vom/internal/download"
	"github.com/mmunki/vom/internal/store"
)

//go:embed static
var staticFS embed.FS

// Server is the VOM application server.
type Server struct {
	Paths   *store.Paths
	Client  *http.Client
	Jobs    *JobManager
	AVDs    *android.Manager
	Version string

	mu      sync.RWMutex
	images  []catalog.Image
	updated time.Time
	source  string
}

// New loads the catalog and prepares the API.
func New(p *store.Paths, version string) (*Server, error) {
	if err := p.Ensure(); err != nil {
		return nil, err
	}
	s := &Server{
		Paths: p, Client: download.Client(), Jobs: NewJobs(),
		AVDs: android.NewManager(p), Version: version,
	}
	images, updated, source, err := catalog.Load(p.Home)
	if err != nil {
		return nil, err
	}
	s.images, s.updated, s.source = images, updated, source
	return s, nil
}

// CatalogSnapshot returns the current images and their source metadata.
func (s *Server) CatalogSnapshot() ([]catalog.Image, time.Time, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]catalog.Image(nil), s.images...), s.updated, s.source
}

func (s *Server) setCatalog(images []catalog.Image, updated time.Time, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.images, s.updated, s.source = images, updated, source
}

// RefreshCatalog downloads the official indexes and swaps them in.
func (s *Server) RefreshCatalog(ctx context.Context) ([]catalog.Image, error) {
	images, err := catalog.Refresh(ctx, s.Paths.Home, s.Client)
	if err != nil {
		return nil, err
	}
	s.setCatalog(images, time.Now().UTC(), "network")
	return images, nil
}

// RefreshInBackground starts a catalog refresh job unless one is already running.
func (s *Server) RefreshInBackground(label string) *Job {
	for _, j := range s.Jobs.List(20) {
		if j.Kind == "catalog" && j.State == "running" {
			return nil
		}
	}
	return s.Jobs.Start("catalog", label, func(ctx context.Context, job *Job) (interface{}, error) {
		s.Jobs.Update(job.ID, "downloading repository indexes", 0, 0)
		images, err := s.RefreshCatalog(ctx)
		if err != nil {
			return nil, err
		}
		s.Jobs.Update(job.ID, "catalog updated", int64(len(images)), int64(len(images)))
		return map[string]int{"images": len(images)}, nil
	})
}

// Handler builds the HTTP router.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/catalog", s.handleCatalog)
	mux.HandleFunc("/api/catalog/refresh", s.handleCatalogRefresh)
	mux.HandleFunc("/api/setup", s.handleSetup)
	mux.HandleFunc("/api/install", s.handleInstall)
	mux.HandleFunc("/api/images", s.handleImages)
	mux.HandleFunc("/api/library", s.handleLibrary)
	mux.HandleFunc("/api/avd", s.handleAVD)
	mux.HandleFunc("/api/avd/start", s.handleAVDStart)
	mux.HandleFunc("/api/avd/stop", s.handleAVDStop)
	mux.HandleFunc("/api/avd/delete", s.handleAVDDelete)
	mux.HandleFunc("/api/avd/log", s.handleAVDLog)
	mux.HandleFunc("/api/avd/screen", s.handleAVDScreen)
	mux.HandleFunc("/api/avd/input", s.handleAVDInput)
	mux.HandleFunc("/api/jobs", s.handleJobs)
	mux.HandleFunc("/api/jobs/cancel", s.handleJobCancel)
	mux.HandleFunc("/", s.handleStatic)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		http.Error(w, "static assets unavailable", http.StatusInternalServerError)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	b, err := fs.ReadFile(sub, name)
	if err != nil {
		name = "index.html"
		b, err = fs.ReadFile(sub, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if name == "index.html" || name == "sw.js" {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	if name == "sw.js" {
		w.Header().Set("Service-Worker-Allowed", "/")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(b)
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return false
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return false
	}
	return true
}

type statusResponse struct {
	Version        string                `json:"version"`
	OS             string                `json:"os"`
	Arch           string                `json:"arch"`
	Host           string                `json:"host"`
	Home           string                `json:"home"`
	RecommendedABI string                `json:"recommendedAbi"`
	Emulator       bool                  `json:"emulator"`
	PlatformTools  bool                  `json:"platformTools"`
	Tools          *android.ToolManifest `json:"tools,omitempty"`
	Acceleration   string                `json:"acceleration"`
	AccelOK        bool                  `json:"accelOk"`
	FreeDisk       uint64                `json:"freeDisk"`
	CatalogSource  string                `json:"catalogSource"`
	CatalogUpdated time.Time             `json:"catalogUpdated"`
	Images         int                   `json:"images"`
	Installed      int                   `json:"installedImages"`
	AVDs           int                   `json:"avds"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	images, updated, source := s.CatalogSnapshot()
	installed, _ := s.Paths.Installed()
	avds, _ := s.AVDs.List()
	accel, ok := acceleration()
	var tools *android.ToolManifest
	if m, err := android.ReadManifest(s.Paths); err == nil {
		tools = &m
	}
	free, _ := download.FreeSpace(s.Paths.Home)
	host, _ := os.Hostname()
	writeJSON(w, http.StatusOK, statusResponse{
		Version: s.Version, OS: runtime.GOOS, Arch: runtime.GOARCH, Host: host,
		Home: s.Paths.Home, RecommendedABI: catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH),
		Emulator: s.Paths.HasEmulator(), PlatformTools: s.Paths.HasPlatformTools(),
		Tools: tools, Acceleration: accel, AccelOK: ok, FreeDisk: free,
		CatalogSource: source, CatalogUpdated: updated, Images: len(images),
		Installed: len(installed), AVDs: len(avds),
	})
}

func acceleration() (string, bool) {
	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/dev/kvm"); err == nil {
			return "KVM", true
		}
		return "software (TCG)", false
	case "darwin":
		return "Hypervisor.framework", true
	case "windows":
		return "Windows Hypervisor Platform", true
	default:
		return "software (TCG)", false
	}
}

type imageView struct {
	catalog.Image
	TagName     string `json:"tagName"`
	Installed   bool   `json:"installed"`
	Recommended bool   `json:"recommended"`
}

type releaseView struct {
	API      int         `json:"api"`
	Version  string      `json:"version"`
	Codename string      `json:"codename"`
	Images   []imageView `json:"images"`
}

type catalogResponse struct {
	Updated        time.Time     `json:"updated"`
	Source         string        `json:"source"`
	RecommendedABI string        `json:"recommendedAbi"`
	MinAPI         int           `json:"minApi"`
	MaxAPI         int           `json:"maxApi"`
	Releases       []releaseView `json:"releases"`
}

func (s *Server) catalogResponse() catalogResponse {
	images, updated, source := s.CatalogSnapshot()
	installed := s.Paths.InstalledMap()
	recommended := catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH)
	groups := catalog.Group(images)
	out := catalogResponse{
		Updated: updated, Source: source, RecommendedABI: recommended,
		MinAPI: catalog.MinAPI, MaxAPI: catalog.MaxAPI,
	}
	for _, g := range groups {
		rv := releaseView{API: g.API, Version: g.Version, Codename: g.Codename}
		for _, img := range g.Images {
			_, isInstalled := installed[img.ID]
			rv.Images = append(rv.Images, imageView{
				Image: img, TagName: catalog.TagName(img.Tag),
				Installed: isInstalled, Recommended: img.ABI == recommended,
			})
		}
		out.Releases = append(out.Releases, rv)
	}
	return out
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	writeJSON(w, http.StatusOK, s.catalogResponse())
}

func (s *Server) handleCatalogRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	job := s.RefreshInBackground("Refresh Android image catalog")
	if job == nil {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "already running"})
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var body struct {
		Force bool `json:"force"`
	}
	if r.ContentLength > 0 && !decodeBody(w, r, &body) {
		return
	}
	for _, j := range s.Jobs.List(20) {
		if j.Kind == "setup" && j.State == "running" {
			writeJSON(w, http.StatusAccepted, j)
			return
		}
	}
	job := s.Jobs.Start("setup", "Install Android emulator and platform-tools", func(ctx context.Context, job *Job) (interface{}, error) {
		m, err := android.Setup(ctx, s.Paths, s.Client, body.Force, s.Jobs.Report(job.ID))
		if err != nil {
			return nil, err
		}
		s.Jobs.Update(job.ID, "ready", 1, 1)
		return m, nil
	})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	images, _, _ := s.CatalogSnapshot()
	img, ok := catalog.Find(images, body.ID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown image id")
		return
	}
	if _, err := os.Stat(filepath.Join(s.Paths.SystemDir(img), "system.img")); err == nil {
		writeError(w, http.StatusConflict, "this image is already installed")
		return
	}
	for _, j := range s.Jobs.List(30) {
		if j.Kind == "install" && j.State == "running" && j.Label == imageLabel(img) {
			writeJSON(w, http.StatusAccepted, j)
			return
		}
	}
	label := imageLabel(img)
	job := s.Jobs.Start("install", label, func(ctx context.Context, job *Job) (interface{}, error) {
		if err := s.installImage(ctx, img, s.Jobs.Report(job.ID)); err != nil {
			return nil, err
		}
		s.Jobs.Update(job.ID, "ready", 1, 1)
		return img, nil
	})
	writeJSON(w, http.StatusAccepted, job)
}

func imageLabel(img catalog.Image) string {
	return fmt.Sprintf("Android %s · %s · %s", img.Version, catalog.TagName(img.Tag), img.ABI)
}

func (s *Server) installImage(ctx context.Context, img catalog.Image, report download.Reporter) error {
	return android.InstallImage(ctx, s.Paths, s.Client, img, report)
}

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "DELETE required")
		return
	}
	id := r.URL.Query().Get("id")
	images, _, _ := s.CatalogSnapshot()
	img, ok := catalog.Find(images, id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown image id")
		return
	}
	if err := s.Paths.RemoveImage(img); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

type libraryResponse struct {
	Images []store.InstalledImage `json:"images"`
	AVDs   []android.AVD          `json:"avds"`
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	images, err := s.Paths.Installed()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	avds, err := s.AVDs.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if images == nil {
		images = []store.InstalledImage{}
	}
	if avds == nil {
		avds = []android.AVD{}
	}
	writeJSON(w, http.StatusOK, libraryResponse{Images: images, AVDs: avds})
}

func (s *Server) handleAVD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		avds, err := s.AVDs.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if avds == nil {
			avds = []android.AVD{}
		}
		writeJSON(w, http.StatusOK, avds)
	case http.MethodPost:
		var body struct {
			ImageID string `json:"imageId"`
			Name    string `json:"name"`
			RAMMB   int    `json:"ramMb"`
			Cores   int    `json:"cores"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			DPI     int    `json:"dpi"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		images, _, _ := s.CatalogSnapshot()
		img, ok := catalog.Find(images, body.ImageID)
		if !ok {
			writeError(w, http.StatusNotFound, "unknown image id")
			return
		}
		avd, err := s.AVDs.Create(img, android.CreateOptions{
			Name: avdName(body.Name, img), RAMMB: body.RAMMB, Cores: body.Cores,
			Width: body.Width, Height: body.Height, DPI: body.DPI,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, avd)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func avdName(name string, img catalog.Image) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fmt.Sprintf("VOM-Android-%s-%s-%s", img.Version, img.Tag, img.ABI)
}

func (s *Server) handleAVDStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Headless bool   `json:"headless"`
		Wipe     bool   `json:"wipe"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	avd, err := s.AVDs.Start(body.Name, body.Headless, body.Wipe)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, avd)
}

func (s *Server) handleAVDStop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := s.AVDs.Stop(body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"stopped": body.Name})
}

func (s *Server) handleAVDDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := s.AVDs.Delete(body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": body.Name})
}

func (s *Server) handleAVDLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	text, err := s.AVDs.LogTail(name, 96*1024)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(text))
}

func (s *Server) handleAVDScreen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	width := 540
	if v := r.URL.Query().Get("w"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 120 || n > 2160 {
			writeError(w, http.StatusBadRequest, "w must be between 120 and 2160")
			return
		}
		width = n
	}
	screen, err := s.AVDs.Screenshot(r.Context(), name, width)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Screen-Width", strconv.Itoa(screen.Width))
	w.Header().Set("X-Screen-Height", strconv.Itoa(screen.Height))
	w.WriteHeader(http.StatusOK)
	w.Write(screen.PNG)
}

func (s *Server) handleAVDInput(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Action   string `json:"action"`
		X        int    `json:"x"`
		Y        int    `json:"y"`
		X2       int    `json:"x2"`
		Y2       int    `json:"y2"`
		Duration int    `json:"duration"`
		KeyCode  int    `json:"keyCode"`
		Text     string `json:"text"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	var err error
	switch body.Action {
	case "tap":
		err = s.AVDs.Tap(r.Context(), body.Name, body.X, body.Y)
	case "swipe":
		err = s.AVDs.Swipe(r.Context(), body.Name, body.X, body.Y, body.X2, body.Y2, body.Duration)
	case "key":
		err = s.AVDs.Key(r.Context(), body.Name, body.KeyCode)
	case "text":
		err = s.AVDs.Text(r.Context(), body.Name, body.Text)
	default:
		writeError(w, http.StatusBadRequest, "action must be tap, swipe, key or text")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	writeJSON(w, http.StatusOK, s.Jobs.List(30))
}

func (s *Server) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !s.Jobs.Cancel(body.ID) {
		writeError(w, http.StatusNotFound, "unknown job")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"canceled": body.ID})
}

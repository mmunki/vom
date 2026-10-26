package android

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mmunki/vom/internal/catalog"
	"github.com/mmunki/vom/internal/store"
)

// CreateOptions controls a new AVD.
type CreateOptions struct {
	Name   string `json:"name"`
	RAMMB  int    `json:"ramMb"`
	Cores  int    `json:"cores"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	DPI    int    `json:"dpi"`
}

// AVD is one virtual device.
type AVD struct {
	Name      string    `json:"name"`
	ImageID   string    `json:"imageId"`
	API       int       `json:"api"`
	APILevel  string    `json:"apiLevel"`
	Version   string    `json:"version"`
	Tag       string    `json:"tag"`
	ABI       string    `json:"abi"`
	Dir       string    `json:"dir"`
	CreatedAt time.Time `json:"createdAt"`
	RAMMB     int       `json:"ramMb"`
	Cores     int       `json:"cores"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	DPI       int       `json:"dpi"`
	Running   bool      `json:"running"`
	Headless  bool      `json:"headless"`
	PID       int       `json:"pid"`
	Log       string    `json:"log"`
}

type runningProc struct {
	cmd      *exec.Cmd
	done     chan struct{}
	logFile  *os.File
	headless bool
	stopping bool
}

// Manager owns AVD creation, process launch and logs.
type Manager struct {
	Paths *store.Paths
	mu    sync.Mutex
	procs map[string]*runningProc
}

// NewManager creates an AVD manager for a data directory.
func NewManager(p *store.Paths) *Manager {
	return &Manager{Paths: p, procs: map[string]*runningProc{}}
}

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

func metaPath(dir string) string { return filepath.Join(dir, "vom-avd.json") }

func (m *Manager) avdDir(name string) string { return filepath.Join(m.Paths.AVD, name+".avd") }

func (m *Manager) pidPath(name string) string { return filepath.Join(m.avdDir(name), "emulator.pid") }

// DefaultName proposes a name for an image.
func DefaultName(img catalog.Image) string {
	return fmt.Sprintf("VOM-Android-%s-%s-%s", img.Version, img.Tag, img.ABI)
}

// Get returns one AVD.
func (m *Manager) Get(name string) (AVD, bool) {
	var avd AVD
	b, err := os.ReadFile(metaPath(m.avdDir(name)))
	if err != nil || json.Unmarshal(b, &avd) != nil {
		return AVD{}, false
	}
	m.decorate(&avd)
	return avd, true
}

// List returns all AVDs, newest first.
func (m *Manager) List() ([]AVD, error) {
	entries, err := os.ReadDir(m.Paths.AVD)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []AVD
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), ".avd") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".avd")
		avd, ok := m.Get(name)
		if ok {
			out = append(out, avd)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].API != out[j].API {
			return out[i].API > out[j].API
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (m *Manager) decorate(a *AVD) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[a.Name]; ok {
		a.Running = true
		a.PID = p.cmd.Process.Pid
		a.Headless = p.headless
	} else if pid, ok := m.readPID(a.Name); ok && processAlive(pid) {
		a.Running = true
		a.PID = pid
	}
	a.Log = filepath.Join(m.Paths.Logs, a.Name+".log")
}

func (m *Manager) defaultCreateOptions(img catalog.Image, opts CreateOptions) CreateOptions {
	if opts.Name == "" {
		opts.Name = DefaultName(img)
	}
	if opts.RAMMB <= 0 {
		opts.RAMMB = 4096
	}
	if opts.Cores <= 0 {
		opts.Cores = 4
	}
	if opts.Width <= 0 {
		opts.Width = 1080
	}
	if opts.Height <= 0 {
		opts.Height = 1920
	}
	if opts.DPI <= 0 {
		opts.DPI = 420
	}
	return opts
}

// Create writes a complete AVD configuration for an installed image.
func (m *Manager) Create(img catalog.Image, opts CreateOptions) (AVD, error) {
	opts = m.defaultCreateOptions(img, opts)
	name := strings.TrimSpace(opts.Name)
	if !validName.MatchString(name) {
		return AVD{}, fmt.Errorf("invalid AVD name %q: use letters, digits, dot, dash or underscore (no spaces)", name)
	}
	if _, ok := m.Get(name); ok {
		return AVD{}, fmt.Errorf("an AVD named %q already exists", name)
	}
	sysDir := m.Paths.SystemDir(img)
	if _, err := os.Stat(filepath.Join(sysDir, "system.img")); err != nil {
		return AVD{}, fmt.Errorf("system image %s is not installed", img.ID)
	}
	dir := m.avdDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return AVD{}, err
	}
	created := time.Now().UTC()
	avd := AVD{
		Name: name, ImageID: img.ID, API: img.API, APILevel: img.APILevel,
		Version: img.Version, Tag: img.Tag, ABI: img.ABI, Dir: dir,
		CreatedAt: created, RAMMB: opts.RAMMB, Cores: opts.Cores,
		Width: opts.Width, Height: opts.Height, DPI: opts.DPI,
		Log: filepath.Join(m.Paths.Logs, name+".log"),
	}
	if err := writeAVDFiles(m.Paths, avd, sysDir); err != nil {
		return AVD{}, err
	}
	b, _ := json.MarshalIndent(avd, "", " ")
	if err := os.WriteFile(metaPath(dir), b, 0o644); err != nil {
		return AVD{}, err
	}
	return avd, nil
}

func cpuArch(abi string) string {
	switch abi {
	case "arm64-v8a":
		return "arm64"
	case "armeabi-v7a":
		return "arm"
	default:
		return abi
	}
}

func writeAVDFiles(p *store.Paths, a AVD, sysDir string) error {
	config := fmt.Sprintf(`avd.ini.encoding=UTF-8
AvdId=%s
avd.ini.displayname=%s
abi.type=%s
hw.cpu.arch=%s
hw.cpu.ncore=%d
hw.ramSize=%d
hw.lcd.width=%d
hw.lcd.height=%d
hw.lcd.density=%d
hw.lcd.backlight=yes
hw.gpu.enabled=yes
hw.gpu.mode=auto
hw.keyboard=yes
hw.mainKeys=no
hw.audioInput=yes
hw.audioOutput=yes
hw.camera.back=virtualscene
hw.camera.front=emulated
hw.sdCard=yes
hw.initialOrientation=Portrait
image.sysdir.1=%s/
tag.id=%s
tag.display=%s
PlayStore.enabled=%t
disk.dataPartition.size=8G
vm.heapSize=512
fastboot.forceColdBoot=yes
`, a.Name, a.Name, a.ABI, cpuArch(a.ABI), a.Cores, a.RAMMB,
		a.Width, a.Height, a.DPI, filepath.ToSlash(sysDir), a.Tag,
		catalog.TagName(a.Tag), a.Tag == "google_apis_playstore")

	if err := os.WriteFile(filepath.Join(a.Dir, "config.ini"), []byte(config), 0o644); err != nil {
		return err
	}
	ini := fmt.Sprintf(`avd.ini.encoding=UTF-8
path=%s
path.rel=avd/%s.avd
target=android-%s
`, filepath.ToSlash(a.Dir), a.Name, a.APILevel)
	return os.WriteFile(filepath.Join(p.AVD, a.Name+".ini"), []byte(ini), 0o644)
}

// Start launches the emulator for an AVD.
func (m *Manager) Start(name string, headless, wipe bool) (AVD, error) {
	avd, ok := m.Get(name)
	if !ok {
		return AVD{}, fmt.Errorf("unknown AVD %q", name)
	}
	m.mu.Lock()
	if _, running := m.procs[name]; running {
		m.mu.Unlock()
		return AVD{}, fmt.Errorf("AVD %q is already running", name)
	}
	m.mu.Unlock()
	if !m.Paths.HasEmulator() {
		return AVD{}, fmt.Errorf("the emulator is not installed yet; run setup first")
	}
	if _, err := os.Stat(filepath.Join(m.Paths.SystemDir(catalog.Image{APILevel: avd.APILevel, Tag: avd.Tag, ABI: avd.ABI}), "system.img")); err != nil {
		// Fall back to the stored directory in config.ini.
		if _, err2 := os.Stat(filepath.Join(avd.Dir, "config.ini")); err2 != nil {
			return AVD{}, fmt.Errorf("the system image of %q is missing", name)
		}
	}
	if err := os.MkdirAll(m.Paths.Logs, 0o755); err != nil {
		return AVD{}, err
	}
	logPath := filepath.Join(m.Paths.Logs, name+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return AVD{}, err
	}
	fmt.Fprintf(logFile, "\n===== VOM start %s =====\n", time.Now().Format(time.RFC3339))

	args := []string{
		"-avd", name,
		"-no-snapshot-save",
		"-no-boot-anim",
		"-no-metrics",
		"-netdelay", "none",
		"-netspeed", "full",
		// Old system images (API 24-27) panic in goldfish_dma_mmap
		// when host DMA is enabled; software rendering is stable.
		"-feature", "-GLDMA",
	}
	if headless {
		args = append(args, "-no-window", "-no-audio", "-gpu", "swiftshader_indirect")
	} else {
		args = append(args, "-gpu", "auto")
	}
	if wipe {
		args = append(args, "-wipe-data")
	}
	cmd := exec.Command(m.Paths.EmulatorBinary(), args...)
	cmd.Dir = m.Paths.Home
	cmd.Env = append(os.Environ(),
		"ANDROID_HOME="+m.Paths.SDK,
		"ANDROID_SDK_ROOT="+m.Paths.SDK,
		"ANDROID_AVD_HOME="+m.Paths.AVD,
		"ANDROID_EMULATOR_HOME="+filepath.Join(m.Paths.Home, ".android"),
		"ANDROID_USER_HOME="+filepath.Join(m.Paths.Home, ".android"),
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return AVD{}, fmt.Errorf("start emulator: %w", err)
	}
	_ = os.WriteFile(m.pidPath(name), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	proc := &runningProc{cmd: cmd, done: make(chan struct{}), logFile: logFile, headless: headless}
	m.mu.Lock()
	m.procs[name] = proc
	m.mu.Unlock()
	go func() {
		err := cmd.Wait()
		_ = os.Remove(m.pidPath(name))
		fmt.Fprintf(logFile, "\n===== VOM stopped %s (err=%v) =====\n", time.Now().Format(time.RFC3339), err)
		logFile.Close()
		m.mu.Lock()
		delete(m.procs, name)
		m.mu.Unlock()
		close(proc.done)
	}()
	avd.Running = true
	avd.Headless = headless
	avd.PID = cmd.Process.Pid
	avd.Log = logPath
	return avd, nil
}

// Stop terminates the emulator process of an AVD.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	proc, ok := m.procs[name]
	if ok {
		proc.stopping = true
	}
	m.mu.Unlock()
	if !ok {
		pid, found := m.readPID(name)
		if !found || !processAlive(pid) {
			_ = os.Remove(m.pidPath(name))
			return fmt.Errorf("AVD %q is not running", name)
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("stop AVD %q: %w", name, err)
		}
		if runtime.GOOS != "windows" {
			_ = p.Signal(os.Interrupt)
			for i := 0; i < 60; i++ {
				if !processAlive(pid) {
					_ = os.Remove(m.pidPath(name))
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		if err := p.Kill(); err != nil && processAlive(pid) {
			return fmt.Errorf("stop AVD %q: %w", name, err)
		}
		_ = os.Remove(m.pidPath(name))
		return nil
	}
	if runtime.GOOS != "windows" {
		_ = proc.cmd.Process.Signal(os.Interrupt)
	} else {
		_ = proc.cmd.Process.Kill()
	}
	select {
	case <-proc.done:
		return nil
	case <-time.After(6 * time.Second):
		if err := proc.cmd.Process.Kill(); err != nil {
			return err
		}
		select {
		case <-proc.done:
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("emulator %q did not stop", name)
		}
	}
}

func (m *Manager) readPID(name string) (int, bool) {
	b, err := os.ReadFile(m.pidPath(name))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// Delete stops and removes an AVD.
func (m *Manager) Delete(name string) error {
	if _, ok := m.Get(name); !ok {
		return fmt.Errorf("unknown AVD %q", name)
	}
	_ = m.Stop(name)
	dir := filepath.Clean(m.avdDir(name))
	root := filepath.Clean(m.Paths.AVD) + string(os.PathSeparator)
	if !strings.HasPrefix(dir, root) {
		return fmt.Errorf("refusing to delete %s", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	os.Remove(filepath.Join(m.Paths.AVD, name+".ini"))
	os.Remove(filepath.Join(m.Paths.Logs, name+".log"))
	return nil
}

// LogTail returns the last n bytes of an AVD log.
func (m *Manager) LogTail(name string, n int64) (string, error) {
	path := filepath.Join(m.Paths.Logs, name+".log")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if n <= 0 || n > st.Size() {
		n = st.Size()
	}
	if _, err := f.Seek(st.Size()-n, 0); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := f.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

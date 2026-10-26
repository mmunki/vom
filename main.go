// VOM is a local Android virtual-machine manager. It downloads the official
// Google system images for Android 7.0 through 17, creates AVDs and launches
// the official Android emulator. The Material 3 interface runs in the browser.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mmunki/vom/internal/android"
	"github.com/mmunki/vom/internal/catalog"
	"github.com/mmunki/vom/internal/download"
	"github.com/mmunki/vom/internal/server"
	"github.com/mmunki/vom/internal/store"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "serve":
			runServe(args[1:])
			return
		case "setup":
			runSetup(args[1:])
			return
		case "versions":
			runVersions(args[1:])
			return
		case "install":
			runInstall(args[1:])
			return
		case "run":
			runRun(args[1:])
			return
		case "list", "ls":
			runList(args[1:])
			return
		case "doctor":
			runDoctor(args[1:])
			return
		case "stop":
			runStop(args[1:])
			return
		case "help", "-h", "--help":
			usage()
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
			usage()
			os.Exit(2)
		}
	}
	runServe(args)
}

func usage() {
	fmt.Fprint(os.Stderr, `VOM - Android virtual machines with Material 3

  vom                       start the local web app
  vom serve [--listen ADDR] [--no-open]
  vom setup [--force]       install the official emulator + platform-tools
  vom versions [--refresh]  list Android 7.0 - 17 images
  vom install API [--tag google_apis] [--abi x86_64] [--id ID]
  vom run API|AVD [--tag T] [--abi A] [--headless] [--wipe] [--wait]
  vom list                  installed images and AVDs
  vom doctor                host and acceleration checks
  vom stop NAME             stop an emulator

Global: --home DIR  (default: `+store.DefaultHome()+`)
`)
}

func newSet(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	home := fs.String("home", store.DefaultHome(), "data directory")
	return fs, home
}

// parseArgs accepts flags before and after positional arguments. The standard
// flag package stops at the first positional argument, which makes commands
// like "vom install 24 --tag google_apis" surprising.
func parseArgs(fs *flag.FlagSet, args []string) {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			continue
		}
		f := fs.Lookup(name)
		if f == nil || isBoolFlag(f) {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	fs.Parse(append(flags, positional...))
}

func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface{ IsBoolFlag() bool }
	b, ok := f.Value.(boolFlag)
	return ok && b.IsBoolFlag()
}

func runServe(args []string) {
	fs, home := newSet("serve")
	listen := fs.String("listen", "127.0.0.1:8765", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open the browser")
	refresh := fs.Bool("refresh", false, "refresh the catalog before starting")
	parseArgs(fs, args)

	p := store.New(*home)
	srv, err := server.New(p, version)
	if err != nil {
		fatal(err)
	}
	images, updated, source := srv.CatalogSnapshot()
	if *refresh || source == "embedded" || catalog.NeedsRefresh(updated) {
		srv.RefreshInBackground("Refresh Android image catalog")
	}
	if len(images) == 0 {
		fatal(errors.New("the image catalog is empty"))
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fatal(err)
	}
	url := "http://" + displayAddr(ln.Addr().String())
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(err)
		}
	}()

	fmt.Printf("VOM %s\n", version)
	fmt.Printf("  interface: %s\n", url)
	fmt.Printf("  data:      %s\n", p.Home)
	fmt.Printf("  images:    Android %d-%d, %s recommended\n", catalog.MinAPI, catalog.MaxAPI,
		catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH))
	fmt.Println("  press Ctrl+C to stop")
	if !*noOpen {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(url)
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdown)
}

func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func loadCatalog(ctx context.Context, p *store.Paths, force bool) ([]catalog.Image, error) {
	images, updated, source, err := catalog.Load(p.Home)
	if err != nil {
		return nil, err
	}
	if force || source == "embedded" || catalog.NeedsRefresh(updated) {
		refreshed, rerr := catalog.Refresh(ctx, p.Home, download.Client())
		if rerr == nil {
			return refreshed, nil
		}
		if force || len(images) == 0 {
			return nil, rerr
		}
		fmt.Fprintf(os.Stderr, "note: using the embedded catalog (%v)\n", rerr)
	}
	return images, nil
}

func progressPrinter(label string) download.Reporter {
	lastStage := ""
	lastPct := -1
	return func(stage string, done, total int64) {
		pct := -1
		if total > 0 {
			pct = int(done * 100 / total)
		}
		if stage != lastStage {
			fmt.Fprintf(os.Stderr, "\n%s: %s", label, stage)
			lastStage = stage
			lastPct = -1
		}
		if stage == "downloading" && pct >= 0 && pct/5 != lastPct/5 {
			fmt.Fprintf(os.Stderr, " %d%% (%s/%s)", pct, store.HumanBytes(done), store.HumanBytes(total))
			lastPct = pct
		}
		if stage == "installed" || stage == "installed\n" {
			fmt.Fprintln(os.Stderr)
		}
	}
}

func runSetup(args []string) {
	fs, home := newSet("setup")
	force := fs.Bool("force", false, "download again even when installed")
	parseArgs(fs, args)
	p := store.New(*home)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	m, err := android.Setup(ctx, p, download.Client(), *force, progressPrinter("Android emulator"))
	if err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Printf("emulator %s installed in %s\n", m.Emulator.Revision, p.Emulator)
	fmt.Printf("platform-tools %s installed in %s\n", m.Platform.Revision, p.PlatformTools)
}

func runVersions(args []string) {
	fs, home := newSet("versions")
	refresh := fs.Bool("refresh", false, "download the latest repository index")
	parseArgs(fs, args)
	p := store.New(*home)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	images, err := loadCatalog(ctx, p, *refresh)
	if err != nil {
		fatal(err)
	}
	installed := p.InstalledMap()
	abi := catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Android %d-%d  (recommended ABI: %s)\n\n", catalog.MinAPI, catalog.MaxAPI, abi)
	for _, r := range catalog.Group(images) {
		fmt.Printf("Android %s  API %d%s\n", r.Version, r.API, codenameSuffix(r.Codename))
		for _, img := range r.Images {
			mark := " "
			if _, ok := installed[img.ID]; ok {
				mark = "*"
			}
			rec := ""
			if img.ABI == abi {
				rec = "  recommended"
			}
			fmt.Printf("  %s %-22s %-12s %8s  %s%s\n", mark, catalog.TagName(img.Tag), img.ABI,
				store.HumanBytes(img.Size), img.ID, rec)
		}
		fmt.Println()
	}
	fmt.Println("* = installed")
}

func codenameSuffix(c string) string {
	if c == "" {
		return ""
	}
	return "  " + c
}

func runInstall(args []string) {
	fs, home := newSet("install")
	tag := fs.String("tag", "", "image tag: default, google_apis, google_apis_playstore")
	abi := fs.String("abi", "", "ABI: x86_64, arm64-v8a, x86")
	id := fs.String("id", "", "exact image id from 'vom versions'")
	refresh := fs.Bool("refresh", false, "refresh the catalog first")
	parseArgs(fs, args)
	if *id == "" && fs.NArg() < 1 {
		fatal(errors.New("usage: vom install API [--tag TAG] [--abi ABI] or --id ID"))
	}
	p := store.New(*home)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	images, err := loadCatalog(ctx, p, *refresh)
	if err != nil {
		fatal(err)
	}
	img, err := resolveImage(images, fs.Arg(0), *id, *tag, *abi)
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(p.SystemDir(img) + string(os.PathSeparator) + "system.img"); err == nil {
		fmt.Printf("Android %s (%s, %s) is already installed in %s\n", img.Version, img.Tag, img.ABI, p.SystemDir(img))
		return
	}
	if err := android.InstallImage(ctx, p, download.Client(), img, progressPrinter("Android "+img.Version)); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Printf("installed %s in %s\n", img.ID, p.SystemDir(img))
}

func resolveImage(images []catalog.Image, arg, id, tag, abi string) (catalog.Image, error) {
	if id != "" {
		if img, ok := catalog.Find(images, id); ok {
			return img, nil
		}
		return catalog.Image{}, fmt.Errorf("unknown image id %q", id)
	}
	api, err := strconv.Atoi(arg)
	if err != nil {
		return catalog.Image{}, fmt.Errorf("API must be a number, got %q", arg)
	}
	if api < catalog.MinAPI || api > catalog.MaxAPI {
		return catalog.Image{}, fmt.Errorf("API %d is outside Android %d-%d", api, catalog.MinAPI, catalog.MaxAPI)
	}
	if abi == "" {
		abi = catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH)
	}
	if img, ok := catalog.Pick(images, api, tag, abi); ok {
		return img, nil
	}
	return catalog.Image{}, fmt.Errorf("no image for API %d tag %q", api, tag)
}

// resolveRunImage prefers an already installed image when the caller did not
// pin a tag or ABI, so "vom run 24" uses what is on disk.
func resolveRunImage(p *store.Paths, images []catalog.Image, arg, tag, abi string) (catalog.Image, error) {
	api, err := strconv.Atoi(arg)
	if err != nil {
		return catalog.Image{}, fmt.Errorf("API must be a number, got %q", arg)
	}
	if tag == "" && abi == "" {
		installed, _ := p.Installed()
		var pick *catalog.Image
		for i := range installed {
			if installed[i].API != api {
				continue
			}
			if pick == nil || installed[i].ABI == catalog.RecommendedABI(runtime.GOOS, runtime.GOARCH) {
				img := installed[i].Image
				pick = &img
			}
		}
		if pick != nil {
			return *pick, nil
		}
	}
	return resolveImage(images, arg, "", tag, abi)
}

func runRun(args []string) {
	fs, home := newSet("run")
	tag := fs.String("tag", "", "image tag")
	abi := fs.String("abi", "", "ABI")
	name := fs.String("name", "", "AVD name")
	headless := fs.Bool("headless", false, "run without a window")
	wipe := fs.Bool("wipe", false, "wipe user data on start")
	wait := fs.Bool("wait", false, "wait until Android has booted")
	timeout := fs.Duration("timeout", 8*time.Minute, "boot timeout with --wait")
	refresh := fs.Bool("refresh", false, "refresh the catalog first")
	parseArgs(fs, args)
	if fs.NArg() < 1 {
		fatal(errors.New("usage: vom run API|AVD-NAME"))
	}
	p := store.New(*home)
	mgr := android.NewManager(p)
	arg := fs.Arg(0)

	var avd android.AVD
	if existing, ok := mgr.Get(arg); ok {
		avd = existing
	} else if *name != "" {
		if existing, ok := mgr.Get(*name); ok {
			avd = existing
		}
	}
	if avd.Name == "" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		images, err := loadCatalog(ctx, p, *refresh)
		if err != nil {
			fatal(err)
		}
		img, err := resolveRunImage(p, images, arg, *tag, *abi)
		if err != nil {
			fatal(err)
		}
		if _, err := os.Stat(p.SystemDir(img) + string(os.PathSeparator) + "system.img"); err != nil {
			fatal(fmt.Errorf("Android %s is not installed yet; run: vom install %d --tag %s --abi %s",
				img.Version, img.API, img.Tag, img.ABI))
		}
		avd, err = mgr.Create(img, android.CreateOptions{Name: *name})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("created AVD %q (%s, %s)\n", avd.Name, avd.ABI, avd.Version)
	}
	if !p.HasEmulator() {
		fatal(errors.New("the emulator is not installed yet; run: vom setup"))
	}
	started, err := mgr.Start(avd.Name, *headless, *wipe)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("started %q (pid %d)\n", started.Name, started.PID)
	fmt.Printf("log: %s\n", started.Log)
	if *headless {
		fmt.Printf("Android is running without a window; use adb to reach it.\n")
	}
	if *wait {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		fmt.Printf("waiting for Android to boot (up to %s)...\n", *timeout)
		serial, err := android.WaitForBoot(ctx, p, *timeout)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Android booted on %s\n", serial)
	}
}

func runList(args []string) {
	fs, home := newSet("list")
	parseArgs(fs, args)
	p := store.New(*home)
	images, err := p.Installed()
	if err != nil {
		fatal(err)
	}
	fmt.Println("Installed system images:")
	if len(images) == 0 {
		fmt.Println("  (none)")
	}
	for _, img := range images {
		fmt.Printf("  Android %-4s API %-4d %-22s %-10s %8s  %s\n", img.Version, img.API,
			catalog.TagName(img.Tag), img.ABI, store.HumanBytes(img.Bytes), img.ID)
	}
	avds, err := android.NewManager(p).List()
	if err != nil {
		fatal(err)
	}
	fmt.Println("\nVirtual devices:")
	if len(avds) == 0 {
		fmt.Println("  (none)")
	}
	for _, a := range avds {
		state := "stopped"
		if a.Running {
			state = fmt.Sprintf("running, pid %d", a.PID)
		}
		fmt.Printf("  %-38s Android %-4s %-10s %-14s %s\n", a.Name, a.Version, a.ABI, catalog.TagName(a.Tag), state)
	}
}

func runDoctor(args []string) {
	fs, home := newSet("doctor")
	parseArgs(fs, args)
	p := store.New(*home)
	accel, ok := "software (TCG)", false
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/kvm"); err == nil {
			accel, ok = "KVM", true
		}
	} else if runtime.GOOS == "darwin" {
		accel, ok = "Hypervisor.framework", true
	} else if runtime.GOOS == "windows" {
		accel, ok = "Windows Hypervisor Platform", true
	}
	free, _ := download.FreeSpace(p.Home)
	fmt.Printf("VOM             %s\n", version)
	fmt.Printf("host            %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("data directory  %s\n", p.Home)
	fmt.Printf("emulator        %v (%s)\n", p.HasEmulator(), p.Emulator)
	fmt.Printf("platform-tools  %v (%s)\n", p.HasPlatformTools(), p.ADBBinary())
	fmt.Printf("acceleration    %s (usable: %v)\n", accel, ok)
	if free > 0 {
		fmt.Printf("free disk       %s\n", store.HumanBytes(int64(free)))
	}
	images, _ := p.Installed()
	fmt.Printf("images          %d installed\n", len(images))
	avds, _ := android.NewManager(p).List()
	fmt.Printf("AVDs            %d\n", len(avds))
}

func runStop(args []string) {
	fs, home := newSet("stop")
	parseArgs(fs, args)
	if fs.NArg() < 1 {
		fatal(errors.New("usage: vom stop AVD-NAME"))
	}
	p := store.New(*home)
	if err := android.NewManager(p).Stop(fs.Arg(0)); err != nil {
		fatal(err)
	}
	fmt.Printf("stopped %q\n", fs.Arg(0))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

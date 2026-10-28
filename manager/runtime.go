package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RuntimeRecipe struct {
	Version     string `json:"version"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Format      string `json:"format"`
	Supported   bool   `json:"supported"`
	Requirement string `json:"requirement"`
}

func platform() string { return runtime.GOOS + "/" + runtime.GOARCH }
func runtimeRecipeForPlatform() RuntimeRecipe {
	r := RuntimeRecipe{Version: "0.83.0", Supported: true}
	switch runtime.GOOS {
	case "windows":
		r.URL = "https://github.com/dosbox-staging/dosbox-staging/releases/download/v0.83.0/dosbox-staging-windows-x64-v0.83.0.zip"
		r.SHA256 = "725b915e325a6d410ce30a10989fd492fdad07f6611fff40ff77f160478810f4"
		r.Format = "zip"
		r.Requirement = "64-bit Windows 10/11 target; Windows execution not tested here"
		if runtime.GOARCH != "amd64" {
			r.Supported = false
		}
	case "linux":
		r.URL = "https://github.com/dosbox-staging/dosbox-staging/releases/download/v0.83.0/dosbox-staging-linux-x86_64-v0.83.0.tar.xz"
		r.SHA256 = "d3a94f7f1c3e68a47ec88d61145506c7904452adb0c9c5928cb8cfe2331d6c5c"
		r.Format = "tar.xz"
		r.Requirement = "Modern x86-64 Linux, C/C++, ALSA and OpenGL system libraries"
		if runtime.GOARCH != "amd64" {
			r.Supported = false
		}
	case "darwin":
		r.URL = "https://github.com/dosbox-staging/dosbox-staging/releases/download/v0.83.0/dosbox-staging-macOS-v0.83.0.dmg"
		r.SHA256 = "d8a771adfb8010fa6b5f7fb5351abfba659273ad01c89f03675a92bdbdae8167"
		r.Format = "dmg"
		r.Requirement = "macOS 12+; Intel and Apple Silicon; Mac execution not tested here"
	default:
		r.Supported = false
	}
	return r
}
func trustedDownloadHost(h string) bool {
	return h == "github.com" || h == "release-assets.githubusercontent.com" || h == "objects.githubusercontent.com" || h == "moo2mod.com" || h == "www.moo2mod.com"
}
func downloadVerifiedWith(client *http.Client, src, dest, want string, max int64) error {
	// Caller controls transport for tests; production restricts every HTTPS redirect.
	resp, e := client.Get(src)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > max {
		return errors.New("download exceeds size limit")
	}
	f, e := os.CreateTemp(filepath.Dir(dest), ".download-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, max+1))
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if n > max {
		return errors.New("download exceeded limit")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return errors.New("truncated download")
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return errors.New("download checksum mismatch; cached package was not replaced")
	}
	return replaceFile(tmp, dest)
}
func downloadClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 8 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || !trustedDownloadHost(req.URL.Hostname()) {
			return errors.New("untrusted download redirect")
		}
		return nil
	}}
}
func downloadVerified(src, dest, want string, max int64) error {
	u, e := url.Parse(src)
	if e != nil || u.Scheme != "https" || !trustedDownloadHost(u.Hostname()) {
		return errors.New("only approved HTTPS distribution hosts may supply packages")
	}
	if e = noSymlinkAncestors(dest); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
		return e
	}
	return downloadVerifiedWith(downloadClient(), src, dest, want, max)
}
func (m *Manager) runtimeCandidates() []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		p, e := filepath.Abs(p)
		if e != nil {
			return
		}
		if i, e := os.Stat(p); e == nil && i.Mode().IsRegular() && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if m.Portable && runtime.GOOS == "windows" {
		// The project's pinned runtime has precedence over global/user installs.
		add(m.harnessRuntimePath())
	}
	add(m.getSettings().RuntimePath)
	managed := filepath.Join(m.Data, "runtime")
	_ = filepath.WalkDir(managed, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() && (strings.EqualFold(d.Name(), "dosbox.exe") || d.Name() == "dosbox") {
			add(p)
		}
		return nil
	})
	for _, n := range []string{"dosbox-staging", "dosbox"} {
		if p, e := exec.LookPath(n); e == nil {
			add(p)
		}
	}
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), `C:\GOG Games`, `C:\Games`} {
			if root == "" {
				continue
			}
			patterns := []string{filepath.Join(root, "Steam", "steamapps", "common", "Master of Orion 2", "DOSBOX", "DOSBox.exe"), filepath.Join(root, "Steam", "steamapps", "common", "Master of Orion 2", "DOSBOX", "dosbox.exe"), filepath.Join(root, "DOSBox-*", "DOSBox.exe"), filepath.Join(root, "DOSBox Staging", "dosbox.exe"), filepath.Join(root, "Master of Orion 2", "DOSBOX", "DOSBox.exe")}
			for _, p := range patterns {
				matches, _ := filepath.Glob(p)
				for _, x := range matches {
					add(x)
				}
			}
		}
	}
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		for _, r := range []string{"/Applications", filepath.Join(home, "Applications")} {
			add(filepath.Join(r, "DOSBox Staging.app", "Contents", "MacOS", "dosbox"))
		}
	}
	return out
}
func (m *Manager) installRuntime() (any, error) {
	if m.Portable && runtime.GOOS == "windows" && exists(m.harnessRuntimePath()) {
		exe, err := m.localHarnessRuntime()
		if err != nil {
			return nil, err
		}
		return map[string]string{"runtime": exe, "version": "0.83.0", "verification": "existing exact supplied harness file manifest"}, nil
	}

	r := runtimeRecipeForPlatform()
	if !r.Supported {
		return nil, errors.New("no packaged runtime recipe for this architecture")
	}
	archive := filepath.Join(m.Data, "cache", "dosbox-"+runtime.GOOS+"-"+r.Version+"."+r.Format)
	if e := requireHash(archive, r.SHA256); e != nil {
		m.progress("Downloading official DOSBox Staging; verifying the published SHA-256 before extraction…")
		if e = downloadVerified(r.URL, archive, r.SHA256, 512*1024*1024); e != nil {
			return nil, e
		}
	}
	parent := filepath.Join(m.Data, "runtime")
	if m.Portable && runtime.GOOS == "windows" {
		parent = filepath.Join(m.Root, "runtime")
	}
	if e := os.MkdirAll(parent, 0700); e != nil {
		return nil, e
	}
	stage, e := os.MkdirTemp(parent, "stage-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	extracted := filepath.Join(stage, "files")
	if e = os.Mkdir(extracted, 0700); e != nil {
		return nil, e
	}
	m.progress("Extracting the verified official runtime; no system-wide changes…")
	switch r.Format {
	case "zip":
		e = extractZip(archive, extracted, "", false)
	case "tar.xz":
		// Extraction is restricted to a checksum-pinned official archive, never an arbitrary uploaded tar.
		var b []byte
		b, e = exec.Command("tar", "-xJf", archive, "-C", extracted, "--no-same-owner", "--no-same-permissions").CombinedOutput()
		if e != nil {
			e = fmt.Errorf("tar extraction failed: %s: %w", b, e)
		}
	case "dmg":
		mount := filepath.Join(stage, "mount")
		if e = os.Mkdir(mount, 0700); e != nil {
			return nil, e
		}
		b, err := exec.Command("hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", mount, archive).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("mount official runtime: %s: %w", b, err)
		}
		apps, _ := filepath.Glob(filepath.Join(mount, "*.app"))
		if len(apps) != 1 {
			_ = exec.Command("hdiutil", "detach", mount).Run()
			return nil, errors.New("expected exactly one runtime app in disk image")
		}
		b, e = exec.Command("ditto", apps[0], filepath.Join(extracted, filepath.Base(apps[0]))).CombinedOutput()
		de := exec.Command("hdiutil", "detach", mount).Run()
		if e == nil && de != nil {
			e = de
		}
		if e != nil {
			return nil, fmt.Errorf("copy runtime: %s: %w", b, e)
		}
	}
	if e != nil {
		return nil, e
	}
	exe := ""
	_ = filepath.WalkDir(extracted, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && d.Type()&os.ModeSymlink == 0 && (strings.EqualFold(d.Name(), "dosbox.exe") || d.Name() == "dosbox") {
			exe = p
		}
		return nil
	})
	if exe == "" {
		return nil, errors.New("verified runtime archive did not contain the expected executable")
	}
	if runtime.GOOS != "windows" {
		if e = os.Chmod(exe, 0700); e != nil {
			return nil, e
		}
	}
	if m.Portable && runtime.GOOS == "windows" {
		sourceDir := filepath.Dir(exe)
		if e = verifyHarnessRuntime(sourceDir); e != nil {
			return nil, e
		}
		dest := filepath.Join(parent, "windows")
		if exists(dest) {
			return nil, errors.New("portable runtime already exists; verification failure requires restoring the supplied harness, not silently replacing it")
		}
		if e = os.Rename(sourceDir, dest); e != nil {
			return nil, e
		}
		exe = filepath.Join(dest, "dosbox.exe")
		if e = atomicJSON(filepath.Join(m.Data, "settings.json"), Settings{exe}); e != nil {
			return nil, e
		}
		return map[string]string{"runtime": exe, "version": r.Version, "verification": "exact supplied harness file manifest"}, nil
	}
	rel, _ := filepath.Rel(extracted, exe)
	dest := filepath.Join(parent, "staging-"+r.Version+"-"+fmt.Sprint(time.Now().UnixNano()))
	if e = os.Rename(extracted, dest); e != nil {
		return nil, e
	}
	exe = filepath.Join(dest, rel)
	if e = atomicJSON(filepath.Join(m.Data, "settings.json"), Settings{exe}); e != nil {
		return nil, e
	}
	return map[string]string{"runtime": exe, "version": r.Version, "archive_sha256": r.SHA256, "execution_test": "not performed by download"}, nil
}
func gameEntrypoint(p Profile) string {
	if p.Engine == "1.50.26" {
		return "ORION150.EXE"
	}
	return "ORION2.EXE"
}
func dosboxConfig(game string, p Profile) (string, error) {
	if e := validateProfile(p); e != nil {
		return "", e
	}
	game, e := filepath.Abs(game)
	if e != nil {
		return "", e
	}
	if strings.ContainsAny(game, "\"\r\n\x00") {
		return "", errors.New("game path cannot be represented safely")
	}
	base := harnessBaseConfig()
	if p.Role == "standalone" {
		return base, nil
	}
	// Networking is transport-specific while the in-game Create/Join role remains
	// MOO2's responsibility. Direct sessions start/connect to a local IPX tunnel;
	// relay services connect every participant to the same tunnel endpoint.
	lines := []string{base, "[ipx]", "ipx=true", "[autoexec]", "@echo off", `mount c "` + game + `"`, "c:", `cd \`}
	command, e := networkCommand(p)
	if e != nil {
		return "", e
	}
	lines = append(lines, command)
	lines = append(lines, gameEntrypoint(p), "exit", "")
	return strings.Join(lines, "\n"), nil
}
func dosboxArguments(cfgPath, game string, p Profile) []string {
	args := []string{"--noprimaryconf", "--conf", cfgPath}
	if p.Fullscreen {
		args = append(args, "--fullscreen")
	}
	if p.Role == "standalone" {
		args = append(args, filepath.Join(game, gameEntrypoint(p)))
	}
	return args
}
func (m *Manager) launch(p Profile) (any, error) {
	r, e := resolve(p)
	if e != nil {
		return nil, e
	}
	dir, _, e := m.activePath(p.ID)
	if e != nil {
		return nil, errors.New("prepare this profile before launching")
	}
	game := filepath.Join(dir, "game")
	kernel := inspectNetworkKernel(game, p.Engine)
	if e = atomicJSON(filepath.Join(m.Data, "logs", "last-network-preflight.json"), map[string]any{"launcher": m.applicationIdentity(), "profile": p.ID, "engine": p.Engine, "kernel": kernel}); e != nil {
		return nil, e
	}
	if !kernel.OK {
		return nil, fmt.Errorf("MOO2-SGC %s blocked launch: RKERNEL.COM is missing or invalid at %s (game directory: %s). Import the complete owned source or Network kernel ZIP, then Prepare / repair selected profile. No DOSBox process started", Version, kernel.KernelPath, game)
	}
	v, e := verifyDir(dir)
	if e != nil {
		return nil, e
	}
	if !v.OK {
		return nil, fmt.Errorf("verification failed; use Repair: %v", v.Bad)
	}
	var mf Manifest
	if e = readWorkspaceManifest(dir, &mf); e != nil {
		return nil, e
	}
	preparedResolution, e := resolve(mf.Profile)
	if e != nil {
		return nil, e
	}
	if r.Fingerprint != preparedResolution.Fingerprint {
		return nil, errors.New("selected engine/mods differ from the prepared environment; Prepare first")
	}
	candidates := m.runtimeCandidates()
	if len(candidates) == 0 {
		return nil, errors.New("DOSBox was not found. Install the verified runtime or select your existing executable")
	}
	exe := candidates[0]
	if m.Portable && runtime.GOOS == "windows" {
		// Do not fall back to an arbitrary installed DOSBox in portable mode.
		// A missing/tampered local runtime is a repairable error, not a reason
		// to discard the validated harness contract.
		exe, e = m.localHarnessRuntime()
		if e != nil {
			return nil, e
		}
	}
	cfg, e := dosboxConfig(game, p)
	if e != nil {
		return nil, e
	}
	cfgPath := filepath.Join(dir, "dosbox-manager.conf")
	if m.isPortableBaseline(p.ID) {
		if e = os.MkdirAll(filepath.Join(m.Root, "config"), 0700); e != nil {
			return nil, e
		}
		cfgPath = filepath.Join(m.Root, "config", "moo2.conf")
	}
	if e = atomicWrite(cfgPath, []byte(cfg)); e != nil {
		return nil, e
	}
	logpath := filepath.Join(m.Data, "logs", "game-"+p.ID+"-"+time.Now().UTC().Format("20060102T150405Z")+".log")
	logfile, e := os.Create(logpath)
	if e != nil {
		return nil, e
	}
	args := dosboxArguments(cfgPath, game, p)
	fmt.Fprintln(logfile, "MOO2-SGC", Version, "engine", p.Engine, "runtime", exe)
	fmt.Fprintln(logfile, "Working directory:", game)
	fmt.Fprintln(logfile, "Network service:", effectiveNetworkService(p), "role:", p.Role)
	fmt.Fprintln(logfile, "Kernel preflight:", encode(kernel))
	fmt.Fprintln(logfile, "Arguments:", encode(args))
	executable, _ := os.Executable()
	launchRecord := map[string]any{"launcher_version": Version, "launcher_executable": executable, "application_root": m.distributionRoot(), "data_root": m.Data, "profile": p.ID, "engine": p.Engine, "game_path": game, "working_directory": game, "game_executable": filepath.Join(game, gameEntrypoint(p)), "runtime": exe, "arguments": args, "network_service": effectiveNetworkService(p), "network": networkPlan(p), "config_path": cfgPath, "kernel": kernel, "log": logpath}
	if e = atomicJSON(filepath.Join(m.Data, "logs", "last-launch.json"), launchRecord); e != nil {
		logfile.Close()
		return nil, e
	}
	if e = m.rememberProfile(p.ID); e != nil {
		logfile.Close()
		return nil, e
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = game
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	m.mu.Lock()
	if m.running != nil {
		m.mu.Unlock()
		logfile.Close()
		return nil, errors.New("a game is already running")
	}
	if e = cmd.Start(); e != nil {
		_ = recordProcessStartFailure(filepath.Join(m.Data, "logs", "last-launch.json"), launchRecord, e)
		m.mu.Unlock()
		logfile.Close()
		return nil, fmt.Errorf("DOSBox failed to start: %w", e)
	}
	m.running = cmd
	m.runtimeDone = make(chan struct{})
	m.runtimeError = nil
	m.runningProfile = p.ID
	m.runningKind = "game"
	m.lastExit = ""
	m.mu.Unlock()
	go func() {
		e := cmd.Wait()
		logfile.Close()
		recordError := recordProcessExit(filepath.Join(m.Data, "logs", "last-launch.json"), launchRecord, e)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.runtimeError = e
		defer close(m.runtimeDone)
		m.running = nil
		m.runningProfile = ""
		m.runningKind = ""
		m.lastExit = "DOSBox exited normally. Game success must be confirmed in-game."
		if e != nil {
			m.lastExit = "DOSBox exit: " + e.Error() + ". Inspect " + logpath
		}
		if recordError != nil {
			m.lastExit += "; could not save process result: " + recordError.Error()
		}
	}()
	return map[string]any{"pid": cmd.Process.Pid, "log": logpath, "arguments": args, "game_path": game, "configuration": cfg, "fingerprint": v.Fingerprint, "launcher_version": Version, "kernel": kernel, "working_directory": game, "network_service": effectiveNetworkService(p), "status": "DOSBox process started; game and IPX connection are not yet verified", "instructions": "In MOO2 choose Multiplayer / Network, then Start New Game or Join Game. Match engine and mod fingerprints on both machines."}, nil
}
func (m *Manager) refreshPatch() (any, error) {
	dst := filepath.Join(m.Data, "cache", "patch-1.50.26.zip")
	m.progress("Refreshing the supported community patch; rejecting bytes that differ from the pinned build…")
	if e := downloadVerified("https://moo2mod.com/patch/MOO2-1.50.26.zip", dst, PatchHash, 64*1024*1024); e != nil {
		return nil, e
	}
	return map[string]string{"cached": "1.50.26", "sha256": PatchHash, "activation": "Existing environments unchanged. Use Prepare/Repair to reconstruct."}, nil
}
func checkUpstream() (any, error) {
	c := downloadClient()
	c.Timeout = 25 * time.Second
	resp, e := c.Get("https://moo2mod.com/")
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 2*1024*1024 {
		return nil, errors.New("upstream page too large")
	}
	versions, e := parseUpstreamVersions(string(b))
	if e != nil {
		return nil, e
	}
	return map[string]any{"latest_observed": versions[0], "available_on_page": versions, "supported_here": []string{"1.50.26"}, "automatic_activation": false, "notice": "A newer upstream release is information only until this manager supports and verifies its exact package. Launcher updates use the separately authenticated SGC release feed."}, nil
}
func parseUpstreamVersions(s string) ([]string, error) {
	matches := regexp.MustCompile(`MOO2-([0-9]+\.[0-9]+\.[0-9]+)\.zip`).FindAllStringSubmatch(s, -1)
	seen := map[string]bool{}
	v := []string{}
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			v = append(v, m[1])
		}
	}
	if len(v) == 0 {
		return nil, errors.New("could not identify a release ZIP on the community page; nothing updated")
	}
	sort.Slice(v, func(i, j int) bool {
		a, b := strings.Split(v[i], "."), strings.Split(v[j], ".")
		for k := 0; k < 3; k++ {
			x, _ := strconv.Atoi(a[k])
			y, _ := strconv.Atoi(b[k])
			if x != y {
				return x > y
			}
		}
		return false
	})
	return v, nil
}
func localAddresses() []string {
	out := []string{}
	as, _ := net.InterfaceAddrs()
	for _, a := range as {
		ip, _, e := net.ParseCIDR(a.String())
		if e == nil && ip.To4() != nil && !ip.IsLoopback() {
			out = append(out, ip.String())
		}
	}
	sort.Strings(out)
	return out
}

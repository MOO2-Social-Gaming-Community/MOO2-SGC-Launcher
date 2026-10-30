// The user entry point: an outer container embeds a signed launcher, never itself.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"moo2manager/shared/buildconfig"
	"moo2manager/shared/desktop"
	d "moo2manager/shared/distribution"
	"moo2manager/shared/layout"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const currentVersion = buildconfig.Version

//go:embed web/*
var web embed.FS

type bootState struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	Root    string `json:"root"`
	Busy    bool   `json:"busy"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Target  string `json:"target,omitempty"`
}
type startup struct {
	mu               sync.Mutex
	state            bootState
	server           *http.Server
	token, host, url string
	headless         bool
	trust            d.Trust
	ctx              context.Context
	cancel           context.CancelFunc
}
type limitedOutput struct {
	mu sync.Mutex
	b  []byte
}

func (w *limitedOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	room := 65536 - len(w.b)
	if room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		w.b = append(w.b, p...)
	}
	return n, nil
}
func (w *limitedOutput) String() string { w.mu.Lock(); defer w.mu.Unlock(); return string(w.b) }
func defaultRoot() (string, error) {
	trust, err := buildconfig.Trust()
	if err != nil {
		return "", err
	}
	return defaultRootForTrust(trust)
}
func defaultRootForTrust(trust d.Trust) (string, error) {
	base, e := os.UserConfigDir()
	if runtime.GOOS == "windows" {
		base, e = os.UserCacheDir()
	}
	if e != nil {
		return "", e
	}
	return filepath.Join(base, "MOO2-SGC", "private-gui-"+currentVersion+"-"+trustFingerprint(trust)[:16]), nil
}
func main() {
	headless := flag.Bool("no-browser", false, "engineering acceptance only: do not open the graphical browser")
	root := flag.String("install-root", "", "engineering acceptance only: isolated installation root")
	version := flag.Bool("version", false, "print version")
	flag.Parse()
	if *version {
		fmt.Println(currentVersion)
		return
	}
	if e := run(*root, *headless); e != nil {
		fmt.Fprintln(os.Stderr, "MOO2-SGC:", e)
		if !*headless {
			desktop.ShowError("MOO2-SGC Launcher", e.Error()+"\n\nYour existing launcher, owned game source and saves were not modified. No administrator privileges or terminal commands are needed.")
		}
		os.Exit(1)
	}
}
func run(root string, headless bool) error {
	var e error
	if root == "" {
		root, e = defaultRoot()
		if e != nil {
			return e
		}
	}
	root, e = filepath.Abs(root)
	if e != nil {
		return e
	}
	trust, e := buildconfig.Trust()
	if e != nil {
		return e
	}
	if e = trust.Validate(); e != nil {
		return e
	}
	if e = ensurePrivateRoot(root, trust); e != nil {
		return e
	}
	if u, err := bootSession(root); err == nil {
		if headless {
			return nil
		}
		return desktop.OpenBrowser(u)
	}
	installer := d.Installer{Root: root, Trust: trust}
	if cur, err := installer.Current(); err == nil {
		if _, rec, err := installer.VerifyGeneration(cur.Current); err == nil && d.VersionAtLeast(rec.Package.Version, currentVersion) {
			if u, err := managerSession(root, rec.Package.Version); err == nil {
				if headless {
					return nil
				}
				return desktop.OpenBrowser(u)
			}
		}
	}
	sessionRoot := filepath.Join(root, "desktop-session")
	release, e := acquireSession(sessionRoot)
	if e != nil {
		for until := time.Now().Add(3 * time.Second); time.Now().Before(until); time.Sleep(100 * time.Millisecond) {
			if u, err := bootSession(root); err == nil {
				if headless {
					return nil
				}
				return desktop.OpenBrowser(u)
			}
		}
		return fmt.Errorf("Could not open the private launcher session: %w. If another launcher window is opening, wait a moment and double-click again. If this folder is read-only, restore write access in your operating system settings. No live or unknown lock was changed", e)
	}
	defer release()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return fmt.Errorf("could not open the local graphical interface: %w", e)
	}
	defer listener.Close()
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &startup{state: bootState{Kind: "moo2-private-desktop", Version: currentVersion, Root: root, Message: "Opening your private launcher…"}, token: hex.EncodeToString(secret), host: listener.Addr().String(), headless: headless, trust: trust, ctx: ctx, cancel: cancel}
	s.url = "http://" + s.host + "/#" + s.token
	s.server = &http.Server{Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	marker := filepath.Join(sessionRoot, "open.txt")
	if e = d.AtomicWrite(marker, []byte(s.url+"\n")); e != nil {
		return fmt.Errorf("the private launcher folder is not writable: %w", e)
	}
	defer os.Remove(marker)
	if !headless {
		if e = desktop.OpenBrowser(s.url); e != nil {
			return fmt.Errorf("Your default browser could not open. Set a default browser in your operating system's Settings, then double-click this file again. Details: %w", e)
		}
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-signals:
			s.cancel()
			_ = s.server.Shutdown(context.Background())
		case <-done:
		}
	}()
	s.begin()
	e = s.server.Serve(listener)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func (s *startup) snapshot() bootState     { s.mu.Lock(); defer s.mu.Unlock(); return s.state }
func (s *startup) progress(message string) { s.mu.Lock(); s.state.Message = message; s.mu.Unlock() }
func (s *startup) begin() bool {
	s.mu.Lock()
	if s.state.Busy || s.state.Target != "" {
		s.mu.Unlock()
		return false
	}
	s.state.Busy = true
	s.state.Error = ""
	s.state.Message = "Checking your private installation…"
	s.mu.Unlock()
	go func() {
		if e := s.launch(); e != nil {
			s.mu.Lock()
			s.state.Busy = false
			s.state.Error = e.Error()
			s.state.Message = "Setup needs your attention. Your current installation remains separate."
			s.mu.Unlock()
		}
	}()
	return true
}
func (s *startup) launch() error {
	root := s.snapshot().Root
	release, e := d.Acquire(root)
	if e != nil {
		return errors.New("This installation is locked by a launcher, game, update, or an interrupted operation. Close the existing launcher and game, then choose Retry. No lock or existing file was removed")
	}
	locked := true
	defer func() {
		if locked {
			release()
		}
	}()
	logs := filepath.Join(root, "logs")
	if e = d.NoLinks(logs); e != nil {
		return e
	}
	if e = os.MkdirAll(logs, 0700); e != nil {
		return e
	}
	entry, version, e := ensureInstalled(s.ctx, root, runtime.GOOS+"-"+runtime.GOARCH, s.trust, bundled, s.progress)
	if e != nil {
		return e
	}
	if runtime.GOOS == "windows" {
		if e = layout.Enable(root); e != nil {
			return e
		}
	}
	if e = s.ctx.Err(); e != nil {
		return e
	}
	if e = recordManagerLaunchAttempt(root, s.trust); e != nil {
		return e
	}
	s.progress("Starting the graphical game chooser…")
	release()
	locked = false
	args := launcherArgs(root, s.headless)
	c := exec.Command(entry, args...)
	desktop.HideConsole(c)
	output := &limitedOutput{}
	c.Stdout = output
	c.Stderr = output
	if e = c.Start(); e != nil {
		return fmt.Errorf("The installed launcher could not start: %w", e)
	}
	exited := make(chan error, 1)
	go func() { exited <- c.Wait() }()
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case e = <-exited:
			return fmt.Errorf("The launcher stopped before its window was ready (%v). %s", e, strings.TrimSpace(output.String()))
		case <-timer.C:
			return errors.New("The launcher is taking longer than expected. It has not been stopped. Close any existing launcher window before retrying; your installation remains protected by its lock")
		case <-tick.C:
			if u, err := managerSession(root, version); err == nil {
				s.mu.Lock()
				s.state.Busy = false
				s.state.Target = u
				s.state.Message = "Ready — opening your game chooser."
				s.mu.Unlock()
				go s.watchChild(exited, output)
				return nil
			}
		}
	}
}
func (s *startup) watchChild(exited <-chan error, output *limitedOutput) {
	e := <-exited
	if e == nil {
		_ = s.server.Shutdown(context.Background())
		return
	}
	s.mu.Lock()
	s.state.Target = ""
	s.state.Busy = false
	s.state.Error = "The launcher stopped unexpectedly. " + strings.TrimSpace(output.String())
	s.state.Message = "Your source game and retained saves are still separate. Retry or exit."
	s.mu.Unlock()
	if !s.headless {
		if err := desktop.OpenBrowser(s.url); err != nil {
			desktop.ShowError("MOO2-SGC Launcher", s.snapshot().Error)
		}
	}
}
func (s *startup) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if r.Host != s.host {
		http.Error(w, "invalid host", 403)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) != 1 {
			http.Error(w, "local session authentication required", 401)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+s.host {
			http.Error(w, "cross-origin request refused", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			if r.Method != "GET" {
				http.Error(w, "GET required", 405)
				return
			}
			_ = json.NewEncoder(w).Encode(s.snapshot())
		case "/api/retry":
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			if !s.begin() {
				http.Error(w, "launcher is already starting or ready", 409)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "retrying"})
		case "/api/recover":
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			s.mu.Lock()
			if s.state.Busy || s.state.Target != "" {
				s.mu.Unlock()
				http.Error(w, "launcher is active", 409)
				return
			}
			s.state.Busy = true
			s.mu.Unlock()
			e := recoverInterrupted(s.snapshot().Root, s.trust)
			s.mu.Lock()
			s.state.Busy = false
			if e != nil {
				s.state.Error = e.Error()
			} else {
				s.state.Error = ""
				s.state.Message = "Interrupted startup markers retained as recovery records. Choose Retry to continue."
			}
			s.mu.Unlock()
			if e != nil {
				http.Error(w, e.Error(), 409)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "recovered"})
			s.begin()
		case "/api/exit":
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			state := s.snapshot()
			if state.Busy || state.Target != "" {
				http.Error(w, "Finish setup, or use Exit launcher in the game chooser. No running game will be stopped.", 409)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
			go func() { time.Sleep(100 * time.Millisecond); s.cancel(); _ = s.server.Shutdown(context.Background()) }()
		default:
			http.NotFound(w, r)
		}
		return
	}
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	switch p {
	case "/index.html", "/app.js", "/style.css":
	default:
		http.NotFound(w, r)
		return
	}
	b, e := web.ReadFile("web" + p)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	switch filepath.Ext(p) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	_, _ = w.Write(b)
}

var _ io.Writer = (*limitedOutput)(nil)

func launcherArgs(root string, noBrowser bool) []string {
	args := []string{"--app-root", root, "--root", root, "--data", filepath.Join(root, "userdata"), "--desktop-gui", "--no-shortcut", "--private-url-file-only"}
	if noBrowser {
		args = append(args, "--no-browser")
	} else {
		args = append(args, "--defer-browser-once")
	}
	return args
}

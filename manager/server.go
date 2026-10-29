package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type actionRequest struct {
	Action       string               `json:"action"`
	Profile      Profile              `json:"profile"`
	ID           string               `json:"id"`
	Generation   string               `json:"generation"`
	RuntimePath  string               `json:"runtime_path"`
	SourcePath   string               `json:"source_path"`
	PayloadKind  string               `json:"payload_kind"`
	OfflinePath  string               `json:"offline_path"`
	PickerKind   string               `json:"picker_kind"`
	Fullscreen   bool                 `json:"fullscreen"`
	Presentation PresentationSettings `json:"presentation"`
}
type localServer struct {
	manager   *Manager
	token     string
	host      string
	server    *http.Server
	noBrowser bool
	pick      func(string) (string, error)
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func readBody(w http.ResponseWriter, r *http.Request, v any) error {
	if r.Method != "POST" {
		return errors.New("POST required")
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("application/json required")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var x any
	if e := d.Decode(&x); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func (s *localServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if r.Host != s.host {
		http.Error(w, "invalid host", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) != 1 {
			jsonResponse(w, 401, map[string]string{"error": "launcher token required"})
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+s.host {
			jsonResponse(w, 403, map[string]string{"error": "cross-origin request refused"})
			return
		}
		switch r.URL.Path {
		case "/api/identity":
			if r.Method != "GET" {
				http.Error(w, "GET required", 405)
				return
			}
			jsonResponse(w, 200, s.manager.applicationIdentity())
		case "/api/state":
			if r.Method != "GET" {
				http.Error(w, "GET required", 405)
				return
			}
			v, e := s.manager.state()
			if e != nil {
				jsonResponse(w, 500, map[string]string{"error": e.Error()})
				return
			}
			jsonResponse(w, 200, v)
		case "/api/addresses":
			jsonResponse(w, 200, localAddresses())
		case "/api/selection":
			var sel struct {
				ID string `json:"id"`
			}
			if e := readBody(w, r, &sel); e != nil {
				jsonResponse(w, 400, map[string]string{"error": e.Error()})
				return
			}
			if e := s.manager.rememberProfile(sel.ID); e != nil {
				jsonResponse(w, 400, map[string]string{"error": e.Error()})
				return
			}
			jsonResponse(w, 200, map[string]string{"selected_profile_id": sel.ID})
		case "/api/resolve":
			var p Profile
			if e := readBody(w, r, &p); e != nil {
				jsonResponse(w, 400, map[string]string{"error": e.Error()})
				return
			}
			v, e := resolve(p)
			if e != nil {
				jsonResponse(w, 400, map[string]string{"error": e.Error()})
				return
			}
			jsonResponse(w, 200, v)
		case "/api/action":
			s.action(w, r)
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
	if p != "/index.html" && p != "/app.js" && p != "/style.css" {
		http.NotFound(w, r)
		return
	}
	b, e := resources.ReadFile("web" + p)
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
func (s *localServer) action(w http.ResponseWriter, r *http.Request) {
	var a actionRequest
	if e := readBody(w, r, &a); e != nil {
		jsonResponse(w, 400, map[string]string{"error": e.Error()})
		return
	}
	if a.Action == "cancel" {
		if err := s.manager.cancelOperation(); err != nil {
			jsonResponse(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "cancellation requested"})
		return
	}
	if a.Action == "quit" || a.Action == "launcher-apply" {
		s.manager.mu.Lock()
		busy := s.manager.job.Busy || s.manager.running != nil
		if !busy {
			s.manager.job.Busy = true
		}
		s.manager.mu.Unlock()
		if busy {
			jsonResponse(w, 409, map[string]string{"error": "close the game / finish the operation before exiting"})
			return
		}
		if a.Action == "launcher-apply" {
			if e := s.manager.launchStagedUpdater(s.noBrowser); e != nil {
				s.manager.mu.Lock()
				s.manager.job.Busy = false
				s.manager.mu.Unlock()
				jsonResponse(w, 400, map[string]string{"error": e.Error()})
				return
			}
		}
		jsonResponse(w, 200, map[string]string{"status": "exiting"})
		go func() { time.Sleep(100 * time.Millisecond); _ = s.server.Shutdown(context.Background()) }()
		return
	}
	var work func() (any, error)
	switch a.Action {
	case "browse":
		if _, _, _, err := pickerType(a.PickerKind); err != nil {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
			return
		}
		work = func() (any, error) {
			pick := s.pick
			if pick == nil {
				pick = nativePick
			}
			path, err := pick(a.PickerKind)
			return pickerResult(a.PickerKind, path, err)
		}
	case "runtime-import":
		work = func() (any, error) { return s.manager.importRuntime(a.SourcePath) }
	case "check-dopefish":
		work = s.manager.checkDopefish
	case "prepare-play":
		work = func() (any, error) { return s.manager.prepareForPlay(a.Profile, a.SourcePath) }
	case "game-detect":
		work = func() (any, error) {
			return map[string]any{"folders": detectGameFolders(), "verified": false, "notice": "Candidate paths only. Import verifies every recognized game file."}, nil
		}
	case "game-import":
		work = func() (any, error) { return s.manager.importFolder(a.SourcePath) }
	case "portable-recover":
		work = s.manager.recoverPortableBaseline
	case "prepare":
		work = func() (any, error) { return s.manager.build(a.Profile) }
	case "presentation-save":
		work = func() (any, error) { return s.manager.savePresentation(a.ID, a.Fullscreen, a.Presentation) }
	case "save":
		work = func() (any, error) { e := s.manager.saveProfile(a.Profile); return a.Profile, e }
	case "verify":
		work = func() (any, error) { return s.manager.verify(a.ID) }
	case "launch":
		work = func() (any, error) { return s.manager.launch(a.Profile) }
	case "remove":
		work = func() (any, error) { return s.manager.removeEnvironment(a.ID) }
	case "activate":
		work = func() (any, error) { return s.manager.activate(a.ID, a.Generation) }
	case "diagnostics":
		work = func() (any, error) { return s.manager.exportDiagnostics(a.ID) }
	case "runtime-install":
		work = s.manager.installRuntime
	case "runtime-select":
		work = func() (any, error) {
			p, e := filepath.Abs(strings.Trim(strings.TrimSpace(a.RuntimePath), "\""))
			if e != nil {
				return nil, e
			}
			if !strings.EqualFold(filepath.Base(p), "dosbox.exe") && filepath.Base(p) != "dosbox" && filepath.Base(p) != "dosbox-staging" {
				return nil, errors.New("select the DOSBox executable itself (dosbox.exe or dosbox)")
			}
			i, e := os.Stat(p)
			if e != nil || !i.Mode().IsRegular() {
				return nil, errors.New("runtime executable not found")
			}
			e = atomicJSON(filepath.Join(s.manager.Data, "settings.json"), Settings{p})
			return map[string]string{"runtime": p, "trust": "user-selected executable; not certified by this manager"}, e
		}
	case "upstream-check":
		work = checkUpstream
	case "payload-import":
		work = func() (any, error) { return s.manager.importPayload(a.PayloadKind, a.SourcePath) }
	case "launcher-check":
		work = func() (any, error) { return s.manager.launcherRelease(a.OfflinePath, false) }
	case "launcher-stage":
		work = func() (any, error) { return s.manager.launcherRelease(a.OfflinePath, true) }
	case "patch-refresh":
		work = s.manager.refreshPatch
	default:
		jsonResponse(w, 400, map[string]string{"error": "unsupported action"})
		return
	}
	if e := s.manager.startJob(a.Action, work); e != nil {
		jsonResponse(w, 409, map[string]string{"error": e.Error()})
		return
	}
	jsonResponse(w, 202, map[string]string{"status": "started"})
}
func printLauncherURL(w io.Writer, privateURL, recoveryPath string, fileOnly bool) {
	if fileOnly {
		fmt.Fprintln(w, "Private local URL is in:", recoveryPath)
		return
	}
	fmt.Fprintln(w, "Open this private local URL:")
	fmt.Fprintln(w, privateURL)
}

func serve(m *Manager, open bool, requestedPort int, privateURLFileOnly bool) error {
	l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", requestedPort))
	if e != nil {
		return e
	}
	defer l.Close()
	s := &localServer{manager: m, token: token(), host: l.Addr().String(), noBrowser: !open}
	s.server = &http.Server{Handler: http.HandlerFunc(s.handler), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	u := "http://" + s.host + "/#" + s.token
	fmt.Println("MOO2-SGC Launcher", Version)
	// Private local recovery URL; never included in diagnostics or a distribution.
	recoveryPath := filepath.Join(m.Data, "open-launcher.txt")
	if e = atomicWrite(recoveryPath, []byte(u+"\n")); e != nil {
		return e
	}
	defer os.Remove(recoveryPath)
	printLauncherURL(os.Stdout, u, recoveryPath, privateURLFileOnly)
	fmt.Println("Use Exit launcher in the UI. Closing the browser tab does not stop the manager.")
	if initialBrowserAllowed(open, m.DeferBrowserOnce) {
		if e = openBrowser(u); e != nil {
			fmt.Println("Browser could not be opened automatically:", e)
			if m.DesktopGUI {
				return fmt.Errorf("the default browser could not open. Set a default browser in Windows Settings, then reopen the launcher: %w", e)
			}
		}
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	defer signal.Stop(signals)
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-signals:
				m.mu.Lock()
				busy := m.job.Busy || m.running != nil
				m.mu.Unlock()
				if busy {
					fmt.Println("A game or operation is active. Finish it before exiting.")
					continue
				}
				_ = s.server.Shutdown(context.Background())
				return
			}
		}
	}()
	e = s.server.Serve(l)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}

// Explicit no-browser is retained by localServer; this flag only defers the
// first open to the desktop parent and is never forwarded to the updater.
func initialBrowserAllowed(open, deferOnce bool) bool { return open && !deferOnce }

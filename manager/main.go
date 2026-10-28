package main

import (
	"flag"
	"fmt"
	d "moo2manager/shared/distribution"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func openBrowser(u string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		c = exec.Command("open", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	if e := c.Start(); e != nil {
		return e
	}
	go func() { _ = c.Wait() }()
	return nil
}
func main() {
	appRoot := flag.String("app-root", "", "stable bootstrap-managed application root; user profiles remain in --data")
	root := flag.String("root", "", "directory containing payloads (defaults to executable directory)")
	data := flag.String("data", "", "portable data directory (defaults to ROOT/data)")
	noBrowser := flag.Bool("no-browser", false, "print the loopback URL without opening a browser")
	privateURLFileOnly := flag.Bool("private-url-file-only", false, "keep the private URL in open-launcher.txt instead of standard output (update handoff)")
	port := flag.Int("port", 0, "local UI port; default chooses an available port")
	ver := flag.Bool("version", false, "print version")
	cmd := flag.String("command", "serve", "serve, prepare, prepare-play, play, verify, recover-portable, check-dopefish, or resolve")
	profile := flag.String("profile", "baseline", "profile ID for CLI commands")
	source := flag.String("source", "", "owned source path (blank discovers the canonical root baseline archive)")
	fullscreen := flag.Bool("fullscreen", false, "fullscreen for this launch only")
	flag.Parse()
	if *ver {
		fmt.Println(Version)
		return
	}
	exe, e := os.Executable()
	if e != nil {
		fail(e)
	}
	if *root == "" {
		*root = filepath.Dir(exe)
	}
	if *data == "" {
		*data = filepath.Join(*root, "data")
	}
	if *appRoot == "" {
		*appRoot = filepath.Join(*data, "application")
	}
	release, lockErr := d.Acquire(*appRoot)
	if lockErr != nil {
		fail(lockErr)
	}
	defer release()
	m, e := newManager(*root, *data)
	if e != nil {
		release()
		fail(e)
	}
	m.AppRoot = *appRoot
	fmt.Fprintln(os.Stderr, "MOO2-SGC Launcher", Version, "executable", exe, "application root", *appRoot, "game root", m.Root, "data", m.Data)
	// An exclusive process lock prevents two managers from mutating one data directory.
	lock := filepath.Join(m.Data, "manager.lock")
	lf, e := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		release()
		fail(fmt.Errorf("data directory is locked. Check open-launcher.txt for an existing window. After a crash, confirm no manager/game is running before removing %s: %w", lock, e))
	}
	fmt.Fprintln(lf, os.Getpid())
	lf.Close()
	code := 0
	func() {
		defer os.Remove(lock)
		switch *cmd {
		case "serve":
			e = serve(m, !*noBrowser, *port, *privateURLFileOnly)
		case "check-dopefish":
			var v any
			v, e = m.checkDopefish()
			if e == nil {
				fmt.Println(encode(v))
				e = m.waitForRuntime()
			}
		case "prepare-play":
			var p Profile
			p, e = m.loadProfile(*profile)
			if e == nil {
				var v any
				v, e = m.prepareForPlay(p, *source)
				if e == nil {
					fmt.Println(encode(v))
				}
			}
		case "recover-portable":
			var v any
			v, e = m.recoverPortableBaseline()
			if e == nil {
				fmt.Println(encode(v))
			}
		case "play":
			var p Profile
			p, e = m.loadProfile(*profile)
			if *fullscreen {
				p.Fullscreen = true
			}
			if e == nil {
				var v any
				v, e = m.launch(p)
				if e == nil {
					fmt.Println(encode(v))
				}
			}
			if e == nil {
				e = m.waitForRuntime()
			}
		case "prepare":
			var p Profile
			p, e = m.loadProfile(*profile)
			if e == nil {
				var v any
				v, e = m.build(p)
				if e == nil {
					fmt.Println(encode(v))
				}
			}
		case "verify":
			var v VerifyResult
			v, e = m.verify(*profile)
			if e == nil {
				fmt.Println(encode(v))
				if !v.OK {
					code = 2
				}
			}
		case "resolve":
			var p Profile
			p, e = m.loadProfile(*profile)
			if e == nil {
				var v Resolution
				v, e = resolve(p)
				if e == nil {
					fmt.Println(encode(v))
				}
			}
		default:
			e = fmt.Errorf("unknown command %q", *cmd)
		}
	}()
	if e != nil {
		fmt.Fprintln(os.Stderr, "Error:", e)
		code = 1
	}
	if code != 0 {
		release()
		os.Exit(code)
	}
}
func fail(e error) { fmt.Fprintln(os.Stderr, "Error:", e); os.Exit(1) }

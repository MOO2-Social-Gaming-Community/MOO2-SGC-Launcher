// MOO2-SGC Setup installs only the signed launcher. No game data, engine offsets,
// PRSL dependencies, website code or cloud credentials belong in this command.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"moo2manager/shared/buildconfig"
	"moo2manager/shared/desktop"
	d "moo2manager/shared/distribution"
	"moo2manager/shared/layout"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var desktopGUIMode bool

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Setup stopped:", e)
		if desktopGUIMode {
			desktop.ShowError("MOO2-SGC Setup", e.Error())
		} else {
			showSetupError(e)
		}
		os.Exit(1)
	}
}
func run() error {
	flag.BoolVar(&desktopGUIMode, "desktop-gui", false, "native errors and terminal-free launcher handoff")
	waitLock := flag.Int("wait-lock", 0, "wait up to 60 seconds for an exiting launcher; never remove its lock")
	root := flag.String("install-root", "", "installation root (Windows default: C:\\Games\\MOO2-SGC; other platforms: user config)")
	offline := flag.String("offline", "", "directory with manifest.json, manifest.sig and signed launcher package")
	command := flag.String("command", "install", "install, ensure-installed, repair, verify, rollback or launch-installed")
	noLaunch := flag.Bool("no-launch", false, "install/verify only; do not open launcher")
	noBrowser := flag.Bool("no-browser", false, "pass --no-browser to the launcher")
	noShortcut := flag.Bool("no-shortcut", false, "do not create or replace shortcuts; preserve this mode through launcher updates")
	version := flag.Bool("version", false, "print setup version")
	status := flag.Bool("trust-status", false, "print embedded non-secret trust configuration")
	portable := flag.Bool("portable", false, "use the portable ROOT/game and ROOT/runtime layout")
	launcherCommand := flag.String("launcher-command", "serve", "serve, prepare-play, play, verify, recover-portable, check-dopefish")
	profile := flag.String("profile", "baseline", "profile passed to the installed launcher")
	source := flag.String("source", "", "owned local source passed to preparation")
	fullscreen := flag.Bool("fullscreen", false, "fullscreen for a direct play command")
	flag.Parse()
	if *version {
		fmt.Println(buildconfig.Version)
		return nil
	}
	trust, e := buildconfig.Trust()
	if e != nil {
		return e
	}
	if *status {
		fmt.Printf("feed=%s channel=%s development=%v keys=%d endpoints=%d\n", trust.Feed, trust.Channel, trust.Development, len(trust.Keys), len(trust.Endpoints))
		return nil
	}
	if e = trust.Validate(); e != nil {
		return e
	}
	fmt.Printf("MOO2-SGC Setup %s (minimum launcher %s)\n", buildconfig.Version, buildconfig.Version)
	if path, err := os.Executable(); err == nil {
		fmt.Println("Setup executable:", path)
	}
	if trust.Development {
		fmt.Println("DEVELOPMENT TRUST: signed offline acceptance build, not an official production release.")
	}
	if *root == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		*root = filepath.Join(base, "MOO2-SGC", "stable")
		if runtime.GOOS == "windows" {
			*root = layout.WindowsRoot
			*portable = true
		}
	}
	*root, e = filepath.Abs(*root)
	if e != nil {
		return e
	}
	fmt.Println("Application root:", *root)
	if *waitLock < 0 || *waitLock > 60 {
		return fmt.Errorf("wait-lock must be between 0 and 60 seconds")
	}
	release, e := d.Acquire(*root)
	until := time.Now().Add(time.Duration(*waitLock) * time.Second)
	for e != nil && *waitLock > 0 && time.Now().Before(until) && errors.Is(e, os.ErrExist) {
		time.Sleep(200 * time.Millisecond)
		release, e = d.Acquire(*root)
	}
	if e != nil {
		return e
	}
	if *portable {
		if e = layout.Enable(*root); e != nil {
			release()
			return e
		}
	}
	switch *launcherCommand {
	case "serve", "prepare-play", "play", "verify", "recover-portable", "check-dopefish":
	default:
		release()
		return errors.New("unsupported launcher command")
	}
	locked := true
	defer func() {
		if locked {
			release()
		}
	}()
	if e = os.MkdirAll(filepath.Join(*root, "logs"), 0700); e != nil {
		return e
	}
	log, e := os.OpenFile(filepath.Join(*root, "logs", "bootstrap.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	fmt.Fprintln(log, time.Now().UTC().Format(time.RFC3339), "setup", buildconfig.Version, "command", *command)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	installer := d.Installer{Root: *root, Trust: trust, Health: health}
	// Offline portable helpers must upgrade an older launcher, but must not
	// reinstall/downgrade a newer verified release on every invocation.
	if *command == "ensure-installed" {
		*command = "install"
		if cur, err := installer.Current(); err == nil {
			if _, rec, err := installer.VerifyGeneration(cur.Current); err == nil && versionAtLeast(rec.Package.Version, buildconfig.Version) {
				*command = "launch-installed"
			}
		}
	}
	var entry string
	minimum := buildconfig.Version
	if cur, err := installer.Current(); err == nil {
		if _, rec, err := installer.VerifyGeneration(cur.Current); err == nil {
			fmt.Println("Previously installed launcher (not the launch target):", rec.Package.Version)
			if d.VersionAtLeast(rec.Package.Version, minimum) {
				minimum = rec.Package.Version
			}
		}
	}
	switch *command {
	case "verify", "launch-installed":
		p, e := installer.Current()
		if e != nil {
			return e
		}
		entry, _, e = installer.VerifyGeneration(p.Current)
		if e != nil {
			return e
		}
		fmt.Println("Installed launcher verified:", entry)
		if *command == "verify" {
			return nil
		}
	case "rollback":
		entry, e = installer.Rollback()
		if e != nil {
			return e
		}
		fmt.Println("Previous launcher restored. Game data and profiles were not changed.")
	case "install", "repair", "apply-staged":
		if *command == "apply-staged" {
			*offline = filepath.Join(*root, "distribution", "pending")
		}
		prior, e := d.LoadState(*root, trust)
		if e != nil {
			return e
		}
		var v d.VerifiedManifest
		client := d.NewClient(trust)
		client.MinimumLauncherVersion = minimum
		client.LauncherPlatform = runtime.GOOS + "-" + runtime.GOARCH
		client.Log = func(f d.Failure) {
			fmt.Fprintln(log, time.Now().UTC().Format(time.RFC3339), f.Provider, f.Kind, f.Message)
			fmt.Println(f.Provider, f.Kind, f.Message)
		}
		if *offline != "" {
			v, e = d.ReadOffline(trust, *root, *offline)
		} else {
			v, e = client.FetchManifest(ctx, prior, time.Now())
		}
		if e != nil {
			return fmt.Errorf("%w. The repository must be public and its Publish prepared release workflow must have completed. A source push alone is not a release. Check GitHub Actions, HTTPS access, and the system clock", e)
		}
		if e = v.RequireLauncher(runtime.GOOS+"-"+runtime.GOARCH, minimum); e != nil {
			return fmt.Errorf("%w. Publish v%s with all release assets, or use its signed offline integration kit. An older launcher will not be started", e, minimum)
		}
		// Record newest authenticated metadata even if the subsequent transfer fails.
		if e = d.Accept(*root, v); e != nil {
			return e
		}
		p, e := v.Select("launcher", runtime.GOOS+"-"+runtime.GOARCH)
		if e != nil {
			return e
		}
		cache := filepath.Join(*root, "cache", "packages")
		var archive string
		fmt.Printf("Verifying launcher %s (%d bytes)…\n", p.Version, p.Size)
		if *offline != "" {
			archive, e = d.CacheOffline(trust, p, *offline, cache)
		} else {
			archive, e = client.Download(ctx, p, cache)
		}
		if e != nil {
			return e
		}
		// Reuse an intact generation on repeat setup; repair always reconstructs it.
		if *command != "repair" {
			if cur, err := installer.Current(); err == nil {
				if path, r, err := installer.VerifyGeneration(cur.Current); err == nil && r.Package.SHA256 == p.SHA256 {
					entry = path
				}
			}
		}
		if entry == "" {
			entry, e = installer.Install(ctx, v, p, archive)
			if e != nil {
				return e
			}
		}
		fmt.Println("Launcher installed and verified:", entry)
	default:
		return fmt.Errorf("unknown command %q", *command)
	}
	// Verify the selected executable again even when an intact generation was reused.
	cur, e := installer.Current()
	if e != nil {
		return e
	}
	verifiedEntry, rec, e := installer.VerifyGeneration(cur.Current)
	if e != nil {
		return e
	}
	if *command != "rollback" && !d.VersionAtLeast(rec.Package.Version, minimum) {
		return fmt.Errorf("installed launcher %s is older than required %s at %s; run Setup normally or use the matching offline kit. Refusing to launch an old generation", rec.Package.Version, minimum, *root)
	}
	if entry != verifiedEntry {
		return errors.New("launcher selection changed during setup")
	}
	if e = health(ctx, entry, rec.Package.Version); e != nil {
		return e
	}
	fmt.Printf("Launching/ready: launcher %s; setup %s; root %s\n", rec.Package.Version, buildconfig.Version, *root)
	fmt.Fprintln(log, "verified launcher", rec.Package.Version, "entry", entry, "root", *root)
	fmt.Fprintln(log, time.Now().UTC().Format(time.RFC3339), "success", *command)
	if !*noLaunch && *command != "verify" && entry != "" && *launcherCommand == "serve" {
		if err := installShortcutIfEnabled(*noShortcut, *root, entry, installShortcut); err != nil {
			fmt.Fprintln(log, "shortcut not created:", err)
			fmt.Println("Shortcut not created:", err)
		}
	}
	if *noLaunch {
		return nil
	}
	release()
	locked = false
	// Immutable executable location; user data lives outside every launcher version.
	args := []string{"--app-root", *root, "--root", *root, "--data", filepath.Join(*root, "userdata")}
	args = append(args, "--command", *launcherCommand, "--profile", *profile)
	if *source != "" {
		args = append(args, "--source", *source)
	}
	if *fullscreen {
		args = append(args, "--fullscreen")
	}
	args = appendLauncherModeArgs(args, *noBrowser, *noShortcut, *command == "apply-staged")
	if desktopGUIMode {
		args = append(args, "--desktop-gui")
	}
	cmd := exec.Command(entry, args...)
	if desktopGUIMode {
		desktop.HideConsole(cmd)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	// Keep the setup console open until launcher exit; ordinary start/quit remains
	// explicit. The launcher itself owns the installation lock while running.
	return cmd.Run()
}
func health(ctx context.Context, p, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, p, "--version")
	desktop.HideConsole(c)
	var out limitedWriter
	c.Stdout = &out
	c.Stderr = &out
	if e := c.Run(); e != nil {
		return e
	}
	if strings.TrimSpace(out.String()) != version {
		return fmt.Errorf("launcher version differs from signed manifest: %q", out.String())
	}
	return nil
}

type limitedWriter struct{ b []byte }

func (w *limitedWriter) Write(p []byte) (int, error) {
	if len(w.b)+len(p) > 4096 {
		return 0, fmt.Errorf("health-check output limit")
	}
	w.b = append(w.b, p...)
	return len(p), nil
}
func (w *limitedWriter) String() string { return string(w.b) }

var _ io.Writer = (*limitedWriter)(nil)

// Releases use exactly major.minor.patch. Reject malformed values rather than
// guessing how to compare an unknown version scheme.
func versionAtLeast(have, want string) bool { return d.VersionAtLeast(have, want) }

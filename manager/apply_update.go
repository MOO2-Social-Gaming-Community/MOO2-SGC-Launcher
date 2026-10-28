package main

import (
	"fmt"
	"moo2manager/shared/buildconfig"
	d "moo2manager/shared/distribution"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// The updater executable is itself inside the signed installed-file index.
// It waits for this launcher to exit and release its lock before changing files.
func (m *Manager) launchStagedUpdater(noBrowser bool) error {
	trust, e := buildconfig.Trust()
	if e != nil {
		return e
	}
	root := m.distributionRoot()
	v, e := d.ReadOffline(trust, root, filepath.Join(root, "distribution", "pending"))
	if e != nil {
		return fmt.Errorf("stage a valid signed launcher update first: %w", e)
	}
	if e = v.RequireLauncher(runtime.GOOS+"-"+runtime.GOARCH, Version); e != nil {
		return e
	}
	installer := d.Installer{Root: root, Trust: trust}
	cur, e := installer.Current()
	if e != nil {
		return e
	}
	entry, receipt, e := installer.VerifyGeneration(cur.Current)
	if e != nil {
		return e
	}
	name := "MOO2-SGC-Setup"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, ok := receipt.Package.Files[name]; !ok {
		return fmt.Errorf("this installation lacks a signed update helper; run your downloaded Setup instead")
	}
	log, e := os.OpenFile(filepath.Join(root, "logs", "update-handoff.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	args := []string{"--install-root", root, "--command", "apply-staged", "--wait-lock", "30"}
	// Preserve this server's browser policy through the helper and restarted launcher.
	if noBrowser {
		args = append(args, "--no-browser")
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(entry), name), args...)
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		return e
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

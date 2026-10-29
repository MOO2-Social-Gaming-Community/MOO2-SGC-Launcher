package main

import (
	"encoding/json"
	"errors"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
	"time"
)

const privateRootMarker = "desktop-installation.json"

type rootContract struct {
	Schema                 int    `json:"schema"`
	Kind                   string `json:"kind"`
	CreatedVersion         string `json:"created_version"`
	RuntimeHistoryContract int    `json:"runtime_history_contract"`
	LaunchAttempted        *bool  `json:"manager_launch_attempted"`
}

func checkPrivateRoot(root string) error {
	var c rootContract
	if e := d.ReadJSON(filepath.Join(root, privateRootMarker), &c); e != nil {
		return e
	}
	if c.Schema != 1 || c.Kind != "moo2-private-gui" || c.CreatedVersion != currentVersion || c.RuntimeHistoryContract != 1 || c.LaunchAttempted == nil {
		return errors.New("private installation ownership/history contract is unknown")
	}
	return nil
}
func ensurePrivateRoot(root string) error {
	if e := d.NoLinks(root); e != nil {
		return e
	}
	if e := checkPrivateRootAfterWriter(root); e == nil {
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	entries, e := os.ReadDir(root)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if len(entries) > 0 {
		if e := checkPrivateRootAfterWriter(root); e == nil {
			return nil
		}
		return errors.New("This is not a fresh private launcher folder. It has existing files but no recognized ownership marker. They were not changed; the single-file candidate must use its separate per-user private installation")
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return e
	}
	// This marker is never retroactively attached to nonempty/legacy installations.
	return publishInitialContract(root)
}
func checkRecoveryContract(root string, trust d.Trust) error {
	if e := checkPrivateRoot(root); e != nil {
		return errors.New("Interrupted recovery is unavailable: this folder has no recognized private ownership/history contract")
	}
	installer := d.Installer{Root: root, Trust: trust}
	cur, e := installer.Current()
	if os.IsNotExist(e) {
		var c rootContract
		if err := d.ReadJSON(filepath.Join(root, privateRootMarker), &c); err == nil && c.LaunchAttempted != nil && !*c.LaunchAttempted {
			return nil
		}
		return errors.New("Application activation history is incomplete; cannot infer that a game was never started")
	}
	if e != nil {
		return e
	}
	_, rec, e := installer.VerifyGeneration(cur.Current)
	if e != nil {
		return e
	}
	if rec.Package.Version != currentVersion {
		return errors.New("Installed launcher has a different runtime-history contract; its missing history cannot authorize recovery")
	}
	return nil
}

// Durable before manager Start: an interrupted first install can prove that it
// never started a manager, even when a staged components directory exists.
func recordManagerLaunchAttempt(root string) error {
	var c rootContract
	if e := d.ReadJSON(filepath.Join(root, privateRootMarker), &c); e != nil {
		return e
	}
	if e := checkPrivateRoot(root); e != nil {
		return e
	}
	attempted := true
	c.LaunchAttempted = &attempted
	return d.AtomicJSON(filepath.Join(root, privateRootMarker), c)
}

// Initial false must NEVER replace a marker written by another fresh launch.
// O_EXCL closes the empty-directory observation race with concurrent startup.
// An interrupted partial write is unknown and is not silently repaired.
func publishInitialContract(root string) error {
	p := filepath.Join(root, privateRootMarker)
	c := rootContract{Schema: 1, Kind: "moo2-private-gui", CreatedVersion: currentVersion, RuntimeHistoryContract: 1, LaunchAttempted: new(bool)}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return checkPrivateRootAfterWriter(root)
	}
	if e != nil {
		return e
	}
	_, writeErr := f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// A simultaneous first click can observe the exclusive marker between create
// and write. Wait only for that publication; never repair or replace its bytes.
func checkPrivateRootAfterWriter(root string) error {
	e := checkPrivateRoot(root)
	for until := time.Now().Add(250 * time.Millisecond); e != nil && !os.IsNotExist(e) && time.Now().Before(until); {
		time.Sleep(10 * time.Millisecond)
		e = checkPrivateRoot(root)
	}
	return e
}

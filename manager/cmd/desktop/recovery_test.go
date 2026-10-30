package main

import (
	"encoding/json"
	"errors"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelSessionLockReleasesWithoutUnlink(t *testing.T) {
	root := t.TempDir()
	release, e := acquireSession(root)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := acquireSession(root); e == nil {
		r()
		t.Fatal("second owner acquired")
	}
	release()
	again, e := acquireSession(root)
	if e != nil {
		t.Fatal(e)
	}
	again()
	if _, e = os.Stat(filepath.Join(root, "session.lock")); e != nil {
		t.Fatal("lock inode disappeared")
	}
}
func TestRecoveryRefusesUnknownRuntimeAndLiveOwners(t *testing.T) {
	for _, tc := range []struct {
		name, record string
		dead         bool
		err          bool
		ok           bool
	}{
		{"no runtime stopped owner", "", true, false, true}, {"exited runtime", `{"process_exited":true}`, true, false, true}, {"failed start", `{"process_start_failed":true}`, true, false, true}, {"unknown runtime", `{}`, true, false, false}, {"invalid record", `broken`, true, false, false}, {"live manager", "", false, false, false}, {"unknown manager", "", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			logs := filepath.Join(root, "userdata", "logs")
			os.MkdirAll(logs, 0700)
			if tc.record != "" {
				os.WriteFile(filepath.Join(logs, "last-launch.json"), []byte(tc.record), 0600)
			}
			lock := filepath.Join(root, "installation.lock")
			os.WriteFile(lock, []byte("12345\n"), 0600)
			markers, e := recoveryMarkers(root, func(int) (bool, error) {
				if tc.err {
					return false, errors.New("unknown")
				}
				return tc.dead, nil
			})
			if (e == nil) != tc.ok {
				t.Fatalf("markers=%v error=%v", markers, e)
			}
			if _, e = os.Stat(lock); e != nil {
				t.Fatal("preflight changed marker")
			}
		})
	}
}
func TestRecoverNeverChangesLiveProcess(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "installation.lock")
	b, _ := json.Marshal(os.Getpid())
	os.WriteFile(lock, b, 0600)
	if e := recoverInterrupted(root, d.Trust{}); e == nil {
		t.Fatal("live process recovered")
	}
	after, _ := os.ReadFile(lock)
	if string(after) != string(b) {
		t.Fatal("lock changed")
	}
}

func TestPrivateRootNeverClaimsExistingFiles(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "owned-save.GAM")
	os.WriteFile(sentinel, []byte("PRIVATE SENTINEL"), 0600)
	if e := ensurePrivateRoot(root, d.Trust{}); e == nil {
		t.Fatal("claimed nonempty unknown root")
	}
	if _, e := os.Stat(filepath.Join(root, privateRootMarker)); !os.IsNotExist(e) {
		t.Fatal("ownership marker was written")
	}
	b, _ := os.ReadFile(sentinel)
	if string(b) != "PRIVATE SENTINEL" {
		t.Fatal("changed existing file")
	}
}
func TestPrivateRootRecoveryContract(t *testing.T) {
	root := t.TempDir()
	if e := checkRecoveryContract(root, d.Trust{}); e == nil {
		t.Fatal("unknown root recovery allowed")
	}
	if e := ensurePrivateRoot(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	if e := checkRecoveryContract(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	os.Mkdir(filepath.Join(root, "components"), 0700)
	if e := checkRecoveryContract(root, d.Trust{}); e != nil {
		t.Fatal("interrupted never-started setup should recover", e)
	}
	if e := recordManagerLaunchAttempt(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	if e := checkRecoveryContract(root, d.Trust{}); e == nil {
		t.Fatal("missing activation history allowed")
	}
}
func TestOutOfRangePIDIsUnknown(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "installation.lock"), []byte("4294967296"), 0600)
	called := false
	if _, e := recoveryMarkers(root, func(int) (bool, error) { called = true; return true, nil }); e == nil {
		t.Fatal("invalid PID accepted")
	}
	if called {
		t.Fatal("invalid PID reached process API")
	}
}

func TestMissingLaunchAttemptedIsUnknown(t *testing.T) {
	root := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"schema": 2, "kind": "moo2-private-gui", "created_version": currentVersion, "runtime_history_contract": 1, "trust_fingerprint": trustFingerprint(d.Trust{})})
	os.WriteFile(filepath.Join(root, privateRootMarker), raw, 0600)
	if e := checkPrivateRoot(root, d.Trust{}); e == nil {
		t.Fatal("missing attempted flag treated as false")
	}
	if e := checkRecoveryContract(root, d.Trust{}); e == nil {
		t.Fatal("missing attempted flag authorized recovery")
	}
}

func TestInitialMarkerCannotOverwriteDurableLaunchAttempt(t *testing.T) {
	root := t.TempDir()
	if e := publishInitialContract(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	if e := recordManagerLaunchAttempt(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(root, privateRootMarker))
	if e != nil {
		t.Fatal(e)
	}
	if e := publishInitialContract(root, d.Trust{}); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(root, privateRootMarker))
	if string(before) != string(after) {
		t.Fatal("delayed initial writer reverted durable true to false")
	}
}

func TestCandidateRootDoesNotAdoptPriorCandidate(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, privateRootMarker)
	old := []byte(`{"schema":1,"kind":"moo2-private-gui","created_version":"0.5.0","runtime_history_contract":1,"manager_launch_attempted":true}`)
	if e := os.WriteFile(p, old, 0600); e != nil {
		t.Fatal(e)
	}
	if e := ensurePrivateRoot(root, d.Trust{}); e == nil {
		t.Fatal("prior private candidate silently adopted")
	}
	after, e := os.ReadFile(p)
	if e != nil || string(after) != string(old) {
		t.Fatal("prior candidate marker changed", e)
	}
	got, e := defaultRoot()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(filepath.Base(got), "private-gui-"+currentVersion+"-") {
		t.Fatal("candidate root not version isolated", got)
	}
}

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPresentationFailureRecordsKnownNoStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("negative launch witness uses a Unix process double, not native Windows acceptance")
	}
	dir := communityFixtureCopy(t)
	m, e := newManager(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	generation := "g-prestart-test"
	parent := filepath.Join(m.Data, "environments", "community")
	if e = os.MkdirAll(parent, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(dir, filepath.Join(parent, generation)); e != nil {
		t.Fatal(e)
	}
	if e = atomicJSON(filepath.Join(m.Data, "active", "community.json"), Active{Generation: generation}); e != nil {
		t.Fatal(e)
	}
	p, e := m.loadProfile("community")
	if e != nil {
		t.Fatal(e)
	}
	// This explicitly labelled process double must never run in the failure case.
	witness := filepath.Join(t.TempDir(), "unexpected-start")
	t.Setenv("MOO2_TEST_RUNTIME_WITNESS", witness)
	executable := filepath.Join(t.TempDir(), "dosbox")
	if e = os.WriteFile(executable, []byte("#!/bin/sh\nprintf started > \"$MOO2_TEST_RUNTIME_WITNESS\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = atomicJSON(filepath.Join(m.Data, "settings.json"), Settings{RuntimePath: executable}); e != nil {
		t.Fatal(e)
	}
	// Deterministic blocked selection destination, including root-owned test runs.
	if e = os.Mkdir(filepath.Join(m.Data, "ui-selection.json"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e = m.launch(p); e == nil || !strings.Contains(e.Error(), "ui-selection.json") {
		t.Fatal("preference failure not returned", e)
	}
	if _, e = os.Stat(witness); !os.IsNotExist(e) {
		t.Fatal("runtime started despite pre-start failure", e)
	}
	if m.running != nil || m.runtimeDone != nil {
		t.Fatal("runtime lifecycle began on failure")
	}
	var rec map[string]any
	if e = readJSON(filepath.Join(m.Data, "logs", "last-launch.json"), &rec); e != nil {
		t.Fatal(e)
	}
	if rec["process_start_failed"] != true || rec["process_started"] != false {
		t.Fatal("known no-start left ambiguous runtime history", rec)
	}
	if rec["process_exited"] == true {
		t.Fatal("no process ran; do not invent a normal exit")
	}
	// Repairing only the preference destination allows an ordinary retry. The
	// process double may now run, proving the negative witness was meaningful.
	if e = os.Remove(filepath.Join(m.Data, "ui-selection.json")); e != nil {
		t.Fatal(e)
	}
	if _, e = m.launch(p); e != nil {
		t.Fatal("retry failed", e)
	}
	if e = m.waitForRuntime(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(witness); e != nil {
		t.Fatal("successful retry did not run explicit process double", e)
	}
	rec = map[string]any{}
	if e = readJSON(filepath.Join(m.Data, "logs", "last-launch.json"), &rec); e != nil {
		t.Fatal(e)
	}
	if rec["process_exited"] != true || rec["process_start_failed"] == true {
		t.Fatal("retry retained previous failure marker", rec)
	}
}

func TestKnownLaunchAbortHistoryWriteFailureStaysFailed(t *testing.T) {
	cause := errors.New("presentation could not be saved")
	blocked := filepath.Join(t.TempDir(), "last-launch.json")
	if e := os.Mkdir(blocked, 0700); e != nil {
		t.Fatal(e)
	}
	e := recordKnownLaunchAbort(blocked, map[string]any{"profile": "community"}, cause)
	if !errors.Is(e, cause) || !strings.Contains(e.Error(), "safe-start history could not be written") {
		t.Fatal("journal failure was hidden or launch failure discarded", e)
	}
	st, e := os.Stat(blocked)
	if e != nil || !st.IsDir() {
		t.Fatal("failed journal destination unexpectedly replaced", e)
	}
	entries, e := os.ReadDir(filepath.Dir(blocked))
	if e != nil || len(entries) != 1 {
		t.Fatal("failed journal write left temporary artifacts", e)
	}
}

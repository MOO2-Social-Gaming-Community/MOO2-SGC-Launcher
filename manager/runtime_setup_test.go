package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimeTestPins(t *testing.T, root string) []BaseFile {
	t.Helper()
	var pins []BaseFile
	for _, n := range []string{"dosbox.exe", "SDL2.dll", "resources/data.txt"} {
		b := []byte("test runtime bytes: " + n)
		if e := atomicWrite(filepath.Join(root, filepath.FromSlash(n)), b); e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(b)
		pins = append(pins, BaseFile{Name: n, Size: int64(len(b)), SHA256: hex.EncodeToString(h[:])})
	}
	return pins
}

func TestRuntimePollingDoesNotClaimHashVerification(t *testing.T) {
	root := t.TempDir()
	pins := runtimeTestPins(t, root)
	p := filepath.Join(root, "dosbox.exe")
	b, _ := os.ReadFile(p)
	b[0] ^= 1
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if e := inspectRuntimeTree(root, pins, false); e != nil {
		t.Fatal("metadata polling unexpectedly hashes", e)
	}
	if e := inspectRuntimeTree(root, pins, true); e == nil {
		t.Fatal("launch-quality verification accepted changed bytes")
	}
	m := newPortableForTest(t)
	s := m.portableRuntimeStatus()
	if s.Ready || s.Verified || s.Status != "missing" {
		t.Fatal(s)
	}
	if e := os.MkdirAll(filepath.Dir(m.harnessRuntimePath()), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(m.harnessRuntimePath(), []byte("incomplete"), 0600); e != nil {
		t.Fatal(e)
	}
	s = m.portableRuntimeStatus()
	if s.Ready || s.Verified || !s.RepairNeeded {
		t.Fatal(s)
	}
}

func TestCopyPinnedRuntimePreservesInputAndRejectsExtraCode(t *testing.T) {
	source := t.TempDir()
	pins := runtimeTestPins(t, source)
	dest := filepath.Join(t.TempDir(), "copy")
	if e := copyPinnedRuntime(source, dest, pins); e != nil {
		t.Fatal(e)
	}
	if e := inspectRuntimeTree(source, pins, true); e != nil {
		t.Fatal("source changed", e)
	}
	if e := inspectRuntimeTree(dest, pins, true); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(source, "untrusted.dll"), []byte("extra"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := copyPinnedRuntime(source, filepath.Join(t.TempDir(), "copy"), pins); e == nil {
		t.Fatal("unmanaged executable accepted")
	}
}

func TestCopyPinnedRuntimeZipUsesContentsNotFilename(t *testing.T) {
	source := t.TempDir()
	pins := runtimeTestPins(t, source)
	archive := filepath.Join(t.TempDir(), "renamed backup.zip")
	f, e := os.Create(archive)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for _, pin := range pins {
		w, e := z.Create("nested/runtime/windows/" + pin.Name)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := os.ReadFile(filepath.Join(source, pin.Name))
		if _, e = w.Write(b); e != nil {
			t.Fatal(e)
		}
	}
	// A source is a collection of inputs, not an installer to execute.
	w, _ := z.Create("other/never-run.cmd")
	_, _ = w.Write([]byte("arbitrary script"))
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(archive)
	dest := filepath.Join(t.TempDir(), "copy")
	if e = copyPinnedRuntimeZip(archive, dest, pins); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(archive)
	if !bytes.Equal(before, after) {
		t.Fatal("source archive changed")
	}
	if exists(filepath.Join(dest, "never-run.cmd")) || exists(filepath.Join(dest, "other")) {
		t.Fatal("unrecognized source entries copied")
	}
}

func TestCommitRuntimeRetainsPartialInstallation(t *testing.T) {
	parent := t.TempDir()
	old := filepath.Join(parent, "windows", "old-note.txt")
	if e := atomicWrite(old, []byte("previous tree")); e != nil {
		t.Fatal(e)
	}
	stage := filepath.Join(parent, "stage-one", "files")
	pins := runtimeTestPins(t, stage)
	backup, e := commitRuntimeTree(parent, stage, func(p string) error { return inspectRuntimeTree(p, pins, true) })
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(backup, "old-note.txt"))
	if e != nil || string(b) != "previous tree" {
		t.Fatal("old runtime not retained", e)
	}
	if e = inspectRuntimeTree(filepath.Join(parent, "windows"), pins, true); e != nil {
		t.Fatal(e)
	}
}

func TestCommitRuntimeRejectsInvalidStageWithoutChangingActive(t *testing.T) {
	parent := t.TempDir()
	old := filepath.Join(parent, "windows", "old-note.txt")
	if e := atomicWrite(old, []byte("previous tree")); e != nil {
		t.Fatal(e)
	}
	stage := filepath.Join(parent, "stage-one", "files")
	_ = runtimeTestPins(t, stage)
	if _, e := commitRuntimeTree(parent, stage, func(string) error { return errors.New("checksum mismatch") }); e == nil {
		t.Fatal("verification failure ignored")
	}
	if _, e := commitRuntimeTree(parent, t.TempDir(), func(string) error { return nil }); e == nil {
		t.Fatal("outside stage accepted")
	}
	b, e := os.ReadFile(old)
	if e != nil || string(b) != "previous tree" {
		t.Fatal("previous runtime changed", e)
	}
}

func TestRuntimeImportCannotMoveItsInput(t *testing.T) {
	m := newPortableForTest(t)
	if e := atomicWrite(m.harnessRuntimePath(), []byte("partial")); e != nil {
		t.Fatal(e)
	}
	if _, e := m.importWindowsRuntime(m.Root); e == nil || !strings.Contains(e.Error(), "already the managed runtime") {
		t.Fatal(e)
	}
	b, e := os.ReadFile(m.harnessRuntimePath())
	if e != nil || string(b) != "partial" {
		t.Fatal("input changed", e)
	}
}

func TestPrivateRuntimeGUIImportAndRepair(t *testing.T) {
	source := os.Getenv("MOO2_HARNESS_FIXTURE")
	if source == "" {
		t.Skip("private exact runtime fixture not supplied")
	}
	if e := verifyHarnessRuntime(source); e != nil {
		t.Fatal(e)
	}
	m := newPortableForTest(t)
	if _, e := m.importWindowsRuntime(source); e != nil {
		t.Fatal(e)
	}
	s := m.portableRuntimeStatus()
	if !s.Ready || s.Verified || s.RepairNeeded {
		t.Fatal(s)
	}
	if _, e := m.localHarnessRuntime(); e != nil {
		t.Fatal(e)
	}
	broken := filepath.Join(filepath.Dir(m.harnessRuntimePath()), "SDL2.dll")
	if e := os.WriteFile(broken, []byte("damaged managed copy"), 0600); e != nil {
		t.Fatal(e)
	}
	if !m.portableRuntimeStatus().RepairNeeded {
		t.Fatal("damage not reported")
	}
	v, e := m.importWindowsRuntime(source)
	if e != nil {
		t.Fatal(e)
	}
	backup := v.(map[string]any)["previous_runtime"].(string)
	b, e := os.ReadFile(filepath.Join(backup, "SDL2.dll"))
	if e != nil || string(b) != "damaged managed copy" {
		t.Fatal("prior damaged runtime not retained", e)
	}
	if _, e = m.localHarnessRuntime(); e != nil {
		t.Fatal(e)
	}
	if e = verifyHarnessRuntime(source); e != nil {
		t.Fatal("source mutated", e)
	}
}

func TestPrivateRuntimeGUIArchiveImport(t *testing.T) {
	source := os.Getenv("MOO2_HARNESS_ARCHIVE_FIXTURE")
	if source == "" {
		t.Skip("private exact runtime archive fixture not supplied")
	}
	before, e := hashFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m := newPortableForTest(t)
	if _, e = m.importWindowsRuntime(source); e != nil {
		t.Fatal(e)
	}
	if _, e = m.localHarnessRuntime(); e != nil {
		t.Fatal(e)
	}
	after, e := hashFile(source)
	if e != nil || before != after {
		t.Fatal("source archive changed", e)
	}
}

func TestOfficialRuntimeGUIArchiveInstallAndRepair(t *testing.T) {
	source := os.Getenv("MOO2_OFFICIAL_RUNTIME_FIXTURE")
	if source == "" {
		t.Skip("original official Windows runtime archive not supplied")
	}
	if e := requireHash(source, WindowsRuntimeArchiveSHA256); e != nil {
		t.Fatal(e)
	}
	m := newPortableForTest(t)
	if _, e := m.installWindowsRuntimeArchive(source); e != nil {
		t.Fatal(e)
	}
	if _, e := m.localHarnessRuntime(); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(m.harnessRuntimePath()); e != nil {
		t.Fatal(e)
	}
	if !m.portableRuntimeStatus().RepairNeeded {
		t.Fatal("missing executable not reported as repairable")
	}
	if _, e := m.installWindowsRuntimeArchive(source); e != nil {
		t.Fatal(e)
	}
	if _, e := m.localHarnessRuntime(); e != nil {
		t.Fatal(e)
	}
	if e := requireHash(source, WindowsRuntimeArchiveSHA256); e != nil {
		t.Fatal("official source changed", e)
	}
}

func TestOfficialRuntimeRejectsUnverifiedArchive(t *testing.T) {
	m := newPortableForTest(t)
	source := filepath.Join(t.TempDir(), "download.zip")
	if e := os.WriteFile(source, []byte("not the approved download"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := m.installWindowsRuntimeArchive(source); e == nil {
		t.Fatal("unverified archive accepted")
	}
	if exists(filepath.Dir(m.harnessRuntimePath())) {
		t.Fatal("runtime created before package verification")
	}
}

package main

// Runtime setup is always local to the isolated launcher installation. A
// browsed runtime is an input, never the destination of repair or cleanup.
import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type RuntimeStatus struct {
	Status       string `json:"status"`
	Ready        bool   `json:"ready"`
	Verified     bool   `json:"verified"`
	RepairNeeded bool   `json:"repair_needed"`
	Path         string `json:"path,omitempty"`
	Message      string `json:"message"`
}

func (m *Manager) runtimeStatus() RuntimeStatus {
	if m.Portable && runtime.GOOS == "windows" {
		return m.portableRuntimeStatus()
	}
	candidates := m.runtimeCandidates()
	if len(candidates) == 0 {
		return RuntimeStatus{Status: "missing", Message: "DOSBox is not installed. Prepare game downloads the approved runtime, or choose an existing executable."}
	}
	return RuntimeStatus{Status: "available", Ready: true, Path: candidates[0], Message: "DOSBox executable is available. This platform's selected executable is not certified by the launcher."}
}

func (m *Manager) portableRuntimeStatus() RuntimeStatus {
	p := m.harnessRuntimePath()
	s := RuntimeStatus{Status: "missing", Path: p, Message: "DOSBox is not installed. Prepare game will download the verified official runtime."}
	if _, e := os.Lstat(filepath.Dir(p)); os.IsNotExist(e) {
		return s
	}
	if e := inspectRuntimeTree(filepath.Dir(p), harnessRuntimeIndex().Files, false); e != nil {
		s.Status, s.RepairNeeded = "repair-needed", true
		s.Message = "Runtime needs repair. Choose Download / repair DOSBox, or import a verified runtime copy. " + e.Error()
		return s
	}
	s.Status, s.Ready = "installed", true
	// Do not claim cryptographic verification based on a fast polling check.
	// Prepare and every launch perform the complete checksum verification.
	s.Message = "DOSBox is installed. Complete checksum verification runs before Play."
	return s
}

func inspectRuntimeTree(dir string, files []BaseFile, hashes bool) error {
	if e := noSymlinkAncestors(dir); e != nil {
		return e
	}
	expected := map[string]bool{}
	for _, r := range files {
		rel, e := safeRel(r.Name)
		if e != nil {
			return e
		}
		p := filepath.Join(dir, rel)
		if e = noSymlinkAncestors(p); e != nil {
			return e
		}
		if e = regularFile(p); e != nil {
			return fmt.Errorf("required runtime file %s: %w", r.Name, e)
		}
		st, e := os.Stat(p)
		if e != nil {
			return e
		}
		if st.Size() != r.Size {
			return fmt.Errorf("runtime file size differs: %s", r.Name)
		}
		if hashes {
			if e = requireHash(p, r.SHA256); e != nil {
				return e
			}
		}
		expected[strings.ToLower(r.Name)] = true
	}
	return walkRegular(dir, func(n, p string) error {
		if expected[strings.ToLower(n)] {
			return nil
		}
		switch strings.ToLower(filepath.Ext(n)) {
		case ".dll", ".exe", ".com", ".conf":
			return fmt.Errorf("unmanaged runtime code/config: %s", n)
		}
		return nil
	})
}

func copyPinnedRuntime(source, dest string, files []BaseFile) error {
	if e := inspectRuntimeTree(source, files, true); e != nil {
		return e
	}
	for _, r := range files {
		rel, e := safeRel(r.Name)
		if e != nil {
			return e
		}
		if e = copyFile(filepath.Join(source, rel), filepath.Join(dest, rel)); e != nil {
			return e
		}
	}
	// Recheck copied bytes too; the browsed source could change while copying.
	return inspectRuntimeTree(dest, files, true)
}

func copyPinnedRuntimeZip(source, dest string, files []BaseFile) error {
	if e := noSymlinkAncestors(source); e != nil {
		return e
	}
	if e := regularFile(source); e != nil {
		return e
	}
	z, e := zip.OpenReader(source)
	if e != nil {
		return e
	}
	defer z.Close()
	entries, e := zipSourceFiles(z)
	if e != nil {
		return e
	}
	prefix, e := zipRoot(entries, "DOSBOX.EXE")
	if e != nil {
		return fmt.Errorf("choose a ZIP with one complete DOSBox runtime: %w", e)
	}
	for _, r := range files {
		rel, e := safeRel(r.Name)
		if e != nil {
			return e
		}
		b, e := readZipKnown(entries[prefix+strings.ToUpper(r.Name)], r.Size)
		if e != nil {
			return fmt.Errorf("runtime archive file %s: %w", r.Name, e)
		}
		h := sha256.Sum256(b)
		if int64(len(b)) != r.Size || hex.EncodeToString(h[:]) != r.SHA256 {
			return fmt.Errorf("runtime archive file differs from the approved runtime: %s", r.Name)
		}
		if e = atomicWrite(filepath.Join(dest, rel), b); e != nil {
			return e
		}
	}
	return inspectRuntimeTree(dest, files, true)
}

// A short rename transaction keeps the previous tree, including a partial or
// damaged installation, for recovery. Nothing from a browsed source is moved.
// If interrupted between renames, the next graphical repair can install a new
// tree; the previous tree is still retained under previous-windows-*/files.
func commitRuntimeTree(parent, staged string, verify func(string) error) (string, error) {
	if e := noSymlinkAncestors(parent); e != nil {
		return "", e
	}
	rel, e := filepath.Rel(parent, staged)
	if e != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("runtime commit requires an operation-owned stage inside the managed runtime directory")
	}
	if e = verify(staged); e != nil {
		return "", e
	}
	dest := filepath.Join(parent, "windows")
	if filepath.Clean(staged) == filepath.Clean(dest) {
		return "", errors.New("runtime staging path is already active")
	}
	if e = noSymlinkAncestors(dest); e != nil {
		return "", e
	}
	backup := ""
	if st, err := os.Lstat(dest); err == nil {
		if !st.IsDir() {
			return "", errors.New("managed runtime destination is not a directory; existing file was not changed")
		}
		holder, err := os.MkdirTemp(parent, "previous-windows-")
		if err != nil {
			return "", err
		}
		backup = filepath.Join(holder, "files")
		if err = os.Rename(dest, backup); err != nil {
			_ = os.Remove(holder)
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if e = os.Rename(staged, dest); e != nil {
		if backup != "" {
			if rollback := os.Rename(backup, dest); rollback != nil {
				return backup, fmt.Errorf("runtime activation failed: %v; previous runtime retained at %s (restore failed: %w)", e, backup, rollback)
			}
		}
		return backup, e
	}
	return backup, nil
}

func runtimeSourceFolder(source string) (string, error) {
	for _, dir := range []string{source, filepath.Join(source, "runtime", "windows"), filepath.Join(source, "windows")} {
		if e := regularFile(filepath.Join(dir, "dosbox.exe")); e == nil {
			return dir, nil
		}
	}
	return "", errors.New("choose the complete DOSBox runtime folder, a launcher folder containing runtime/windows, or its ZIP; the source will stay unchanged")
}

func (m *Manager) importRuntime(source string) (any, error) {
	if !m.Portable || runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return nil, errors.New("verified Windows runtime import is available in the Windows 64-bit portable launcher; other platforms can select an existing native executable")
	}
	return m.importWindowsRuntime(source)
}

// The normal download path and the optional browsed ZIP path use the same
// verified file-copy transaction. This archive pin is an extra provenance
// check for the official download, not a dependency on an old private harness.
func (m *Manager) installWindowsRuntimeArchive(archive string) (any, error) {
	if e := requireHash(archive, WindowsRuntimeArchiveSHA256); e != nil {
		return nil, fmt.Errorf("official DOSBox archive was not verified; no runtime was replaced: %w", e)
	}
	v, e := m.importWindowsRuntime(archive)
	if e != nil {
		return nil, e
	}
	r := v.(map[string]any)
	r["archive_sha256"] = WindowsRuntimeArchiveSHA256
	r["verification"] = "official archive and all 660 installed files matched compiled SHA-256"
	return r, nil
}

func (m *Manager) importWindowsRuntime(source string) (any, error) {
	p, e := localPath(source)
	if e != nil {
		return nil, e
	}
	st, e := os.Stat(p)
	if e != nil {
		return nil, e
	}
	folder := ""
	if st.IsDir() {
		folder, e = runtimeSourceFolder(p)
		if e != nil {
			return nil, e
		}
		p = folder
	}
	active := filepath.Dir(m.harnessRuntimePath())
	if rel, err := filepath.Rel(active, p); err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)) {
		// Do not move an input that happens to be within our destination tree.
		return nil, errors.New("this is already the managed runtime; choose Download / repair DOSBox instead, or select a separate original runtime copy")
	}
	if e = m.checkCancellation(); e != nil {
		return nil, e
	}
	parent := filepath.Join(m.Root, "runtime")
	if e = noSymlinkAncestors(parent); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(parent, 0700); e != nil {
		return nil, e
	}
	stage, e := os.MkdirTemp(parent, "runtime-import-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	tree := filepath.Join(stage, "files")
	m.progress("Verifying and copying the selected runtime into this launcher; the original stays unchanged…")
	if st.IsDir() {
		if e = copyPinnedRuntime(folder, tree, harnessRuntimeIndex().Files); e != nil {
			return nil, e
		}
	} else if strings.EqualFold(filepath.Ext(p), ".zip") {
		if e = copyPinnedRuntimeZip(p, tree, harnessRuntimeIndex().Files); e != nil {
			return nil, e
		}
	} else {
		return nil, errors.New("select a complete runtime folder or ZIP, not just dosbox.exe")
	}
	if e = m.checkCancellation(); e != nil {
		return nil, e
	}
	if e = m.beginRuntimeCommit(); e != nil {
		return nil, e
	}
	defer m.endRuntimeCommit()
	backup, e := commitRuntimeTree(parent, tree, verifyHarnessRuntime)
	if e != nil {
		return nil, e
	}
	return map[string]any{"runtime": m.harnessRuntimePath(), "version": "0.83.0", "verification": "all required runtime files matched compiled SHA-256", "source_modified": false, "previous_runtime": backup}, nil
}

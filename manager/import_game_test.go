package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRecord(name string, b []byte) BaseFile {
	h := sha256.Sum256(b)
	return BaseFile{name, int64(len(b)), hex.EncodeToString(h[:])}
}
func TestFolderSnapshotBoundary(t *testing.T) {
	cases := []struct {
		name    string
		change  func(string)
		wantErr bool
	}{
		{"recognized", func(string) {}, false},
		{"modified", func(p string) { os.WriteFile(filepath.Join(p, "orion2.exe"), []byte("baD"), 0600) }, true},
		{"missing", func(p string) { os.Remove(filepath.Join(p, "orion2.exe")) }, true},
		{"wrong-size", func(p string) { os.WriteFile(filepath.Join(p, "orion2.exe"), []byte("longer"), 0600) }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			os.Mkdir(source, 0700)
			os.WriteFile(filepath.Join(source, "orion2.exe"), []byte("DOS"), 0600)
			os.WriteFile(filepath.Join(source, "GAME.LBX"), []byte("data"), 0600)
			os.WriteFile(filepath.Join(source, "SAVE1.GAM"), []byte("private save"), 0600)
			os.WriteFile(filepath.Join(source, "unrelated.exe"), []byte("do not import"), 0600)
			c.change(source)
			out := filepath.Join(root, "snapshot.zip")
			e := writeBaseSnapshot(source, out, []BaseFile{testRecord("ORION2.EXE", []byte("DOS")), testRecord("GAME.LBX", []byte("data"))})
			if (e != nil) != c.wantErr {
				t.Fatal(e)
			}
			if c.wantErr {
				if _, e = os.Stat(out); !os.IsNotExist(e) {
					t.Fatal("failed snapshot retained")
				}
				return
			}
			z, e := zip.OpenReader(out)
			if e != nil {
				t.Fatal(e)
			}
			defer z.Close()
			if len(z.File) != 2 {
				t.Fatal("extra files copied")
			}
			for _, f := range z.File {
				if !strings.HasPrefix(f.Name, "base/") {
					t.Fatal(f.Name)
				}
			}
			b, _ := os.ReadFile(filepath.Join(source, "SAVE1.GAM"))
			if string(b) != "private save" {
				t.Fatal("source save modified")
			}
		})
	}
}
func TestBaseIndexIsOnlyFingerprints(t *testing.T) {
	v := baseIndex()
	if v.Schema != 1 || len(v.Files) < 100 {
		t.Fatal("missing index")
	}
	seen := map[string]bool{}
	for _, f := range v.Files {
		if seen[f.Name] || strings.ContainsAny(f.Name, "/\\") || len(f.SHA256) != 64 || f.Size <= 0 {
			t.Fatal(f.Name)
		}
		seen[f.Name] = true
	}
	if !seen["ORION2.EXE"] || !seen["GAME.LBX"] {
		t.Fatal("missing essentials")
	}
}
func TestImportedBaseTraversal(t *testing.T) {
	m, e := newManager(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r := ImportedBase{1, "../../outside.zip", strings.Repeat("a", 64), len(baseIndex().Files), "test"}
	if e = atomicJSON(filepath.Join(m.Data, "cache", "base-source.json"), r); e != nil {
		t.Fatal(e)
	}
	if _, _, _, e = m.importedBase(); e == nil {
		t.Fatal("unsafe pointer accepted")
	}
}
func TestPrepareRejectsMissingSourceBeforeDownloads(t *testing.T) {
	m, e := newManager(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p := defaultProfiles()[0]
	if _, e = m.prepareForPlay(p, filepath.Join(t.TempDir(), "missing.zip")); e == nil {
		t.Fatal("missing source accepted")
	}
}
func TestLocalImportRequiresAbsolutePath(t *testing.T) {
	for _, p := range []string{"", "relative", "../source"} {
		if _, e := localPath(p); e == nil {
			t.Fatal(p)
		}
	}
}

func TestFolderWorkspaceVerificationPinsActualFiles(t *testing.T) {
	root := t.TempDir()
	want := testRecord("GAME.LBX", []byte("known"))
	path := filepath.Join(root, want.Name)
	os.WriteFile(path, []byte("known"), 0600)
	installed := map[string]FileRecord{want.Name: {want.SHA256, want.Size, false}}
	if e := verifyPinnedBase(root, installed, []BaseFile{want}); e != nil {
		t.Fatal(e)
	}
	// Changing both the file and its local receipt does not change compiled trust.
	os.WriteFile(path, []byte("other"), 0600)
	changed := testRecord(want.Name, []byte("other"))
	installed[want.Name] = FileRecord{changed.SHA256, changed.Size, false}
	if e := verifyPinnedBase(root, installed, []BaseFile{want}); e == nil {
		t.Fatal("changed source and receipt accepted")
	}
	delete(installed, want.Name)
	if e := verifyPinnedBase(root, installed, []BaseFile{want}); e == nil {
		t.Fatal("missing base member accepted")
	}
}
func TestUnknownWorkspaceSourceKindRejected(t *testing.T) {
	for _, m := range []Manifest{
		{SourceBase: BaseHash, SourceBaseKind: "anything"},
		{SourceBase: strings.Repeat("a", 64)},
		{SourceBase: "bad", SourceBaseKind: "fingerprinted-dos-131"},
	} {
		if e := verifyBaseIdentity(m, t.TempDir()); e == nil {
			t.Fatal("unknown source accepted")
		}
	}
	if e := verifyBaseIdentity(Manifest{SourceBase: BaseHash}, t.TempDir()); e == nil {
		t.Fatal("historical archive provenance accepted without installed files")
	}
}

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func payloadFixture(t *testing.T) (fstest.MapFS, d.Trust) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	trust := d.Trust{Schema: 1, Feed: "moo2-desktop-test", Channel: "stable", Development: true, Keys: map[string]string{"test": hex.EncodeToString(pub)}}
	body := []byte("#!/bin/sh\nprintf '" + currentVersion + "\\n'\n")
	fileSum := sha256.Sum256(body)
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	h := &zip.FileHeader{Name: "launcher", Method: zip.Deflate}
	h.SetMode(0755)
	w, e := z.CreateHeader(h)
	if e != nil {
		t.Fatal(e)
	}
	w.Write(body)
	z.Close()
	sum := sha256.Sum256(archive.Bytes())
	p := d.Package{ID: "launcher", Kind: "launcher", Version: currentVersion, Platform: "linux-amd64", Filename: "launcher.zip", Format: "zip", Entrypoint: "launcher", Size: int64(archive.Len()), SHA256: hex.EncodeToString(sum[:]), Redistribution: "project-owned", Files: map[string]d.FileRecord{"launcher": {SHA256: hex.EncodeToString(fileSum[:]), Size: int64(len(body))}}}
	p.Signature = d.Sign(d.PackageMessage(p), "test", key)
	m := d.Manifest{Schema: 1, Feed: trust.Feed, Channel: trust.Channel, Revision: 1, Release: currentVersion, Published: time.Now().Add(-time.Minute), Expires: time.Now().Add(time.Hour), Packages: []d.Package{p}}
	b, _ := json.Marshal(m)
	sig, _ := json.Marshal(d.SignManifest(b, "test", key))
	return fstest.MapFS{"embedded/manifest.json": {Data: b}, "embedded/manifest.sig": {Data: sig}, "embedded/launcher.zip": {Data: archive.Bytes()}}, trust
}
func TestEmbeddedPayloadRejectsTampering(t *testing.T) {
	for _, mutate := range []string{"none", "manifest", "signature", "archive", "missing", "wrong-platform"} {
		t.Run(mutate, func(t *testing.T) {
			files, trust := payloadFixture(t)
			platform := "linux-amd64"
			switch mutate {
			case "manifest":
				files["embedded/manifest.json"].Data = append(files["embedded/manifest.json"].Data, ' ')
			case "signature":
				files["embedded/manifest.sig"].Data = []byte(`{}`)
			case "archive":
				files["embedded/launcher.zip"].Data = append(files["embedded/launcher.zip"].Data, 'x')
			case "missing":
				delete(files, "embedded/manifest.json")
			case "wrong-platform":
				platform = "darwin-arm64"
			}
			root := t.TempDir()
			_, _, _, e := verifiedPayload(files, trust, root, platform)
			if mutate == "none" && e != nil {
				t.Fatal(e)
			}
			if mutate != "none" && e == nil {
				t.Fatal("accepted invalid payload")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatal("verification mutated destination")
			}
		})
	}
}
func TestEnsureInstalledOfflineReuseAndRepair(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell health fixture is Unix-only; Windows native executable test remains separate")
	}
	files, trust := payloadFixture(t)
	root := t.TempDir()
	messages := []string{}
	progress := func(s string) { messages = append(messages, s) }
	entry, version, e := ensureInstalled(context.Background(), root, "linux-amd64", trust, files, progress)
	if e != nil {
		t.Fatal(e)
	}
	if version != currentVersion {
		t.Fatal("version")
	}
	if len(messages) < 2 {
		t.Fatal("no visible progress")
	}
	data := filepath.Join(root, "userdata", "save.GAM")
	os.MkdirAll(filepath.Dir(data), 0700)
	os.WriteFile(data, []byte("SYNTHETIC SAVE"), 0600)
	again, _, e := ensureInstalled(context.Background(), root, "linux-amd64", trust, fstest.MapFS{}, progress)
	if e != nil || again != entry {
		t.Fatalf("installed offline reuse: %v", e)
	}
	os.WriteFile(entry, []byte("broken"), 0700)
	repaired, _, e := ensureInstalled(context.Background(), root, "linux-amd64", trust, files, progress)
	if e != nil {
		t.Fatal(e)
	}
	if repaired == entry {
		t.Fatal("overwrote immutable generation")
	}
	b, _ := os.ReadFile(data)
	if string(b) != "SYNTHETIC SAVE" {
		t.Fatal("userdata changed")
	}
	for _, part := range []string{".key", ".pem", ".lbx"} {
		filepath.WalkDir(root, func(p string, _ fs.DirEntry, _ error) error {
			if strings.HasSuffix(p, part) {
				t.Fatal("unexpected private/game payload")
			}
			return nil
		})
	}
}

package main

import (
	"encoding/json"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
	"testing"
)

func TestSameVersionDifferentTrustUsesSeparateRoots(t *testing.T) {
	a := d.Trust{Keys: map[string]string{"development-offline": "public-key-a"}}
	b := d.Trust{Keys: map[string]string{"development-offline": "public-key-b"}}
	ra, err := defaultRootForTrust(a)
	if err != nil {
		t.Fatal(err)
	}
	again, err := defaultRootForTrust(a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := defaultRootForTrust(b)
	if err != nil {
		t.Fatal(err)
	}
	if ra != again || ra == rb {
		t.Fatalf("trust-isolation/reuse failed: %q %q %q", ra, again, rb)
	}
	root := t.TempDir()
	if err := ensurePrivateRoot(root, a); err != nil {
		t.Fatal(err)
	}
	if err := recordManagerLaunchAttempt(root, a); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "synthetic-save.GAM")
	if err := os.WriteFile(sentinel, []byte("save stays unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, privateRootMarker))
	if err := ensurePrivateRoot(root, a); err != nil {
		t.Fatal("same build did not reuse root", err)
	}
	if err := ensurePrivateRoot(root, b); err == nil {
		t.Fatal("rebuilt same-version trust adopted explicit old root")
	}
	if err := checkRecoveryContract(root, b); err == nil {
		t.Fatal("wrong trust authorized recovery")
	}
	after, _ := os.ReadFile(filepath.Join(root, privateRootMarker))
	saved, _ := os.ReadFile(sentinel)
	if string(after) != string(before) || string(saved) != "save stays unchanged" {
		t.Fatal("old data changed")
	}
}

func TestFullTrustFingerprintRequiredEvenWithMatchingShortPrefix(t *testing.T) {
	trust := d.Trust{Keys: map[string]string{"a": "public-a", "b": "public-b"}}
	reordered := d.Trust{Keys: map[string]string{"b": "public-b", "a": "public-a"}}
	if trustFingerprint(trust) != trustFingerprint(reordered) {
		t.Fatal("map ordering changed identity")
	}
	root := t.TempDir()
	if err := ensurePrivateRoot(root, trust); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, privateRootMarker)
	raw, _ := os.ReadFile(path)
	var c rootContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	c.TrustFingerprint = c.TrustFingerprint[:16] + "000000000000000000000000000000000000000000000000"
	if err := d.AtomicJSON(path, c); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateRoot(root, trust); err == nil {
		t.Fatal("short prefix allowed conflicting full identity")
	}
}

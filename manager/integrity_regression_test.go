package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The fixture stays private. Copy it before testing: never alter an owned source
// or a prepared environment another test/actual emulator may be using.
func communityFixtureCopy(t *testing.T) string {
	t.Helper()
	source := os.Getenv("MOO2_COMMUNITY_FIXTURE")
	if source == "" {
		t.Skip("private prepared Community fixture not supplied")
	}
	dir := t.TempDir()
	if e := walkRegular(source, func(n, src string) error { return copyFile(src, filepath.Join(dir, filepath.FromSlash(n))) }); e != nil {
		t.Fatal(e)
	}
	v, e := verifyDir(dir)
	if e != nil || !v.OK {
		t.Fatalf("initial fixture invalid: %+v, %v", v, e)
	}
	return dir
}
func TestAddedEditableConfigurationChangesFingerprint(t *testing.T) {
	dir := communityFixtureCopy(t)
	initial, _ := verifyDir(dir)
	file := filepath.Join(dir, "game", "150", "build", "CUSTOM.CFG")
	if e := os.WriteFile(file, []byte("# first custom config\n"), 0600); e != nil {
		t.Fatal(e)
	}
	first, e := verifyDir(dir)
	if e != nil || !first.OK {
		t.Fatal(first, e)
	}
	if first.Fingerprint == initial.Fingerprint {
		t.Error("new editable CFG omitted from compatibility fingerprint")
	}
	if e = os.WriteFile(file, []byte("# changed custom config\n"), 0600); e != nil {
		t.Fatal(e)
	}
	second, e := verifyDir(dir)
	if e != nil || !second.OK {
		t.Fatal(second, e)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Error("changed editable CFG omitted from compatibility fingerprint")
	}
	if e = os.Remove(file); e != nil {
		t.Fatal(e)
	}
	final, e := verifyDir(dir)
	if e != nil || final.Fingerprint != initial.Fingerprint {
		t.Fatal("deleting optional config did not restore identity", final, e)
	}
}
func TestMissingSelectedConfigurationCannotHideInManifest(t *testing.T) {
	dir := communityFixtureCopy(t)
	var mf Manifest
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		t.Fatal(e)
	}
	delete(mf.Files, "150/ENABLE.CFG")
	if e := atomicJSON(filepath.Join(dir, "manifest.json"), mf); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(dir, "game", "150", "ENABLE.CFG")); e != nil {
		t.Fatal(e)
	}
	v, e := verifyDir(dir)
	if e == nil && v.OK {
		t.Fatal("missing selected-mod config accepted after local receipt edit")
	}
}
func TestPatchCodeCannotBeBlessedByLocalManifest(t *testing.T) {
	dir := communityFixtureCopy(t)
	var mf Manifest
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		t.Fatal(e)
	}
	name := "150/scripts/main/MAIN0.LUA"
	file := filepath.Join(dir, "game", filepath.FromSlash(name))
	b := []byte("-- changed local script\n")
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	h, e := hashFile(file)
	if e != nil {
		t.Fatal(e)
	}
	mf.Files[name] = FileRecord{SHA256: h, Size: int64(len(b))}
	if e := atomicJSON(filepath.Join(dir, "manifest.json"), mf); e != nil {
		t.Fatal(e)
	}
	v, e := verifyDir(dir)
	if e == nil && v.OK {
		t.Fatal("modified patch code accepted after local receipt edit")
	}
}

func TestLegacyReceiptStillRequiresCompiledBaseIdentity(t *testing.T) {
	mf := Manifest{Schema: 1, SourceBase: BaseHash, Files: map[string]FileRecord{}}
	if e := verifyBaseIdentity(mf, t.TempDir()); e == nil {
		t.Fatal("legacy archive hash bypassed installed-file identity")
	}
}
func TestPatchInventoryContainsRequiredCodeAndConfig(t *testing.T) {
	index := patchIndex()
	if len(index.Files) != 128 {
		t.Fatal("unexpected patch inventory size", len(index.Files))
	}
	required := map[string]bool{"ORION150.EXE": false, "ORION2.CFG": false, "150/ENABLE.CFG": false, "150/USER.CFG": false, "150/scripts/main/MAIN0.LUA": false}
	seen := map[string]bool{}
	for _, p := range index.Files {
		if _, e := safeRel(p.Name); e != nil || seen[p.Name] || len(p.SHA256) != 64 || p.Size < 0 {
			t.Fatal("invalid patch pin", p)
		}
		seen[p.Name] = true
		if _, ok := required[p.Name]; ok {
			required[p.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatal("required patch pin missing", name)
		}
	}
}

// Only a fully verified Steam-derived fixture can lack the historical 1.31
// executable legitimately. An explicitly supplied legacy fixture must satisfy
// the legacy case; missing/wrong bytes never become a successful assertion.
func skipLegacyReceiptForSteam(mf Manifest, missing bool, explicitLegacy bool) bool {
	if !missing || explicitLegacy {
		return false
	}
	return mf.SourceBaseKind == "known-edition-v2:steam-en-140b23" || mf.SourceBaseKind == "known-edition-v2:steam-en-140b23-harness"
}

func TestLegacyReceiptSkipIsLineageSpecific(t *testing.T) {
	for _, kind := range []string{"known-edition-v2:steam-en-140b23", "known-edition-v2:steam-en-140b23-harness", "known-edition-v2:manual-en-140b23", "known-edition-v2:legacy-en-131", "unknown", ""} {
		steam := kind == "known-edition-v2:steam-en-140b23" || kind == "known-edition-v2:steam-en-140b23-harness"
		for _, missing := range []bool{false, true} {
			for _, explicit := range []bool{false, true} {
				if got := skipLegacyReceiptForSteam(Manifest{SourceBaseKind: kind}, missing, explicit); got != (steam && missing && !explicit) {
					t.Fatalf("kind=%s missing=%v explicit=%v skip=%v", kind, missing, explicit, got)
				}
			}
		}
	}
}

func TestLegacyReceiptKeepsValidFilesAndRejectsForgedBase(t *testing.T) {
	legacy := os.Getenv("MOO2_LEGACY_COMMUNITY_FIXTURE")
	if legacy != "" {
		t.Setenv("MOO2_COMMUNITY_FIXTURE", legacy)
	}
	dir := communityFixtureCopy(t)
	game := filepath.Join(dir, "game")
	var mf Manifest
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		t.Fatal(e)
	}
	// communityFixtureCopy above verifies the entire fixture before this gate.
	// Steam starts at 1.40b23 and has no retained ORION131.EXE; do not invent one.
	_, statErr := os.Stat(filepath.Join(game, "ORION131.EXE"))
	if skipLegacyReceiptForSteam(mf, os.IsNotExist(statErr), legacy != "") {
		t.Skip("verified Steam 1.40b23 Community fixture has no official 1.31 executable; set MOO2_LEGACY_COMMUNITY_FIXTURE to a genuine legacy-derived Community fixture for this 1.31-dependent case")
	}
	if e := copyFile(filepath.Join(game, "ORION131.EXE"), filepath.Join(game, "ORION2.EXE")); e != nil {
		t.Fatal(e)
	}
	for _, r := range baseIndex().Files {
		mf.Files[r.Name] = FileRecord{SHA256: r.SHA256, Size: r.Size, Mutable: isUserFile(r.Name)}
	}
	mf.Schema = 1
	mf.SourceBaseKind = ""
	mf.SourceBase = BaseHash
	if e := verifyBaseIdentity(mf, game); e != nil {
		t.Fatal("valid legacy bytes must remain supported", e)
	}
	b := []byte("changed base data")
	file := filepath.Join(game, "GAME.LBX")
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	h, e := hashFile(file)
	if e != nil {
		t.Fatal(e)
	}
	mf.Files["GAME.LBX"] = FileRecord{SHA256: h, Size: int64(len(b))}
	if e = verifyBaseIdentity(mf, game); e == nil {
		t.Fatal("forged legacy receipt blessed changed base data")
	}
}

func TestGeneratedGameOutputsDoNotBlockRelaunch(t *testing.T) {
	dir := communityFixtureCopy(t)
	initial, _ := verifyDir(dir)
	for _, name := range []string{"EXTRACT.CFG", "ORION2.LOG", "150.SET", "NEW150.SET", "SAVE10.GAM"} {
		if e := os.WriteFile(filepath.Join(dir, "game", name), []byte("runtime-output fixture, not a real save"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	after, e := verifyDir(dir)
	if e != nil || !after.OK || after.Fingerprint != initial.Fingerprint {
		t.Fatal("runtime outputs changed compatibility or blocked verification", after, e)
	}
	preserved := t.TempDir()
	if e = preserveUserState(filepath.Join(dir, "game"), preserved, false); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"150.SET", "NEW150.SET", "SAVE10.GAM"} {
		if !exists(filepath.Join(preserved, name)) {
			t.Fatal("personal state not retained", name)
		}
	}
	for _, name := range []string{"EXTRACT.CFG", "ORION2.LOG"} {
		if exists(filepath.Join(preserved, name)) {
			t.Fatal("generated output incorrectly migrated", name)
		}
	}
}
func TestUnknownExtensionConfigChangesFingerprint(t *testing.T) {
	dir := communityFixtureCopy(t)
	cfg := filepath.Join(dir, "game", "150", "USER.CFG")
	if e := os.WriteFile(cfg, []byte("include \\150\\build\\EXTRA.TXT;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	extra := filepath.Join(dir, "game", "150", "build", "EXTRA.TXT")
	if e := os.WriteFile(extra, []byte("ship_initiative = 0;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	first, e := verifyDir(dir)
	if e != nil || !first.OK {
		t.Fatal(first, e)
	}
	if e = os.WriteFile(extra, []byte("ship_initiative = 1;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	second, e := verifyDir(dir)
	if e != nil || !second.OK || first.Fingerprint == second.Fingerprint {
		t.Fatal("extra TXT config bytes missing from identity", second, e)
	}
}
func TestUnknownCodeCannotBeAddedToReceipt(t *testing.T) {
	dir := communityFixtureCopy(t)
	var mf Manifest
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		t.Fatal(e)
	}
	name := "150/scripts/main/EXTRA.LUA"
	file := filepath.Join(dir, "game", filepath.FromSlash(name))
	b := []byte("-- unsupported extra code\n")
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	h, _ := hashFile(file)
	mf.Files[name] = FileRecord{SHA256: h, Size: int64(len(b))}
	if e := atomicJSON(filepath.Join(dir, "manifest.json"), mf); e != nil {
		t.Fatal(e)
	}
	v, e := verifyDir(dir)
	if e == nil && v.OK {
		t.Fatal("local receipt blessed added code")
	}
}
func TestForgedSelectionReceiptRejected(t *testing.T) {
	dir := communityFixtureCopy(t)
	var mf Manifest
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		t.Fatal(e)
	}
	other := mf.Profile
	other.Core = "150i"
	resolved, e := resolve(other)
	if e != nil {
		t.Fatal(e)
	}
	mf.Resolution.Fingerprint = resolved.Fingerprint
	if e = atomicJSON(filepath.Join(dir, "manifest.json"), mf); e != nil {
		t.Fatal(e)
	}
	v, e := verifyDir(dir)
	if e == nil && v.OK {
		t.Fatal("forged resolution selected unprepared ruleset")
	}
}
func TestExcludedOutputsCannotBeCustomInputs(t *testing.T) {
	for _, line := range []string{`include \EXTRACT.CFG;`, `include ?..\EXTRACT.CFG;`, `include \SAVE1.GAM;`, `include \150.SET;`, `pregame_script = \ORION2.LOG;`, `main.lua = \MOX.SET;`, `newgame_postprocessor_script = \NEW150.SET;`, `sound.lbx = \SOUND.LBX;`} {
		t.Run(line, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "USER.CFG")
			os.WriteFile(file, []byte(line), 0600)
			if e := validateEditableConfigReferences("150/USER.CFG", file); e == nil {
				t.Fatal("excluded input accepted", line)
			}
		})
	}
	file := filepath.Join(t.TempDir(), "USER.CFG")
	os.WriteFile(file, []byte("# include \\EXTRACT.CFG; ignored comment\ninclude \\150\\build\\EXTRA.TXT;\n"), 0600)
	if e := validateEditableConfigReferences("150/USER.CFG", file); e != nil {
		t.Fatal(e)
	}
}

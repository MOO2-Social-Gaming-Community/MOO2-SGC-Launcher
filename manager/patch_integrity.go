package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func patchIndex() BaseIndex {
	b, e := resources.ReadFile("assets/patch-files.json")
	if e != nil {
		panic(e)
	}
	var index BaseIndex
	if e = json.Unmarshal(b, &index); e != nil || index.Schema != 1 {
		panic("invalid compiled patch inventory")
	}
	return index
}

// A writable workspace receipt cannot bless different scripts/rulesets or omit
// required files. Generated selection and explicitly editable CFGs stay distinct.
func verifyPatchIdentity(mf Manifest, game string, resolution Resolution) error {
	for _, pin := range patchIndex().Files {
		expected := pin
		if pin.Name == "150/ENABLE.CFG" {
			sum := sha256.Sum256([]byte(resolution.Config))
			expected.SHA256 = hex.EncodeToString(sum[:])
			expected.Size = int64(len(resolution.Config))
		}
		rec, ok := mf.Files[pin.Name]
		if !ok {
			return fmt.Errorf("required patch file absent from manifest: %s", pin.Name)
		}
		file := filepath.Join(game, filepath.FromSlash(pin.Name))
		if e := noSymlinkAncestors(file); e != nil {
			return e
		}
		if e := regularFile(file); e != nil {
			return fmt.Errorf("required patch file missing: %s: %w", pin.Name, e)
		}
		if isUserFile(pin.Name) {
			continue
		}
		if rec.SHA256 != expected.SHA256 || rec.Size != expected.Size {
			return fmt.Errorf("patch file identity differs from supported recipe: %s", pin.Name)
		}
		if e := requireHash(file, expected.SHA256); e != nil {
			return fmt.Errorf("patch file differs from supported recipe: %s: %w", pin.Name, e)
		}
	}
	// Keep this explicit: selected-mod verification must not depend on a local
	// receipt enumerating ENABLE.CFG, even if inventories evolve later.
	b, e := os.ReadFile(filepath.Join(game, "150", "ENABLE.CFG"))
	if e != nil || string(b) != resolution.Config {
		return fmt.Errorf("selected mods configuration missing or changed")
	}
	return nil
}

func compiledWorkspaceFiles(mf Manifest) map[string]bool {
	records := baseIndex().Files
	if mf.Schema == 2 || mf.Schema == 3 {
		edition, e := editionByID(strings.TrimPrefix(mf.SourceBaseKind, "known-edition-v2:"))
		if e == nil {
			records = expectedBaseFiles(edition, mf.Profile.Engine)
			if mf.Schema >= 3 {
				records = normalizedBaseFiles(edition, mf.Profile.Engine)
			}
		}
	}
	allowed := map[string]bool{"DIG.INI": true, "MDI.INI": true, "ORIONCD.INI": true}
	for _, r := range records {
		allowed[r.Name] = true
	}
	if mf.Profile.Engine == "1.50.26" {
		for _, r := range patchIndex().Files {
			allowed[r.Name] = true
		}
	}
	return allowed
}
func isRuntimeOutput(name string) bool {
	name = strings.ToLower(filepath.ToSlash(name))
	return name == "extract.cfg" || name == "orion2.log"
}
func protectedContent(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".exe", ".com", ".dll", ".lbx", ".lua", ".cfg", ".conf", ".bat":
		return true
	}
	return false
}

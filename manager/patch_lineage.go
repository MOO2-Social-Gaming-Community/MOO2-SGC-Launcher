package main

// All transforms operate on disposable workspaces, not the selected source.
// In particular no 2006 Windows patcher (or its XP system edits) is executed.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const BaselineEngineHash = "7ae2ac2e5904ca330009af2827279d889906b0b9b7a8854c38eb707a56e955b5"
const CDEngineHash = "558c2bb51354fa48ba3189021374d67873ea0e7b37bab77e32f44eda89e135bc"
const Official131URL = "https://moo2mod.com/patch/MOO2-1.31.en.zip"

type ByteAddition struct {
	Offset int `json:"offset"`
	Add    int `json:"add"`
}
type PatchInsertion struct {
	Offset int    `json:"offset"`
	Hex    string `json:"hex,omitempty"`
	Zeros  int    `json:"zeros,omitempty"`
}
type PatchWrite struct {
	Offset int    `json:"offset"`
	Hex    string `json:"hex"`
}
type BaselineRecipe struct {
	Schema     int              `json:"schema"`
	InputSHA   string           `json:"input_sha256"`
	OutputSHA  string           `json:"output_sha256"`
	OutputSize int              `json:"output_size"`
	PatcherSHA string           `json:"patcher_sha256"`
	Additions  []ByteAddition   `json:"byte_additions"`
	Insertions []PatchInsertion `json:"insertions"`
	Writes     []PatchWrite     `json:"writes"`
}

func baselineRecipe() BaselineRecipe {
	b, e := resources.ReadFile("assets/patch-140b23.json")
	if e != nil {
		panic(e)
	}
	var r BaselineRecipe
	if e = json.Unmarshal(b, &r); e != nil {
		panic(e)
	}
	return r
}
func applyBaselineBytes(src []byte, r BaselineRecipe) ([]byte, error) {
	h := sha256.Sum256(src)
	if r.Schema != 1 || hex.EncodeToString(h[:]) != r.InputSHA || r.OutputSize > 8<<20 || r.OutputSize < len(src) {
		return nil, errors.New("1.40b23 transform requires the exact official English DOS 1.31 executable")
	}
	added := map[int]int{}
	for _, a := range r.Additions {
		if a.Offset < 0 || a.Offset >= len(src) || a.Add < 0 || a.Add > 255 {
			return nil, errors.New("invalid baseline header transform")
		}
		if _, ok := added[a.Offset]; ok {
			return nil, errors.New("duplicate baseline transform")
		}
		added[a.Offset] = a.Add
	}
	inserts := map[int][]byte{}
	sum := len(src)
	for _, x := range r.Insertions {
		if x.Offset < 0 || x.Offset > len(src) || x.Zeros < 0 || x.Zeros > 1<<20 || x.Zeros > 0 && x.Hex != "" {
			return nil, errors.New("invalid baseline insertion")
		}
		if _, ok := inserts[x.Offset]; ok {
			return nil, errors.New("duplicate baseline insertion")
		}
		b, e := hex.DecodeString(x.Hex)
		if e != nil {
			return nil, e
		}
		if x.Zeros > 0 {
			b = make([]byte, x.Zeros)
		}
		sum += len(b)
		if sum > r.OutputSize {
			return nil, errors.New("baseline insertion exceeds output size")
		}
		inserts[x.Offset] = b
	}
	out := make([]byte, 0, r.OutputSize)
	for i, v := range src {
		out = append(out, inserts[i]...)
		out = append(out, byte(int(v)+added[i]))
	}
	out = append(out, inserts[len(src)]...)
	if len(out) != r.OutputSize {
		return nil, errors.New("baseline output size differs")
	}
	for _, x := range r.Writes {
		b, e := hex.DecodeString(x.Hex)
		if e != nil {
			return nil, e
		}
		if x.Offset < 0 || x.Offset > len(out)-len(b) {
			return nil, errors.New("out of bounds baseline edit")
		}
		copy(out[x.Offset:], b)
	}
	h = sha256.Sum256(out)
	if hex.EncodeToString(h[:]) != r.OutputSHA || r.OutputSHA != BaselineEngineHash {
		return nil, errors.New("baseline output differs from verified Steam 1.40b23 executable")
	}
	return out, nil
}
func official131Index() BaseIndex {
	b, e := resources.ReadFile("assets/official-131-files.json")
	if e != nil {
		panic(e)
	}
	var r BaseIndex
	if e = json.Unmarshal(b, &r); e != nil {
		panic(e)
	}
	return r
}

// Per-file authenticity permits safe publisher ZIP repackaging without accepting
// different executable/data bytes. Only pinned DOS members are installed.
func (m *Manager) importOfficial131(source string) (any, error) {
	p, e := localPath(source)
	if e != nil {
		return nil, e
	}
	if e = regularFile(p); e != nil {
		return nil, e
	}
	tmp, e := os.MkdirTemp(filepath.Join(m.Data, "cache"), "official-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(tmp)
	staged := filepath.Join(tmp, "verified.zip")
	if e = writeZipSnapshot(p, staged, official131Index().Files); e != nil {
		return nil, fmt.Errorf("official 1.31 package: %w", e)
	}
	dst := filepath.Join(m.Data, "cache", "official-1.31.zip")
	h, e := hashFile(staged)
	if e != nil {
		return nil, e
	}
	if e = replaceFile(staged, dst); e != nil {
		return nil, e
	}
	return map[string]any{"cached": dst, "version": "1.31", "files": len(official131Index().Files), "sha256": h, "source_modified": false, "verification": "every installed file matched compiled SHA-256"}, nil
}
func (m *Manager) ensureOfficial131() error {
	dst := filepath.Join(m.Data, "cache", "official-1.31.zip")
	if _, e := os.Stat(dst); e == nil {
		_, e = m.importOfficial131(dst)
		if e == nil {
			return nil
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	m.progress("Downloading official English DOS 1.31 prerequisite; verifying every installed file…")
	return m.downloadOfficial131(downloadClient(), Official131URL)
}
func (m *Manager) downloadOfficial131(client *http.Client, url string) error {
	ctx := m.downloadContext()
	downloaded, e := downloadTemporary(ctx, client, url, filepath.Join(m.Data, "cache"), 16<<20, m.downloadProgress("Official DOS 1.31 patch"))
	if e != nil {
		return m.downloadError(ctx, e)
	}
	defer os.Remove(downloaded.path)
	if e = m.checkCancellation(); e != nil {
		return e
	}
	m.progress("Verifying official DOS 1.31 patch files; the existing prepared game is unchanged…")
	_, e = m.importOfficial131(downloaded.path)
	return e
}
func engineRank(v string) int {
	switch v {
	case "1.2":
		return 12
	case "1.31":
		return 131
	case "1.40b23":
		return 140
	case "1.50.26":
		return 150
	}
	return -1
}

type LineageStep struct {
	Version   string `json:"version"`
	Operation string `json:"operation"`
	EngineSHA string `json:"engine_sha256"`
}

func expectedLineage(s SourceEdition, target string) ([]LineageStep, error) {
	if engineRank(target) < engineRank(s.Version) || engineRank(target) < 0 {
		return nil, fmt.Errorf("cannot reconstruct %s from %s without its original source. Import the CD 1.2 files for a CD profile; no downgrade is applied to your source", target, s.Label)
	}
	hash := BaseEngineHash
	if s.Version == "1.2" {
		hash = CDEngineHash
	} else if s.Version == "1.40b23" {
		hash = BaselineEngineHash
	}
	steps := []LineageStep{{s.Version, "recognized owned source", hash}}
	if engineRank(target) >= 131 && s.Version == "1.2" {
		steps = append(steps, LineageStep{"1.31", "official prerequisite", BaseEngineHash})
	}
	if engineRank(target) >= 140 && engineRank(s.Version) < 140 {
		steps = append(steps, LineageStep{"1.40b23", "verified historical patch transform", BaselineEngineHash})
	}
	if target == "1.50.26" {
		steps = append(steps, LineageStep{"1.50.26", "community add-on", EngineHash})
	}
	return steps, nil
}
func expectedBaseFiles(s SourceEdition, target string) []BaseFile {
	records := map[string]BaseFile{}
	for _, f := range s.Files {
		records[f.Name] = f
	}
	if s.Version == "1.2" && engineRank(target) >= 131 {
		for _, f := range official131Index().Files {
			records[f.Name] = f
		}
	}
	if engineRank(target) >= 140 {
		records["ORION2.EXE"] = BaseFile{"ORION2.EXE", 2644842, BaselineEngineHash}
	}
	out := []BaseFile{}
	for _, f := range records {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (m *Manager) extractKnownSource(base, game string, s SourceEdition) error {
	// Content-addressed source ZIPs are private and already verified; stream each
	// approved member directly into this fresh stage and verify again while copying.
	// Recompressing hundreds of megabytes for every rebuild is unnecessary.
	return extractVerifiedMembers(base, game, s.Files)
}
func (m *Manager) applyLineage(game string, s SourceEdition, target string) ([]LineageStep, error) {
	steps, e := expectedLineage(s, target)
	if e != nil {
		return nil, e
	}
	if s.Version == "1.2" && engineRank(target) >= 131 {
		stage, e := os.MkdirTemp(filepath.Dir(game), "patch131-")
		if e != nil {
			return nil, e
		}
		defer os.RemoveAll(stage)
		normalized := filepath.Join(stage, "patch.zip")
		if e = writeZipSnapshot(filepath.Join(m.Data, "cache", "official-1.31.zip"), normalized, official131Index().Files); e != nil {
			return nil, e
		}
		files := filepath.Join(stage, "files")
		if e = extractZip(normalized, files, "base/", false); e != nil {
			return nil, e
		}
		m.progress("Applying verified official 1.31 prerequisite inside the managed workspace…")
		for _, r := range official131Index().Files {
			b, e := os.ReadFile(filepath.Join(files, r.Name))
			if e != nil {
				return nil, e
			}
			if e = atomicWrite(filepath.Join(game, r.Name), b); e != nil {
				return nil, e
			}
		}
		if e = requireHash(filepath.Join(game, "ORION2.EXE"), BaseEngineHash); e != nil {
			return nil, e
		}
	}
	if engineRank(target) >= 140 && engineRank(s.Version) < 140 {
		m.progress("Constructing baseline 1.40b23; verifying exact output against the Steam engine…")
		p := filepath.Join(game, "ORION2.EXE")
		src, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		if e = atomicWrite(filepath.Join(game, "ORION131.EXE"), src); e != nil {
			return nil, e
		}
		out, e := applyBaselineBytes(src, baselineRecipe())
		if e != nil {
			return nil, e
		}
		if e = atomicWrite(p, out); e != nil {
			return nil, e
		}
	}
	if engineRank(target) >= 140 {
		if e := m.ensureStageKernel(game); e != nil {
			return nil, e
		}
	}
	if e := normalizeHarnessGame(game, s, target); e != nil {
		return nil, e
	}
	return steps, nil
}
func verifyEditionManifest(mf Manifest, game string) error {
	id := strings.TrimPrefix(mf.SourceBaseKind, "known-edition-v2:")
	s, e := editionByID(id)
	if e != nil {
		return e
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(mf.SourceBase) {
		return errors.New("invalid source archive receipt")
	}
	expected, e := expectedLineage(s, mf.Profile.Engine)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(expected)
	b, _ := json.Marshal(mf.Lineage)
	if string(a) != string(b) {
		return errors.New("installation lineage differs from supported recipe")
	}
	records := expectedBaseFiles(s, mf.Profile.Engine)
	if mf.Schema >= 3 {
		records = normalizedBaseFiles(s, mf.Profile.Engine)
	}
	return verifyPinnedBase(game, mf.Files, records)
}
func writeDefaultGameConfig(game string) error {
	for _, n := range []string{"DIG.INI", "MDI.INI", "ORIONCD.INI"} {
		b, e := resources.ReadFile("assets/harness/" + n)
		if e != nil {
			return e
		}
		// Normalize pre-existing case aliases on case-sensitive development hosts.
		entries, e := os.ReadDir(game)
		if e != nil {
			return e
		}
		for _, f := range entries {
			if f.Name() != n && strings.EqualFold(f.Name(), n) {
				if e = os.Remove(filepath.Join(game, f.Name())); e != nil {
					return e
				}
			}
		}
		if e = atomicWrite(filepath.Join(game, n), b); e != nil {
			return e
		}
	}
	return nil
}

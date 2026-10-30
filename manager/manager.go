package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"moo2manager/shared/layout"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileRecord struct {
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Mutable bool   `json:"mutable"`
}
type Manifest struct {
	Schema         int                   `json:"schema"`
	Lineage        []LineageStep         `json:"lineage,omitempty"`
	Manager        string                `json:"manager"`
	Created        string                `json:"created"`
	Profile        Profile               `json:"profile"`
	Resolution     Resolution            `json:"resolution"`
	Files          map[string]FileRecord `json:"files"`
	SourceBase     string                `json:"source_base"`
	SourceBaseKind string                `json:"source_base_kind,omitempty"`
	SourcePatch    string                `json:"source_patch"`
}
type Active struct {
	Generation string `json:"generation"`
}
type VerifyResult struct {
	OK          bool     `json:"ok"`
	Checked     int      `json:"checked"`
	Bad         []string `json:"bad"`
	UserChanges []string `json:"user_changes"`
	Fingerprint string   `json:"fingerprint"`
	Generation  string   `json:"generation"`
}
type Job struct {
	ID              uint64 `json:"id"` // Monotonic in this manager session; never a session credential.
	Busy            bool   `json:"busy"`
	Kind            string `json:"kind"`
	Message         string `json:"message"`
	Error           string `json:"error"`
	Result          any    `json:"result,omitempty"`
	Cancelable      bool   `json:"cancelable"`
	CancelRequested bool   `json:"cancel_requested"`
}
type Settings struct {
	RuntimePath string `json:"runtime_path"`
}
type Manager struct {
	Root, Data       string
	Portable         bool
	AppRoot          string
	DesktopGUI       bool
	DeferBrowserOnce bool // Initial parent redirect only; never an update policy.
	NoShortcut       bool // Per-process policy, deliberately not saved in user settings.
	mu               sync.Mutex
	op               sync.Mutex
	job              Job
	jobContext       context.Context
	cancelJob        context.CancelFunc
	running          *exec.Cmd
	runningProfile   string
	runningKind      string
	runtimeDone      chan struct{}
	runtimeError     error
	uiMu             sync.Mutex
	lastExit         string
}

func newManager(root, data string) (*Manager, error) {
	root, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	data, e = filepath.Abs(data)
	if e != nil {
		return nil, e
	}
	if e = noSymlinkAncestors(data); e != nil {
		return nil, e
	}
	for _, d := range []string{"profiles", "active", "environments", "cache", "logs"} {
		if e = os.MkdirAll(filepath.Join(data, d), 0700); e != nil {
			return nil, e
		}
	}
	portable, e := layout.IsPortable(root)
	if e != nil {
		return nil, e
	}
	m := &Manager{Root: root, Data: data, Portable: portable, job: Job{Message: "Ready. Prepare the portable baseline first; community patches remain optional."}}
	for _, p := range defaultProfiles() {
		f := filepath.Join(data, "profiles", p.ID+".json")
		if _, e = os.Stat(f); os.IsNotExist(e) {
			if e = atomicJSON(f, p); e != nil {
				return nil, e
			}
		}
	}
	return m, nil
}
func (m *Manager) progress(s string) {
	m.mu.Lock()
	if !m.job.CancelRequested {
		m.job.Message = s
	}
	m.mu.Unlock()
}
func (m *Manager) startJob(kind string, fn func() (any, error)) error {
	m.mu.Lock()
	if m.job.Busy || m.running != nil {
		m.mu.Unlock()
		return errors.New("another operation or game is running; finish it first")
	}
	m.jobContext, m.cancelJob = context.WithCancel(context.Background())
	m.job = Job{ID: m.job.ID + 1, Busy: true, Kind: kind, Message: "Starting " + kind, Cancelable: kind == "prepare-play" || kind == "prepare" || kind == "runtime-install" || kind == "runtime-import" || kind == "patch-refresh"}
	m.mu.Unlock()
	go func() {
		m.op.Lock()
		defer m.op.Unlock()
		r, e := fn()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.cancelJob()
		m.cancelJob = nil
		m.jobContext = nil
		m.job.Busy = false
		m.job.Cancelable = false
		m.job.Result = r
		if errors.Is(e, errOperationCanceled) {
			m.job.Message = "Canceled safely. Your existing prepared game is kept. You can retry when ready."
		} else if e != nil {
			m.job.Error = e.Error()
			m.job.Message = kind + " stopped"
		} else {
			m.job.Message = kind + " completed"
		}
	}()
	return nil
}
func (m *Manager) loadProfile(id string) (Profile, error) {
	var p Profile
	if !idPattern.MatchString(id) {
		return p, errors.New("invalid profile ID")
	}
	e := readJSON(filepath.Join(m.Data, "profiles", id+".json"), &p)
	if e != nil {
		return p, e
	}
	if p.ID != id {
		return p, errors.New("profile ID mismatch")
	}
	_, e = resolve(p)
	return p, e
}
func (m *Manager) saveProfile(p Profile) error {
	if _, e := resolve(p); e != nil {
		return e
	}
	if e := atomicJSON(filepath.Join(m.Data, "profiles", p.ID+".json"), p); e != nil {
		return e
	}
	return m.rememberProfile(p.ID)
}
func (m *Manager) profiles() ([]Profile, error) {
	entries, e := os.ReadDir(filepath.Join(m.Data, "profiles"))
	if e != nil {
		return nil, e
	}
	out := []Profile{}
	for _, f := range entries {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		p, e := m.loadProfile(strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (m *Manager) activePath(id string) (string, string, error) {
	if m.isPortableBaseline(id) {
		return m.portableActive()
	}
	if !idPattern.MatchString(id) {
		return "", "", errors.New("invalid profile ID")
	}
	var a Active
	if e := readJSON(filepath.Join(m.Data, "active", id+".json"), &a); e != nil {
		return "", "", e
	}
	if !idPattern.MatchString(a.Generation) {
		return "", "", errors.New("invalid generation")
	}
	p := filepath.Join(m.Data, "environments", id, a.Generation)
	if e := noSymlinkAncestors(p); e != nil {
		return "", "", e
	}
	return p, a.Generation, nil
}
func (m *Manager) payload(which string) (string, string, string, error) {
	if which == "base" {
		if _, e := os.Stat(filepath.Join(m.Data, "cache", "base-source.json")); e == nil {
			return m.importedBase()
		} else if !os.IsNotExist(e) {
			return "", "", "", e
		}
	}
	name, want, prefix := "base.zip", BaseHash, "Master of Orion 2/"
	if which == "patch" {
		name, want, prefix = "patch-1.50.26.zip", PatchHash, "MOO2-1.50.26/patch/"
	}
	candidates := []string{filepath.Join(m.Data, "cache", name), filepath.Join(m.Root, "payloads", name)}
	for _, p := range candidates {
		if _, e := os.Stat(p); e == nil {
			if e = regularFile(p); e != nil {
				return "", "", "", e
			}
			if e = requireHash(p, want); e != nil {
				return "", "", "", e
			}
			return p, want, prefix, nil
		}
	}
	return "", "", "", fmt.Errorf("missing %s; import your local archive in Packages & updates, or retain the existing private payloads folder", name)
}
func (m *Manager) build(p Profile) (any, error) {
	if e := m.checkCancellation(); e != nil {
		return nil, e
	}
	if m.isPortableBaseline(p.ID) && p.Engine != "1.40b23" {
		return nil, errors.New("portable baseline is fixed at 1.40b23; choose or duplicate a different profile for another engine")
	}
	if m.Portable && exists(m.baselineJournal()) {
		return nil, errors.New("recover the interrupted baseline transaction first")
	}
	r, e := resolve(p)
	if e != nil {
		return nil, e
	}
	m.progress("Verifying your source archive before installation…")
	base, baseHash, _, e := m.payload("base")
	if e != nil {
		return nil, e
	}
	edition, e := m.currentEdition()
	if e != nil {
		return nil, e
	}
	if _, e = expectedLineage(edition, p.Engine); e != nil {
		return nil, e
	}
	if edition.Version == "1.2" && engineRank(p.Engine) >= 131 {
		if e = m.ensureOfficial131(); e != nil {
			return nil, e
		}
	}
	var patch, pp string
	if p.Engine == "1.50.26" {
		patch, _, pp, e = m.payload("patch")
		if e != nil {
			return nil, e
		}
	}
	parent := filepath.Join(m.Data, "environments", p.ID)
	if e = os.MkdirAll(parent, 0700); e != nil {
		return nil, e
	}
	stage, e := os.MkdirTemp(parent, "stage-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	game := filepath.Join(stage, "game")
	m.progress("Extracting a fresh game copy; archived personal saves stay in the source ZIP…")
	if e = m.extractKnownSource(base, game, edition); e != nil {
		return nil, e
	}
	lineage, e := m.applyLineage(game, edition, p.Engine)
	if e != nil {
		return nil, e
	}
	if p.Engine == "1.50.26" {
		m.progress("Applying the unmodified community 1.50.26 overlay…")
		if e = extractZip(patch, game, pp, false); e != nil {
			return nil, e
		}
		if e = atomicWrite(filepath.Join(game, "150", "ENABLE.CFG"), []byte(r.Config)); e != nil {
			return nil, e
		}
	}
	// The harness mounts the executable directory; the relative CD pointer stays portable.
	if e = writeDefaultGameConfig(game); e != nil {
		return nil, e
	}
	wantBase := BaseEngineHash
	if p.Engine == "1.2" {
		wantBase = CDEngineHash
	} else if engineRank(p.Engine) >= 140 {
		wantBase = BaselineEngineHash
	}
	if e = requireHash(filepath.Join(game, "ORION2.EXE"), wantBase); e != nil {
		return nil, e
	}
	if p.Engine == "1.50.26" {
		if e = requireHash(filepath.Join(game, "ORION150.EXE"), EngineHash); e != nil {
			return nil, e
		}
	}
	if e = m.checkCancellation(); e != nil {
		return nil, e
	}
	old, _, oe := m.activePath(p.ID)
	if oe == nil {
		var om Manifest
		if e = readWorkspaceManifest(old, &om); e != nil {
			return nil, e
		}
		// Never migrate personal state across engine versions automatically.
		if om.Profile.Engine == p.Engine {
			m.progress("Preserving this profile's saves and editable settings; retaining the previous generation…")
			// Old launcher defaults are NOT copied over the proven harness audio on
			// first migration. Saves survive; schema-3 user audio stays editable.
			e = preserveUserState(filepath.Join(old, "game"), game, om.Schema < 3)
			if e != nil {
				return nil, e
			}
		}
	} else if !os.IsNotExist(oe) {
		return nil, oe
	} else if m.isPortableBaseline(p.ID) {
		if e = m.preserveUnregisteredBaseline(game); e != nil {
			return nil, e
		}
	}
	manifest := Manifest{Schema: 3, Lineage: lineage, Manager: Version, Created: time.Now().UTC().Format(time.RFC3339Nano), Profile: p, Resolution: r, SourceBase: baseHash, Files: map[string]FileRecord{}}
	manifest.SourceBaseKind = "known-edition-v2:" + edition.ID
	if p.Engine == "1.50.26" {
		manifest.SourcePatch = PatchHash
	}
	m.progress("Hashing installed files and writing the resolved profile lock…")
	e = walkRegular(game, func(n, f string) error {
		h, e := hashFile(f)
		if e != nil {
			return e
		}
		i, e := os.Stat(f)
		if e != nil {
			return e
		}
		manifest.Files[n] = FileRecord{h, i.Size(), isUserFile(n)}
		return nil
	})
	if e != nil {
		return nil, e
	}
	if e = atomicJSON(filepath.Join(stage, "manifest.json"), manifest); e != nil {
		return nil, e
	}
	v, e := verifyDir(stage)
	if e != nil {
		return nil, e
	}
	if !v.OK {
		return nil, fmt.Errorf("staging verification failed: %v", v.Bad)
	}
	if e = m.beginCommit(); e != nil {
		return nil, e
	}
	if m.isPortableBaseline(p.ID) {
		// Write project-owned config before activation; an unwritable config must
		// not replace a previously working game directory.
		cfg, e := dosboxConfig(filepath.Join(m.Root, "game"), p)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Join(m.Root, "config"), 0700); e != nil {
			return nil, e
		}
		if e = atomicWrite(filepath.Join(m.Root, "config", "moo2.conf"), []byte(cfg)); e != nil {
			return nil, e
		}
		result, e := m.commitPortableBaseline(stage, p, v)
		if e != nil {
			return nil, e
		}
		if e = m.saveProfile(p); e != nil {
			return nil, e
		}
		return result, nil
	}
	generation := "g" + fmt.Sprint(time.Now().UnixNano())
	dest := filepath.Join(parent, generation)
	cfg, e := dosboxConfig(filepath.Join(dest, "game"), p)
	if e != nil {
		return nil, e
	}
	if e = atomicWrite(filepath.Join(stage, "dosbox-manager.conf"), []byte(cfg)); e != nil {
		return nil, e
	}
	if e = os.Rename(stage, dest); e != nil {
		return nil, e
	}
	// Activation pointer is the final transaction commit; prior generations are not deleted.
	if e = atomicJSON(filepath.Join(m.Data, "active", p.ID+".json"), Active{generation}); e != nil {
		return nil, e
	}
	if e = m.saveProfile(p); e != nil {
		return nil, e
	}
	v.Generation = generation
	return map[string]any{"verification": v, "path": dest, "prsl": "not injected", "chat": "not injected", "archived_saves": "retained only in original source ZIP", "engine": p.Engine, "source_version": edition.Version, "effective_baseline": "1.40b23", "lineage": lineage}, nil
}
func verifyDir(dir string) (VerifyResult, error) {
	v := VerifyResult{OK: true, Bad: []string{}, UserChanges: []string{}}
	var mf Manifest
	if e := noSymlinkAncestors(dir); e != nil {
		return v, e
	}
	if e := readWorkspaceManifest(dir, &mf); e != nil {
		return v, e
	}
	if (mf.Schema != 1 && mf.Schema != 2 && mf.Schema != 3) || len(mf.Files) == 0 || len(mf.Files) > 10000 {
		return v, errors.New("unsupported or empty manifest")
	}
	resolution, e := resolve(mf.Profile)
	if e != nil {
		return v, e
	}
	if mf.Resolution.Fingerprint != resolution.Fingerprint {
		return v, errors.New("stored selection fingerprint differs from prepared profile")
	}
	if mf.Profile.Engine == "1.50.26" && mf.SourcePatch != PatchHash {
		return v, errors.New("unrecognized patch fingerprint")
	}
	game := filepath.Join(dir, "game")
	if mf.Schema >= 3 {
		want, _ := resources.ReadFile("assets/harness/ORIONCD.INI")
		got, err := os.ReadFile(filepath.Join(game, "ORIONCD.INI"))
		if err != nil || string(got) != string(want) {
			return v, errors.New("generated ORIONCD.INI differs from harness contract")
		}
	}
	if e = verifyBaseIdentity(mf, game); e != nil {
		return v, e
	}
	if mf.Profile.Engine == "1.50.26" {
		if e = verifyPatchIdentity(mf, game, resolution); e != nil {
			return v, e
		}
	}
	allowed := compiledWorkspaceFiles(mf)
	seen := map[string]bool{}
	cfgSet := map[string]bool{}
	extraFiles := map[string]string{}
	actualFiles := map[string]string{}
	for n, rec := range mf.Files {
		if isRuntimeOutput(n) {
			continue
		}
		if strings.EqualFold(filepath.Ext(n), ".cfg") {
			cfgSet[n] = true
		}
		rel, e := safeRel(n)
		if e != nil {
			return v, e
		}
		f := filepath.Join(game, rel)
		if e = noSymlinkAncestors(f); e != nil {
			return v, e
		}
		if strings.EqualFold(n, "150/ENABLE.CFG") && mf.Profile.Engine == "1.50.26" {
			b, e := os.ReadFile(f)
			if e != nil || string(b) != resolution.Config {
				v.Bad = append(v.Bad, n+" (selected mods differ)")
			}
		}
		v.Checked++
		h, e := hashFile(f)
		if e != nil || h != rec.SHA256 {
			if isUserFile(n) && (e == nil || os.IsNotExist(e) && isPersonalSeed(strings.ToLower(n))) {
				v.UserChanges = append(v.UserChanges, n)
			} else {
				v.Bad = append(v.Bad, n)
			}
		}
		seen[n] = true
	}
	e = walkRegular(game, func(n, f string) error {
		actualFiles[strings.ToUpper(n)] = f
		if isRuntimeOutput(n) {
			return nil
		}
		if !allowed[n] && !isUserFile(n) {
			if protectedContent(n) {
				v.Bad = append(v.Bad, n+" (outside compiled file inventory)")
			}
			h, e := hashFile(f)
			if e != nil {
				return e
			}
			extraFiles[n] = h
		}
		if strings.EqualFold(filepath.Ext(n), ".cfg") {
			cfgSet[n] = true
		}
		if seen[n] {
			return nil
		}
		if isUserFile(n) {
			v.UserChanges = append(v.UserChanges, n)
			return nil
		}
		switch strings.ToLower(filepath.Ext(n)) {
		case ".exe", ".com", ".dll", ".lbx", ".lua", ".cfg", ".conf", ".bat":
			v.Bad = append(v.Bad, n+" (unmanaged code/config/data)")
		}
		return nil
	})
	if e != nil {
		return v, e
	}
	if mf.Profile.Engine == "1.50.26" {
		if e = validateConfigDependencies(actualFiles); e != nil {
			return v, e
		}
	}
	exe, want := "ORION2.EXE", BaseEngineHash
	if mf.Profile.Engine == "1.2" {
		want = CDEngineHash
	}
	if mf.Profile.Engine == "1.40b23" {
		want = BaselineEngineHash
	}
	if mf.Profile.Engine == "1.50.26" {
		exe, want = "ORION150.EXE", EngineHash
	}
	if e = requireHash(filepath.Join(game, exe), want); e != nil {
		v.Bad = append(v.Bad, exe+" (exact engine check)")
	}
	// 1.40b23 and 1.50 networking both require the LAN-fixed RKERNEL.COM.
	// Older workspaces created before 0.4.5 may have launched single-player
	// successfully while lacking this file; fail verification before MOO2 can
	// crash in its Network Game path and direct the user to Repair.
	if engineRank(mf.Profile.Engine) >= 140 {
		if e = requireHash(filepath.Join(game, "RKERNEL.COM"), Kernel140Hash); e != nil {
			v.Bad = append(v.Bad, "RKERNEL.COM (required 1.40b23/1.50 network kernel; Prepare / repair this profile)")
		}
	}
	// Include active editable CFG contents in the multiplayer fingerprint; not display/network settings.
	h := sha256.New()
	h.Write([]byte(resolution.Fingerprint))
	cfgNames := []string{}
	for n := range cfgSet {
		cfgNames = append(cfgNames, n)
	}
	sort.Strings(cfgNames)
	for _, n := range cfgNames {
		b, e := os.ReadFile(filepath.Join(game, filepath.FromSlash(n)))
		if e != nil {
			v.Bad = append(v.Bad, n)
			continue
		}
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	// Conservative compatibility identity covers the installed immutable game
	// assets as well as selected CFGs. Different CD/store LBX variants must not
	// look identical merely because their DOS executables match.
	contentNames := []string{}
	for n := range mf.Files {
		ext := strings.ToLower(filepath.Ext(n))
		if (ext == ".lbx" && !isUserFile(n)) || ext == ".com" || ext == ".lua" || n == exe {
			contentNames = append(contentNames, n)
		}
	}
	sort.Strings(contentNames)
	for _, n := range contentNames {
		h.Write([]byte(strings.ToUpper(n)))
		h.Write([]byte{0})
		h.Write([]byte(mf.Files[n].SHA256))
		h.Write([]byte{0})
	}
	extraNames := []string{}
	for n := range extraFiles {
		extraNames = append(extraNames, n)
	}
	sort.Strings(extraNames)
	for _, n := range extraNames {
		h.Write([]byte("extra:" + strings.ToUpper(n)))
		h.Write([]byte{0})
		h.Write([]byte(extraFiles[n]))
		h.Write([]byte{0})
	}
	v.Fingerprint = hex.EncodeToString(h.Sum(nil))
	sort.Strings(v.Bad)
	sort.Strings(v.UserChanges)
	v.OK = len(v.Bad) == 0
	return v, nil
}
func (m *Manager) verify(id string) (VerifyResult, error) {
	dir, g, e := m.activePath(id)
	if e != nil {
		return VerifyResult{}, e
	}
	v, e := verifyDir(dir)
	v.Generation = g
	return v, e
}
func (m *Manager) history(id string) ([]map[string]string, error) {
	if m.isPortableBaseline(id) {
		return m.portableHistory()
	}
	if !idPattern.MatchString(id) {
		return nil, errors.New("invalid profile")
	}
	entries, e := os.ReadDir(filepath.Join(m.Data, "environments", id))
	if os.IsNotExist(e) {
		return []map[string]string{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []map[string]string{}
	for _, d := range entries {
		if !d.IsDir() || !strings.HasPrefix(d.Name(), "g") {
			continue
		}
		var mf Manifest
		if e = readJSON(filepath.Join(m.Data, "environments", id, d.Name(), "manifest.json"), &mf); e != nil {
			continue
		}
		out = append(out, map[string]string{"generation": d.Name(), "created": mf.Created, "engine": mf.Profile.Engine, "core": mf.Profile.Core})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["generation"] > out[j]["generation"] })
	return out, nil
}
func (m *Manager) activate(id, generation string) (any, error) {
	if m.isPortableBaseline(id) {
		return m.activatePortable(generation)
	}
	if !idPattern.MatchString(id) || !idPattern.MatchString(generation) {
		return nil, errors.New("invalid generation or profile")
	}
	dir := filepath.Join(m.Data, "environments", id, generation)
	v, e := verifyDir(dir)
	if e != nil {
		return nil, e
	}
	if !v.OK {
		return nil, fmt.Errorf("generation is corrupt: %v", v.Bad)
	}
	var mf Manifest
	if e = readWorkspaceManifest(dir, &mf); e != nil {
		return nil, e
	}
	if mf.Profile.ID != id {
		return nil, errors.New("generation belongs to another profile")
	}
	// No save merging on rollback: the current generation remains retained.
	if e = atomicJSON(filepath.Join(m.Data, "active", id+".json"), Active{generation}); e != nil {
		return nil, e
	}
	e = m.saveProfile(mf.Profile)
	return v, e
}
func (m *Manager) removeEnvironment(id string) (any, error) {
	if !idPattern.MatchString(id) {
		return nil, errors.New("invalid profile")
	}
	p := filepath.Join(m.Data, "active", id+".json")
	if m.isPortableBaseline(id) {
		p = filepath.Join(m.Root, "state", "baseline-active.json")
	}
	e := os.Remove(p)
	if e != nil {
		return nil, e
	}
	return map[string]string{"status": "deactivated", "saves": "All generations and saves retained. Prepare to reinstall, or activate a retained generation."}, nil
}
func (m *Manager) getSettings() Settings {
	var s Settings
	_ = readJSON(filepath.Join(m.Data, "settings.json"), &s)
	return s
}
func (m *Manager) exportDiagnostics(id string) (any, error) {
	p, e := m.loadProfile(id)
	if e != nil {
		return nil, e
	}
	r, e := resolve(p)
	if e != nil {
		return nil, e
	}
	v, ve := m.verify(id)
	var lastDiagnostic any
	_ = readJSON(filepath.Join(m.Data, "logs", "last-network-check.json"), &lastDiagnostic)
	var lastLaunch any
	_ = readJSON(filepath.Join(m.Data, "logs", "last-launch.json"), &lastLaunch)
	report := map[string]any{"schema": 1, "manager": Version, "last_launch": lastLaunch, "last_network_check": lastDiagnostic, "application": m.applicationIdentity(), "platform": platform(), "profile": p, "resolution": r, "verification": v, "runtime_candidates": m.runtimeCandidates(), "live_prsl": false, "new_chat": false, "saved_games_included": false, "portable_root": m.Root, "portable": m.Portable, "harness_contract": "0.3.0 handoff / DOSBox Staging 0.83.0", "timestamp": time.Now().UTC().Format(time.RFC3339)}
	if ve != nil {
		report["verification_error"] = ve.Error()
	}
	name := "diagnostics-" + id + "-" + time.Now().UTC().Format("20060102T150405Z") + ".json"
	f := filepath.Join(m.Data, "logs", name)
	e = atomicJSON(f, report)
	return map[string]any{"path": f, "report": report}, e
}
func (m *Manager) state() (any, error) {
	ps, e := m.profiles()
	if e != nil {
		return nil, e
	}
	active := map[string]string{}
	hist := map[string]any{}
	network := map[string]NetworkPreflight{}
	prepared := map[string]any{}
	for _, p := range ps {
		dir, g, e := m.activePath(p.ID)
		if e == nil {
			active[p.ID] = g
			network[p.ID] = inspectNetworkKernel(filepath.Join(dir, "game"), p.Engine)
			var mf Manifest
			if err := readWorkspaceManifest(dir, &mf); err == nil {
				if r, err := resolve(mf.Profile); err == nil {
					prepared[p.ID] = map[string]string{"fingerprint": r.Fingerprint, "engine": mf.Profile.Engine, "core": mf.Profile.Core}
				}
			}
		}
		h, _ := m.history(p.ID)
		hist[p.ID] = h
	}
	m.mu.Lock()
	j, r, kind, exit := m.job, m.runningProfile, m.runningKind, m.lastExit
	m.mu.Unlock()
	return map[string]any{"application": m.applicationIdentity(), "prepared": prepared, "runtime_status": m.runtimeStatus(), "network_preflight": network, "portable": m.Portable, "portable_root": m.Root, "portable_recovery_pending": m.Portable && exists(m.baselineJournal()), "default_profile": "baseline", "selected_profile_id": m.selectedProfileID(), "version": Version, "platform": platform(), "profiles": ps, "catalog": catalog(), "active": active, "history": hist, "job": j, "running": r, "running_kind": kind, "last_exit": exit, "data_path": m.Data, "runtime_candidates": m.runtimeCandidates(), "settings": m.getSettings(), "runtime_recipe": runtimeRecipeForPlatform(), "network_services": networkServices(), "prsl_available": false, "chat_available": false, "distribution": m.distributionStatus(), "source": m.sourceStatus(), "effective_baseline": "1.40b23"}, nil
}
func encode(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func (m *Manager) applicationIdentity() map[string]any {
	exe, _ := os.Executable()
	return map[string]any{"version": Version, "executable": exe, "pid": os.Getpid(), "root": m.distributionRoot(), "portable": m.Portable, "data": m.Data}
}

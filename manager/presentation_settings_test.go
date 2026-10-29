package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPresentationLegacyDefaultsAndRuntimeWiring(t *testing.T) {
	p := validProfile()
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	var old map[string]any
	if e = json.Unmarshal(b, &old); e != nil {
		t.Fatal(e)
	}
	delete(old, "presentation")
	b, _ = json.Marshal(old)
	var loaded Profile
	if e = json.Unmarshal(b, &loaded); e != nil {
		t.Fatal(e)
	}
	cfg, e := dosboxConfig(t.TempDir(), loaded)
	if e != nil {
		t.Fatal(e)
	}
	if cfg != harnessBaseConfig() {
		t.Fatal("old profile defaults changed the certified harness configuration")
	}
	before, e := resolve(loaded)
	if e != nil {
		t.Fatal(e)
	}
	loaded.Fullscreen = true
	loaded.Presentation = PresentationSettings{WindowSize: "1024x768", MuteAudio: true, MuteInactive: true, PauseInactive: true}
	cfg, e = dosboxConfig(t.TempDir(), loaded)
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range []string{"window_size = 1024x768", "mute_when_inactive = on", "pause_when_inactive = on", "[mixer]\nnosound = on"} {
		if !strings.Contains(cfg, line) {
			t.Fatalf("missing runtime setting %q", line)
		}
	}
	after, e := resolve(loaded)
	if e != nil {
		t.Fatal(e)
	}
	if before.Fingerprint != after.Fingerprint || before.Config != after.Config {
		t.Fatal("presentation settings changed game rules or compatibility")
	}
	args := dosboxArguments("run.conf", "game", loaded)
	if !strings.Contains(strings.Join(args, " "), "--fullscreen") {
		t.Fatal("fullscreen not passed to runtime")
	}
	for _, role := range []string{"host", "join"} {
		loaded.Role = role
		loaded.Host = "127.0.0.1"
		loaded.Port = 21300
		cfg, e = dosboxConfig(t.TempDir(), loaded)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(cfg, "pause_when_inactive = off") || strings.Contains(cfg, "pause_when_inactive = on") {
			t.Fatal("network game must not pause when launcher receives focus")
		}
		if !loaded.Presentation.PauseInactive {
			t.Fatal("network role erased saved single-player preference")
		}
	}
}

func TestPresentationEnumValidation(t *testing.T) {
	for _, size := range []string{"", "800x600", "1024x768", "1280x960", "small", "medium", "large"} {
		p := validProfile()
		p.Presentation.WindowSize = size
		if _, e := resolve(p); e != nil {
			t.Fatalf("%q: %v", size, e)
		}
	}
	for _, size := range []string{"0x0", "999999x999999", "900x700", "800x600\n[autoexec]\nexit", "LARGE", " auto ", "../../x"} {
		p := validProfile()
		p.Presentation.WindowSize = size
		if _, e := resolve(p); e == nil {
			t.Fatalf("accepted %q", size)
		}
		if _, e := dosboxConfig(t.TempDir(), p); e == nil {
			t.Fatalf("runtime accepted %q", size)
		}
	}
}

func TestPresentationPersistenceIsScopedAndSurvivesRestart(t *testing.T) {
	s := testServer(t)
	m := s.manager
	p, e := m.loadProfile("community")
	if e != nil {
		t.Fatal(e)
	}
	otherPath := filepath.Join(m.Data, "profiles", "baseline.json")
	otherBefore, e := os.ReadFile(otherPath)
	if e != nil {
		t.Fatal(e)
	}
	prefs := PresentationSettings{WindowSize: "large", MuteAudio: true, MuteInactive: true, PauseInactive: true}
	saved, e := m.savePresentation(p.ID, true, prefs)
	if e != nil {
		t.Fatal(e)
	}
	expected := p
	expected.Fullscreen = true
	expected.Presentation = prefs
	if !reflect.DeepEqual(saved, expected) {
		t.Fatal("presentation save changed unrelated profile fields", saved, expected)
	}
	restarted, e := newManager(m.Root, m.Data)
	if e != nil {
		t.Fatal(e)
	}
	got, e := restarted.loadProfile(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("preferences did not survive reopen")
	}
	if restarted.selectedProfileID() != p.ID {
		t.Fatal("saved profile selection did not survive reopen")
	}
	otherAfter, _ := os.ReadFile(otherPath)
	if !bytes.Equal(otherBefore, otherAfter) {
		t.Fatal("unrelated legacy profile changed")
	}
	entries, e := os.ReadDir(filepath.Join(m.Data, "environments"))
	if e != nil || len(entries) != 0 {
		t.Fatal("settings save prepared or touched game generations", e)
	}
	before, _ := os.ReadFile(filepath.Join(m.Data, "profiles", p.ID+".json"))
	if _, e = m.savePresentation(p.ID, false, PresentationSettings{WindowSize: "invalid"}); e == nil {
		t.Fatal("invalid setting accepted")
	}
	after, _ := os.ReadFile(filepath.Join(m.Data, "profiles", p.ID+".json"))
	if !bytes.Equal(before, after) {
		t.Fatal("invalid save changed profile")
	}
	if _, e = m.savePresentation("../escape", false, PresentationSettings{}); e == nil {
		t.Fatal("path traversal accepted")
	}
	if _, e = m.savePresentation("missing", false, PresentationSettings{}); e == nil {
		t.Fatal("missing profile created implicitly")
	}
}

func TestPresentationSaveFailureReported(t *testing.T) {
	s := testServer(t)
	m := s.manager
	if e := os.Mkdir(filepath.Join(m.Data, "ui-selection.json"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := m.savePresentation("community", true, PresentationSettings{WindowSize: "small"}); e == nil {
		t.Fatal("selection persistence failure hidden")
	}
	if _, e := m.loadProfile("community"); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(m.Data, "ui-selection.json")); e != nil {
		t.Fatal(e)
	}
	if _, e := m.savePresentation("community", true, PresentationSettings{WindowSize: "small"}); e != nil {
		t.Fatal("retry failed", e)
	}
}

func TestPresentationAPIValidationAndAuthentication(t *testing.T) {
	s := testServer(t)
	body := `{"action":"presentation-save","id":"community","fullscreen":true,"presentation":{"window_size":"medium","mute_audio":true}}`
	for _, tc := range []struct {
		auth, origin string
		code         int
	}{{"", "", 401}, {"Bearer testtoken", "https://evil.test", 403}} {
		w := req(s, "POST", "/api/action", body, tc.auth, tc.origin, s.host)
		if w.Code != tc.code {
			t.Fatal(w.Code)
		}
	}
	for _, bad := range []string{
		`{"action":"presentation-save","id":"community","presentation":{"command":"exit"}}`,
		`{"action":"presentation-save","id":"community","presentation":{"mute_audio":"yes"}}`,
		`{"action":"presentation-save","id":"community","presentation":{"window_size":123}}`,
	} {
		w := req(s, "POST", "/api/action", bad, "Bearer testtoken", "", s.host)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := req(s, "POST", "/api/action", `{"action":"presentation-save","id":"community","presentation":{"window_size":"99999x1"}}`, "Bearer testtoken", "", s.host)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if j := awaitGUIJob(t, s.manager); j.Error == "" {
		t.Fatal("bad enum accepted")
	}
	w = req(s, "POST", "/api/action", body, "Bearer testtoken", "", s.host)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if j := awaitGUIJob(t, s.manager); j.Error != "" {
		t.Fatal(j.Error)
	}
	p, e := s.manager.loadProfile("community")
	if e != nil || !p.Fullscreen || p.Presentation.WindowSize != "medium" || !p.Presentation.MuteAudio {
		t.Fatal(p, e)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func awaitGUIJob(t *testing.T, m *Manager) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		j := m.job
		m.mu.Unlock()
		if !j.Busy {
			return j
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func TestIdentityRoutePrivateAndCheap(t *testing.T) {
	s := testServer(t)
	for _, tc := range []struct {
		method, auth, origin string
		code                 int
	}{
		{"GET", "", "", 401}, {"GET", "Bearer testtoken", "https://other.test", 403}, {"POST", "Bearer testtoken", "", 405}, {"GET", "Bearer testtoken", "", 200},
	} {
		w := req(s, tc.method, "/api/identity", "", tc.auth, tc.origin, s.host)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "testtoken") {
			t.Fatal("private token leaked")
		}
	}
}
func TestBrowseAPIFixedKindsAndCancel(t *testing.T) {
	s := testServer(t)
	calls := 0
	s.pick = func(kind string) (string, error) { calls++; return "", nil }
	for _, body := range []string{`{"action":"browse","picker_kind":"shell"}`, `{"action":"browse","picker_kind":"game-folder","command":"anything"}`} {
		w := req(s, "POST", "/api/action", body, "Bearer testtoken", "", s.host)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatal("unsupported request opened dialog")
	}
	w := req(s, "POST", "/api/action", `{"action":"browse","picker_kind":"game-folder"}`, "Bearer testtoken", "", s.host)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	j := awaitGUIJob(t, s.manager)
	b, _ := json.Marshal(j.Result)
	if calls != 1 || j.Error != "" || !strings.Contains(string(b), `"canceled":true`) {
		t.Fatal(calls, j, string(b))
	}
	if _, e := os.Stat(filepath.Join(s.manager.Data, "cache", "base-source.json")); !os.IsNotExist(e) {
		t.Fatal("cancel imported source", e)
	}
}
func TestBrowseRequiresAuthAndReturnsOnlyChosenPath(t *testing.T) {
	s := testServer(t)
	chosen := t.TempDir()
	calls := 0
	s.pick = func(string) (string, error) { calls++; return chosen, nil }
	for _, tc := range []struct {
		auth, origin string
		code         int
	}{{"", "", 401}, {"Bearer testtoken", "https://evil.test", 403}} {
		w := req(s, "POST", "/api/action", `{"action":"browse","picker_kind":"game-folder"}`, tc.auth, tc.origin, s.host)
		if w.Code != tc.code {
			t.Fatal(w.Code)
		}
	}
	if calls != 0 {
		t.Fatal("unauthorized chooser opened")
	}
	w := req(s, "POST", "/api/action", `{"action":"browse","picker_kind":"game-folder"}`, "Bearer testtoken", "", s.host)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	j := awaitGUIJob(t, s.manager)
	b, _ := json.Marshal(j.Result)
	if !strings.Contains(string(b), chosen) || j.Error != "" {
		t.Fatal(string(b), j)
	}
}
func TestDesktopOnboardingRecommendationPreservesSavedChoice(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	if m.selectedProfileID() != "baseline" {
		t.Fatal("legacy default changed")
	}
	m.DesktopGUI = true
	if m.selectedProfileID() != "community" {
		t.Fatal("desktop recommendation not standard Community")
	}
	p, _ := m.loadProfile("community")
	if p.Engine != "1.50.26" || p.Core != "150" || len(p.Mods) != 0 || p.Role != "standalone" {
		t.Fatal(p)
	}
	if e := m.rememberProfile("baseline"); e != nil {
		t.Fatal(e)
	}
	if m.selectedProfileID() != "baseline" {
		t.Fatal("explicit legacy choice overwritten")
	}
}
func TestCancellationStopsBeforeCommitAndCanRetry(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	gate := make(chan struct{})
	if e := m.startJob("prepare-play", func() (any, error) { <-gate; return nil, m.beginCommit() }); e != nil {
		t.Fatal(e)
	}
	if e := m.cancelOperation(); e != nil {
		t.Fatal(e)
	}
	close(gate)
	j := awaitGUIJob(t, m)
	if j.Error != "" || !strings.Contains(j.Message, "Canceled safely") || j.Cancelable {
		t.Fatal(j)
	}
	if e := m.startJob("prepare-play", func() (any, error) { return nil, m.checkCancellation() }); e != nil {
		t.Fatal(e)
	}
	j = awaitGUIJob(t, m)
	if j.Error != "" || j.CancelRequested {
		t.Fatal("retry retains cancellation", j)
	}
}
func TestCancellationFenceAndRuntimeSubstep(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	m.job = Job{Busy: true, Kind: "prepare-play", Cancelable: true}
	if e := m.beginRuntimeCommit(); e != nil {
		t.Fatal(e)
	}
	if e := m.cancelOperation(); e == nil {
		t.Fatal("accepted cancellation during runtime commit")
	}
	m.endRuntimeCommit()
	if e := m.cancelOperation(); e != nil {
		t.Fatal("later game preparation cannot cancel", e)
	}
	if !errors.Is(m.beginCommit(), errOperationCanceled) {
		t.Fatal("canceled game activated")
	}
	m.job = Job{Busy: true, Kind: "runtime-import", Cancelable: true}
	if e := m.beginRuntimeCommit(); e != nil {
		t.Fatal(e)
	}
	m.endRuntimeCommit()
	if e := m.cancelOperation(); e == nil {
		t.Fatal("direct runtime import reopened cancellation after commit")
	}
}
func TestPrepareCancellationDoesNotReadOrMutateSource(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	m.job = Job{CancelRequested: true}
	original := filepath.Join(t.TempDir(), "owned.zip")
	before := []byte("not a game: must not be read or changed")
	if e := os.WriteFile(original, before, 0600); e != nil {
		t.Fatal(e)
	}
	_, e := m.prepareForPlay(validProfile(), original)
	if !errors.Is(e, errOperationCanceled) {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(original)
	if string(after) != string(before) {
		t.Fatal("original changed")
	}
}
func TestPreparedStateUsesActualGenerationProfile(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	p, _ := m.loadProfile("community")
	r, _ := resolve(p)
	dir := filepath.Join(m.Data, "environments", p.ID, "g1")
	if e := os.MkdirAll(filepath.Join(dir, "game"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := atomicJSON(filepath.Join(dir, "manifest.json"), Manifest{Schema: 3, Profile: p}); e != nil {
		t.Fatal(e)
	}
	if e := atomicJSON(filepath.Join(m.Data, "active", p.ID+".json"), Active{"g1"}); e != nil {
		t.Fatal(e)
	}
	// Saved desired profile changes, but prepared identity must remain the old copy.
	p.Core = "150i"
	if e := m.saveProfile(p); e != nil {
		t.Fatal(e)
	}
	v, e := m.state()
	if e != nil {
		t.Fatal(e)
	}
	prepared := v.(map[string]any)["prepared"].(map[string]any)[p.ID].(map[string]string)
	if prepared["fingerprint"] != r.Fingerprint || prepared["core"] != "150" {
		t.Fatal(prepared)
	}
}

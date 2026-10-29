package main

import (
	"errors"
	"path/filepath"
)

// Stored with user data, not sessionStorage: the localhost port changes on restart.
// This is UI preference only. Selecting a profile never prepares or patches it.
type UISelection struct {
	Schema    int    `json:"schema"`
	ProfileID string `json:"profile_id"`
}

func (m *Manager) rememberProfile(id string) error {
	if _, e := m.loadProfile(id); e != nil {
		return errors.New("select an existing valid profile")
	}
	m.uiMu.Lock()
	defer m.uiMu.Unlock()
	return atomicJSON(filepath.Join(m.Data, "ui-selection.json"), UISelection{1, id})
}
func (m *Manager) selectedProfileID() string {
	m.uiMu.Lock()
	defer m.uiMu.Unlock()
	var saved UISelection
	if e := readJSON(filepath.Join(m.Data, "ui-selection.json"), &saved); e == nil && saved.Schema == 1 {
		if _, e = m.loadProfile(saved.ProfileID); e == nil {
			return saved.ProfileID
		}
	}
	// A desktop first run recommends the unmodified Community standard profile.
	// Existing saved choices and legacy CLI defaults are never silently changed.
	if m.DesktopGUI {
		return "community"
	}
	// First run, removed profile, invalid preferences: retain the accepted baseline default.
	return "baseline"
}

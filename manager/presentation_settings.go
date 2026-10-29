package main

import (
	"errors"
	"strings"
)

// Presentation preferences never change game rules, emulated sound hardware,
// CPU timing or compatibility fingerprints. Empty values preserve 0.5.0 defaults.
type PresentationSettings struct {
	WindowSize    string `json:"window_size,omitempty"`
	MuteAudio     bool   `json:"mute_audio,omitempty"`
	MuteInactive  bool   `json:"mute_inactive,omitempty"`
	PauseInactive bool   `json:"pause_inactive,omitempty"`
}

func validatePresentation(s PresentationSettings) error {
	switch s.WindowSize {
	case "", "800x600", "1024x768", "1280x960", "small", "medium", "large":
		return nil
	default:
		return errors.New("choose one of the supported starting window sizes")
	}
}

func presentationBaseConfig(p Profile) string {
	cfg := harnessBaseConfig()
	if p.Presentation.WindowSize != "" && p.Presentation.WindowSize != "800x600" {
		cfg = strings.Replace(cfg, "window_size = 800x600", "window_size = "+p.Presentation.WindowSize, 1)
	}
	if p.Presentation.MuteInactive {
		cfg = strings.Replace(cfg, "mute_when_inactive = off", "mute_when_inactive = on", 1)
	}
	// Keep network sessions alive even when the player switches to the launcher.
	if p.Presentation.PauseInactive && p.Role == "standalone" {
		cfg = strings.Replace(cfg, "pause_when_inactive = off", "pause_when_inactive = on", 1)
	}
	if p.Presentation.MuteAudio {
		cfg += "\n[mixer]\nnosound = on\n"
	}
	return cfg
}

// Save only presentation fields. Editing settings cannot accidentally save an
// unsaved engine/mod/network change or alter prepared files and personal saves.
func (m *Manager) savePresentation(id string, fullscreen bool, settings PresentationSettings) (Profile, error) {
	if e := validatePresentation(settings); e != nil {
		return Profile{}, e
	}
	p, e := m.loadProfile(id)
	if e != nil {
		return Profile{}, e
	}
	p.Fullscreen, p.Presentation = fullscreen, settings
	if e = m.saveProfile(p); e != nil {
		return Profile{}, e
	}
	return p, nil
}

package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"moo2manager/shared/buildconfig"
	"net"
	"regexp"
	"sort"
	"strings"
)

const Version = buildconfig.Version
const BaseHash = "87e657bc2e8b02714856f13c3c63066cd58549f33a9c2e917d1bca6936142d2b"
const PatchHash = "0ac9151e9cf752ec34b6db998d8b390f48e492d8443e8f1658112b93c7f6a921"
const EngineHash = "2db296e052419250d21866f7c23ac2978a33b3c05b451a9516f06599b91c3f5c"
const BaseEngineHash = "4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f"

//go:embed assets/*.json assets/harness/* assets/diagnostics/* web/*
var resources embed.FS

type Mod struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Group        string   `json:"group"`
	Order        int      `json:"order"`
	Description  string   `json:"description"`
	Path         string   `json:"path"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies"`
	Source       string   `json:"source"`
}
type Profile struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Engine         string               `json:"engine"`
	Core           string               `json:"core"`
	Mods           []string             `json:"mods"`
	Fullscreen     bool                 `json:"fullscreen"`
	Presentation   PresentationSettings `json:"presentation,omitempty"`
	Role           string               `json:"role"`
	NetworkService string               `json:"network_service,omitempty"`
	Host           string               `json:"host"`
	Port           int                  `json:"port"`
}
type Resolution struct {
	Requested   Profile     `json:"profile"`
	Mods        []Mod       `json:"mods"`
	Automatic   []string    `json:"automatic"`
	Fingerprint string      `json:"fingerprint"`
	Config      string      `json:"config"`
	Warnings    []string    `json:"warnings"`
	Network     NetworkPlan `json:"network"`
}

type NetworkServiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Available   bool   `json:"available"`
}

func networkServices() []NetworkServiceInfo {
	return []NetworkServiceInfo{
		{ID: "direct", Name: "Direct / LAN", Description: "Host a DOSBox IPX tunnel locally, or join a host by address. LAN needs no router forwarding; internet hosting normally does.", Available: true},
		{ID: "dopefish", Name: "moo2.thedopefish.com", Description: "Legacy third-party public DOSBox IPX rendezvous service. Everyone connects to the same service; create or join the MOO2 game inside MOO2.", Host: DopefishHost, Port: DopefishPort, Available: true},
		{ID: "sgc", Name: "MOO2-SGC Online", Description: "Reserved for the future MOO2-SGC matchmaking / relay service.", Available: false},
	}
}

func effectiveNetworkService(p Profile) string {
	if p.Role == "standalone" {
		return "none"
	}
	if p.NetworkService == "" || p.NetworkService == "none" {
		return "direct"
	} // migration for profiles created before 0.4.5
	return p.NetworkService
}
func validHostOrIPv4(s string) bool {
	if ip := net.ParseIP(s); ip != nil {
		return ip.To4() != nil
	}
	if len(s) < 1 || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) < 1 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

func catalog() []Mod {
	b, _ := resources.ReadFile("assets/catalog.json")
	var all []Mod
	if err := json.Unmarshal(b, &all); err != nil {
		panic(err)
	}
	out := []Mod{}
	for _, m := range all {
		if m.Group == "_user" {
			continue
		}
		if m.Group == "0" {
			m.Group = ""
		}
		out = append(out, m)
	}
	return out
}
func validateProfile(p Profile) error {
	if e := validatePresentation(p.Presentation); e != nil {
		return e
	}
	if !idPattern.MatchString(p.ID) {
		return errors.New("profile ID must be 1–40 lowercase letters, digits, underscores or hyphens")
	}
	if len(strings.TrimSpace(p.Name)) == 0 || len(p.Name) > 100 {
		return errors.New("profile name must be 1–100 characters")
	}
	if engineRank(p.Engine) < 0 {
		return errors.New("supported engines: 1.2, 1.31 (legacy), 1.40b23 baseline and 1.50.26")
	}
	if p.Role != "standalone" && p.Role != "host" && p.Role != "join" {
		return errors.New("choose Play, Host or Join")
	}
	service := effectiveNetworkService(p)
	if p.Role != "standalone" {
		switch service {
		case "direct":
			if p.Port < 1 || p.Port > 65535 {
				return errors.New("UDP port must be 1–65535")
			}
			if p.Role == "host" && p.Port < 1024 {
				return errors.New("direct hosting uses UDP port 1024–65535 for cross-platform compatibility")
			}
			if p.Role == "join" && !validHostOrIPv4(p.Host) {
				return errors.New("Direct Join requires a valid IPv4 address or DNS hostname")
			}
		case "dopefish":
			// Endpoint and UDP port are project-defined, not user input.
		case "sgc":
			return errors.New("MOO2-SGC Online is reserved for the future matchmaking / relay service and is not available yet")
		default:
			return errors.New("choose a supported network service")
		}
	}
	if len(p.Mods) > 100 {
		return errors.New("too many mods")
	}
	return nil
}
func resolve(p Profile) (Resolution, error) {
	r := Resolution{Requested: p, Mods: []Mod{}, Automatic: []string{}, Warnings: []string{}}
	if err := validateProfile(p); err != nil {
		return r, err
	}
	if p.Engine != "1.50.26" {
		if p.Core != "" || len(p.Mods) > 0 {
			return r, errors.New("this historical-engine profile cannot use 1.50 mods; clear the ruleset and add-ons")
		}
	} else {
		byID := map[string]Mod{}
		for _, m := range catalog() {
			byID[m.ID] = m
		}
		core, ok := byID[p.Core]
		if !ok || core.Group != "Core" {
			return r, errors.New("choose one supported Core ruleset")
		}
		explicit := map[string]bool{p.Core: true}
		for _, id := range p.Mods {
			explicit[id] = true
		}
		selected := map[string]bool{}
		visiting := map[string]bool{}
		groups := map[string]string{}
		var add func(string) error
		add = func(id string) error {
			if selected[id] {
				return nil
			}
			if visiting[id] {
				return fmt.Errorf("dependency cycle at %s", id)
			}
			if strings.EqualFold(id, "prsl") || strings.EqualFold(id, "chat") {
				return errors.New("PRSL and the new Chat extension have no certified live adapters; they cannot be enabled")
			}
			m, ok := byID[id]
			if !ok {
				return fmt.Errorf("unknown or unsupported mod %q", id)
			}
			if m.Group != "" {
				if prev, ok := groups[m.Group]; ok && prev != id {
					return fmt.Errorf("%s conflict: %s and %s. A Core ruleset may require its own map/mirror mod; remove the competing choice", m.Group, prev, id)
				}
				groups[m.Group] = id
			}
			visiting[id] = true
			for _, d := range m.Dependencies {
				if err := add(d); err != nil {
					return err
				}
			}
			delete(visiting, id)
			selected[id] = true
			if !explicit[id] {
				r.Automatic = append(r.Automatic, id)
			}
			return nil
		}
		if err := add(p.Core); err != nil {
			return r, err
		}
		for _, id := range p.Mods {
			if err := add(id); err != nil {
				return r, err
			}
		}
		for id := range selected {
			r.Mods = append(r.Mods, byID[id])
		}
		sort.Slice(r.Mods, func(i, j int) bool {
			if r.Mods[i].Order != r.Mods[j].Order {
				return r.Mods[i].Order < r.Mods[j].Order
			}
			return r.Mods[i].ID < r.Mods[j].ID
		})
		sort.Strings(r.Automatic)
		r.Config = "# Generated by MOO2 Mod Manager. No PRSL or Chat injection.\r\n"
		for _, m := range r.Mods {
			r.Config += "enable " + m.ID + ";\r\n"
		}
	}
	// Local name, network role and display settings intentionally do not affect game compatibility.
	identity := struct{ Engine, Base, Patch, Config string }{p.Engine, BaseHash, "", r.Config}
	if p.Engine == "1.50.26" {
		identity.Patch = PatchHash
	}
	if p.Engine == "1.40b23" {
		identity.Base = BaselineEngineHash
	}
	if p.Engine == "1.2" {
		identity.Base = CDEngineHash
	}
	b, _ := json.Marshal(identity)
	sum := sha256.Sum256(b)
	r.Fingerprint = hex.EncodeToString(sum[:])
	r.Network = networkPlan(p)
	r.Warnings = append(r.Warnings, "Structural configuration checks only: no gameplay or cross-platform multiplayer certification.", "Loaded MOO2 saves restore saved gameplay configuration; start a new game to test newly selected rules.")
	return r, nil
}
func defaultProfiles() []Profile {
	return []Profile{
		{ID: "community", Name: "Community — standard", Engine: "1.50.26", Core: "150", Mods: []string{}, Role: "standalone", NetworkService: "none", Port: 21300},
		{ID: "multiplayer", Name: "Community — multiplayer", Engine: "1.50.26", Core: "150m", Mods: []string{}, Role: "host", NetworkService: "dopefish", Port: 213},
		{ID: "original", Name: "Legacy official DOS 1.31", Engine: "1.31", Mods: []string{}, Role: "standalone", NetworkService: "none", Port: 21300},
		{ID: "baseline", Name: "Portable baseline — 1.40b23", Engine: "1.40b23", Mods: []string{}, Role: "standalone", NetworkService: "none", Port: 21300},
		{ID: "cd-original", Name: "Original CD — 1.2", Engine: "1.2", Mods: []string{}, Role: "standalone", NetworkService: "none", Port: 21300},
	}
}

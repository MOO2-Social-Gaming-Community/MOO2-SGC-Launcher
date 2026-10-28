package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDopefishDiagnosticIsGameIndependent(t *testing.T) {
	s := dopefishDiagnosticConfig()
	for _, want := range []string{"[ipx]", "ipx = true", "IPXNET CONNECT moo2.thedopefish.com 213", "IPXNET STATUS", "type EXIT"} {
		if !strings.Contains(s, want) {
			t.Fatal(want)
		}
	}
	for _, line := range strings.Split(s, "\n") {
		l := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(l, "MOUNT ") || strings.HasPrefix(l, "IMGMOUNT ") || strings.HasPrefix(l, "ORION") || l == "EXIT" || strings.Contains(l, "STARTSERVER") {
			t.Fatal("unsafe diagnostic", l)
		}
	}
	if !strings.Contains(s, "fullscreen = off") {
		t.Fatal("diagnostic must leave output readable")
	}
}
func TestDiagnosticWithoutRuntimeFailsWithoutGameMutation(t *testing.T) {
	// An empty selected runtime set is normal for a fresh portable Linux fixture.
	m, _ := newManager(t.TempDir(), t.TempDir())
	// No game archive or environment is needed to generate the diagnostic.
	if len(m.runtimeCandidates()) == 0 {
		if _, e := m.checkDopefish(); e == nil {
			t.Fatal("pretended to connect")
		}
	}
	entries, _ := os.ReadDir(filepath.Join(m.Data, "environments"))
	if len(entries) != 0 {
		t.Fatal("diagnostic created game")
	}
}
func TestNetworkPlanAndLaunchCommandStayIdentical(t *testing.T) {
	for _, engine := range []string{"1.40b23", "1.50.26"} {
		for _, role := range []string{"host", "join"} {
			p := Profile{ID: "test", Name: "Test", Engine: engine, Role: role, NetworkService: "dopefish", Port: 21300}
			if engine == "1.50.26" {
				p.Core = "150"
			}
			r, e := resolve(p)
			if e != nil {
				t.Fatal(e)
			}
			cfg, e := dosboxConfig(t.TempDir(), p)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(cfg, r.Network.Command) || r.Network.Command != "IPXNET CONNECT moo2.thedopefish.com 213" || strings.Contains(cfg, "STARTSERVER") {
				t.Fatal(cfg, r.Network)
			}
			if r.Network.ConnectionVerified {
				t.Fatal("configuration falsely certified connection")
			}
		}
	}
	p := validProfile()
	p.Role = "standalone"
	p.NetworkService = "dopefish"
	r, e := resolve(p)
	if e != nil || r.Network.Enabled || r.Network.Command != "" {
		t.Fatal(r, e)
	}
}

func TestRuntimeCompletionDistinguishesProcessFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process fixture")
	}
	for _, code := range []string{"0", "47"} {
		t.Run(code, func(t *testing.T) {
			root := t.TempDir()
			exe := filepath.Join(root, "dosbox")
			if e := os.WriteFile(exe, []byte("#!/bin/sh\n# Explicit process test double; no DOSBox or connection\nexit "+code+"\n"), 0700); e != nil {
				t.Fatal(e)
			}
			m, e := newManager(root, filepath.Join(root, "userdata"))
			if e != nil {
				t.Fatal(e)
			}
			if e = atomicJSON(filepath.Join(m.Data, "settings.json"), Settings{exe}); e != nil {
				t.Fatal(e)
			}
			if _, e = m.checkDopefish(); e != nil {
				t.Fatal(e)
			}
			e = m.waitForRuntime()
			if (e == nil) != (code == "0") {
				t.Fatalf("runtime exit %s: %v", code, e)
			}
			var record map[string]any
			if e = readJSON(filepath.Join(m.Data, "logs", "last-network-check.json"), &record); e != nil {
				t.Fatal(e)
			}
			expected := float64(0)
			if code == "47" {
				expected = 47
			}
			if record["process_exit_code"] != expected || record["process_exited"] != true {
				t.Fatal(record)
			}
			if record["connection_verified"] != false || record["game_started"] != false {
				t.Fatal("invented connectivity", record)
			}
			m.mu.Lock()
			running := m.running != nil
			m.mu.Unlock()
			if running {
				t.Fatal("process completion did not release gate")
			}
		})
	}
}

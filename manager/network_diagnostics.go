package main

import (
	"errors"
	"fmt"
	"moo2manager/shared/desktop"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const DopefishHost = "moo2.thedopefish.com"
const DopefishPort = 213

type NetworkPlan struct {
	Enabled            bool   `json:"enabled"`
	Service            string `json:"service"`
	Role               string `json:"role"`
	Command            string `json:"command,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
	Instructions       string `json:"instructions"`
	ConnectionVerified bool   `json:"connection_verified"`
}

func networkCommand(p Profile) (string, error) {
	if e := validateProfile(p); e != nil {
		return "", e
	}
	switch effectiveNetworkService(p) {
	case "none":
		return "", nil
	case "dopefish":
		return fmt.Sprintf("IPXNET CONNECT %s %d", DopefishHost, DopefishPort), nil
	case "direct":
		if p.Role == "host" {
			return fmt.Sprintf("IPXNET STARTSERVER %d", p.Port), nil
		}
		return fmt.Sprintf("IPXNET CONNECT %s %d", p.Host, p.Port), nil
	}
	return "", errors.New("unsupported network service")
}
func networkPlan(p Profile) NetworkPlan {
	c, _ := networkCommand(p)
	n := NetworkPlan{Enabled: p.Role != "standalone", Service: effectiveNetworkService(p), Role: p.Role, Command: c}
	switch n.Service {
	case "none":
		n.Instructions = "Local play: no IPX connection is configured. Choose Create or Join network game before using MOO2 Multiplayer / Network."
	case "dopefish":
		n.Endpoint = fmt.Sprintf("%s UDP %d", DopefishHost, DopefishPort)
		n.Instructions = "Both players connect to the public IPX service. Create or join the named game INSIDE MOO2. Match engine and ruleset; remote availability is not verified by configuration."
	case "direct":
		if p.Role == "host" {
			n.Endpoint = fmt.Sprintf("Local IPX server UDP %d", p.Port)
		} else {
			n.Endpoint = fmt.Sprintf("%s UDP %d", p.Host, p.Port)
		}
		n.Instructions = "Direct/LAN: one local IPX server and clients connected to the same address and UDP port. Match the in-game engine/ruleset."
	}
	return n
}
func dopefishDiagnosticConfig() string {
	b, e := resources.ReadFile("assets/diagnostics/dopefish.conf")
	if e != nil {
		panic(e)
	} // embedded build input
	return string(b)
}

// This launches a real DOSBox CONNECT/STATUS check only when the user requests it.
// It does not equate process startup/exit with IPX connectivity and mounts no game.
func (m *Manager) checkDopefish() (any, error) {
	candidates := m.runtimeCandidates()
	if len(candidates) == 0 {
		return nil, errors.New("DOSBox was not found; select or install the verified runtime first")
	}
	exe := candidates[0]
	var e error
	if m.Portable && runtime.GOOS == "windows" {
		exe, e = m.localHarnessRuntime()
		if e != nil {
			return nil, e
		}
	}
	parent := filepath.Join(m.Data, "network-checks")
	if e = noSymlinkAncestors(parent); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(parent, 0700); e != nil {
		return nil, e
	}
	dir, e := os.MkdirTemp(parent, "dopefish-")
	if e != nil {
		return nil, e
	}
	cfgPath := filepath.Join(dir, "dopefish.conf")
	if e = atomicWrite(cfgPath, []byte(dopefishDiagnosticConfig())); e != nil {
		return nil, e
	}
	logPath := filepath.Join(m.Data, "logs", "dopefish-"+filepath.Base(dir)+".log")
	logfile, e := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	args := []string{"--noprimaryconf", "--conf", cfgPath}
	record := map[string]any{
		"launcher": m.applicationIdentity(), "kind": "dopefish-connection-check", "requested_at": time.Now().UTC().Format(time.RFC3339),
		"runtime": exe, "arguments": args, "config_path": cfgPath, "working_directory": dir, "log": logPath,
		"endpoint": fmt.Sprintf("%s UDP %d", DopefishHost, DopefishPort), "game_started": false,
		"connection_verified": false, "status": "awaiting manual observation of DOSBox CONNECT/STATUS",
		"instructions": "Read the DOSBox CONNECT/STATUS messages; capture that screen. Type EXIT before starting the game. Process exit alone is not connectivity proof.",
	}
	fmt.Fprintln(logfile, "MOO2-SGC", Version, "Dopefish connection diagnostic; no game mounted or launched")
	fmt.Fprintln(logfile, "Arguments:", encode(args))
	if e = atomicJSON(filepath.Join(m.Data, "logs", "last-network-check.json"), record); e != nil {
		logfile.Close()
		return nil, e
	}
	cmd := exec.Command(exe, args...)
	if m.DesktopGUI {
		desktop.SuppressConsole(cmd)
	}
	cmd.Dir = dir
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	m.mu.Lock()
	if m.running != nil {
		m.mu.Unlock()
		logfile.Close()
		return nil, errors.New("close the game or previous network check first")
	}
	if e = cmd.Start(); e != nil {
		_ = recordProcessStartFailure(filepath.Join(m.Data, "logs", "last-network-check.json"), record, e)
		m.mu.Unlock()
		logfile.Close()
		return nil, fmt.Errorf("DOSBox diagnostic failed to start: %w", e)
	}
	m.running = cmd
	m.runtimeDone = make(chan struct{})
	m.runtimeError = nil
	m.runningProfile = "Dopefish connection check"
	m.runningKind = "network-check"
	m.lastExit = ""
	m.mu.Unlock()
	go func() {
		e := cmd.Wait()
		logfile.Close()
		recordError := recordProcessExit(filepath.Join(m.Data, "logs", "last-network-check.json"), record, e)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.runtimeError = e
		defer close(m.runtimeDone)
		m.running = nil
		m.runningProfile = ""
		m.runningKind = ""
		m.lastExit = "Network diagnostic closed. Read the captured DOSBox CONNECT/STATUS result; exit code is not proof of connection."
		if e != nil {
			m.lastExit = "Network diagnostic process failed: " + e.Error() + ". Inspect " + logPath
		}
		if recordError != nil {
			m.lastExit += "; could not save process result: " + recordError.Error()
		}
	}()
	return record, nil
}

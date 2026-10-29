package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type deadMarker struct {
	path  string
	info  os.FileInfo
	bytes []byte
}

func recoveryMarkers(root string, dead func(int) (bool, error)) ([]deadMarker, error) {
	// A dead manager can leave a live emulator behind. Missing PID/ambiguous
	// pre-start records are NOT evidence of exit: retain all protective locks.
	for _, name := range []string{"last-launch.json", "last-network-check.json"} {
		p := filepath.Join(root, "userdata", "logs", name)
		if e := d.NoLinks(p); e != nil {
			return nil, e
		}
		st, e := os.Stat(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() || st.Size() > d.MaxMetadata {
			return nil, errors.New("runtime history is not safely readable")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		var rec struct {
			Exited      bool `json:"process_exited"`
			StartFailed bool `json:"process_start_failed"`
		}
		if e = json.Unmarshal(b, &rec); e != nil {
			return nil, e
		}
		if !rec.Exited && !rec.StartFailed {
			return nil, errors.New("A previous game or connection check has no confirmed exit record. Its process may still be running. Recovery cannot safely clear this lock; nothing was changed")
		}
	}
	markers := []deadMarker{}
	for _, p := range []string{filepath.Join(root, "userdata", "manager.lock"), filepath.Join(root, "installation.lock")} {
		if e := d.NoLinks(p); e != nil {
			return nil, e
		}
		st, e := os.Stat(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() || st.Size() > 64 {
			return nil, errors.New("lock owner is unknown")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
		if e != nil || pid <= 0 || int64(pid) > 2147483647 {
			return nil, errors.New("lock owner is unknown")
		}
		stopped, e := dead(pid)
		if e != nil || !stopped {
			return nil, errors.New("A launcher process is still running or its status cannot be verified. Close it normally before retrying; its lock was not changed")
		}
		markers = append(markers, deadMarker{p, st, b})
	}
	if len(markers) == 0 {
		return nil, errors.New("no interrupted launcher markers need recovery")
	}
	return markers, nil
}
func recoverInterrupted(root string, trust d.Trust) error {
	if e := checkRecoveryContract(root, trust); e != nil {
		return e
	}
	markers, e := recoveryMarkers(root, processDead)
	if e != nil {
		return e
	}
	// Recheck every marker before changing any; a new/unknown owner always wins.
	for _, m := range markers {
		st, e := os.Stat(m.path)
		if e != nil || !os.SameFile(st, m.info) {
			return errors.New("lock changed during recovery; retry without changing files")
		}
		b, e := os.ReadFile(m.path)
		if e != nil || !bytes.Equal(b, m.bytes) {
			return errors.New("lock owner changed during recovery")
		}
	}
	for _, m := range markers {
		dest := m.path + fmt.Sprintf(".recovered-%d", time.Now().UnixNano())
		if e = os.Rename(m.path, dest); e != nil {
			return e
		}
	}
	return nil
}

package main

import (
	"fmt"
	"os/exec"
	"time"
)

// Startup and process completion are separate facts. Neither certifies that
// DOSBox connected, MOO2 displayed correctly, or a multiplayer turn succeeded.
func recordProcessExit(path string, initial map[string]any, processError error) error {
	result := make(map[string]any, len(initial)+5)
	for k, v := range initial {
		result[k] = v
	}
	result["process_exited"] = true
	result["finished_at"] = time.Now().UTC().Format(time.RFC3339)
	result["process_exit_code"] = 0
	result["status"] = "Process exited normally; game and IPX success still require observation"
	if processError != nil {
		result["process_exit_code"] = -1
		if exit, ok := processError.(*exec.ExitError); ok {
			result["process_exit_code"] = exit.ExitCode()
		}
		result["process_error"] = processError.Error()
		result["status"] = "Runtime process failed; inspect the process log"
	}
	return atomicJSON(path, result)
}
func (m *Manager) waitForRuntime() error {
	m.mu.Lock()
	done := m.runtimeDone
	m.mu.Unlock()
	if done == nil {
		return nil
	}
	<-done
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runtimeError != nil {
		return fmt.Errorf("runtime process failed: %w", m.runtimeError)
	}
	return nil
}

func recordProcessStartFailure(path string, initial map[string]any, processError error) error {
	result := make(map[string]any, len(initial)+4)
	for k, v := range initial {
		result[k] = v
	}
	result["process_start_failed"] = true
	result["process_started"] = false
	result["process_error"] = processError.Error()
	result["status"] = "Runtime process could not start; no diagnostic/game process ran"
	return atomicJSON(path, result)
}

// Use only before cmd.Start has been attempted. Failure to save this fact keeps
// the journal conservative and the launch failed; it never permits execution.
func recordKnownLaunchAbort(path string, initial map[string]any, cause error) error {
	if e := recordProcessStartFailure(path, initial, cause); e != nil {
		return fmt.Errorf("%w; no runtime started, but its safe-start history could not be written: %w", cause, e)
	}
	return cause
}

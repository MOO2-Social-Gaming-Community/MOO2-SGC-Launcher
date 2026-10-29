package main

import "errors"

var errOperationCanceled = errors.New("preparation canceled; existing prepared game and original source kept")

func (m *Manager) cancelOperation() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.job.Busy || !m.job.Cancelable {
		return errors.New("this operation is finishing a safe commit and cannot be canceled; wait for it to finish")
	}
	m.job.CancelRequested = true
	if m.cancelJob != nil {
		m.cancelJob()
	}
	m.job.Message = "Stopping the download or finishing the current verification step; keeping your prepared game"
	return nil
}
func (m *Manager) checkCancellation() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job.CancelRequested {
		return errOperationCanceled
	}
	return nil
}
func (m *Manager) beginCommit() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job.CancelRequested {
		return errOperationCanceled
	}
	m.job.Cancelable = false
	return nil
}

// Runtime activation is a short transaction. Reject cancellation during its
// rename, then allow it again for later game-copy work in the combined pipeline.
func (m *Manager) beginRuntimeCommit() error { return m.beginCommit() }
func (m *Manager) endRuntimeCommit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job.Busy && m.job.Kind == "prepare-play" {
		m.job.Cancelable = true
	}
}

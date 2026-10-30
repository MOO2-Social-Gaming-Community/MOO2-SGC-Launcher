package main

import (
	"errors"
	"testing"
)

func TestJobIDsDistinguishIdenticalCompletedRetries(t *testing.T) {
	m := testServer(t).manager
	var previous uint64
	for i := 0; i < 3; i++ {
		if e := m.startJob("prepare-play", func() (any, error) { return nil, errors.New("same failure") }); e != nil {
			t.Fatal(e)
		}
		j := awaitGUIJob(t, m)
		if j.ID == 0 || j.ID <= previous {
			t.Fatal("new job did not receive a distinct increasing ID", j.ID, previous)
		}
		if j.Error != "same failure" || j.Busy {
			t.Fatal(j)
		}
		previous = j.ID
	}
	// IDs distinguish jobs within a process, not users or persistent installations.
	restarted, e := newManager(m.Root, m.Data)
	if e != nil {
		t.Fatal(e)
	}
	if restarted.job.ID != 0 {
		t.Fatal("job identity unexpectedly persisted across restart")
	}
}

func TestRejectedBusyJobDoesNotConsumeIdentity(t *testing.T) {
	m := testServer(t).manager
	gate := make(chan struct{})
	if e := m.startJob("test", func() (any, error) { <-gate; return "done", nil }); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	id := m.job.ID
	m.mu.Unlock()
	if e := m.startJob("test", func() (any, error) { return nil, nil }); e == nil {
		t.Fatal("busy job accepted")
	}
	m.mu.Lock()
	after := m.job.ID
	m.mu.Unlock()
	if after != id {
		t.Fatal("rejected job replaced active identity")
	}
	close(gate)
	if j := awaitGUIJob(t, m); j.ID != id {
		t.Fatal("completion changed identity")
	}
}

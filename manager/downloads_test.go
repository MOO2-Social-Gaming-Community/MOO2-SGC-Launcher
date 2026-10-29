package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type downloadTransport func(*http.Request) (*http.Response, error)

func (f downloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type interruptedDownloadBody struct {
	ctx     context.Context
	started chan struct{}
	first   bool
}

func (b *interruptedDownloadBody) Read(p []byte) (int, error) {
	if !b.first {
		b.first = true
		n := copy(p, "partial")
		return n, nil
	}
	close(b.started)
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (b *interruptedDownloadBody) Close() error { return nil }

func expectUnchangedDownload(t *testing.T, path string) {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil || string(b) != "previous verified package" {
		t.Fatalf("existing cache changed: %q %v", b, e)
	}
	entries, e := os.ReadDir(filepath.Dir(path))
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".download-") {
			t.Fatalf("partial download retained: %s", entry.Name())
		}
	}
}

func TestCancelDownloadDuringHeadersAndBodyThenRetry(t *testing.T) {
	for _, body := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[body], func(t *testing.T) {
			m, e := newManager(t.TempDir(), t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(m.Data, "cache", "package.zip")
			if e = os.WriteFile(path, []byte("previous verified package"), 0600); e != nil {
				t.Fatal(e)
			}
			started := make(chan struct{})
			var oldProgress func(int64, int64)
			client := &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
				if body {
					return &http.Response{StatusCode: 200, ContentLength: 100, Body: &interruptedDownloadBody{ctx: r.Context(), started: started}}, nil
				}
				close(started)
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}
			if e = m.startJob("prepare-play", func() (any, error) {
				ctx := m.downloadContext()
				oldProgress = m.downloadProgress("old package")
				e := downloadVerifiedWithContext(ctx, client, "https://github.com/test", path, strings.Repeat("0", 64), 1024, oldProgress)
				if e != nil {
					return nil, m.downloadError(ctx, e)
				}
				return map[string]bool{"ready_for_launch": true}, nil
			}); e != nil {
				t.Fatal(e)
			}
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			if e = m.cancelOperation(); e != nil {
				t.Fatal(e)
			}
			j := awaitGUIJob(t, m)
			if j.Error != "" || j.Result != nil || !strings.Contains(j.Message, "Canceled safely") {
				t.Fatalf("cancel became success/error: %+v", j)
			}
			expectUnchangedDownload(t, path)
			m.mu.Lock()
			oldMessage := m.job.Message
			m.mu.Unlock()
			m.downloadProgress("stale package")(99, 100)
			m.mu.Lock()
			unchanged := m.job.Message == oldMessage
			m.mu.Unlock()
			if !unchanged {
				t.Fatal("late canceled progress replaced cancellation")
			}
			data := []byte("complete verified package")
			h := sha256.Sum256(data)
			client = &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
				if r.Context().Err() != nil {
					t.Error("retry inherited canceled context")
				}
				return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}
			if e = m.startJob("runtime-install", func() (any, error) {
				m.progress("Retry has its own progress")
				oldProgress(99, 100)
				m.mu.Lock()
				stale := m.job.Message != "Retry has its own progress"
				m.mu.Unlock()
				if stale {
					return nil, errors.New("old canceled progress replaced retry")
				}
				return nil, downloadVerifiedWithContext(m.downloadContext(), client, "https://github.com/test", path, hex.EncodeToString(h[:]), 1024, m.downloadProgress("retry package"))
			}); e != nil {
				t.Fatal(e)
			}
			j = awaitGUIJob(t, m)
			if j.Error != "" || j.CancelRequested || j.Busy {
				t.Fatalf("retry failed: %+v", j)
			}
			b, e := os.ReadFile(path)
			if e != nil || !bytes.Equal(b, data) {
				t.Fatal("retry did not install verified cache", e)
			}
		})
	}
}

func TestDownloadFailureCleanupAndVerification(t *testing.T) {
	for _, tc := range []struct {
		name        string
		length, max int64
		data        string
		status      int
		hash        string
	}{
		{"checksum", 3, 10, "bad", 200, strings.Repeat("0", 64)},
		{"truncated", 10, 20, "short", 200, ""},
		{"unknown-size-overflow", -1, 3, "oversized", 200, ""},
		{"declared-overflow", 50, 10, "", 200, ""},
		{"HTTP-failure", 0, 10, "", 503, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache.zip")
			os.WriteFile(path, []byte("previous verified package"), 0600)
			client := &http.Client{Transport: downloadTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, ContentLength: tc.length, Body: io.NopCloser(strings.NewReader(tc.data))}, nil
			})}
			if e := downloadVerifiedWithContext(context.Background(), client, "https://github.com/test", path, tc.hash, tc.max, nil); e == nil {
				t.Fatal("invalid download accepted")
			}
			expectUnchangedDownload(t, path)
		})
	}
}

func TestDownloadProgressIsBoundedAndHonest(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	m.downloadProgress("DOSBox")(1<<20, 4<<20)
	if !strings.Contains(m.job.Message, "25%") || !strings.Contains(m.job.Message, "Verification follows") {
		t.Fatal(m.job.Message)
	}
	m.downloadProgress("DOSBox")(1<<20, -1)
	if strings.Contains(m.job.Message, "%") || !strings.Contains(m.job.Message, "total size not supplied") {
		t.Fatal(m.job.Message)
	}
	calls := 0
	meter := downloadMeter{report: func(int64, int64) { calls++ }, last: time.Now()}
	for i := 0; i < 100; i++ {
		meter.Write([]byte("x"))
	}
	if calls != 0 {
		t.Fatal("per-chunk progress not throttled", calls)
	}
	meter.last = time.Now().Add(-time.Second)
	meter.Write([]byte("x"))
	if calls != 1 || meter.received != 101 {
		t.Fatal(calls, meter.received)
	}
}

func TestOfficialPatchDownloadCanceledBeforeImport(t *testing.T) {
	m, _ := newManager(t.TempDir(), t.TempDir())
	started := make(chan struct{})
	client := &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	if e := m.startJob("prepare-play", func() (any, error) { return nil, m.downloadOfficial131(client, "https://moo2mod.com/test") }); e != nil {
		t.Fatal(e)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	if e := m.cancelOperation(); e != nil {
		t.Fatal(e)
	}
	j := awaitGUIJob(t, m)
	if j.Error != "" || j.Result != nil || !strings.Contains(j.Message, "Canceled safely") {
		t.Fatal(j)
	}
	if _, e := os.Stat(filepath.Join(m.Data, "cache", "official-1.31.zip")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("canceled patch imported", e)
	}
}

// Exercise the real Go HTTP transport too: a transfer that sends no more bytes
// must be interruptible, not just a test body that cooperatively checks a flag.
func TestDownloadRealTransportCancellation(t *testing.T) {
	for _, headersSent := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting-for-headers", true: "waiting-for-body"}[headersSent], func(t *testing.T) {
			started := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if headersSent {
					w.Header().Set("Content-Length", "100")
					_, _ = w.Write([]byte("partial"))
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}))
			defer srv.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			path := filepath.Join(t.TempDir(), "cache.zip")
			if e := os.WriteFile(path, []byte("previous verified package"), 0600); e != nil {
				t.Fatal(e)
			}
			finished := make(chan error, 1)
			go func() { finished <- downloadVerifiedWithContext(ctx, srv.Client(), srv.URL, path, "", 1024, nil) }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("HTTP request did not start")
			}
			cancel()
			select {
			case e := <-finished:
				if !errors.Is(e, context.Canceled) {
					t.Fatalf("request not canceled: %v", e)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Cancel waited for the network to finish")
			}
			expectUnchangedDownload(t, path)
		})
	}
}

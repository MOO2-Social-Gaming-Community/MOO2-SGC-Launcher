package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type downloadedFile struct{ path, sha256 string }

type downloadMeter struct {
	report          func(int64, int64)
	total, received int64
	last            time.Time
}

func (p *downloadMeter) Write(b []byte) (int, error) {
	p.received += int64(len(b))
	if p.report != nil && time.Since(p.last) >= 250*time.Millisecond {
		p.report(p.received, p.total)
		p.last = time.Now()
	}
	return len(b), nil
}

// Downloads never replace a cached or active package until the caller verifies
// the complete bytes. Canceling the request interrupts blocked headers/body as
// well as an active transfer; failed partial files are removed on every return.
func downloadTemporary(ctx context.Context, client *http.Client, src, dir string, max int64, report func(int64, int64)) (result downloadedFile, err error) {
	if max <= 0 || max >= 1<<62 {
		return result, errors.New("invalid download size limit")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if e != nil {
		return result, e
	}
	resp, e := client.Do(req)
	if e != nil {
		return result, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > max {
		return result, errors.New("download exceeds size limit")
	}
	f, e := os.CreateTemp(dir, ".download-")
	if e != nil {
		return result, e
	}
	name := f.Name()
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	h := sha256.New()
	meter := &downloadMeter{report: report, total: resp.ContentLength, last: time.Now()}
	if report != nil {
		report(0, resp.ContentLength)
	}
	n, e := io.Copy(io.MultiWriter(f, h, meter), io.LimitReader(resp.Body, max+1))
	if e != nil {
		return result, e
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	if n > max {
		return result, errors.New("download exceeded size limit")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return result, errors.New("truncated download")
	}
	if e = f.Sync(); e != nil {
		return result, e
	}
	if e = f.Close(); e != nil {
		return result, e
	}
	if report != nil {
		report(n, resp.ContentLength)
	}
	return downloadedFile{name, hex.EncodeToString(h.Sum(nil))}, nil
}

func downloadVerifiedWithContext(ctx context.Context, client *http.Client, src, dest, want string, max int64, report func(int64, int64)) error {
	d, e := downloadTemporary(ctx, client, src, filepath.Dir(dest), max, report)
	if e != nil {
		return e
	}
	defer os.Remove(d.path)
	if d.sha256 != want {
		return errors.New("download checksum mismatch; cached package was not replaced")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	return replaceFile(d.path, dest)
}

func downloadVerifiedWith(client *http.Client, src, dest, want string, max int64) error {
	return downloadVerifiedWithContext(context.Background(), client, src, dest, want, max, nil)
}

func preparePackageDownload(src, dest string) error {
	u, e := url.Parse(src)
	if e != nil || u.Scheme != "https" || !trustedDownloadHost(u.Hostname()) {
		return errors.New("only approved HTTPS distribution hosts may supply packages")
	}
	if e = noSymlinkAncestors(dest); e != nil {
		return e
	}
	return os.MkdirAll(filepath.Dir(dest), 0700)
}

func downloadVerified(src, dest, want string, max int64) error {
	if e := preparePackageDownload(src, dest); e != nil {
		return e
	}
	return downloadVerifiedWith(downloadClient(), src, dest, want, max)
}

func (m *Manager) downloadContext() context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobContext != nil {
		return m.jobContext
	}
	return context.Background()
}

func (m *Manager) downloadProgress(label string) func(int64, int64) {
	ctx := m.downloadContext()
	return func(received, total int64) {
		if ctx.Err() != nil {
			return
		}
		const mib = 1024 * 1024
		if total > 0 {
			m.progress(fmt.Sprintf("Downloading %s: %.1f / %.1f MiB (%d%%). Verification follows before use.", label, float64(received)/mib, float64(total)/mib, received*100/total))
		} else {
			m.progress(fmt.Sprintf("Downloading %s: %.1f MiB received (total size not supplied). Verification follows before use.", label, float64(received)/mib))
		}
	}
}

func (m *Manager) downloadError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return errOperationCanceled
	}
	return fmt.Errorf("download stopped: %w. Your prepared game is unchanged. Check your connection and retry, or import a previously downloaded supported package", err)
}

func (m *Manager) downloadPackage(label, src, dest, want string, max int64) error {
	if e := m.checkCancellation(); e != nil {
		return e
	}
	if e := preparePackageDownload(src, dest); e != nil {
		return e
	}
	ctx := m.downloadContext()
	m.progress("Connecting to download " + label + "; your prepared game is unchanged…")
	if e := downloadVerifiedWithContext(ctx, downloadClient(), src, dest, want, max, m.downloadProgress(label)); e != nil {
		return m.downloadError(ctx, e)
	}
	m.progress("Download verified: " + label + ". Continuing preparation…")
	return nil
}

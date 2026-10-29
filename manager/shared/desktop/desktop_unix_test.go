//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserOpenerFailureVisibleWithoutSecret(t *testing.T) {
	dir := t.TempDir()
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if e := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 17\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir)
	err := OpenBrowser("http://127.0.0.1:42/#private-secret")
	if err == nil {
		t.Fatal("nonzero browser opener hidden")
	}
	if strings.Contains(err.Error(), "private-secret") {
		t.Fatal("credential leak")
	}
}

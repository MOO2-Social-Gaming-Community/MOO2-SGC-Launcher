package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLauncherURLLoggingPolicy(t *testing.T) {
	const privateURL = "http://127.0.0.1:12345/#synthetic-session-token"
	recovery := filepath.Join(t.TempDir(), "open-launcher.txt")
	for _, fileOnly := range []bool{false, true} {
		name := "ordinary-console"
		if fileOnly {
			name = "update-handoff"
		}
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			printLauncherURL(&out, privateURL, recovery, fileOnly)
			if fileOnly {
				if strings.Contains(out.String(), privateURL) || strings.Contains(out.String(), "synthetic-session-token") {
					t.Fatal("private URL or token in handoff output")
				}
				if !strings.Contains(out.String(), recovery) {
					t.Fatal("missing recovery file guidance")
				}
			} else if !strings.Contains(out.String(), privateURL) {
				t.Fatal("ordinary console lost its private recovery URL")
			}
		})
	}
}

func TestBrowserStartFailureDoesNotEchoPrivateURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows opener lookup differs; no native Windows process fixture")
	}
	t.Setenv("PATH", t.TempDir()) // No browser can execute.
	const privateURL = "http://127.0.0.1:12345/#synthetic-session-token"
	err := openBrowser(privateURL)
	if err == nil {
		t.Fatal("expected unavailable opener error")
	}
	if strings.Contains(err.Error(), privateURL) || strings.Contains(err.Error(), "synthetic-session-token") {
		t.Fatal("browser start failure disclosed private URL")
	}
}

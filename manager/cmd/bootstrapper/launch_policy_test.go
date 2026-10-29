package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestShortcutWriterSelection(t *testing.T) {
	// Never invoke the real platform shortcut writer, even on Windows.
	for _, noShortcut := range []bool{false, true} {
		for _, writerErr := range []error{nil, errors.New("synthetic shortcut failure")} {
			calls := 0
			write := func(root, entry string) error {
				calls++
				if root != "isolated root" || entry != "verified launcher" {
					t.Fatalf("wrong writer target: %q %q", root, entry)
				}
				return writerErr
			}
			// Repeated launches and updates must never call the writer in this mode.
			for n := 0; n < 3; n++ {
				err := installShortcutIfEnabled(noShortcut, "isolated root", "verified launcher", write)
				if noShortcut && err != nil || !noShortcut && err != writerErr {
					t.Fatalf("noShortcut=%v error=%v want=%v", noShortcut, err, writerErr)
				}
			}
			want := 3
			if noShortcut {
				want = 0
			}
			if calls != want {
				t.Fatalf("noShortcut=%v writer calls=%d want=%d", noShortcut, calls, want)
			}
		}
	}
}

func TestLauncherModeArgsIndependentPolicies(t *testing.T) {
	for _, noBrowser := range []bool{false, true} {
		for _, noShortcut := range []bool{false, true} {
			for _, applyingStaged := range []bool{false, true} {
				base := []string{"--app-root", "isolated root", "--command", "serve"}
				want := append([]string(nil), base...)
				if noBrowser {
					want = append(want, "--no-browser")
				}
				if noShortcut {
					want = append(want, "--no-shortcut")
				}
				if applyingStaged {
					want = append(want, "--private-url-file-only")
				}
				if got := appendLauncherModeArgs(base, noBrowser, noShortcut, applyingStaged); !reflect.DeepEqual(got, want) {
					t.Fatalf("got %q want %q", got, want)
				}
			}
		}
	}
}

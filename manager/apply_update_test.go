package main

import (
	"reflect"
	"testing"
)

func TestStagedUpdaterArgsIndependentPolicies(t *testing.T) {
	for _, noBrowser := range []bool{false, true} {
		for _, noShortcut := range []bool{false, true} {
			want := []string{"--install-root", "isolated root", "--command", "apply-staged", "--wait-lock", "30"}
			if noBrowser {
				want = append(want, "--no-browser")
			}
			if noShortcut {
				want = append(want, "--no-shortcut")
			}
			// All subsequent handoffs use the same explicit mode, not saved settings.
			for n := 0; n < 3; n++ {
				if got := stagedUpdaterArgs("isolated root", noBrowser, noShortcut); !reflect.DeepEqual(got, want) {
					t.Fatalf("handoff %d got %q want %q", n, got, want)
				}
			}
		}
	}
}

func TestDesktopUpdateBrowserPolicyRemainsOrthogonal(t *testing.T) {
	for _, desktopGUI := range []bool{false, true} {
		for _, noBrowser := range []bool{false, true} {
			for _, deferOnce := range []bool{false, true} {
				for _, noShortcut := range []bool{false, true} {
					m := &Manager{DesktopGUI: desktopGUI, DeferBrowserOnce: deferOnce, NoShortcut: noShortcut}
					got := m.updaterArgs("isolated root", noBrowser)
					want := stagedUpdaterArgs("isolated root", noBrowser, noShortcut)
					if desktopGUI {
						want = append(want, "--desktop-gui")
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("desktop=%v headless=%v deferred=%v shortcuts=%v got=%q want=%q", desktopGUI, noBrowser, deferOnce, noShortcut, got, want)
					}
					if initialBrowserAllowed(!noBrowser, deferOnce) != (!noBrowser && !deferOnce) {
						t.Fatal("initial browser policy changed")
					}
					// A restart no longer has one-time deferral, but retains explicit headless.
					if initialBrowserAllowed(!noBrowser, false) != !noBrowser {
						t.Fatal("update browser policy changed")
					}
				}
			}
		}
	}
}

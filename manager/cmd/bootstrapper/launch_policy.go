package main

// noShortcut is explicit process policy, not an installation-wide setting.
// Callers reopening an isolated test installation must pass it again.
func installShortcutIfEnabled(noShortcut bool, root, entry string, write func(string, string) error) error {
	if noShortcut {
		return nil
	}
	return write(root, entry)
}

func appendLauncherModeArgs(args []string, noBrowser, noShortcut, applyingStaged bool) []string {
	if noBrowser {
		args = append(args, "--no-browser")
	}
	if noShortcut {
		args = append(args, "--no-shortcut")
	}
	if applyingStaged {
		// The updater captures Setup's inherited stdout/stderr in update-handoff.log.
		// Keep diagnostics, but leave the credential only in the private recovery
		// file (and browser, unless --no-browser).
		args = append(args, "--private-url-file-only")
	}
	return args
}

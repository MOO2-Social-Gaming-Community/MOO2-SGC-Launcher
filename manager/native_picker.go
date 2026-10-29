package main

import "errors"

// The HTTP API selects a fixed dialog type, never a command or dialog script.
func pickerType(kind string) (folder bool, title, filter string, err error) {
	switch kind {
	case "game-folder":
		return true, "Choose your installed MOO2 DOS game folder", "", nil
	case "game-zip":
		return false, "Choose your owned MOO2 ZIP", "ZIP archives\x00*.zip\x00\x00", nil
	case "runtime-folder":
		return true, "Choose an existing DOSBox Staging runtime folder", "", nil
	case "runtime-zip":
		return false, "Choose a DOSBox Staging runtime ZIP", "ZIP archives\x00*.zip\x00\x00", nil
	case "runtime-executable":
		return false, "Choose your DOSBox executable", "DOSBox executable\x00*.exe\x00All files\x00*.*\x00\x00", nil
	case "payload-archive":
		return false, "Choose a local package or owned network kernel", "Supported files\x00*.zip;RKERNEL.COM\x00All files\x00*.*\x00\x00", nil
	case "offline-release":
		return true, "Choose a signed offline launcher release folder", "", nil
	default:
		return false, "", "", errors.New("unsupported file chooser")
	}
}

func pickerResult(kind, path string, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if path == "" {
		return map[string]any{"picker_kind": kind, "canceled": true}, nil
	}
	p, err := localPath(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"picker_kind": kind, "path": p, "canceled": false}, nil
}

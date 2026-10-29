//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func nativePick(kind string) (string, error) {
	folder, title, _, err := pickerType(kind)
	if err != nil {
		return "", err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		script := "POSIX path of (choose file with prompt \"Choose a local MOO2 or DOSBox file\")"
		if folder {
			script = "POSIX path of (choose folder with prompt \"Choose a local MOO2 or DOSBox folder\")"
		}
		cmd = exec.Command("osascript", "-e", script)
	} else if p, e := exec.LookPath("zenity"); e == nil {
		args := []string{"--file-selection", "--title=" + title}
		if folder {
			args = append(args, "--directory")
		}
		cmd = exec.Command(p, args...)
	} else if p, e := exec.LookPath("kdialog"); e == nil {
		arg := "--getopenfilename"
		if folder {
			arg = "--getexistingdirectory"
		}
		cmd = exec.Command(p, "--title", title, arg, ".")
	} else {
		return "", errors.New("this desktop has no supported file chooser; Windows 11 has a built-in chooser. Advanced local-path entry remains available here")
	}
	b, err := cmd.Output()
	if exit, ok := err.(*exec.ExitError); ok && ((runtime.GOOS != "darwin" && exit.ExitCode() == 1) || (runtime.GOOS == "darwin" && strings.Contains(string(exit.Stderr), "(-128)"))) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("the desktop file chooser could not open; retry from a logged-in desktop session")
	}
	return strings.TrimSpace(string(b)), nil
}

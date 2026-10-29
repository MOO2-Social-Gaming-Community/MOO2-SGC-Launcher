//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

func lockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func unlockFile(f *os.File)     { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func processDead(pid int) (bool, error) {
	if pid <= 0 || int64(pid) > 2147483647 {
		return false, errors.New("invalid process identity")
	}
	e := syscall.Kill(pid, 0)
	if e == syscall.ESRCH {
		return true, nil
	}
	if e == nil {
		return false, nil
	}
	return false, e
}

//go:build windows

package main

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

func lockFile(f *os.File) error {
	var ov syscall.Overlapped
	r, _, e := kernel.NewProc("LockFileEx").Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&ov)))
	if r == 0 {
		return e
	}
	return nil
}
func unlockFile(f *os.File) {
	var ov syscall.Overlapped
	_, _, _ = kernel.NewProc("UnlockFileEx").Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ov)))
}
func processDead(pid int) (bool, error) {
	if pid <= 0 || int64(pid) > 2147483647 {
		return false, errors.New("invalid process identity")
	}
	h, e := syscall.OpenProcess(syscall.SYNCHRONIZE|0x1000, false, uint32(pid))
	if e != nil {
		if e == syscall.Errno(87) {
			return true, nil
		}
		return false, e
	}
	defer syscall.CloseHandle(h)
	result, e := syscall.WaitForSingleObject(h, 0)
	if e != nil {
		return false, e
	}
	if result == 0 {
		return true, nil
	}
	if result == 258 {
		return false, nil
	}
	return false, errors.New("process state is unknown")
}

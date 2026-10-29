//go:build windows

// Package desktop contains OS integration, never user-supplied shell commands.
package desktop

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

func HideConsole(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
func OpenBrowser(u string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	target, e := syscall.UTF16PtrFromString(u)
	if e != nil {
		return e
	}
	result, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	if result <= 32 {
		return fmt.Errorf("Windows could not open the default browser (code %d)", result)
	}
	return nil
}
func ShowError(title, message string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)
	_, _, _ = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
}

// SuppressConsole leaves the runtime's show-window policy untouched. In
// particular, do not set STARTF_USESHOWWINDOW/SW_HIDE for the game's SDL window.
// CREATE_NO_WINDOW suppresses only a console; native game visibility still needs
// Windows acceptance on the actual supported DOSBox runtime.
func SuppressConsole(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}

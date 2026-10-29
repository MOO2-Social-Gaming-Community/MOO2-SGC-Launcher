//go:build windows

package desktop

import (
	"os/exec"
	"testing"
)

func TestRuntimeDoesNotRequestHiddenGameWindow(t *testing.T) {
	c := exec.Command("not-executed.exe")
	SuppressConsole(c)
	if c.SysProcAttr == nil || c.SysProcAttr.HideWindow || c.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("runtime should suppress console without hiding game window")
	}
}
func TestHelperMayHideItsConsoleHost(t *testing.T) {
	c := exec.Command("not-executed.exe")
	HideConsole(c)
	if c.SysProcAttr == nil || !c.SysProcAttr.HideWindow || c.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("helper console policy missing")
	}
}

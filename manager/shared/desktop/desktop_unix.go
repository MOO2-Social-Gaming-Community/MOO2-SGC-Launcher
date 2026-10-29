//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

func HideConsole(c *exec.Cmd) {}
func OpenBrowser(u string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	c := exec.Command(name, u)
	if e := c.Start(); e != nil {
		return fmt.Errorf("default browser opener is unavailable")
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case e := <-done:
		if e != nil {
			return fmt.Errorf("default browser opener failed")
		}
		return nil
	case <-time.After(2 * time.Second):
		return nil // Some desktop openers stay alive with the browser.
	}
}
func ShowError(title, message string) {
	var c *exec.Cmd
	if runtime.GOOS == "darwin" {
		// Arguments, not interpolated AppleScript: paths/error text cannot become code.
		c = exec.Command("osascript", "-e", "on run argv\ndisplay alert (item 1 of argv) message (item 2 of argv) as critical\nend run", title, message)
	} else if p, e := exec.LookPath("zenity"); e == nil {
		c = exec.Command(p, "--error", "--title="+title, "--text="+message)
	} else if p, e := exec.LookPath("xmessage"); e == nil {
		c = exec.Command(p, "-center", "-title", title, message)
	}
	if c == nil || c.Run() != nil {
		fmt.Fprintln(os.Stderr, title+": "+message)
	}
}

func SuppressConsole(c *exec.Cmd) {}

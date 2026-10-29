package main

import (
	"fmt"
	d "moo2manager/shared/distribution"
	"os"
	"path/filepath"
)

// Kernel ownership is released on crash. Keep the file so unlink/recreate cannot
// split the lock across two inodes while another process is still running.
func acquireSession(root string) (func(), error) {
	path := filepath.Join(root, "session.lock")
	if e := d.NoLinks(path); e != nil {
		return nil, e
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = lockFile(f); e != nil {
		f.Close()
		return nil, fmt.Errorf("private launcher session is active: %w", e)
	}
	return func() { unlockFile(f); f.Close() }, nil
}

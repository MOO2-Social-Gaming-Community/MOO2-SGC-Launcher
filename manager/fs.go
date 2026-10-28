package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func hashFile(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func requireHash(p, expected string) error {
	h, e := hashFile(p)
	if e != nil {
		return e
	}
	if h != expected {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filepath.Base(p), expected, h)
	}
	return nil
}
func safeRel(n string) (string, error) {
	n = strings.TrimSuffix(n, "/")
	if n == "" || strings.ContainsAny(n, "\\:\x00\r\n\t\"*?<>|") || strings.HasPrefix(n, "/") {
		return "", fmt.Errorf("unsafe relative path %q", n)
	}
	for _, r := range n {
		if r < 32 {
			return "", errors.New("control character in path")
		}
	}
	for _, part := range strings.Split(n, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("unsafe path component %q", part)
		}
		stem := strings.ToUpper(strings.Split(part, ".")[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || regexp.MustCompile(`^(COM|LPT)[1-9]$`).MatchString(stem) {
			return "", errors.New("Windows reserved filename")
		}
	}
	return filepath.FromSlash(n), nil
}
func regularFile(p string) error {
	i, e := os.Lstat(p)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", p)
	}
	return nil
}
func noSymlinkAncestors(p string) error {
	for {
		i, e := os.Lstat(p)
		if e == nil && i.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink refused: %s", p)
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		parent := filepath.Dir(p)
		if parent == p {
			return nil
		}
		p = parent
	}
}

// extractZip only writes into a fresh, operation-owned staging tree. It never runs archive contents.
func extractZip(archive, dest, prefix string, stripOldSaves bool) error {
	if e := noSymlinkAncestors(dest); e != nil {
		return e
	}
	z, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer z.Close()
	type entry struct {
		f   *zip.File
		rel string
	}
	entries := []entry{}
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		n := strings.TrimPrefix(f.Name, prefix)
		if n == "" {
			continue
		}
		rel, e := safeRel(n)
		if e != nil {
			return e
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return errors.New("archive symlink refused")
		}
		if f.Flags&1 != 0 {
			return errors.New("encrypted archive refused")
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return errors.New("nonregular archive entry refused")
		}
		key := strings.ToLower(filepath.ToSlash(rel))
		if seen[key] {
			return errors.New("duplicate or case-colliding archive entry")
		}
		seen[key] = true
		if f.UncompressedSize64 > 1024*1024*1024-total {
			return errors.New("archive exceeds 1 GiB extraction budget")
		}
		total += f.UncompressedSize64
		if len(entries) > 10000 {
			return errors.New("too many archive entries")
		}
		if stripOldSaves && isPersonalSeed(key) {
			continue
		}
		entries = append(entries, entry{f, rel})
	}
	if len(entries) == 0 {
		return errors.New("expected archive subtree is missing")
	}
	for _, entry := range entries {
		target := filepath.Join(dest, entry.rel)
		if e := noSymlinkAncestors(target); e != nil {
			return e
		}
		if _, e := os.Lstat(target); e == nil {
			return fmt.Errorf("overlay attempted to replace existing file %s", entry.rel)
		} else if !os.IsNotExist(e) {
			return e
		}
		if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		// Reject case aliases left by another layer on a case-sensitive filesystem.
		siblings, _ := os.ReadDir(filepath.Dir(target))
		for _, s := range siblings {
			if strings.EqualFold(s.Name(), filepath.Base(target)) {
				return errors.New("overlay case collision")
			}
		}
		in, e := entry.f.Open()
		if e != nil {
			return e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return e
		}
		n, e := io.Copy(out, io.LimitReader(in, int64(entry.f.UncompressedSize64)+1))
		in.Close()
		ce := out.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if n != int64(entry.f.UncompressedSize64) {
			return errors.New("extracted size mismatch")
		}
	}
	return nil
}
func isPersonalSeed(n string) bool {
	return !strings.Contains(n, "/") && (strings.HasSuffix(n, ".gam") || strings.HasSuffix(n, ".rac") || n == "hof.m2" || n == "mox.set" || n == "150.set" || n == "new150.set")
}
func isUserFile(n string) bool {
	n = strings.ToLower(filepath.ToSlash(n))
	if isPersonalSeed(n) {
		return true
	}
	return n == "dig.ini" || n == "mdi.ini" || n == "sound.lbx" || n == "150/user.cfg" || strings.HasPrefix(n, "150/build/") && strings.HasSuffix(n, ".cfg")
}
func atomicJSON(p string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(p, append(b, '\n'))
}
func atomicWrite(p string, b []byte) error {
	if e := noSymlinkAncestors(p); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".write-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return replaceFile(tmp, p)
}
func readJSON(p string, v any) error {
	if i, e := os.Stat(p); e != nil {
		return e
	} else if i.Size() > 8*1024*1024 {
		return errors.New("JSON file exceeds size limit")
	}
	if e := regularFile(p); e != nil {
		return e
	}
	f, e := os.Open(p)
	if e != nil {
		return e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 8*1024*1024))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return e
	}
	var x any
	if e = d.Decode(&x); e != io.EOF {
		return errors.New("extra or oversized JSON data")
	}
	return nil
}
func copyFile(src, dst string) error {
	if e := regularFile(src); e != nil {
		return e
	}
	if e := noSymlinkAncestors(dst); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return e
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func walkRegular(root string, fn func(string, string) error) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink refused: %s", p)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("nonregular file refused")
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		return fn(filepath.ToSlash(rel), p)
	})
}
func joinedArchivePath(prefix, name string) string { return path.Join(prefix, name) }

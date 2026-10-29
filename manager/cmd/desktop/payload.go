package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"moo2manager/shared/buildconfig"
	"moo2manager/shared/desktop"
	d "moo2manager/shared/distribution"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed embedded/*
var bundled embed.FS

func verifiedPayload(files fs.FS, trust d.Trust, root, platform string) (d.VerifiedManifest, d.Package, []byte, error) {
	var v d.VerifiedManifest
	var p d.Package
	b, e := fs.ReadFile(files, "embedded/manifest.json")
	if e != nil {
		return v, p, nil, errors.New("this executable has no embedded signed release; use the complete single-file candidate")
	}
	sig, e := fs.ReadFile(files, "embedded/manifest.sig")
	if e != nil {
		return v, p, nil, e
	}
	prior, e := d.LoadState(root, trust)
	if e != nil {
		return v, p, nil, e
	}
	v, e = trust.VerifyManifest(b, sig, time.Now(), prior, true)
	if e != nil {
		return v, p, nil, fmt.Errorf("embedded release verification failed: %w", e)
	}
	if e = v.RequireLauncher(platform, buildconfig.Version); e != nil {
		return v, p, nil, e
	}
	p, e = v.Select("launcher", platform)
	if e != nil {
		return v, p, nil, e
	}
	if e = trust.VerifyPackage(p); e != nil {
		return v, p, nil, e
	}
	if strings.ContainsAny(p.Filename, "/\\") {
		return v, p, nil, errors.New("invalid embedded package filename")
	}
	archive, e := fs.ReadFile(files, "embedded/"+p.Filename)
	if e != nil {
		return v, p, nil, e
	}
	sum := sha256.Sum256(archive)
	if int64(len(archive)) != p.Size || hex.EncodeToString(sum[:]) != p.SHA256 {
		return v, p, nil, errors.New("embedded package size or SHA-256 mismatch; nothing was executed")
	}
	return v, p, archive, nil
}

func ensureInstalled(ctx context.Context, root, platform string, trust d.Trust, files fs.FS, progress func(string)) (string, string, error) {
	installer := d.Installer{Root: root, Trust: trust, Health: health}
	progress("Checking the installed launcher…")
	if cur, e := installer.Current(); e == nil {
		if entry, rec, e := installer.VerifyGeneration(cur.Current); e == nil && d.VersionAtLeast(rec.Package.Version, buildconfig.Version) {
			if e = health(ctx, entry, rec.Package.Version); e != nil {
				return "", "", e
			}
			return entry, rec.Package.Version, nil
		}
	}
	progress("Verifying the included offline release…")
	v, p, archive, e := verifiedPayload(files, trust, root, platform)
	if e != nil {
		return "", "", e
	}
	if e = d.Accept(root, v); e != nil {
		return "", "", e
	}
	cache := filepath.Join(root, "cache", "packages", p.SHA256+".zip")
	if e = d.AtomicWrite(cache, archive); e != nil {
		return "", "", e
	}
	progress("Preparing your isolated launcher application…")
	entry, e := installer.Install(ctx, v, p, cache)
	if e != nil {
		return "", "", e
	}
	return entry, p.Version, nil
}

func health(ctx context.Context, entry, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, entry, "--version")
	desktop.HideConsole(c)
	out := &limitedOutput{}
	c.Stdout = out
	c.Stderr = out
	if e := c.Run(); e != nil {
		return fmt.Errorf("launcher could not start: %w", e)
	}
	if strings.TrimSpace(out.String()) != version {
		return errors.New("launcher version does not match its signed release")
	}
	return nil
}

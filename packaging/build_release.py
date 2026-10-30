#!/usr/bin/env python3
"""Build per-platform launcher packages + setup executables and sign a catalog.
No game payload is read or distributed. Ed25519 keys must live outside output.
Production: pass a maintainer-created trust configuration and private-key file.
Development: --development creates a disposable offline-only trust identity.
"""
from __future__ import annotations
import argparse, base64, datetime as dt, hashlib, json, os, pathlib, subprocess, tempfile, zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
VERSION = (ROOT / "VERSION").read_text().strip()
from source_fingerprint import source_index
TARGETS = ("windows-amd64", "linux-amd64", "darwin-amd64", "darwin-arm64")

def run(args: list[str], **kw) -> str:
    return subprocess.check_output(args, cwd=ROOT / "manager", text=True, **kw).strip()

def zip_files(dest: pathlib.Path, files: dict[str, pathlib.Path]) -> None:
    with zipfile.ZipFile(dest, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        for name, file in sorted(files.items()):
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = (0o100755 if file.stat().st_mode & 0o111 else 0o100644) << 16
            z.writestr(info, file.read_bytes())

def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--out", type=pathlib.Path, required=True)
    p.add_argument("--targets", nargs="+", choices=TARGETS, default=list(TARGETS))
    p.add_argument("--trust", type=pathlib.Path)
    p.add_argument("--key", type=pathlib.Path)
    p.add_argument("--key-id", default="maintainer-1")
    p.add_argument("--revision", type=int, required=True)
    p.add_argument("--development", action="store_true")
    p.add_argument("--days", type=int, default=90, help="Signed manifest validity, maximum 93 days")
    p.add_argument("--github-base", help="HTTPS versioned release-asset base, ending in /")
    p.add_argument("--r2-base", help="optional HTTPS public mirror base, ending in /")
    a = p.parse_args()
    if not 1 <= a.days <= 93: raise SystemExit("--days must be 1..93")
    out = a.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    if any(out.iterdir()):
        raise SystemExit("Refusing nonempty release output")
    if not a.development and (not a.trust or not a.key or not a.github_base):
        raise SystemExit("Production requires --trust, --key, and --github-base")
    with tempfile.TemporaryDirectory(prefix="sgc-signing-") as secure:
        secret = pathlib.Path(secure)
        if a.development:
            key = secret / "development.key"
            line = run(["go", "run", "./cmd/releasectl", "--command", "keygen", "--key", str(key), "--key-id", "development-offline"])
            pub = line.rsplit(": ", 1)[1]
            key_id = "development-offline"
            trust = {"schema": 1, "feed": "moo2-sgc-development", "channel": "alpha", "development": True,
                     "keys": {key_id: pub}, "endpoints": [], "allowed_hosts": []}
            if a.github_base or a.r2_base:
                raise SystemExit("Disposable development keys are offline-only")
        else:
            key, key_id = a.key.resolve(), a.key_id
            trust = json.loads(a.trust.read_text())
            if trust.get("development"):
                raise SystemExit("Production trust must not be marked development")
            if not trust.get("endpoints"):
                raise SystemExit("Production needs configured metadata endpoints")
        trust_file = secret / "trust.json"
        trust_file.write_text(json.dumps(trust, indent=2) + "\n")
        (out / "trust-public.json").write_text(json.dumps(trust, indent=2) + "\n")
        encoded = base64.b64encode(json.dumps(trust, separators=(",", ":")).encode()).decode()
        flags = f"-s -w -X moo2manager/shared/buildconfig.Encoded={encoded}"
        source_file=secret / "SOURCE-FINGERPRINT.json"
        source_file.write_text(json.dumps(source_index(ROOT),indent=2)+"\n")
        packages = []
        artifacts = []
        for target in dict.fromkeys(a.targets):
            system, arch = target.split("-")
            env = dict(os.environ, CGO_ENABLED="0", GOOS=system, GOARCH=arch)
            native_dir = out / "binaries" / target
            native_dir.mkdir(parents=True)
            launcher_name = "MOO2-SGC-Launcher.exe" if system == "windows" else "moo2-sgc-launcher"
            setup_name = "MOO2-SGC-Setup.exe" if system == "windows" else "MOO2-SGC-Setup"
            launcher = native_dir / launcher_name
            setup = native_dir / setup_name
            for path, cmd in ((launcher, "."), (setup, "./cmd/bootstrapper")):
                run(["go", "build", "-trimpath", "-buildvcs=false", "-ldflags="+flags+(" -H=windowsgui" if system == "windows" else ""), "-o", str(path), cmd], env=env)
                path.chmod(0o755)
            filename = f"launcher-{VERSION}-{target}.zip"
            files = {launcher_name: launcher, setup_name: setup, "SOURCE-FINGERPRINT.json": source_file,
                     "THIRD-PARTY-NOTICES.md": ROOT / "THIRD-PARTY-NOTICES.md",
                     "GO-LICENSE.txt": ROOT / "docs" / "GO-LICENSE.txt"}
            zip_files(out / filename, files)
            mirrors = []
            for provider, priority, base in (("github", 10, a.github_base), ("cloudflare-r2", 20, a.r2_base)):
                if base:
                    if not base.startswith("https://") or not base.endswith("/"):
                        raise SystemExit("Mirror bases must be HTTPS and end in /")
                    mirrors.append({"provider": provider, "priority": priority, "url": base+filename})
            packages.append({"id": "launcher", "kind": "launcher", "version": VERSION, "platform": target,
                             "filename": filename, "format": "zip", "entrypoint": launcher_name,
                             "size": 1, "sha256": "0"*64, "signature": {"key_id": key_id, "signature": ""},
                             "mirrors": mirrors, "redistribution": "project-owned"})
            artifacts.append({"target": target, "setup_bytes": setup.stat().st_size,
                              "launcher_bytes": launcher.stat().st_size, "launcher_zip_bytes": (out/filename).stat().st_size,
                              "compiled": True, "executed_here": False})
        now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
        manifest = {"schema": 1, "feed": trust["feed"], "channel": trust["channel"], "revision": a.revision,
                    "release": VERSION, "published": (now-dt.timedelta(minutes=1)).isoformat().replace("+00:00", "Z"),
                    "expires": (now+dt.timedelta(days=a.days)).isoformat().replace("+00:00", "Z"),
                    "packages": packages}
        (out / "manifest.draft.json").write_text(json.dumps(manifest, indent=2)+"\n")
        print(run(["go", "run", "./cmd/releasectl", "--command", "sign", "--trust", str(trust_file),
                   "--key", str(key), "--key-id", key_id, "--dir", str(out)]))
        (out / "manifest.draft.json").unlink()
        (out / "BUILD-RESULTS.json").write_text(json.dumps({"version":VERSION, "development":a.development,
            "toolchain":run(["go","version"]), "targets":artifacts, "game_data_included":False,
            "windows_authenticode":False,"macos_notarization":False},indent=2)+"\n")
        sums=[]
        for file in sorted(out.rglob("*")):
            if file.is_file(): sums.append(hashlib.sha256(file.read_bytes()).hexdigest()+"  "+file.relative_to(out).as_posix())
        (out / "SHA256SUMS.txt").write_text("\n".join(sums)+"\n")
        print(json.dumps(artifacts, indent=2))
    # Disposable development private key is deleted with TemporaryDirectory.

if __name__ == "__main__":
    main()

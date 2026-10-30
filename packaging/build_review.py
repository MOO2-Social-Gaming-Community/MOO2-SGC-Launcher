#!/usr/bin/env python3
"""Build a clean single-file review bundle. Never publish or read release secrets.

This uses disposable OFFLINE development trust, not a production update feed.
Repeated builds are reproducible as a process, not byte-identical: keys, signed
timestamps and manifest expiry change. Retain and review the exact built bytes.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import zipfile
from source_fingerprint import source_index
from build_desktop import trust_fingerprint

ROOT = Path(__file__).resolve().parents[1]
TARGETS = ('windows-amd64', 'linux-amd64', 'darwin-amd64', 'darwin-arm64')
VERSION_RE = r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def native_target() -> str:
    system = {'Windows': 'windows', 'Linux': 'linux', 'Darwin': 'darwin'}[platform.system()]
    arch = {'AMD64': 'amd64', 'x86_64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}[platform.machine()]
    return system + '-' + arch


def check_gui_pe(data: bytes) -> None:
    """Require x64 PE32+ GUI, with no claim of Authenticode certification."""
    try:
        pe = struct.unpack_from('<I', data, 0x3c)[0]
        if data[:2] != b'MZ' or data[pe:pe+4] != b'PE\0\0':
            raise ValueError('not a PE executable')
        machine = struct.unpack_from('<H', data, pe+4)[0]
        optional = pe+24
        magic = struct.unpack_from('<H', data, optional)[0]
        subsystem = struct.unpack_from('<H', data, optional+68)[0]
        if (machine, magic, subsystem) != (0x8664, 0x20b, 2):
            raise ValueError('expected x64 Windows GUI executable')
    except struct.error as e:
        raise ValueError('truncated Windows executable') from e


def verify_embedded(release: Path, binary: Path, target: str, root: Path = ROOT) -> dict:
    version = (root/'VERSION').read_text().strip()
    if not re.fullmatch(VERSION_RE, version):
        raise ValueError('VERSION must contain three numeric components')
    trust = json.loads((release/'trust-public.json').read_text())
    if trust.get('development') is not True or trust.get('endpoints') or trust.get('allowed_hosts'):
        raise ValueError('review bundle must use offline development trust')
    # Go verifies signatures, manifest expiry, package hashes and file inventory.
    subprocess.run(['go', 'run', './cmd/releasectl', '--command', 'verify', '--trust',
                    str(release/'trust-public.json'), '--dir', str(release)], cwd=root/'manager', check=True)
    manifest_bytes = (release/'manifest.json').read_bytes()
    manifest = json.loads(manifest_bytes)
    if manifest['release'] != version or len(manifest['packages']) != 1:
        raise ValueError('review build must contain exactly one matching platform package')
    package = manifest['packages'][0]
    if (package['id'], package['kind'], package['platform'], package['version']) != ('launcher', 'launcher', target, version):
        raise ValueError('unexpected review package')
    filename = package['filename']
    if '/' in filename or '\\' in filename or filename in ('', '.', '..'):
        raise ValueError('unsafe package filename')
    archive = release/filename
    content = binary.read_bytes()
    for data in (manifest_bytes, (release/'manifest.sig').read_bytes(), archive.read_bytes()):
        if content.count(data) != 1:
            raise ValueError('outer executable does not contain exact signed payload once')
    windows = target == 'windows-amd64'
    launcher = 'MOO2-SGC-Launcher.exe' if windows else 'moo2-sgc-launcher'
    setup = 'MOO2-SGC-Setup.exe' if windows else 'MOO2-SGC-Setup'
    expected = {launcher, setup, 'SOURCE-FINGERPRINT.json', 'THIRD-PARTY-NOTICES.md', 'GO-LICENSE.txt'}
    with zipfile.ZipFile(archive) as z:
        if len(z.infolist()) != 5 or set(z.namelist()) != expected:
            raise ValueError('unexpected inner payload; no game/runtime/data files permitted')
        if json.loads(z.read('SOURCE-FINGERPRINT.json')) != source_index(root):
            raise ValueError('source changed after build')
        if windows:
            for name in (launcher, setup):
                check_gui_pe(z.read(name))
    if windows:
        check_gui_pe(content)
    return {'version': version, 'revision': manifest['revision'], 'target': target, 'manifest_expires': manifest['expires'],
            'source_index_sha256': digest((json.dumps(source_index(root), indent=2)+'\n').encode()),
            'embedded_package_sha256': package['sha256'], 'trust_fingerprint': trust_fingerprint(trust)}


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--revision', type=int, required=True)
    p.add_argument('--evidence-out', type=Path, help='optional separate new directory for signed public build inputs; never part of player bundle')
    p.add_argument('--target', choices=TARGETS, default='windows-amd64')
    p.add_argument('--run-native-smoke', action='store_true', help='headless process/API only, never rendered GUI/gameplay')
    a = p.parse_args()
    out = a.out.resolve()
    evidence_out = a.evidence_out.resolve() if a.evidence_out else None
    if evidence_out and (evidence_out.exists() or evidence_out.is_relative_to(out) or out.is_relative_to(evidence_out)):
        raise SystemExit('Evidence directory must be new and separate from player output')
    if out.exists():
        raise SystemExit('Refusing existing output; retain exact reviewed builds, use a new folder')
    for name in ('manager', 'packaging', 'tests', '.github', 'support'):
        if out.is_relative_to(ROOT/name) or (evidence_out and evidence_out.is_relative_to(ROOT/name)):
            raise SystemExit('Output must not be inside indexed source inputs')
    if a.revision < 1 or a.revision >= 2**64:
        raise SystemExit('Revision must be a positive uint64')
    if a.run_native_smoke and a.target != native_target():
        raise SystemExit('Native smoke requires a matching build and host OS/architecture')
    out.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='moo2-review-', dir=out.parent) as tmp:
        temp = Path(tmp); release = temp/'inner'; desktop = temp/'desktop'; assets = temp/'assets'
        assets.mkdir()
        subprocess.run([sys.executable, str(ROOT/'packaging/build_release.py'), '--development', '--revision', str(a.revision),
                        '--targets', a.target, '--out', str(release)], check=True)
        subprocess.run([sys.executable, str(ROOT/'packaging/build_desktop.py'), '--release', str(release),
                        '--targets', a.target, '--out', str(desktop)], check=True)
        binary = desktop/('MOO2-SGC-'+a.target+('.exe' if a.target.startswith('windows-') else ''))
        metadata = verify_embedded(release, binary, a.target)
        name = 'MOO2-SGC-Windows.exe' if a.target == 'windows-amd64' else 'MOO2-SGC-'+a.target
        shutil.copyfile(binary, assets/name); (assets/name).chmod(0o755)
        smoke = None
        if a.run_native_smoke:
            evidence = temp/'smoke.json'
            subprocess.run([sys.executable, str(ROOT/'tests/desktop_native_smoke.py'), '--binary', str(assets/name),
                            '--version', metadata['version'], '--output', str(evidence)], check=True)
            smoke = json.loads(evidence.read_text())
        metadata.update(schema=1, status='private review build; not a stable release',
                        filename=name, bytes=(assets/name).stat().st_size, sha256=digest((assets/name).read_bytes()),
                        toolchain=subprocess.check_output(['go', 'version'], text=True).strip(),
                        build_host=native_target(), offline_development_trust=True, game_data_included=False,
                        runtime_included=False, os_signing_performed=False, rendered_gui_tested=False,
                        gameplay_tested=False, native_process_api_smoke=smoke, publication_performed=False)
        (assets/'BUILD-RESULTS.json').write_text(json.dumps(metadata, indent=2)+'\n', encoding='utf-8')
        (assets/'READ-ME-FIRST.txt').write_text(
            'MOO2-SGC '+metadata['version']+' — private review build, not stable\n\n'+
            ('Windows players open MOO2-SGC-Windows.exe. No terminal or Go is required.\n' if a.target == 'windows-amd64' else 'Internal '+a.target+' build; no one-open consumer acceptance claimed.\n')+
            'Choose your owned game in the browser interface; game/runtime files are not bundled.\n'
            'Each independently rebuilt development identity has a separate private installation.\n'
            'Existing installations and saves are not migrated or deleted.\n'
            'This is not OS-signed. Do not disable security protection to run it.\n'
            'Automated native process/API tests do not prove visible windows, audio or gameplay.\n'
            'No public release or stable download link was created by this build.\n'
            'Keep these exact bytes together with BUILD-RESULTS.json and SHA256SUMS.txt.\n'
            'Review in a draft only after completing docs/STABLE-RELEASE-GATE.md.\n', encoding='utf-8')
        files = sorted(assets.iterdir())
        (assets/'SHA256SUMS.txt').write_text(''.join(digest(f.read_bytes())+'  '+f.name+'\n' for f in files), encoding='utf-8')
        # Optional reviewer inputs contain only signed public artifacts, never keys.
        if evidence_out:
            evidence_out.mkdir(parents=True, exist_ok=False)
            shutil.copytree(release, evidence_out/'inner')
            shutil.copytree(desktop, evidence_out/'desktop')
        # No release command, token, signing credential or network publish exists here.
        if out.exists():
            raise SystemExit('Output appeared during build; refusing replacement')
        os.rename(assets, out)
    print('Review assets saved to', out)


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Portable native outer/manager process + authenticated API smoke.

Fresh temporary roots only, explicit --no-browser, no emulator/game. A Windows
runner passing this does NOT prove visible browser/picker/game/audio behavior.
No remote URLs, environment proxies, credentials, or real installation paths.
"""
from __future__ import annotations
import argparse
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ValueError('local smoke API redirect refused')


def local_session(marker: Path) -> tuple[str, str]:
    raw = marker.read_text().strip()
    if not re.fullmatch(r'http://127\.0\.0\.1:[1-9][0-9]{0,4}/#[a-f0-9]{64}', raw):
        raise ValueError('noncanonical local session marker')
    u = urllib.parse.urlsplit(raw)
    if not 1 <= u.port <= 65535:
        raise ValueError('invalid local port')
    return raw.split('/#')[0], u.fragment


def api(marker: Path, path: str, body=None, authenticated=True):
    base, token = local_session(marker)
    headers = {'Authorization': 'Bearer '+token} if authenticated else {}
    data = None
    if body is not None:
        headers['Content-Type'] = 'application/json'; data = json.dumps(body).encode()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    req = urllib.request.Request(base+'/api/'+path, data, headers)
    with opener.open(req, timeout=5) as response:
        return json.load(response)


def wait_for(fn, proc=None, seconds=60):
    end = time.monotonic()+seconds
    while time.monotonic() < end:
        if proc is not None and proc.poll() is not None:
            raise AssertionError('outer exited before API ready: '+str(proc.returncode))
        try:
            result = fn()
            if result:
                return result
        except (FileNotFoundError, ConnectionError, urllib.error.URLError):
            pass
        time.sleep(.1)
    raise AssertionError('native process/API smoke timed out')


def smoke(binary: Path, version: str) -> dict:
    checks = []
    def check(name, ok):
        if not ok:
            raise AssertionError(name)
        checks.append(name); print('PASS', name, flush=True)
    with tempfile.TemporaryDirectory(prefix='moo2-native-review-') as temporary:
        temp = Path(temporary)
        # Spaces and Unicode exercise typical Windows user-profile paths.
        root = temp/'private launcher é'; marker = root/'userdata/open-launcher.txt'
        boot = root/'desktop-session/open.txt'
        processes = []
        def launch():
            log = (temp/('process-'+str(len(processes))+'.log')).open('wb')
            proc = subprocess.Popen([str(binary), '--install-root', str(root), '--no-browser'], stdout=log, stderr=log)
            log.close(); processes.append(proc)
            wait_for(lambda: boot.exists(), proc)
            def ready():
                state = api(boot, 'status')
                if state.get('error'):
                    raise AssertionError('startup error: '+state['error'])
                return state.get('target')
            wait_for(ready, proc)
            return proc
        try:
            proc = launch()
            identity = api(marker, 'identity'); pid = identity['pid']
            check('native embedded manager version and isolated root', identity['version'] == version and Path(identity['root']).resolve() == root.resolve())
            try:
                api(marker, 'identity', authenticated=False)
                raise AssertionError('unauthenticated identity accepted')
            except urllib.error.HTTPError as e:
                check('session authentication enforced', e.code == 401)
            state = api(marker, 'state')
            check('fresh source/runtime not falsely ready or auto-launched', not state['running'] and not any(state['active'].values()))
            check('unavailable optional services not enabled', not state['prsl_available'] and not state['chat_available'])
            pointer = (root/'components/launcher/current.json').read_bytes()
            sentinel = root/'userdata/synthetic-save.GAM'; sentinel.write_bytes(b'SYNTHETIC TEST ONLY')
            duplicate = subprocess.run([str(binary), '--install-root', str(root), '--no-browser'], capture_output=True, timeout=20)
            check('second open reuses same live manager', duplicate.returncode == 0 and api(marker, 'identity')['pid'] == pid)
            check('repeat open preserves installed generation and save sentinel', (root/'components/launcher/current.json').read_bytes() == pointer and sentinel.read_bytes() == b'SYNTHETIC TEST ONLY')
            api(marker, 'action', {'action': 'quit'}); proc.wait(timeout=20)
            check('manager and outer exit cleanly', proc.returncode == 0)
            wait_for(lambda: not (root/'installation.lock').exists() and not (root/'userdata/manager.lock').exists())
            check('operation locks released', not boot.exists())
            proc = launch()
            check('later restart retains generation and save sentinel', (root/'components/launcher/current.json').read_bytes() == pointer and sentinel.read_bytes() == b'SYNTHETIC TEST ONLY')
            api(marker, 'action', {'action': 'quit'}); proc.wait(timeout=20)
            check('second clean shutdown', proc.returncode == 0)
            check('captured normal output contains no private session URL', all(b'http://127.0.0.1:' not in log.read_bytes() for log in temp.glob('process-*.log')))
        finally:
            # Best-effort authenticated manager shutdown, then only our own outer.
            if marker.exists():
                try:
                    api(marker, 'action', {'action': 'quit'})
                except (OSError, ValueError):
                    pass
            for proc in processes:
                if proc.poll() is None:
                    try:
                        proc.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        proc.terminate(); proc.wait(timeout=10)
    return {'platform': platform.system(), 'architecture': platform.machine(), 'version': version,
            'checks': checks, 'count': len(checks), 'mode': 'native process/API, explicit no-browser',
            'rendered_gui_tested': False, 'game_executed': False, 'network_downloads_used': False}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--binary', type=Path, required=True)
    p.add_argument('--version', required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    result = smoke(a.binary.resolve(), a.version)
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_text(json.dumps(result, indent=2)+'\n', encoding='utf-8')


if __name__ == '__main__':
    main()

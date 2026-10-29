#!/usr/bin/env python3
"""Linux lifecycle/argument regression; platform shortcut writes use Go test doubles.
Runs disposable signed offline installations and an xdg-open recorder only.
Never runs a browser, game, real shortcut writer, or external service.
"""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--release', type=Path, required=True)
p.add_argument('--evidence', type=Path, required=True)
a = p.parse_args()
release = a.release.resolve()
setup = release / 'binaries/linux-amd64/MOO2-SGC-Setup'
results = []
counts = {}


def check(name, value):
    if not value:
        raise AssertionError(name)
    results.append(name)
    print('PASS', name, flush=True)


def until(fn, timeout=40):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = fn()
        if value:
            return value
        time.sleep(.05)
    raise AssertionError('timed out waiting for process lifecycle condition')


def group_alive(group):
    for path in Path('/proc').glob('[0-9]*/stat'):
        try:
            fields = path.read_text().rsplit(')', 1)[1].split()
            if int(fields[2]) == group and fields[0] != 'Z':
                return True
        except (FileNotFoundError, ProcessLookupError):
            pass
    return False


def process_args(pid):
    return Path('/proc', str(pid), 'cmdline').read_bytes().decode().rstrip('\0').split('\0')


def parent_pid(pid):
    return int(Path('/proc', str(pid), 'stat').read_text().rsplit(')', 1)[1].split()[1])


with tempfile.TemporaryDirectory(prefix='shortcut-free-handoff-') as td:
    temp = Path(td)
    recorder = temp / 'bin'
    recorder.mkdir()
    counter = temp / 'calls'
    opener = recorder / 'xdg-open'
    opener.write_text('#!/bin/sh\nprintf "call\\n" >> "$OPENER_COUNT"\n')
    opener.chmod(0o755)
    env = dict(os.environ, PATH=str(recorder) + os.pathsep + os.environ['PATH'], OPENER_COUNT=str(counter))

    def count():
        return len(counter.read_text().splitlines()) if counter.exists() else 0

    for entrypoint in ('setup', 'direct'):
        for no_shortcut in (False, True):
            for no_browser in (False, True):
                case = f'{entrypoint}-shortcut-{"off" if no_shortcut else "default"}-browser-{"off" if no_browser else "normal"}'
                root = temp / case
                flags = (['--no-shortcut'] if no_shortcut else []) + (['--no-browser'] if no_browser else [])
                if entrypoint == 'setup':
                    command = [str(setup), '--install-root', str(root), '--offline', str(release), *flags]
                else:
                    subprocess.run([str(setup), '--install-root', str(root), '--offline', str(release), '--no-launch', *flags],
                                   env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=60)
                    pointer = json.loads((root / 'components/launcher/current.json').read_text())
                    launcher = root / 'components/launcher/versions' / pointer['current'] / 'files/moo2-sgc-launcher'
                    command = [str(launcher), '--app-root', str(root), '--root', str(root), '--data', str(root / 'userdata'), *flags]
                marker = root / 'userdata/open-launcher.txt'
                console_path = temp / (case + '-private-console.txt')
                start = count()
                # Captures/credentials stay temporary, never in retained evidence.
                with console_path.open('wb') as console:
                    proc = subprocess.Popen(command, env=env, stdout=console, stderr=console, start_new_session=True)
                secrets = []
                try:
                    until(marker.exists)
                    credentials = marker.read_text().strip()
                    local, token = credentials.split('/#')

                    def api(route, body=None, bearer=None):
                        headers = {'Authorization': 'Bearer ' + (token if bearer is None else bearer)}
                        data = None
                        if body is not None:
                            headers['Content-Type'] = 'application/json'
                            data = json.dumps(body).encode()
                        with urllib.request.urlopen(urllib.request.Request(local + '/api/' + route, data, headers), timeout=5) as r:
                            return json.load(r)

                    def assert_process_modes(stage, helper):
                        pid = int((root / 'userdata/manager.lock').read_text())
                        args = process_args(pid)
                        check(case + stage + ' launcher preserves explicit shortcut policy', ('--no-shortcut' in args) == no_shortcut)
                        check(case + stage + ' launcher preserves browser policy', ('--no-browser' in args) == no_browser)
                        check(case + stage + ' launcher private-output policy', ('--private-url-file-only' in args) == (stage != ' initial'))
                        if helper:
                            args = process_args(parent_pid(pid))
                            check(case + stage + ' parent is signed Setup helper', Path(args[0]).name == 'MOO2-SGC-Setup')
                            check(case + stage + ' helper preserves explicit shortcut policy', ('--no-shortcut' in args) == no_shortcut)
                            check(case + stage + ' helper preserves browser policy', ('--no-browser' in args) == no_browser)
                        return pid

                    check(case + ' initial API usable', bool(api('state')['version']))
                    assert_process_modes(' initial', entrypoint == 'setup')
                    until(lambda: credentials in console_path.read_text())
                    check(case + ' initial direct/Setup URL convenience unchanged', True)
                    sentinel = root / 'userdata/saves/synthetic.GAM'
                    sentinel.parent.mkdir(parents=True, exist_ok=True)
                    original = b'SYNTHETIC ONLY\x00\xff'
                    sentinel.write_bytes(original)
                    pointer = root / 'components/launcher/current.json'
                    original_pointer = pointer.read_bytes()
                    secrets.extend((credentials, token))
                    for cycle in range(1, 4):
                        before_credentials, before_token = credentials, token
                        before_pid = int((root / 'userdata/manager.lock').read_text())
                        api('action', {'action': 'launcher-stage', 'offline_path': str(release)})
                        state = until(lambda: (s if not s['job']['busy'] else None) if (s := api('state')) else None)
                        check(case + f' update {cycle} signed stage succeeds', not state['job']['error'])
                        api('action', {'action': 'launcher-apply'})

                        def restarted():
                            try:
                                value = marker.read_text().strip()
                                return value if value != before_credentials else None
                            except FileNotFoundError:
                                return None

                        credentials = until(restarted)
                        local, token = credentials.split('/#')
                        secrets.extend((credentials, token))
                        check(case + f' update {cycle} restarted API usable', bool(api('state')['version']))
                        pid = assert_process_modes(f' update {cycle}', True)
                        check(case + f' update {cycle} new manager owns lock', pid != before_pid)
                        try:
                            api('state', bearer=before_token)
                        except urllib.error.HTTPError as error:
                            check(case + f' update {cycle} old credential refused', error.code == 401)
                        else:
                            raise AssertionError('old credential accepted by restarted session')
                        expected = 0 if no_browser else cycle + 1
                        if expected:
                            until(lambda: count() - start == expected)
                        time.sleep(.15)
                        check(case + f' update {cycle} opener count', count() - start == expected)
                        handoff = root / 'logs/update-handoff.log'
                        until(lambda: 'Private local URL is in: ' + str(marker) in handoff.read_text())
                        logs = '\n'.join(f.read_text() for f in (root / 'logs').rglob('*') if f.is_file())
                        check(case + f' update {cycle} logs retain no session credentials',
                              all(secret not in logs for secret in [*secrets, 'http://127.0.0.1:']))
                        check(case + f' update {cycle} private file permissions',
                              all(f.stat().st_mode & 0o777 == 0o600 for f in (marker, handoff)))
                    proc.wait(timeout=15)
                    check(case + ' original process clean exit', proc.returncode == 0)
                    check(case + ' synthetic userdata unchanged after three updates', sentinel.read_bytes() == original)
                    check(case + ' same-package updates do not claim version migration', pointer.read_bytes() == original_pointer)
                    counts[case] = count() - start
                    api('action', {'action': 'quit'})
                    until(lambda: not group_alive(proc.pid))
                    check(case + ' every child process stopped', not group_alive(proc.pid))
                    check(case + ' locks and private marker cleaned', all(not f.exists() for f in
                          (root / 'installation.lock', root / 'userdata/manager.lock', marker)))
                    subprocess.run([str(setup), '--install-root', str(root), '--command', 'verify', *flags], env=env,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True, timeout=60)
                    check(case + ' signed installed generation verifies', True)
                finally:
                    if group_alive(proc.pid):
                        os.killpg(proc.pid, signal.SIGTERM)
                    if proc.poll() is None:
                        proc.wait(timeout=10)
                    if group_alive(proc.pid):
                        os.killpg(proc.pid, signal.SIGKILL)
                        until(lambda: not group_alive(proc.pid))
    check('recorder stores only call counts, never credentials', set(counter.read_text().splitlines()) == {'call'})
a.evidence.parent.mkdir(parents=True, exist_ok=True)
a.evidence.write_text(json.dumps({'count': len(results), 'checks': results, 'opener_calls': counts,
    'browser_executed': False, 'windows_shortcut_writer_executed': False,
    'scope': 'Linux process arguments across initial Setup/direct launch and three same-package signed offline updates; safe writer-selection double tested separately in Go; not Windows/native browser/gameplay acceptance'}, indent=2) + '\n')
print('TOTAL', len(results), 'COUNTS', json.dumps(counts))

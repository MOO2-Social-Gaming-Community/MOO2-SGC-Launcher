#!/usr/bin/env python3
"""Linux process-double regression. xdg-open records calls only, never arguments.
No browser, game, external service, or production trust is used.
"""
import argparse, json, os, signal, subprocess, tempfile, time
from pathlib import Path
import urllib.request, urllib.error

p = argparse.ArgumentParser()
p.add_argument('--release', type=Path, required=True)
p.add_argument('--evidence', type=Path, required=True)
p.add_argument('--expect-headless-restart', type=int, default=0)
p.add_argument('--check-log-privacy', action='store_true',
               help='also check private recovery/console behavior and token-free handoff diagnostics')
a = p.parse_args()
release = a.release.resolve()
setup = release / 'binaries/linux-amd64/MOO2-SGC-Setup'
results = []
def check(name, value):
    if not value: raise AssertionError(name)
    results.append(name)
    print('PASS', name, flush=True)
def until(fn, timeout=40):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = fn()
        if value: return value
        time.sleep(.05)
    raise AssertionError('timed out waiting for process lifecycle condition')
def group_alive(group):
    for path in Path('/proc').glob('[0-9]*/stat'):
        try:
            fields = path.read_text().rsplit(')', 1)[1].split()
            if int(fields[2]) == group and fields[0] != 'Z': return True
        except (FileNotFoundError, ProcessLookupError): pass
    return False
counts = {}
with tempfile.TemporaryDirectory(prefix='headless-handoff-') as td:
    temp = Path(td)
    recorder = temp / 'bin'; recorder.mkdir()
    counter = temp / 'calls'
    opener = recorder / 'xdg-open'
    opener.write_text('#!/bin/sh\nprintf "call\\n" >> "$OPENER_COUNT"\n')
    opener.chmod(0o755)
    env = dict(os.environ, PATH=str(recorder)+os.pathsep+os.environ['PATH'], OPENER_COUNT=str(counter))
    def count(): return len(counter.read_text().splitlines()) if counter.exists() else 0
    def boot(root, *args):
        return subprocess.run([str(setup), '--install-root', str(root), *args], env=env,
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60, check=True)
    # Recorder is installed before even setup health-checks execute a manager.
    for mode in ('headless', 'normal'):
        root = temp / mode
        boot(root, '--offline', str(release), '--no-launch')
        sentinel = root/'userdata/saves/synthetic.GAM'
        sentinel.parent.mkdir(parents=True, exist_ok=True)
        sentinel.write_bytes(b'SYNTHETIC ONLY\x00\xff')
        original = sentinel.read_bytes()
        pointer = root/'components/launcher/current.json'
        before_pointer = pointer.read_bytes()
        marker = root/'userdata/open-launcher.txt'
        start = count()
        # Console capture contains a private URL, is never retained as evidence,
        # and is deleted with the disposable installation even on assertion failure.
        console_path = temp / (mode+'-console.txt')
        with console_path.open('wb') as console:
            proc = subprocess.Popen([str(setup), '--install-root', str(root), '--command', 'launch-installed',
                                     *(['--no-browser'] if mode == 'headless' else [])], env=env,
                                    stdout=console, stderr=console, start_new_session=True)
        try:
            until(marker.exists)
            credentials = marker.read_text().strip()
            local, token = credentials.split('/#')
            initial_token = token
            def api(route, body=None):
                headers = {'Authorization': 'Bearer '+token}
                data = None
                if body is not None:
                    headers['Content-Type'] = 'application/json'; data = json.dumps(body).encode()
                req = urllib.request.Request(local+'/api/'+route, data, headers)
                with urllib.request.urlopen(req, timeout=5) as r: return json.load(r)
            check(mode+' initial API usable', bool(api('state')['version']))
            if mode == 'normal': until(lambda: count() == start+1)
            time.sleep(.25)
            initial = count()-start
            check(mode+' initial opener count', initial == (0 if mode == 'headless' else 1))
            if a.check_log_privacy:
                check(mode+' ordinary launch still prints private recovery URL', credentials in console_path.read_text())
                check(mode+' initial private marker permission', marker.stat().st_mode & 0o777 == 0o600)
            # No pending update: refusal must not strand the original manager busy.
            try: api('action', {'action':'launcher-apply'})
            except urllib.error.HTTPError as e: check(mode+' invalid pending refused', e.code == 400)
            else: raise AssertionError('invalid pending accepted')
            check(mode+' original manager remains usable', not api('state')['job']['busy'])
            check(mode+' failed apply leaves active pointer intact', pointer.read_bytes() == before_pointer)
            api('action', {'action':'launcher-stage', 'offline_path':str(release)})
            state = until(lambda: (s if not s['job']['busy'] else None) if (s := api('state')) else None)
            check(mode+' stage valid signed package', not state['job']['error'])
            oldpid = (root/'userdata/manager.lock').read_text()
            api('action', {'action':'launcher-apply'})
            proc.wait(timeout=15)
            check(mode+' initial manager/setup clean exit', proc.returncode == 0)
            def restarted():
                try:
                    value = marker.read_text().strip()
                    return value if value != credentials else None
                except FileNotFoundError: return None
            local, token = until(restarted).split('/#')
            check(mode+' restarted API usable', bool(api('state')['version']))
            check(mode+' new manager owns lock', (root/'userdata/manager.lock').read_text() != oldpid)
            expected = a.expect_headless_restart if mode == 'headless' else 1
            if expected: until(lambda: count() == start+initial+expected)
            time.sleep(.5)
            restart = count()-start-initial
            check(mode+' restart opener count', restart == expected)
            if a.check_log_privacy:
                handoff = root/'logs/update-handoff.log'
                handoff_text = handoff.read_text()
                check(mode+' handoff log remains private', handoff.stat().st_mode & 0o777 == 0o600)
                check(mode+' restarted private marker permission', marker.stat().st_mode & 0o777 == 0o600)
                check(mode+' handoff log excludes session URLs and tokens',
                      all(secret not in handoff_text for secret in (credentials, initial_token, token, 'http://127.0.0.1:')))
                check(mode+' handoff retains Setup and launcher diagnostics',
                      all(message in handoff_text for message in ('MOO2-SGC Setup', 'Launcher installed and verified:',
                          'Launching/ready:', 'MOO2-SGC Launcher', 'Use Exit launcher in the UI.')))
                check(mode+' handoff explains recovery file location',
                      'Private local URL is in: '+str(marker) in handoff_text)
                check(mode+' bootstrap log retains apply-staged success',
                      'success apply-staged' in (root/'logs/bootstrap.log').read_text())
            counts[mode] = {'initial':initial, 'restart':restart}
            check(mode+' synthetic userdata preserved', sentinel.read_bytes() == original)
            check(mode+' same-package apply is not migration', pointer.read_bytes() == before_pointer)
            api('action', {'action':'quit'})
            until(lambda: not group_alive(proc.pid))
            check(mode+' all child processes stopped', not group_alive(proc.pid))
            check(mode+' locks and private marker removed', all(not x.exists() for x in
                  (root/'installation.lock', root/'userdata/manager.lock', marker)))
            boot(root, '--command', 'verify')
            check(mode+' signed installed generation verifies after exit', True)
            if a.check_log_privacy:
                all_logs = '\n'.join(path.read_text() for path in (root/'logs').rglob('*') if path.is_file())
                check(mode+' retained application logs exclude both sessions after exit',
                      all(secret not in all_logs for secret in (credentials, initial_token, token, 'http://127.0.0.1:')))
        finally:
            if group_alive(proc.pid): os.killpg(proc.pid, signal.SIGTERM)
            if proc.poll() is None: proc.wait(timeout=10)
            if group_alive(proc.pid):
                os.killpg(proc.pid, signal.SIGKILL)
                until(lambda: not group_alive(proc.pid))
    check('recorder stored no URLs or tokens', set(counter.read_text().splitlines()) == {'call'})
a.evidence.parent.mkdir(parents=True, exist_ok=True)
a.evidence.write_text(json.dumps({'count':len(results), 'checks':results, 'opener_calls':counts,
    'browser_executed':False, 'scope':'Linux xdg-open process double; same-package signed offline self-update; not native browser acceptance',
    'observation_seconds_after_ready':0.5}, indent=2)+'\n')
print('TOTAL', len(results), 'COUNTS', json.dumps(counts))

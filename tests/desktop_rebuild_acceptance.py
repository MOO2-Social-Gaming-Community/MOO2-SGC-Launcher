#!/usr/bin/env python3
"""Exercise two independently signed, same-version native review EXEs.
No GUI/game, no real installation: proves fresh trust cannot adopt prior data.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
from desktop_native_smoke import api, wait_for


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--first', type=Path, required=True, help='first build_review output directory')
    p.add_argument('--second', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    inputs = [a.first.resolve(), a.second.resolve()]
    builds = [json.loads((path/'BUILD-RESULTS.json').read_text()) for path in inputs]
    if builds[0]['version'] != builds[1]['version'] or builds[0]['trust_fingerprint'] == builds[1]['trust_fingerprint']:
        raise AssertionError('requires same version, independently signed trust identities')
    checks = []
    def check(name, condition):
        if not condition: raise AssertionError(name)
        checks.append(name); print('PASS', name, flush=True)
    with tempfile.TemporaryDirectory(prefix='moo2-rebuild-') as td:
        home = Path(td)
        env = dict(os.environ, XDG_CONFIG_HOME=str(home), LOCALAPPDATA=str(home), APPDATA=str(home))
        roots = [home/'MOO2-SGC'/('private-gui-'+b['version']+'-'+b['trust_fingerprint'][:16]) for b in builds]
        binaries = [inputs[i]/b['filename'] for i,b in enumerate(builds)]
        processes = []
        def start(i):
            process = subprocess.Popen([str(binaries[i]), '--no-browser'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            processes.append(process)
            boot = roots[i]/'desktop-session/open.txt'
            wait_for(lambda: boot.exists(), process)
            def ready():
                status = api(boot, 'status')
                if status.get('error'): raise AssertionError(status['error'])
                return status.get('target')
            wait_for(ready, process)
            return process, roots[i]/'userdata/open-launcher.txt'
        try:
            first, am = start(0)
            sentinel = roots[0]/'userdata/synthetic-save.GAM'; sentinel.write_bytes(b'FIRST BUILD SAVE')
            names = [roots[0]/'desktop-installation.json', roots[0]/'components/launcher/current.json', am, sentinel]
            before = [n.read_bytes() for n in names]
            second, bm = start(1)
            check('same-version fresh trust uses separate live manager/root', roots[0] != roots[1] and api(am,'identity')['pid'] != api(bm,'identity')['pid'])
            check('new build leaves previous ownership/generation/session/save untouched', [n.read_bytes() for n in names] == before)
            check('full trust identity recorded in each ownership contract', all(json.loads((r/'desktop-installation.json').read_text())['trust_fingerprint'] == b['trust_fingerprint'] for r,b in zip(roots,builds)))
            conflict = subprocess.run([str(binaries[1]), '--no-browser', '--install-root', str(roots[0])], env=env, capture_output=True, timeout=20)
            check('explicit mismatched root refused before live-session reuse', conflict.returncode != 0 and b'different or unknown build identity' in conflict.stderr)
            check('explicit refusal preserves old files', [n.read_bytes() for n in names] == before)
            repeated = subprocess.run([str(binaries[0]), '--no-browser'], env=env, capture_output=True, timeout=20)
            check('same executable still reuses its live root', repeated.returncode == 0 and [n.read_bytes() for n in names] == before)
            api(am,'action',{'action':'quit'}); first.wait(timeout=20)
            api(bm,'action',{'action':'quit'}); second.wait(timeout=20)
            reopened, marker = start(0)
            check('original executable reopens original save after other build exits', sentinel.read_bytes() == b'FIRST BUILD SAVE')
            api(marker,'action',{'action':'quit'}); reopened.wait(timeout=20)
            check('all owned processes exit cleanly', all(p.returncode == 0 for p in processes))
        finally:
            for root in roots:
                try: api(root/'userdata/open-launcher.txt','action',{'action':'quit'})
                except (OSError,ValueError): pass
            for process in processes:
                if process.poll() is None:
                    try: process.wait(timeout=10)
                    except subprocess.TimeoutExpired: process.terminate(); process.wait(timeout=10)
    a.output.parent.mkdir(parents=True,exist_ok=True)
    a.output.write_text(json.dumps({'checks':checks,'count':len(checks),'version':builds[0]['version'],'browser_rendered':False,'game_executed':False},indent=2)+'\n')


if __name__ == '__main__': main()

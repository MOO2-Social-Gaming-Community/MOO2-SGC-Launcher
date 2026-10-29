#!/usr/bin/env python3
"""Wrap an already signed offline development launcher package in one executable.
Maintainer-only builder. It does not publish, access credentials, or bundle games.
The outer executable is built after the inner package is signed: no recursion.
"""
from __future__ import annotations
import argparse, base64, hashlib, json, os, pathlib, platform, shutil, subprocess, tempfile, zipfile
from source_fingerprint import source_index
ROOT=pathlib.Path(__file__).resolve().parents[1]
TARGETS=('windows-amd64','linux-amd64','darwin-amd64','darwin-arm64')

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--release',type=pathlib.Path,required=True)
    p.add_argument('--out',type=pathlib.Path,required=True)
    p.add_argument('--targets',nargs='+',choices=TARGETS,default=list(TARGETS))
    a=p.parse_args();release=a.release.resolve();out=a.out.resolve()
    if out.exists() and any(out.iterdir()):raise SystemExit('Refusing nonempty output directory')
    trust=json.loads((release/'trust-public.json').read_text())
    if not trust.get('development') or trust.get('endpoints') or trust.get('allowed_hosts'):
        raise SystemExit('This private-candidate builder accepts offline development trust only')
    subprocess.run(['go','run','./cmd/releasectl','--command','verify','--trust',str(release/'trust-public.json'),'--dir',str(release)],cwd=ROOT/'manager',check=True)
    manifest=json.loads((release/'manifest.json').read_text());version=(ROOT/'VERSION').read_text().strip()
    if manifest['release']!=version:raise SystemExit('Signed release version differs from source')
    current=source_index(ROOT);encoded=base64.b64encode(json.dumps(trust,separators=(',',':')).encode()).decode()
    packages={p['platform']:p for p in manifest['packages'] if p['id']=='launcher'}
    out.mkdir(parents=True,exist_ok=True);results=[]
    with tempfile.TemporaryDirectory(prefix='moo2-desktop-build-') as td:
        module=pathlib.Path(td)/'manager'
        shutil.copytree(ROOT/'manager',module,ignore=shutil.ignore_patterns('out','__pycache__','*.zip','*.exe'))
        embedded=module/'cmd/desktop/embedded'
        for target in a.targets:
            pkg=packages[target];archive=release/pkg['filename']
            if pkg['version']!=version:raise SystemExit('Mixed launcher version')
            with zipfile.ZipFile(archive) as z:
                if json.loads(z.read('SOURCE-FINGERPRINT.json'))!=current:
                    raise SystemExit('Source changed since signed payload build; rebuild release before embedding')
            for f in embedded.iterdir():f.unlink()
            for name in ('manifest.json','manifest.sig',pkg['filename']):shutil.copyfile(release/name,embedded/name)
            system,arch=target.split('-');name='MOO2-SGC-'+target+('.exe' if system=='windows' else '')
            binary=out/name;flags='-s -w -X moo2manager/shared/buildconfig.Encoded='+encoded
            if system=='windows':flags+=' -H=windowsgui'
            env=dict(os.environ,GOOS=system,GOARCH=arch,CGO_ENABLED='0')
            subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(binary),'./cmd/desktop'],cwd=module,env=env,check=True)
            binary.chmod(0o755)
            results.append({'target':target,'file':name,'version':version,'bytes':binary.stat().st_size,'sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'embedded_package_sha256':pkg['sha256'],'gui_subsystem':system=='windows','cross_compiled':target!=({'Linux':'linux','Darwin':'darwin','Windows':'windows'}.get(platform.system(),'unknown')+'-'+{'x86_64':'amd64','AMD64':'amd64','aarch64':'arm64','arm64':'arm64'}.get(platform.machine(),'unknown')),'native_os_tested':False,'os_signed':False})
    (out/'BUILD-RESULTS.json').write_text(json.dumps({'development':True,'offline_launcher_install':True,'game_data_included':False,'source_index_sha256':hashlib.sha256((json.dumps(current,indent=2)+'\n').encode()).hexdigest(),'artifacts':results},indent=2)+'\n')
    (out/'SHA256SUMS.txt').write_text(''.join(r['sha256']+'  '+r['file']+'\n' for r in results))
    print(json.dumps(results,indent=2))
if __name__=='__main__':main()

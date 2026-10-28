#!/usr/bin/env python3
"""Execute the signed offline bootstrap and real local manager API on Linux.
Uses caller-supplied paths; never publishes, launches MOO2, or includes game data.
"""
from pathlib import Path
import argparse, hashlib, json, os, shutil, subprocess, tempfile, time, urllib.request, urllib.error
p=argparse.ArgumentParser();p.add_argument('--release',type=Path,required=True);p.add_argument('--evidence',type=Path,required=True);p.add_argument('--base',type=Path);p.add_argument('--patch',type=Path);a=p.parse_args()
a.evidence.mkdir(parents=True,exist_ok=True)
release=a.release.resolve();setup=release/'binaries/linux-amd64/MOO2-SGC-Setup';results=[]
def check(name,cond,details=None):
 if not cond:raise AssertionError(name+': '+str(details))
 results.append({'test':name,'result':'PASS','details':details});print('PASS',name,flush=True)
def digest(p):
 with p.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
with tempfile.TemporaryDirectory(prefix='sgc-bootstrap-test-') as temp:
 root=Path(temp)/'installed'
 def boot(*args,ok=True):
  r=subprocess.run([str(setup),'--install-root',str(root),*args],text=True,capture_output=True,timeout=60)
  if ok and r.returncode:raise AssertionError(r.stdout+r.stderr)
  if not ok and not r.returncode:raise AssertionError('expected refusal')
  return r
 check('real Linux setup version',boot('--version').stdout.strip()==json.loads((release/'manifest.json').read_text())['release'])
 check('embedded key is explicitly development-only','development=true' in boot('--trust-status').stdout)
 r=boot('--offline',str(release),'--no-launch')
 check('signed offline launcher install succeeds','installed and verified' in r.stdout)
 pointer=root/'components/launcher/current.json';first=json.loads(pointer.read_text())
 check('setup health-check executed actual compiled launcher','current' in first)
 # Synthetic sentinels, not game fixtures or loadable saves. Keep byte-level
 # preservation assertions non-vacuous even when --base/--patch are omitted.
 sentinels={'saves/acceptance.GAM':b'SYNTHETIC save sentinel\x00\xff',
            'profiles/acceptance.txt':b'SYNTHETIC profile sentinel\n',
            'settings/acceptance.cfg':b'SYNTHETIC settings sentinel\r\n'}
 for name,data in sentinels.items():
  target=root/'userdata'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(data)
 def preserved(name):
  check(name,all((root/'userdata'/n).read_bytes()==data for n,data in sentinels.items()))
 check('metadata high-water revision persisted',json.loads((root/'distribution/trusted-state.json').read_text())['revision']==1)
 boot('--command','verify');check('installed signed file index verifies',True)
 before=pointer.read_bytes();boot('--offline',str(release),'--no-launch');check('repeated setup reuses intact generation',pointer.read_bytes()==before)
 boot('--command','repair','--offline',str(release),'--no-launch');second=json.loads(pointer.read_text());check('repair creates new generation retaining rollback',second['current']!=first['current'] and second['previous']==first['current'])
 boot('--command','rollback','--no-launch');check('explicit rollback reactivates verified prior launcher',json.loads(pointer.read_text())['current']==first['current'])
 preserved('repair and rollback preserve synthetic user-data bytes')
 boot('--command','rollback','--no-launch')
 current=json.loads(pointer.read_text())['current'];entry=root/'components/launcher/versions'/current/'files/moo2-sgc-launcher'
 contents=entry.read_bytes();bad=bytearray(contents);bad[-100]^=1;entry.write_bytes(bad)
 r=boot('--command','verify',ok=False);check('installed executable corruption refused','corrupted' in r.stderr)
 boot('--command','repair','--offline',str(release),'--no-launch');check('repair restores executable from signed bytes',True)
 boot('--command','verify');check('repaired generation verifies independently',True)
 before=pointer.read_bytes();r=boot('--command','rollback','--no-launch',ok=False)
 check('rollback refuses corrupted previous generation','corrupted' in r.stderr)
 check('failed rollback preserves active generation',pointer.read_bytes()==before)
 preserved('corruption recovery preserves synthetic user-data bytes')
 bad_release=Path(temp)/'bad-release';shutil.copytree(release,bad_release,ignore=shutil.ignore_patterns('binaries'))
 with (bad_release/'manifest.json').open('ab') as f:f.write(b' ')
 before=pointer.read_bytes();r=boot('--offline',str(bad_release),'--no-launch',ok=False)
 check('tampered metadata signature refused','signature mismatch' in r.stderr)
 check('tampered release leaves active launcher unchanged',pointer.read_bytes()==before)
 # A fresh install has no good cached package that could hide archive tampering.
 (bad_release/'manifest.json').write_bytes((release/'manifest.json').read_bytes())
 package=next(x for x in json.loads((release/'manifest.json').read_text())['packages'] if x['platform']=='linux-amd64')
 archive=bad_release/package['filename'];bad=bytearray(archive.read_bytes());bad[-100]^=1;archive.write_bytes(bad)
 installed_root=root
 try:
  root=Path(temp)/'tampered-package-install'
  r=boot('--offline',str(bad_release),'--no-launch',ok=False)
  check('tampered package refused on fresh install','SHA-256' in r.stderr)
  check('tampered package never activates a launcher',not (root/'components/launcher/current.json').exists())
 finally:root=installed_root
 # Exercise actual setup -> actual launcher process handoff and local HTTP API.
 log=(Path(temp)/'launcher.log').open('w')
 proc=subprocess.Popen([str(setup),'--install-root',str(root),'--command','launch-installed','--no-browser'],stdout=log,stderr=log)
 try:
  marker=root/'userdata/open-launcher.txt'
  for _ in range(200):
   if marker.exists():break
   if proc.poll() is not None:raise RuntimeError('launcher handoff failed')
   time.sleep(.05)
  local,token=marker.read_text().strip().split('/#')
  def api(route,body=None):
   headers={'Authorization':'Bearer '+token}
   data=None
   if body is not None:headers['Content-Type']='application/json';data=json.dumps(body).encode()
   with urllib.request.urlopen(urllib.request.Request(local+'/api/'+route,data,headers),timeout=10) as r:return json.load(r)
  def job(action,**kw):
   api('action',{'action':action,**kw})
   for _ in range(800):
    s=api('state');j=s['job']
    if not j['busy']:return j
    time.sleep(.05)
   raise TimeoutError(action)
  s=api('state');check('bootstrap hands off to real local launcher API',s['version']==json.loads((release/'manifest.json').read_text())['release'])
  check('launcher uses stable user-data directory',Path(s['data_path'])==root/'userdata')
  check('production feed explicitly unconfigured',not s['distribution']['production_feed_configured'])
  check('PRSL remains optional and unavailable',not s['prsl_available'] and not s['chat_available'])
  r=boot('--command','repair','--offline',str(release),'--no-launch',ok=False);check('bootstrap refuses update while launcher owns lock','locked' in r.stderr)
  j=job('launcher-check',offline_path=str(release));check('launcher verifies offline release metadata',not j['error'] and j['result']['revision']==1)
  j=job('launcher-stage',offline_path=str(release));check('launcher stages signed next-start update',not j['error'] and not j['result']['activated'])
  if a.base and a.patch:
   hashes=[digest(a.base),digest(a.patch)]
   for kind,file in [('base',a.base),('patch',a.patch)]:
    j=job('payload-import',payload_kind=kind,source_path=str(file.resolve()))
    check('real '+kind+' archive imported locally',not j['error'] and not j['result']['source_modified'] and not j['result']['uploaded'])
   profile=next(x for x in api('state')['profiles'] if x['id']=='community')
   j=job('prepare',profile=profile);check('bootstrapped launcher prepares real community workspace',not j['error'],j.get('result'))
   j=job('verify',id='community');check('real imported installation verifies',not j['error'] and j['result']['ok'],j.get('result',{}).get('checked'))
   check('owned source archives remain byte-identical',hashes==[digest(a.base),digest(a.patch)])
  api('action',{'action':'quit'});proc.wait(timeout=10);check('setup and launcher exit cleanly together',proc.returncode==0)
  check('both installer and manager locks released',not (root/'installation.lock').exists() and not (root/'userdata/manager.lock').exists())
 finally:
  if proc.poll() is None:proc.terminate();proc.wait(timeout=10)
  log.close()
 user_files={str(x.relative_to(root/'userdata')):digest(x) for x in (root/'userdata').rglob('*') if x.is_file() and x.name!='open-launcher.txt'}
 # First cover intact same-package idempotence, then force reconstruction from
 # staged signed bytes. Neither case represents an older-version migration.
 before=pointer.read_bytes()
 boot('--command','apply-staged','--no-launch');check('same-package staged apply reuses intact generation',pointer.read_bytes()==before)
 current=json.loads(pointer.read_text())['current'];entry=root/'components/launcher/versions'/current/'files/moo2-sgc-launcher'
 bad=bytearray(entry.read_bytes());bad[-100]^=1;entry.write_bytes(bad)
 boot('--command','apply-staged','--no-launch')
 check('staged apply replaces corrupted generation after shutdown',json.loads(pointer.read_text())['current']!=current)
 boot('--command','verify');check('staged replacement verifies independently',True)
 after={str(x.relative_to(root/'userdata')):digest(x) for x in (root/'userdata').rglob('*') if x.is_file() and x.name!='open-launcher.txt'}
 check('applying launcher update preserves every user-data file',user_files==after)
 preserved('staged replacement preserves synthetic user-data bytes')
 r=boot('--no-launch',ok=False);check('no invented live release feed attempted','no production feed configured' in r.stderr)
(a.evidence/'bootstrap-integration.json').write_text(json.dumps({'tests':results,'count':len(results),'platform':'linux-amd64','executed':'real compiled bootstrapper and launcher, actual local HTTP API','network_provider_tests':'separate TLS fixture unit tests; not live GitHub/R2','game_execution':False,'prsl_execution':False},indent=2)+'\n')
print('TOTAL',len(results))

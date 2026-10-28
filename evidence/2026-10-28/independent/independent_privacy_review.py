#!/usr/bin/env python3
"""Independent Linux updater privacy regression. Evidence never copies token-bearing output.
Disposable installed roots; only loopback HTTP. xdg-open never launches a browser.
"""
import argparse, hashlib, json, os, re, signal, stat, subprocess, tempfile, time
from pathlib import Path
import urllib.request, urllib.error

p=argparse.ArgumentParser();p.add_argument('--release',type=Path,required=True);p.add_argument('--evidence',type=Path,required=True);p.add_argument('--expect-leak',action='store_true');p.add_argument('--extended',action='store_true');a=p.parse_args()
release=a.release.resolve();setup=release/'binaries/linux-amd64/MOO2-SGC-Setup';launcher=release/'binaries/linux-amd64/moo2-sgc-launcher'
checks=[];observations=[];groups=[]
def check(name,condition):
    if not condition: raise AssertionError(name)
    checks.append(name);print('PASS',name,flush=True)
def until(fn,timeout=45):
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        try: value=fn()
        except (FileNotFoundError,ConnectionRefusedError,urllib.error.URLError):value=None
        if value:return value
        time.sleep(.05)
    raise AssertionError('lifecycle readiness timeout')
def alive(group):
    for path in Path('/proc').glob('[0-9]*/stat'):
        try:
            fields=path.read_text().rsplit(')',1)[1].split()
            if int(fields[2])==group and fields[0]!='Z':return True
        except (FileNotFoundError,ProcessLookupError):pass
    return False
def stop(proc):
    if alive(proc.pid):os.killpg(proc.pid,signal.SIGTERM)
    if proc.poll() is None:proc.wait(timeout=10)
    if alive(proc.pid):
        os.killpg(proc.pid,signal.SIGKILL);until(lambda:not alive(proc.pid))
def api(credentials,route,body=None,override_token=None):
    local,token=credentials.split('/#')
    headers={'Authorization':'Bearer '+(token if override_token is None else override_token)};data=None
    if body is not None:headers['Content-Type']='application/json';data=json.dumps(body).encode()
    with urllib.request.urlopen(urllib.request.Request(local+'/api/'+route,data,headers),timeout=5) as r:return json.load(r)
def credentials(marker):
    value=marker.read_text().strip()
    if re.fullmatch(r'http://127\.0\.0\.1:[0-9]+/#[a-f0-9]+',value):return value
    return None
def inspect_capture(name,output,credential,expect_url,diagnostics=True):
    raw=output.read_bytes();token=credential.split('/#')[1].encode()
    check(name+' private URL expectation', (credential.encode() in raw)==expect_url)
    check(name+' bare token expectation', (token in raw)==expect_url)
    check(name+' private URL pattern expectation',bool(re.search(rb'http://127\.0\.0\.1:\d+/#[a-f0-9]+',raw))==expect_url)
    if diagnostics:
        check(name+' launcher identity diagnostics preserved',b'MOO2-SGC Launcher' in raw)
        check(name+' launcher exit guidance preserved',b'Exit launcher' in raw)
    return raw

def run():
  with tempfile.TemporaryDirectory(prefix='independent-privacy-') as td:
    temp=Path(td);recorder=temp/'bin';recorder.mkdir();calls=temp/'calls'
    opener=recorder/'xdg-open'
    # Echo the URL to both streams deliberately: nil child streams must discard it.
    opener.write_text('#!/bin/sh\nprintf "call\\n" >> "$OPENER_COUNT"\nprintf "%s\\n" "$@"\nprintf "%s\\n" "$@" >&2\nexit 1\n');opener.chmod(0o755)
    env=dict(os.environ,PATH=str(recorder),OPENER_COUNT=str(calls))
    def count():return len(calls.read_text().splitlines()) if calls.exists() else 0
    def boot(root,*args):
        r=subprocess.run([str(setup),'--install-root',str(root),*args],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=60)
        check('setup command '+str(len(checks))+' succeeds',r.returncode==0)
    def start(root,args,out,environment=env,exe=setup):
        f=out.open('wb');cmd=[str(exe)]
        if exe==setup:cmd+=['--install-root',str(root)]
        else:cmd+=['--app-root',str(root),'--root',str(root),'--data',str(root/'userdata')]
        proc=subprocess.Popen(cmd+args,env=environment,stdout=f,stderr=f,start_new_session=True);f.close();groups.append(proc)
        return proc
    modes=[('headless',True,'success'),('normal',False,'success')]
    if a.extended:modes += [('missing-opener',False,'missing'),('unstartable-opener',False,'invalid')]
    for name,headless,open_mode in modes:
        root=temp/name;boot(root,'--offline',str(release),'--no-launch')
        test_env=env
        if open_mode=='missing':
            missing=temp/'empty-path';missing.mkdir(exist_ok=True);test_env=dict(env,PATH=str(missing))
        elif open_mode=='invalid':
            invalid=temp/'invalid-path';invalid.mkdir(exist_ok=True);bad=invalid/'xdg-open';bad.write_text('#!/missing-test-only-interpreter\n');bad.chmod(0o755);test_env=dict(env,PATH=str(invalid))
        marker=root/'userdata/open-launcher.txt';out=temp/(name+'-initial.txt');before=count()
        proc=start(root,['--command','launch-installed']+(['--no-browser'] if headless else []),out,test_env)
        try:
            first=until(lambda:credentials(marker));until(lambda:api(first,'state'))
            time.sleep(.2)
            check(name+' recovery marker mode 0600',stat.S_IMODE(marker.stat().st_mode)==0o600)
            raw=inspect_capture(name+' initial',out,first,True)
            check(name+' initial setup diagnostics preserved',b'MOO2-SGC Setup' in raw)
            check(name+' initial opener count',count()-before==(0 if headless or open_mode!='success' else 1))
            api(first,'action',{'action':'launcher-stage','offline_path':str(release)})
            stage=until(lambda:(x if not x['job']['busy'] else None) if (x:=api(first,'state')) else None)
            check(name+' stage succeeded',not stage['job']['error'])
            initial_count=count();api(first,'action',{'action':'launcher-apply'})
            proc.wait(timeout=15);check(name+' initial process clean exit',proc.returncode==0)
            second=until(lambda:(c if c!=first else None) if (c:=credentials(marker)) else None);until(lambda:api(second,'state'))
            time.sleep(.7)
            check(name+' restart opener count',count()-initial_count==(0 if headless or open_mode!='success' else 1))
            log=root/'logs/update-handoff.log';raw=inspect_capture(name+' handoff log',log,second,a.expect_leak)
            check(name+' handoff log mode 0600',stat.S_IMODE(log.stat().st_mode)==0o600)
            check(name+' setup diagnostics retained in handoff log',b'MOO2-SGC Setup' in raw and b'Launching/ready:' in raw)
            check(name+' initial token absent from handoff log',first.split('/#')[1].encode() not in raw)
            if not a.expect_leak:check(name+' recovery marker guidance',b'open-launcher.txt' in raw)
            if open_mode!='success':
                check(name+' opener failure diagnostic retained',b'Browser could not be opened automatically:' in raw)
            try:api(second,'state',override_token=first.split('/#')[1])
            except urllib.error.HTTPError as e:check(name+' old bearer token refused',e.code==401)
            else:raise AssertionError(name+' old bearer token unexpectedly valid')
            check(name+' new marker mode 0600',stat.S_IMODE(marker.stat().st_mode)==0o600)
            api(second,'action',{'action':'quit'});until(lambda:not alive(proc.pid))
            check(name+' all descendants exited',not alive(proc.pid))
            check(name+' private marker and locks removed',not marker.exists() and not (root/'userdata/manager.lock').exists() and not (root/'installation.lock').exists())
            observations.append({'mode':name,'url_in_handoff_log':a.expect_leak,'headless':headless,'opener':open_mode,'observation_after_ready_seconds':.7})
            # Direct CLI staged apply intentionally suppresses URL and directs to private marker.
            if a.extended and headless:
                manual=temp/'manual-apply.txt';manual_proc=start(root,['--command','apply-staged','--no-browser'],manual)
                try:
                    c=until(lambda:credentials(marker));until(lambda:api(c,'state'));time.sleep(.2)
                    raw=inspect_capture('manual apply-staged no-browser',manual,c,a.expect_leak)
                    if not a.expect_leak:check('manual apply-staged recovery guidance',str(marker).encode() in raw)
                    api(c,'action',{'action':'quit'});manual_proc.wait(timeout=15)
                    check('manual apply-staged clean exit',manual_proc.returncode==0)
                finally:stop(manual_proc)
        finally:stop(proc)
    if a.extended:
        # Explicit direct launcher CLI remains useful both normal/headless; opt-in private mode is separate.
        for label,args,expect_url in [('direct-headless',['--no-browser'],True),('direct-normal',[],True),('direct-private-headless',['--no-browser','--private-url-file-only'],False)]:
            root=temp/label;out=temp/(label+'.txt');proc=start(root,args,out,env,launcher);marker=root/'userdata/open-launcher.txt';before=count()
            try:
                c=until(lambda:credentials(marker));until(lambda:api(c,'state'));time.sleep(.2)
                inspect_capture(label,out,c,expect_url)
                api(c,'action',{'action':'quit'});proc.wait(timeout=15);check(label+' clean exit',proc.returncode==0)
            finally:stop(proc)
    if a.extended:
        # Recovery-file failure must fail without ever emitting a token or starting an opener.
        root=temp/'marker-write-error';marker=root/'userdata/open-launcher.txt';marker.mkdir(parents=True);before=count()
        r=subprocess.run([str(launcher),'--root',str(root),'--app-root',str(root),'--data',str(root/'userdata'),'--no-browser','--private-url-file-only'],env=env,capture_output=True,timeout=15)
        check('marker-write error fails startup',r.returncode!=0)
        check('marker-write error preserves useful diagnostic',b'Error:' in r.stderr and b'open-launcher.txt' in r.stderr)
        check('marker-write error emits no private URL',not re.search(rb'http://127\.0\.0\.1:\d+/#[a-f0-9]+',r.stdout+r.stderr))
        check('marker-write error does not start opener',count()==before)
        check('marker-write error cleans process locks',not (root/'userdata/manager.lock').exists() and not (root/'installation.lock').exists())
        # If the handoff log cannot be opened, keep the original manager usable and disclose no token in API error.
        root=temp/'handoff-log-error';boot(root,'--offline',str(release),'--no-launch');(root/'logs/update-handoff.log').mkdir();marker=root/'userdata/open-launcher.txt';out=temp/'log-error-console.txt'
        proc=start(root,['--command','launch-installed','--no-browser'],out)
        try:
            c=until(lambda:credentials(marker));until(lambda:api(c,'state'));pointer=root/'components/launcher/current.json';before=pointer.read_bytes()
            api(c,'action',{'action':'launcher-stage','offline_path':str(release)})
            staged=until(lambda:(x if not x['job']['busy'] else None) if (x:=api(c,'state')) else None)
            check('log-open-error stage succeeded',not staged['job']['error'])
            try:api(c,'action',{'action':'launcher-apply'})
            except urllib.error.HTTPError as e:
                check('log-open-error apply refused',e.code==400);error=e.read()
                check('log-open-error API diagnostic excludes token',c.encode() not in error and c.split('/#')[1].encode() not in error)
            else:raise AssertionError('log-open-error apply unexpectedly accepted')
            check('log-open-error keeps original manager usable',proc.poll() is None and not api(c,'state')['job']['busy'])
            check('log-open-error preserves current generation',pointer.read_bytes()==before)
            check('log-open-error preserves current recovery marker',credentials(marker)==c)
            api(c,'action',{'action':'quit'});proc.wait(timeout=15);check('log-open-error clean exit',proc.returncode==0)
            check('log-open-error cleans private marker and locks',not marker.exists() and not (root/'userdata/manager.lock').exists() and not (root/'installation.lock').exists())
        finally:stop(proc)
    check('recorder contains call counts only',calls.exists() and set(calls.read_text().splitlines())=={'call'})
    check('all test process groups stopped',all(not alive(proc.pid) for proc in groups))

try:run()
except Exception as e:
    # Do not print exception messages: HTTP/process exceptions can embed bearer URLs.
    report={'result':'FAIL','exception_type':type(e).__name__,'failed_assertion':str(e) if type(e) is AssertionError else 'redacted','passed_count':len(checks),'checks':checks}
    a.evidence.parent.mkdir(parents=True,exist_ok=True);a.evidence.write_text(json.dumps(report,indent=2)+'\n')
    print('FAIL',report['exception_type'],report['failed_assertion'],flush=True);raise SystemExit(1)
report={'result':'PASS','count':len(checks),'checks':checks,'observations':observations,'expected_baseline_leak':a.expect_leak,'extended':a.extended,'linux_launcher_sha256':hashlib.sha256(launcher.read_bytes()).hexdigest(),'setup_sha256':hashlib.sha256(setup.read_bytes()).hexdigest(),'browser_executed':False,'external_network':False,'scope':'Independent signed offline same-package update and localhost API; Linux process double only; no native browser acceptance or old-release migration.'}
a.evidence.parent.mkdir(parents=True,exist_ok=True);a.evidence.write_text(json.dumps(report,indent=2)+'\n');print('TOTAL',len(checks),flush=True)

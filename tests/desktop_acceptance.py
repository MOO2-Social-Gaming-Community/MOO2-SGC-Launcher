#!/usr/bin/env python3
"""Real Linux desktop-container/API lifecycle; browser is a recorder, never rendered.
Runs only fresh isolated roots and synthetic saves. No game or runtime is launched.
"""
import argparse, json, os, pathlib, signal, subprocess, sys, tempfile, time, urllib.request, urllib.error
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parents[1]/"packaging"))
from build_desktop import trust_fingerprint
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--binary',type=pathlib.Path,required=True);p.add_argument('--release',type=pathlib.Path,required=True);p.add_argument('--evidence',type=pathlib.Path,required=True);a=p.parse_args();binary=a.binary.resolve();release=a.release.resolve();checks=[]
version=json.loads((release/'manifest.json').read_text())['release']; build_identity=trust_fingerprint(json.loads((release/'trust-public.json').read_text())); candidate_root='MOO2-SGC/private-gui-'+version+'-'+build_identity[:16]
def check(name,ok):
    if not ok:raise AssertionError(name)
    checks.append(name);print('PASS',name,flush=True)
def until(fn,timeout=45):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        try:
            v=fn()
            if v:return v
        except (FileNotFoundError,ConnectionError,urllib.error.URLError):pass
        time.sleep(.05)
    raise AssertionError('timed out')
def credentials(marker):
    raw=marker.read_text().strip();host,token=raw.split('/#');return host,token
def api(marker,path,body=None,method=None):
    host,token=credentials(marker);data=None if body is None else json.dumps(body).encode();headers={'Authorization':'Bearer '+token};
    if data is not None:headers['Content-Type']='application/json'
    r=urllib.request.Request(host+'/api/'+path,data,headers,method=method)
    try:
        with urllib.request.urlopen(r,timeout=5) as res:return json.load(res)
    except urllib.error.HTTPError as e:
        print('API ERROR',path,e.code,e.read().decode(),flush=True);raise
def args(pid):return pathlib.Path('/proc',str(pid),'cmdline').read_bytes().decode().rstrip('\0').split('\0')
with tempfile.TemporaryDirectory(prefix='moo2-desktop-acceptance-') as td:
    temp=pathlib.Path(td);binpath=temp/'bin';binpath.mkdir();counter=temp/'opener-count';opener=binpath/'xdg-open';opener.write_text('#!/bin/sh\nprintf "call\\n" >> "$OPENER_COUNT"\n');opener.chmod(0o755)
    env=dict(os.environ,PATH=str(binpath)+os.pathsep+os.environ['PATH'],OPENER_COUNT=str(counter))
    processes=[]
    def owned(root):
        root.mkdir(parents=True,exist_ok=True);(root/'desktop-installation.json').write_text(json.dumps({'schema':2,'trust_fingerprint':build_identity,'kind':'moo2-private-gui','created_version':version,'runtime_history_contract':1,'manager_launch_attempted':False}))
    def launch(label,exe=binary):
        config=temp/label;root=config/candidate_root;log=temp/(label+'.log');f=log.open('wb');proc=subprocess.Popen([str(exe)],env=dict(env,XDG_CONFIG_HOME=str(config)),stdout=f,stderr=f,start_new_session=True);f.close();processes.append(proc);marker=root/'desktop-session/open.txt';until(marker.exists);return proc,root,marker,log
    def quit_boot(proc,marker):api(marker,'exit',{},'POST');proc.wait(timeout=10);check('startup exits cleanly',proc.returncode==0)
    try:
        proc,root,boot,log=launch('fresh');ready=until(lambda:(s if s.get('target') else None) if (s:=api(boot,'status')) else None)
        manager=root/'userdata/open-launcher.txt';identity=api(manager,'identity');pid=identity['pid'];initial_url=manager.read_text();pointer=(root/'components/launcher/current.json').read_bytes()
        check('no-argument executable installs and opens local GUI automatically',identity['version']==version and counter.read_text().count('call')==1)
        check('isolated per-user root used',identity['root']==str(root))
        check('private child policy hidden-output/no-shortcut/desktop mode',all(x in args(pid) for x in ['--desktop-gui','--defer-browser-once','--no-shortcut','--private-url-file-only']))
        sentinel=root/'userdata/synthetic-save.GAM';sentinel.write_bytes(b'SYNTHETIC SAVE ONLY')
        repeated=subprocess.run([str(binary)],env=dict(env,XDG_CONFIG_HOME=str(temp/'fresh')),capture_output=True,timeout=10)
        check('second double-click reopens instead of reinstalling',repeated.returncode==0 and counter.read_text().count('call')==2 and api(manager,'identity')['pid']==pid and (root/'components/launcher/current.json').read_bytes()==pointer)
        check('second click preserves userdata',sentinel.read_bytes()==b'SYNTHETIC SAVE ONLY')
        check('private markers restricted to owner',all(f.stat().st_mode&0o777==0o600 for f in (boot,manager)))
        state=api(manager,'state');check('missing game/runtime remains graphical, not auto-play',not state['running'] and not any(state['active'].values()))
        api(manager,'action',{'action':'launcher-stage','offline_path':str(release)});state=until(lambda:(s if not s['job']['busy'] else None) if (s:=api(manager,'state')) else None);check('signed offline update stages',not state['job']['error'])
        api(manager,'action',{'action':'launcher-apply'});until(lambda:manager.read_text()!=initial_url);identity=until(lambda:api(manager,'identity'));newpid=identity['pid'];until(lambda:counter.read_text().count('call')==3)
        check('desktop update opens GUI and preserves no-shortcut',newpid!=pid and '--desktop-gui' in args(newpid) and '--no-shortcut' in args(newpid) and '--no-browser' not in args(newpid))
        proc.wait(timeout=15);check('outer exits cleanly during signed helper handoff',proc.returncode==0)
        newurl=manager.read_text();logs='\n'.join(f.read_text(errors='replace') for f in (root/'logs').rglob('*') if f.is_file())
        check('handoff logs do not retain old/new session credentials',all(v.strip() not in logs for v in (initial_url,newurl)) and all(v.strip().split('#')[1] not in logs for v in (initial_url,newurl)))
        api(manager,'action',{'action':'quit'});until(lambda:not (root/'installation.lock').exists());check('GUI exit cleans launcher locks',(not (root/'userdata/manager.lock').exists()))
        proc2,root2,boot2,log2=launch('fresh');until(lambda:api(boot2,'status').get('target'));check('later no-argument open reuses intact generation',(root2/'components/launcher/current.json').read_bytes()==pointer and sentinel.read_bytes()==b'SYNTHETIC SAVE ONLY');api(root2/'userdata/open-launcher.txt','action',{'action':'quit'});proc2.wait(timeout=10)
        # Explicit headless is not the same as one-time initial GUI deferral.
        headless_config=temp/'headless';before=counter.read_text().count('call')
        hp=subprocess.Popen([str(binary),'--no-browser'],env=dict(env,XDG_CONFIG_HOME=str(headless_config)),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True);processes.append(hp)
        hr=headless_config/candidate_root;hb=hr/'desktop-session/open.txt';until(hb.exists);until(lambda:api(hb,'status').get('target'));hm=hr/'userdata/open-launcher.txt';old=hm.read_text()
        check('explicit headless initial launch opens no browser',counter.read_text().count('call')==before and '--no-browser' in args(api(hm,'identity')['pid']))
        api(hm,'action',{'action':'launcher-stage','offline_path':str(release)});until(lambda:not api(hm,'state')['job']['busy']);api(hm,'action',{'action':'launcher-apply'});until(lambda:hm.read_text()!=old);hi=until(lambda:api(hm,'identity'));hp.wait(timeout=15);time.sleep(.2)
        check('explicit headless update stays headless and shortcut-free',counter.read_text().count('call')==before and all(x in args(hi['pid']) for x in ['--no-browser','--no-shortcut','--desktop-gui']))
        api(hm,'action',{'action':'quit'});until(lambda:not (hr/'installation.lock').exists())
        # A crashed outer cannot strand its kernel lock or duplicate the live manager.
        cp,cr,cb,_=launch('outer-crash');until(lambda:api(cb,'status').get('target'));cm=cr/'userdata/open-launcher.txt';ci=api(cm,'identity');cp.kill();cp.wait(timeout=5)
        reopened=subprocess.run([str(binary)],env=dict(env,XDG_CONFIG_HOME=str(temp/'outer-crash')),capture_output=True,timeout=10)
        check('outer crash reopens the still-live authenticated manager',reopened.returncode==0 and api(cm,'identity')['pid']==ci['pid'])
        api(cm,'action',{'action':'quit'});until(lambda:not (cr/'installation.lock').exists())
        again,_,againboot,_=launch('outer-crash');until(lambda:api(againboot,'status').get('target'));check('kernel startup lock auto-released after crash',again.poll() is None);api(cm,'action',{'action':'quit'});again.wait(timeout=10)
        # Permission failure occurs only in our own deliberately read-only fixture.
        readonly=temp/'read-only';readonly.mkdir(mode=0o500)
        permission=subprocess.run([str(binary),'--no-browser','--install-root',str(readonly)],capture_output=True,timeout=10)
        check('unwritable private folder fails visibly without mutation',permission.returncode!=0 and b'permission denied' in permission.stderr.lower() and not list(readonly.iterdir()));readonly.chmod(0o700)
        # Existing unknown data cannot be adopted as a candidate installation.
        unowned=temp/'unowned';unowned.mkdir();sent=unowned/'source-sentinel';sent.write_bytes(b'UNCHANGED')
        refused=subprocess.run([str(binary),'--no-browser','--install-root',str(unowned)],capture_output=True,timeout=10)
        check('existing unowned folders refused without creating ownership marker',refused.returncode!=0 and sent.read_bytes()==b'UNCHANGED' and not (unowned/'desktop-installation.json').exists())
        # Concurrent first clicks must not overwrite durable launch ownership.
        concurrent_config=temp/'concurrent';concurrent=[]
        for _ in range(2):
            x=subprocess.Popen([str(binary)],env=dict(env,XDG_CONFIG_HOME=str(concurrent_config)),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True);concurrent.append(x);processes.append(x)
        rr=concurrent_config/candidate_root;bb=rr/'desktop-session/open.txt';until(bb.exists);until(lambda:api(bb,'status').get('target'));until(lambda:any(x.poll()==0 for x in concurrent));owner=json.loads((rr/'desktop-installation.json').read_text())
        check('concurrent first clicks reuse one session without resetting ownership',owner['manager_launch_attempted'] is True and all(x.poll() in (None,0) for x in concurrent))
        api(rr/'userdata/open-launcher.txt','action',{'action':'quit'})
        for x in concurrent:x.wait(timeout=10)
        # Live lock: recovery must not remove it, and startup remains actionable.
        live_root=temp/('live/'+candidate_root);owned(live_root);live_lock=live_root/'installation.lock';live_lock.write_text(str(os.getpid())+'\n')
        live,_,boot_live,_=launch('live');until(lambda:api(boot_live,'status').get('error'))
        try:api(boot_live,'recover',{},'POST');raise AssertionError('live lock recovered')
        except urllib.error.HTTPError as e:check('live owner refuses graphical recovery',e.code==409 and live_lock.read_text()==str(os.getpid())+'\n')
        quit_boot(live,boot_live)
        # Dead startup with no possible surviving runtime: explicit GUI recovery.
        dead=subprocess.Popen(['/bin/true']);dead.wait();dead_pid=dead.pid
        stale_root=temp/('stale/'+candidate_root);owned(stale_root);(stale_root/'userdata').mkdir(parents=True)
        for f in (stale_root/'installation.lock',stale_root/'userdata/manager.lock'):f.write_text(str(dead_pid)+'\n')
        stale,_,boot_stale,_=launch('stale');until(lambda:api(boot_stale,'status').get('error'));api(boot_stale,'recover',{},'POST');until(lambda:api(boot_stale,'status').get('target'))
        check('recovery continues automatically without a hidden Retry button',bool(list(stale_root.glob('installation.lock.recovered-*'))) and bool(list((stale_root/'userdata').glob('manager.lock.recovered-*'))))
        api(stale_root/'userdata/open-launcher.txt','action',{'action':'quit'});stale.wait(timeout=10)
        unknown_root=temp/('unknown/'+candidate_root);owned(unknown_root);(unknown_root/'userdata/logs').mkdir(parents=True);(unknown_root/'installation.lock').write_text(str(dead_pid)+'\n');(unknown_root/'userdata/logs/last-launch.json').write_text('{}')
        unknown,_,boot_unknown,_=launch('unknown');until(lambda:api(boot_unknown,'status').get('error'))
        try:api(boot_unknown,'recover',{},'POST');raise AssertionError('unknown game state recovered')
        except urllib.error.HTTPError as e:check('possible orphan game retains protective lock',e.code==409 and (unknown_root/'installation.lock').exists())
        quit_boot(unknown,boot_unknown)
        # Mutate only a copy of the embedded signed manifest; ELF remains intact.
        content=binary.read_bytes();manifest=(release/'manifest.json').read_bytes();check('embedded manifest locatable exactly once',content.count(manifest)==1)
        mutated=manifest.replace(('\"release\": \"'+version+'\"').encode(), ('\"release\": \"'+('9' if version[0]!='9' else '8')+version[1:]+'\"').encode(),1);check('tamper fixture changes signed bytes only',mutated!=manifest and len(mutated)==len(manifest))
        damaged=temp/'damaged-desktop';damaged.write_bytes(content.replace(manifest,mutated,1));damaged.chmod(0o755)
        broken,broken_root,broken_boot,_=launch('invalid-payload',damaged);error=until(lambda:api(broken_boot,'status').get('error'));check('invalid signed payload is visible and never activated','verification failed' in error and not (broken_root/'components/launcher/current.json').exists());quit_boot(broken,broken_boot)
        check('normal console logs contain no private session URL','http://127.0.0.1:' not in log.read_text())
    finally:
        for proc in processes:
            if proc.poll() is None:
                try:os.killpg(proc.pid,signal.SIGTERM)
                except ProcessLookupError:pass
                try:proc.wait(timeout=5)
                except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);proc.wait()
a.evidence.parent.mkdir(parents=True,exist_ok=True);a.evidence.write_text(json.dumps({'checks':checks,'count':len(checks),'platform':'linux-amd64','browser':'recorder only, not rendered','native_windows_tested':False,'game_executed':False,'network_release_used':False},indent=2)+'\n')
print('TOTAL',len(checks))

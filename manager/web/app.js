'use strict';
const $=id=>document.getElementById(id);
const fragment=location.hash.slice(1);
if(fragment){sessionStorage.setItem('moo2-token',fragment);history.replaceState(null,'',location.pathname);}
const auth=sessionStorage.getItem('moo2-token')||'';
let state=null,currentID='baseline',lastResult='',lastJob='',first=true,offline=false;
let selectionQueue=Promise.resolve(),previewRevision=0,pendingDuplicate=null;
function error(e){$('error').textContent=String(e.message||e);$('error').classList.remove('hidden');}
function clearError(){$('error').classList.add('hidden');}
async function api(path,body){const r=await fetch('/api/'+path,{method:body===undefined?'GET':'POST',headers:{Authorization:'Bearer '+auth,...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});const v=await r.json();if(!r.ok)throw Error(v.error||r.statusText);return v;}
function selected(){return {id:currentID,name:$('name').value.trim(),engine:$('engine').value,core:$('engine').value!=='1.50.26'?'':$('core').value,mods:$('engine').value!=='1.50.26'?[]:[...document.querySelectorAll('#mods input:checked')].map(x=>x.dataset.mod),fullscreen:$('fullscreen').checked,role:$('role').value,network_service:$('role').value==='standalone'?'none':$('network-service').value,host:$('host').value.trim(),port:Number($('port').value)};}
function option(value,text){const o=document.createElement('option');o.value=value;o.textContent=text;return o;}
function modUI(p){
 const core=$('core');core.replaceChildren();if(p.engine==='1.50.26'){for(const m of state.catalog.filter(m=>m.group==='Core'))core.append(option(m.id,m.name+' · '+m.version));core.value=p.core||'150';}else{core.append(option('','Not applicable — original '+p.engine+' rules'));core.value='';}core.disabled=p.engine!=='1.50.26';
 const wrap=$('mods');wrap.replaceChildren();for(const m of state.catalog.filter(m=>m.group!=='Core')){
  const row=document.createElement('div');row.className='mod-row';const label=document.createElement('label');const cb=document.createElement('input');cb.type='checkbox';cb.dataset.mod=m.id;cb.checked=(p.mods||[]).includes(m.id);cb.disabled=p.engine!=='1.50.26';const text=document.createElement('span');text.textContent=m.name+(m.group?' · '+m.group:'');label.append(cb,text);const desc=document.createElement('small');desc.textContent=m.description;row.append(label,desc);wrap.append(row);cb.addEventListener('change',preview);
 }
 $('mod-count').textContent=p.engine==='1.50.26'?state.catalog.length+' bundled':'Requires engine 1.50.26';$('addons').open=p.engine==='1.50.26'&&$('addons').open;updateCoreDesc();
}
function updateCoreDesc(){const m=state.catalog.find(x=>x.id===$('core').value);$('core-desc').textContent=$('engine').value!=='1.50.26'?'Original executable; community rulesets are unavailable.':m?.description||'';}
function networkUI(){const role=$('role').value,svc=$('network-service').value;const active=role!=='standalone';$('network-service').disabled=!active;$('direct-network-fields').classList.toggle('hidden',!active||svc!=='direct');const rec=(state?.network_services||[]).find(x=>x.id===svc);$('network-service-desc').textContent=!active?'No network tunnel is configured for local play.':(rec?.description||'');if(svc==='dopefish'){$('port').value=213;$('host').value='';}if(svc==='direct'&&Number($('port').value)<1024&&role==='host')$('port').value=21300;}
function loadProfile(id,persist=false){const p=state.profiles.find(x=>x.id===id);if(!p)return;currentID=id;$('profiles').value=id;$('name').value=p.name;$('engine').value=p.engine;$('role').value=p.role;$('network-service').value=(['direct','dopefish'].includes(p.network_service)?p.network_service:'direct');$('port').value=p.port||21300;$('host').value=p.host||'';$('fullscreen').checked=p.fullscreen;modUI(p);$('engine').disabled=!!state.portable&&id==='baseline';renderHistory();networkUI();preview();if(persist)selectionQueue=selectionQueue.then(()=>api('selection',{id})).catch(error);}
function renderHistory(){const s=$('history');s.replaceChildren();const entries=state.history[currentID]||[];if(!entries.length)s.append(option('','No prepared environment'));for(const h of entries){s.append(option(h.generation,h.created.slice(0,19)+' · '+h.engine+' / '+(h.core||'original')+(state.active[currentID]===h.generation?' · ACTIVE':'')));}if(state.active[currentID])s.value=state.active[currentID];}
async function preview(){const revision=++previewRevision;try{updateCoreDesc();const p=selected();const r=await api('resolve',p);if(revision!==previewRevision)return;$('resolution').textContent='Resolved: '+(r.mods.map(m=>m.name).join(' + ')||('DOS '+p.engine))+'\nRequired by other selections: '+(r.automatic.join(', ')||'none')+'\nSelection fingerprint: '+r.fingerprint+'\nNot a gameplay certification. Verify files for the effective configuration fingerprint.';
 const n=r.network||{};$('network-plan').textContent='Selected game: '+p.engine+' / '+(p.engine==='1.50.26'?'ORION150.EXE':'ORION2.EXE')+'\n'+(n.enabled?(n.endpoint+'\n'+n.command):'LOCAL PLAY — no IPX tunnel')+'\n'+(n.instructions||'');
 }catch(e){if(revision!==previewRevision)return;$('resolution').textContent=e.message;$('network-plan').textContent='Selection needs attention: '+e.message;}}
async function action(action,extra={}){clearError();try{await selectionQueue;const v=await api('action',{action,...extra});if((action==='quit'||action==='launcher-apply')){offline=true;$('progress').textContent=action==='launcher-apply'?'Applying staged update; the launcher will open a new tab.':'Launcher exited. You may close this tab.';return;}await poll();return v;}catch(e){error(e);}}
function completeDuplicate(job){if(!pendingDuplicate||job.busy||job.kind!=='save'||job.result?.id!==pendingDuplicate.id)return;const pending=pendingDuplicate;pendingDuplicate=null;if(!job.error&&state.profiles.some(p=>p.id===pending.id)&&currentID===pending.previous)loadProfile(pending.id,true);}
function resultObject(r){return JSON.stringify(r,null,2);}
async function poll(){if(offline)return;try{
 state=await api('state');if(first){const remembered=state.selected_profile_id||state.default_profile||'baseline';currentID=state.profiles.some(p=>p.id===remembered)?remembered:'baseline';}const ai=state.application||{};$('application-identity').textContent='Running launcher '+state.version+' | '+(ai.executable||'')+' | Application root: '+(ai.root||'')+' | Game root: '+(state.portable_root||'');$('version').textContent=state.version; if(state.distribution){const d=state.distribution;$('distribution-status').textContent=(d.production_feed_configured?'Signed GitHub feed configured (remote availability is checked on request).':'Production feed not configured. ')+(d.development_trust?'Development signing key; offline test packages only. ':'')+' GitHub primary; Cloudflare R2 optional. '+(d.trusted_revision?'Trusted revision: '+d.trusted_revision+'. ':'')+'Application root: '+d.app_root;}$('data-path').textContent=(state.portable?'Portable root: '+state.portable_root+' | ':'')+'Data: '+state.data_path;
 const source=state.source||{};$('source-status').textContent=(source.version?'Imported source: '+source.edition+' · '+source.version:source.status||'Import a supported owned source.')+' | Effective baseline: 1.40b23';
 const profileList=$('profiles');const old=profileList.value;profileList.replaceChildren();for(const p of state.profiles)profileList.append(option(p.id,p.name+(state.active[p.id]?' · prepared':'')));profileList.value=currentID;
 const runtime=state.runtime_candidates;$('runtime-status').textContent=runtime.length?'Available: '+runtime[0]:'DOSBox not found. Download it or choose your existing executable.';
 const rs=$('runtime-select');const prior=rs.value;rs.replaceChildren();if(!runtime.length)rs.append(option('','None detected'));for(const p of runtime)rs.append(option(p,p));if(runtime.includes(prior))rs.value=prior;
 $('portable-recover').classList.toggle('hidden',!state.portable);$('runtime-path').disabled=!!state.portable&&state.platform==='windows/amd64';$('use-runtime').disabled=!!state.portable&&state.platform==='windows/amd64';$('runtime-requirement').textContent=state.runtime_recipe.requirement;$('install-runtime').disabled=!state.runtime_recipe.supported;
 const kp=(state.network_preflight||{})[currentID];$('kernel-status').textContent=kp?(kp.required?(kp.ok?'Network kernel verified — ':'Network kernel needs repair — ')+kp.kernel_path:'No baseline network-kernel check for this original engine.'):'Prepare this profile to verify its network kernel.';
 $('running').textContent=state.running?(state.running_kind==='network-check'?'Dopefish diagnostic running — read CONNECT/STATUS in DOSBox, then type EXIT. No game is running.':'Game process running for '+state.running+'. Close MOO2 normally before rebuilding or exiting the launcher.'):state.last_exit||'';
 const job=state.job;$('progress').textContent=job.message+(job.busy?' …':'');
 const fingerprint=JSON.stringify([job.kind,job.busy,job.error,job.result]);
 if(fingerprint!==lastResult){lastResult=fingerprint;if(job.result){$('result').textContent=resultObject(job.result);if(job.kind==='game-detect'){const g=$('game-folders');g.replaceChildren(option('','Choose or paste a path below'));for(const f of job.result.folders||[])g.append(option(f,f));if(job.result.folders?.length===1){g.value=job.result.folders[0];$('game-source').value=g.value;}}}if(job.error){$('result').textContent=job.error;error(job.error);}}
 const blocked=job.busy||!!state.running;for(const b of document.querySelectorAll('button'))b.disabled=blocked;
 if(!state.runtime_recipe.supported)$('install-runtime').disabled=true;
 $('check-dopefish').disabled=blocked||!runtime.length;
 if(state.portable&&state.platform==='windows/amd64')$('use-runtime').disabled=true;
 $('launch').disabled=blocked||!state.active[currentID]||!runtime.length||!!(kp&&kp.required&&!kp.ok);
 if(first){first=false;loadProfile(currentID);const addresses=await api('addresses');$('addresses').textContent='This computer’s IPv4 addresses: '+(addresses.join(' · ')||'none detected')+'. Give the reachable LAN address to joining players.';}
 if(!job.busy&&lastJob!==fingerprint){renderHistory();if(job.kind==='activate'&&!job.error)loadProfile(currentID);lastJob=fingerprint;}
 completeDuplicate(job);
 }catch(e){error(e);}}
$('profiles').addEventListener('change',()=>loadProfile($('profiles').value,true));
$('engine').addEventListener('change',()=>{const p=selected();p.mods=[];if(p.engine==='1.50.26')p.core='150';modUI(p);preview();});
$('core').addEventListener('change',preview);$('host').addEventListener('input',preview);$('port').addEventListener('input',preview);$('role').addEventListener('change',()=>{networkUI();preview();});$('network-service').addEventListener('change',()=>{networkUI();preview();});
$('resolve').onclick=preview;
$('save').onclick=()=>action('save',{profile:selected()});
$('prepare').onclick=()=>action('prepare',{profile:selected()});
$('verify').onclick=()=>action('verify',{id:currentID});
$('launch').onclick=()=>action('launch',{profile:selected()});
$('quit').onclick=()=>action('quit');
$('diagnostics').onclick=()=>action('diagnostics',{id:currentID});
$('upstream-check').onclick=()=>action('upstream-check');
$('patch-refresh').onclick=()=>action('patch-refresh');
$('install-runtime').onclick=()=>action('runtime-install');
$('use-runtime').onclick=()=>action('runtime-select',{runtime_path:$('runtime-path').value.trim()||$('runtime-select').value});
$('remove').onclick=()=>{if(confirm('Deactivate this profile? All existing files and saves remain retained.'))action('remove',{id:currentID});};
$('activate').onclick=()=>{const g=$('history').value;if(g&&confirm('Activate this retained generation? Newer saves remain in their own generation and are NOT merged.'))action('activate',{id:currentID,generation:g});};
$('duplicate').onclick=async()=>{if(pendingDuplicate)return;const id=prompt('New profile ID: lowercase letters, digits, hyphens; up to 40 characters');if(!id)return;if(state.profiles.some(p=>p.id===id)){error('That profile ID already exists.');return;}const p=selected();p.id=id;p.name+=' — copy';pendingDuplicate={id,previous:currentID};const accepted=await action('save',{profile:p});if(!accepted)pendingDuplicate=null;};
if(!auth){error('Open the private URL printed by the launcher or stored in data/open-launcher.txt. This page has no authorization token.');}else{poll();setInterval(poll,1500);}

$('import-payload').onclick=()=>action('payload-import',{payload_kind:$('payload-kind').value,source_path:$('payload-path').value.trim()});
$('launcher-check').onclick=()=>action('launcher-check',{offline_path:$('offline-release').value.trim()});
$('launcher-stage').onclick=()=>action('launcher-stage',{offline_path:$('offline-release').value.trim()});

$('detect-game').onclick=()=>action('game-detect');
$('game-folders').onchange=()=>{$('game-source').value=$('game-folders').value;};
$('prepare-play').onclick=()=>action('prepare-play',{profile:selected(),source_path:$('game-source').value.trim()});
$('launcher-apply').onclick=()=>{if(confirm('Close this launcher, apply the staged signed application update, and reopen it? Game profiles and saves are retained.'))action('launcher-apply');};

$("portable-recover").onclick=()=>action("portable-recover");

$('check-dopefish').onclick=()=>{if(confirm('Open a separate DOSBox connection check to moo2.thedopefish.com UDP 213? No game is mounted or launched. Read CONNECT/STATUS, then type EXIT before playing.'))action('check-dopefish');};

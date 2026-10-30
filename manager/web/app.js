'use strict';
const $=id=>document.getElementById(id);
const fragment=location.hash.slice(1);
let auth=fragment,tokenStored=false;
if(fragment){try{sessionStorage.setItem('moo2-token',fragment);tokenStored=true;}catch(_){/* Keep the fragment for reload when storage is blocked. */}}
else{try{auth=sessionStorage.getItem('moo2-token')||'';}catch(_){auth='';}}
if(fragment&&tokenStored){try{history.replaceState(null,'',location.pathname);}catch(_){/* The authenticated page can still work without history access. */}}
for(const b of document.querySelectorAll('button'))b.disabled=true;
let state=null,currentID='baseline',lastResult='',lastJob='',first=true,offline=false;
let selectionQueue=Promise.resolve(),previewRevision=0,pendingDuplicate=null;
let previewFingerprint='',previewProfile='',previewValid=false,requestPending=false,pollPromise=null,connectionFailed=false;
function error(e){$('error').textContent=String(e.message||e);$('error').classList.remove('hidden');}
function clearError(){$('error').classList.add('hidden');}
async function api(path,body){const r=await fetch('/api/'+path,{method:body===undefined?'GET':'POST',headers:{Authorization:'Bearer '+auth,...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});const v=await r.json();if(!r.ok)throw Error(v.error||r.statusText);return v;}
function selected(){return {id:currentID,name:$('name').value.trim(),engine:$('engine').value,core:$('engine').value!=='1.50.26'?'':$('core').value,mods:$('engine').value!=='1.50.26'?[]:[...document.querySelectorAll('#mods input:checked')].map(x=>x.dataset.mod),fullscreen:$('fullscreen').checked,presentation:presentationDraft(),role:$('role').value,network_service:$('role').value==='standalone'?'none':$('network-service').value,host:$('host').value.trim(),port:Number($('port').value)};}
function presentationDraft(){return {window_size:$('window-size').value,mute_audio:$('mute-audio').checked,mute_inactive:$('mute-inactive').checked,pause_inactive:$('pause-inactive').checked};}
function presentationKey(fullscreen,p={}){return JSON.stringify([!!fullscreen,p.window_size||'800x600',!!p.mute_audio,!!p.mute_inactive,!!p.pause_inactive]);}
function presentationDirty(){const p=state?.profiles.find(x=>x.id===currentID);return !!p&&presentationKey($('fullscreen').checked,presentationDraft())!==presentationKey(p.fullscreen,p.presentation);}
function loadPresentation(p){const v=p.presentation||{};$('fullscreen').checked=!!p.fullscreen;$('window-size').value=v.window_size||'800x600';$('mute-audio').checked=!!v.mute_audio;$('mute-inactive').checked=!!v.mute_inactive;$('pause-inactive').checked=!!v.pause_inactive;}
function cancelPresentation(){const p=state?.profiles.find(x=>x.id===currentID);if(p){loadPresentation(p);preview();}}
function resetPresentation(){loadPresentation({fullscreen:false,presentation:{}});preview();}
function acceptPresentationResult(job){
 if(job.busy||job.error||!['presentation-save','save'].includes(job.kind)||!job.result?.id)return;
 const p=state.profiles.find(x=>x.id===job.result.id);if(!p)return;
 // A completed save can race the server's earlier profile-list snapshot.
 // Update saved metadata only; never overwrite newer form edits or navigation.
 p.fullscreen=!!job.result.fullscreen;p.presentation=job.result.presentation||{};
}
function option(value,text){const o=document.createElement('option');o.value=value;o.textContent=text;return o;}
function modUI(p){
 const core=$('core');core.replaceChildren();if(p.engine==='1.50.26'){for(const m of state.catalog.filter(m=>m.group==='Core'))core.append(option(m.id,m.name+' · '+m.version));core.value=p.core||'150';}else{core.append(option('','Not applicable — original '+p.engine+' rules'));core.value='';}core.disabled=p.engine!=='1.50.26';
 const wrap=$('mods');wrap.replaceChildren();for(const m of state.catalog.filter(m=>m.group!=='Core')){
  const row=document.createElement('div');row.className='mod-row';const label=document.createElement('label');const cb=document.createElement('input');cb.type='checkbox';cb.dataset.mod=m.id;cb.checked=(p.mods||[]).includes(m.id);cb.disabled=p.engine!=='1.50.26';const text=document.createElement('span');text.textContent=m.name+(m.group?' · '+m.group:'');label.append(cb,text);const desc=document.createElement('small');desc.textContent=m.description;row.append(label,desc);wrap.append(row);cb.addEventListener('change',preview);
 }
 $('mod-count').textContent=p.engine==='1.50.26'?state.catalog.length+' choices':'Requires 1.50.26';updateCoreDesc();
}
function updateCoreDesc(){const m=state.catalog.find(x=>x.id===$('core').value);$('core-desc').textContent=$('engine').value!=='1.50.26'?'Original executable; community rulesets are unavailable.':m?.description||'';}
function networkUI(){const role=$('role').value,svc=$('network-service').value;const active=role!=='standalone';$('network-service').disabled=!active;$('direct-network-fields').classList.toggle('hidden',!active||svc!=='direct');const rec=(state?.network_services||[]).find(x=>x.id===svc);$('network-service-desc').textContent=!active?'No network tunnel is configured for local play.':(rec?.description||'');if(svc==='dopefish'){$('port').value=213;$('host').value='';}if(svc==='direct'&&Number($('port').value)<1024&&role==='host')$('port').value=21300;}
function loadProfile(id,persist=false){const p=state.profiles.find(x=>x.id===id);if(!p)return;currentID=id;$('profiles').value=id;$('name').value=p.name;$('engine').value=p.engine;$('role').value=p.role;$('network-service').value=(['direct','dopefish'].includes(p.network_service)?p.network_service:'direct');$('port').value=p.port||21300;$('host').value=p.host||'';loadPresentation(p);modUI(p);renderHistory();networkUI();preview();if(persist)selectionQueue=selectionQueue.then(()=>api('selection',{id})).catch(error);}
function renderHistory(){const s=$('history');s.replaceChildren();const entries=state.history[currentID]||[];if(!entries.length)s.append(option('','No prepared environment'));for(const h of entries){s.append(option(h.generation,h.created.slice(0,19)+' · '+h.engine+' / '+(h.core||'original')+(state.active[currentID]===h.generation?' · ACTIVE':'')));}if(state.active[currentID])s.value=state.active[currentID];}
function sourceChoice(){const path=$('game-source').value.trim();$('source-choice').textContent=path?'Selected: '+path:'Using the verified imported copy when available.';$('clear-source').classList.toggle('hidden',!path);updateControls();}
function readiness(){
 if(!state)return {ready:false,message:'Connecting to the local launcher…'};
 if(connectionFailed)return {ready:false,message:'Connection lost. Waiting for a fresh response from the local launcher…'};
 if(!previewValid||previewProfile!==currentID)return {ready:false,message:'Check your selected game version and ruleset before preparing or playing.'};
 if($('game-source').value.trim())return {ready:false,message:'New source selected. Choose Prepare selected game to verify and use it.'};
 const prepared=(state.prepared||{})[currentID];
 if(!state.active[currentID]||!prepared)return {ready:false,message:'Not prepared yet. Choose Prepare selected game above.'};
 if(prepared.fingerprint!==previewFingerprint)return {ready:false,message:'Your game version or rules changed. Prepare again to use this selection.'};
 const runtime=state.runtime_status||{};
 if(!runtime.ready)return {ready:false,message:runtime.message||'Runtime needs attention. Prepare downloads or repairs verified DOSBox.'};
 const kernel=(state.network_preflight||{})[currentID];
 if(kernel&&kernel.required&&!kernel.ok)return {ready:false,message:'An owned network-kernel file is missing or changed. Prepare to repair this game copy.'};
 return {ready:true,message:'Prepared: '+$('name').value+' · '+$('engine').value+($('core').value?' / '+$('core').value:'')+'\nPlay verifies the actual game and runtime files again before starting.'};
}
function updateControls(){
 if(!state)return;
 const blocked=requestPending||state.job.busy||!!state.running||offline||connectionFailed;
 for(const b of document.querySelectorAll('button'))b.disabled=blocked;
 for(const id of ['profiles','name','engine','core','role','network-service','host','port','fullscreen','window-size','mute-audio','mute-inactive','pause-inactive','game-folders','game-source','runtime-select','runtime-path','payload-kind','payload-path','offline-release','history'])$(id).disabled=blocked;
 $('engine').disabled=blocked||!!state.portable&&currentID==='baseline';$('core').disabled=blocked||$('engine').value!=='1.50.26';$('network-service').disabled=blocked||$('role').value==='standalone';
 for(const cb of document.querySelectorAll('#mods input'))cb.disabled=blocked||$('engine').value!=='1.50.26';
 const portableWindows=!!state.portable&&state.platform==='windows/amd64';
 $('runtime-path').disabled=blocked||portableWindows;$('runtime-select').disabled=blocked||portableWindows;$('use-runtime').disabled=blocked||portableWindows;$('browse-runtime-executable').disabled=blocked||portableWindows;
 $('install-runtime').disabled=blocked||!state.runtime_recipe.supported;
 $('import-runtime').disabled=blocked||!$('runtime-source').value.trim();
 $('check-dopefish').disabled=blocked||!(state.runtime_status||{}).ready;
 $('prepare-play').disabled=blocked||!previewValid;$('prepare').disabled=blocked||!previewValid;
 const settingsDirty=presentationDirty();$('save-presentation').disabled=blocked||!settingsDirty;$('cancel-presentation').disabled=blocked||!settingsDirty;
 $('reset-presentation').disabled=blocked||presentationKey($('fullscreen').checked,presentationDraft())===presentationKey(false,{});
 $('pause-inactive').disabled=blocked||$('role').value!=='standalone';
 $('presentation-status').textContent=settingsDirty?'Unsaved changes for this profile. Save settings or Play to remember them.':'Saved for this profile. Changes apply on the next launch.';
 const result=readiness();$('ready-status').textContent=result.message;$('ready-status').classList.toggle('needs-action',!result.ready);$('launch').disabled=blocked||!result.ready;
 const cancelable=state.job.busy&&state.job.cancelable&&!state.job.cancel_requested;
 $('cancel-operation').classList.toggle('hidden',!state.job.busy||!state.job.cancelable);$('cancel-operation').disabled=requestPending||offline||connectionFailed||!cancelable;$('cancel-operation').textContent=state.job.cancel_requested?'Stopping safely…':'Cancel preparation';
}
async function preview(){
 const revision=++previewRevision;previewValid=false;previewFingerprint='';previewProfile='';updateControls();
 if(connectionFailed)return;
 try{updateCoreDesc();const p=selected();const r=await api('resolve',p);if(revision!==previewRevision||p.id!==currentID)return;previewFingerprint=r.fingerprint;previewProfile=p.id;previewValid=true;
 $('resolution').textContent='Resolved: '+(r.mods.map(m=>m.name).join(' + ')||('DOS '+p.engine))+'\nRequired by other selections: '+(r.automatic.join(', ')||'none')+'\nSelection fingerprint: '+r.fingerprint+'\n'+(r.warnings||[]).join('\n');
 const n=r.network||{};$('network-plan').textContent='Selected game: '+p.engine+' / '+(p.engine==='1.50.26'?'ORION150.EXE':'ORION2.EXE')+'\n'+(n.enabled?(n.endpoint+'\n'+n.command):'LOCAL PLAY — no IPX tunnel')+'\n'+(n.instructions||'');
 }catch(e){if(revision!==previewRevision)return;$('resolution').textContent=e.message;$('network-plan').textContent='Selection needs attention: '+e.message;}finally{if(revision===previewRevision)updateControls();}
}
async function action(kind,extra={}){
 if(requestPending||offline||connectionFailed)return;
 requestPending=true;clearError();updateControls();
 try{await selectionQueue;if(connectionFailed||offline)return;const v=await api('action',{action:kind,...extra});if(kind==='quit'||kind==='launcher-apply'){offline=true;$('progress').textContent=kind==='launcher-apply'?'Applying staged update; a new launcher page will open.':'Launcher exited. You may close this tab.';return v;}
 // A poll started before this POST may describe the old job. Drain it first,
 // then request fresh state; no overlapping polls or stale completion enables Play.
 if(pollPromise)await pollPromise;await poll();return v;
 }catch(e){error(e);}finally{requestPending=false;updateControls();}
}
function completeDuplicate(job){if(!pendingDuplicate||job.busy||job.kind!=='save'||job.result?.id!==pendingDuplicate.id)return;const pending=pendingDuplicate;if(job.error){pendingDuplicate=null;return;}if(!state.profiles.some(p=>p.id===pending.id))return;pendingDuplicate=null;if(currentID===pending.previous)loadProfile(pending.id,true);}
function resultObject(r){return JSON.stringify(r,null,2);}
function applyPicker(result){
 if(result.canceled){$('progress').textContent='Selection canceled. Your previous choice is unchanged.';return;}
 const kind=result.picker_kind,path=result.path;if(!path)return;
 if(kind==='game-folder'||kind==='game-zip'){$('game-source').value=path;sourceChoice();}
 if(kind==='runtime-folder'||kind==='runtime-zip')$('runtime-source').value=path;
 if(kind==='runtime-executable')$('runtime-path').value=path;
 if(kind==='payload-archive')$('payload-path').value=path;
 if(kind==='offline-release')$('offline-release').value=path;
}
function poll(){if(offline)return Promise.resolve();if(pollPromise)return pollPromise;pollPromise=readState().finally(()=>{pollPromise=null;});return pollPromise;}
async function readState(){try{
 state=await api('state');if(connectionFailed){connectionFailed=false;clearError();if(!first)preview();}if(first){const remembered=state.selected_profile_id||state.default_profile||'baseline';currentID=state.profiles.some(p=>p.id===remembered)?remembered:'baseline';}
 const ai=state.application||{};$('application-identity').textContent='Launcher '+state.version+' | '+(ai.executable||'')+' | Application root: '+(ai.root||'')+' | Game root: '+(state.portable_root||'');$('version').textContent=state.version;
 if(state.distribution){const d=state.distribution;$('distribution-status').textContent=(d.production_feed_configured?'Signed release feed configured; checked only on request.':'No online launcher update feed configured. ')+(d.development_trust?'Development signing trust; private test packages only. ':'')+'Application root: '+d.app_root;}
 $('data-path').textContent='Data: '+state.data_path;const source=state.source||{};$('source-status').textContent=source.version?'Imported source: '+source.edition+' · '+source.version:source.status||'No imported game yet. Browse to your owned files above.';
 const list=$('profiles');list.replaceChildren();for(const p of state.profiles)list.append(option(p.id,p.name+(state.active[p.id]?' · prepared':'')));list.value=currentID;
 const runtime=state.runtime_candidates||[];$('runtime-status').textContent=(state.runtime_status||{}).message||'Runtime status not yet available.';
 const rs=$('runtime-select'),prior=rs.value;rs.replaceChildren();if(!runtime.length)rs.append(option('','None detected'));for(const p of runtime)rs.append(option(p,p));if(runtime.includes(prior))rs.value=prior;
 $('portable-recover').classList.toggle('hidden',!state.portable);$('runtime-requirement').textContent=state.runtime_recipe.requirement;
 const kp=(state.network_preflight||{})[currentID];$('kernel-status').textContent=kp?(kp.required?(kp.ok?'Network kernel verified — ':'Network kernel needs repair — ')+kp.kernel_path:'No baseline network-kernel check for this engine.'):'Prepare this profile to verify its files.';
 $('running').textContent=state.running?(state.running_kind==='network-check'?'Diagnostic running — read CONNECT / STATUS and close its DOSBox window. No game is running.':'MOO2 process running for '+state.running+'. Close the game normally before preparing or exiting.'):state.last_exit||'';
 const job=state.job;$('progress').textContent=job.message+(job.busy?' …':'');
 const fingerprint=JSON.stringify([job.id,job.kind,job.busy,job.error,job.result,job.cancel_requested]);
 acceptPresentationResult(job);
 if(fingerprint!==lastResult){lastResult=fingerprint;if(job.result){$('result').textContent=resultObject(job.result);
  if(job.kind==='game-detect'){const g=$('game-folders');g.replaceChildren(option('','Choose an installation or Browse above'));for(const f of job.result.folders||[])g.append(option(f,f));if(job.result.folders?.length===1){g.value=job.result.folders[0];$('game-source').value=g.value;sourceChoice();}else if(!job.result.folders?.length){$('progress').textContent='No matching installation found automatically. Use Browse game folder or Browse game ZIP.';}}
  if(job.kind==='browse'&&!job.busy)applyPicker(job.result);
  if(job.kind==='prepare-play'&&!job.busy&&!job.error){$('game-source').value='';sourceChoice();}
 }if(job.error){$('result').textContent=job.error;error(job.error);}}
 if(first){first=false;loadProfile(currentID);const addresses=await api('addresses');$('addresses').textContent='This computer’s IPv4 addresses: '+(addresses.join(' · ')||'none detected')+'. Share a reachable LAN address with joining players.';}
 if(!job.busy&&lastJob!==fingerprint){renderHistory();if(job.kind==='activate'&&!job.error)loadProfile(currentID);lastJob=fingerprint;}
 completeDuplicate(job);updateControls();
 }catch(e){connectionFailed=true;++previewRevision;previewValid=false;previewFingerprint='';previewProfile='';error('The local launcher could not be reached. Waiting to reconnect; open the launcher file again if it has stopped. '+e.message);if(state)updateControls();}
}
$('profiles').addEventListener('change',()=>{if(presentationDirty()&&!confirm('Discard the unsaved display and sound changes for this profile?')){$('profiles').value=currentID;return;}loadProfile($('profiles').value,true);});
$('engine').addEventListener('change',()=>{const p=selected();p.mods=[];if(p.engine==='1.50.26')p.core='150';modUI(p);preview();});
$('name').addEventListener('input',preview);$('core').addEventListener('change',preview);$('host').addEventListener('input',preview);$('port').addEventListener('input',preview);$('role').addEventListener('change',()=>{networkUI();preview();});$('network-service').addEventListener('change',()=>{networkUI();preview();});
$('game-source').addEventListener('input',sourceChoice);$('clear-source').onclick=()=>{$('game-source').value='';$('game-folders').value='';sourceChoice();};
$('resolve').onclick=preview;
$('save').onclick=()=>action('save',{profile:selected()});
$('prepare').onclick=()=>action('prepare-play',{profile:selected(),source_path:$('game-source').value.trim()});
$('verify').onclick=()=>action('verify',{id:currentID});
$('launch').onclick=()=>action('launch',{profile:selected()});
$('quit').onclick=()=>action('quit');
$('cancel-operation').onclick=()=>action('cancel');
$('diagnostics').onclick=()=>action('diagnostics',{id:currentID});
$('upstream-check').onclick=()=>action('upstream-check');
$('patch-refresh').onclick=()=>action('patch-refresh');
$('install-runtime').onclick=()=>action('runtime-install');
$('import-runtime').onclick=()=>action('runtime-import',{source_path:$('runtime-source').value.trim()});
$('use-runtime').onclick=()=>action('runtime-select',{runtime_path:$('runtime-path').value.trim()||$('runtime-select').value});
$('remove').onclick=()=>{if(confirm('Deactivate this profile? All existing files and saves remain retained.'))action('remove',{id:currentID});};
$('activate').onclick=()=>{const g=$('history').value;if(g&&confirm('Activate this retained generation? Newer saves remain in their own generation and are NOT merged.'))action('activate',{id:currentID,generation:g});};
$('duplicate').onclick=async()=>{if(pendingDuplicate)return;const id=prompt('New profile ID: lowercase letters, digits, hyphens; up to 40 characters');if(!id)return;if(state.profiles.some(p=>p.id===id)){error('That profile ID already exists.');return;}const p=selected();p.id=id;p.name+=' — copy';pendingDuplicate={id,previous:currentID};const accepted=await action('save',{profile:p});if(!accepted)pendingDuplicate=null;};
if(!auth){error('This page is missing its private session. Open the launcher file again; it will open the authorized page automatically.');}else{poll();setInterval(poll,1500);}
$('import-payload').onclick=()=>action('payload-import',{payload_kind:$('payload-kind').value,source_path:$('payload-path').value.trim()});
$('launcher-check').onclick=()=>action('launcher-check',{offline_path:$('offline-release').value.trim()});
$('launcher-stage').onclick=()=>action('launcher-stage',{offline_path:$('offline-release').value.trim()});
$('detect-game').onclick=()=>action('game-detect');
$('game-folders').onchange=()=>{$('game-source').value=$('game-folders').value;sourceChoice();};
$('prepare-play').onclick=()=>action('prepare-play',{profile:selected(),source_path:$('game-source').value.trim()});
$('launcher-apply').onclick=()=>{if(confirm('Close this launcher, apply the staged signed update, and reopen it? Game profiles and saves are retained.'))action('launcher-apply');};
$('portable-recover').onclick=()=>action('portable-recover');
$('check-dopefish').onclick=()=>{if(confirm('Open an optional connection check to moo2.thedopefish.com UDP 213? No game is mounted or launched. Read CONNECT / STATUS, then close its DOSBox window.'))action('check-dopefish');};
for(const kind of ['game-folder','game-zip','runtime-folder','runtime-zip','runtime-executable','offline-release'])$('browse-'+kind).onclick=()=>action('browse',{picker_kind:kind});
$('browse-payload').onclick=()=>action('browse',{picker_kind:'payload-archive'});

$('find-after-install').onclick=()=>action('game-detect');
for(const id of ['fullscreen','window-size','mute-audio','mute-inactive','pause-inactive'])$(id).addEventListener('change',preview);
$('save-presentation').onclick=()=>action('presentation-save',{id:currentID,fullscreen:$('fullscreen').checked,presentation:presentationDraft()});
$('cancel-presentation').onclick=cancelPresentation;
$('reset-presentation').onclick=resetPresentation;

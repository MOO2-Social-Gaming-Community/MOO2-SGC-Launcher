'use strict';
let token=location.hash.slice(1);try{token=token||sessionStorage.getItem('moo2-startup-session')||'';if(location.hash)sessionStorage.setItem('moo2-startup-session',token);}catch{}history.replaceState(null,'',location.pathname);
const el=id=>document.getElementById(id);let stopped=false,target='',pollTimer=null;
async function api(path,method='GET'){const r=await fetch('/api/'+path,{method,headers:{Authorization:'Bearer '+token}});if(!r.ok)throw Error(await r.text());return r.json();}
function safeTarget(raw){try{const u=new URL(raw);return u.protocol==='http:'&&u.hostname==='127.0.0.1'&&/^\d+$/.test(u.port)&&!u.username&&!u.password&&!u.search&&u.pathname==='/'&&/^#[a-f0-9]{64}$/.test(u.hash)?u.href:'';}catch{return '';}}
function showError(message){el('error').textContent=message;el('error').hidden=false;}
async function poll(){if(pollTimer!==null)clearTimeout(pollTimer);if(stopped)return;try{const s=await api('status');el('status').textContent=s.message;el('root').textContent='Private installation: '+s.root;el('error').hidden=!s.error;if(s.error)showError(s.error);el('retry').hidden=!s.error;el('recover').hidden=!s.error;el('recover').disabled=s.busy;el('retry').disabled=s.busy;el('exit').disabled=s.busy||!!s.target;target=safeTarget(s.target);el('open').hidden=!target;if(target){stopped=true;location.replace(target);return;}}catch(e){showError('The launcher connection closed. Double-click the same launcher file to reopen it. '+e.message);el('exit').disabled=true;}pollTimer=setTimeout(poll,600);}
el('retry').onclick=async()=>{el('retry').disabled=true;try{await api('retry','POST');await poll();}catch(e){showError(e.message);el('retry').disabled=false;}};
el('recover').onclick=async()=>{if(!confirm('Check whether the interrupted launcher is safely stopped? Recovery never clears live or uncertain process locks.'))return;try{await api('recover','POST');await poll();}catch(e){showError(e.message);}};
el('open').onclick=()=>{if(target)location.replace(target);};
el('exit').onclick=async()=>{try{await api('exit','POST');stopped=true;el('status').textContent='Launcher closed. You may close this tab.';el('exit').disabled=true;el('retry').hidden=true;}catch(e){showError(e.message);}};
if(!/^[a-f0-9]{64}$/.test(token)){stopped=true;showError('This tab is missing its private session. Double-click the launcher file to open it safely.');}else poll();

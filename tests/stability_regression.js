'use strict';
// Real app functions with deterministic DOM/network doubles. Not browser/native acceptance.
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const source=fs.readFileSync(__dirname+'/../manager/web/app.js','utf8');
const html=fs.readFileSync(__dirname+'/../manager/web/index.html','utf8');
function deferred(){let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject};}
function harness(){
 const e={};for(const [,id] of html.matchAll(/id="([^"]+)"/g))e[id]={value:'',checked:false,textContent:'',disabled:false,events:{},classList:{add(){},remove(){},toggle(){}},addEventListener(k,v){this.events[k]=v},replaceChildren(){},append(){}};
 const c={document:{getElementById:id=>{assert(e[id],'missing '+id);return e[id]},querySelectorAll:selector=>selector==='button'?['prepare-play','launch','cancel-operation','quit'].map(id=>e[id]):[],createElement:()=>({})},location:{hash:'',pathname:'/'},sessionStorage:{getItem:()=>''},history:{},confirm:()=>true,setInterval(){throw Error('unexpected unauthenticated polling')}};
 vm.createContext(c);vm.runInContext(source+`\nglobalThis.test={readState,preview,action,readiness,updateControls,setAPI(fn){api=fn},setSelectionQueue(p){selectionQueue=p},setState(s){state=s},setReady(){currentID='community';first=false;previewProfile='community';previewFingerprint='same';previewValid=true;},snapshot(){return {previewRevision,previewValid,previewFingerprint,connectionFailed,requestPending,currentID}}};`,c);
 const profile={id:'community',name:'Community',engine:'1.50.26',core:'150',mods:[],role:'standalone',port:21300,fullscreen:false};
 const base={version:'test',profiles:[profile],catalog:[],active:{community:'g1'},prepared:{community:{fingerprint:'same'}},history:{community:[]},runtime_status:{ready:true},runtime_recipe:{supported:true,requirement:''},network_preflight:{},runtime_candidates:[],job:{id:0,kind:'',busy:false,message:'Ready'}};
 e.name.value=profile.name;e.engine.value=profile.engine;e.core.value=profile.core;e.role.value=profile.role;e.port.value='21300';e['window-size'].value='800x600';
 c.test.setState(base);c.test.setReady();return {e,t:c.test,base};
}
async function identicalRetry(){
 const {e,t,base}=harness();let id=1,posts=0;
 t.setAPI(async(path)=>{if(path==='action'){posts++;id++;return {status:'started'}}if(path==='state')return {...base,job:{id,kind:'prepare-play',busy:false,message:'prepare-play stopped',error:'Choose a supported game source'}};throw Error(path)});
 await t.readState();assert.equal(e.error.textContent,'Choose a supported game source');
 // Action clears the prior error before an immediate identical failure arrives.
 e.error.textContent='';await t.action('prepare-play');assert.equal(posts,1);
 assert.equal(e.error.textContent,'Choose a supported game source','a new operation with identical failure must display its error again');
 // Repeated successful picker result is also a new operation, even with no busy poll.
 t.setAPI(async()=>({...base,job:{id:10,kind:'browse',busy:false,result:{picker_kind:'game-folder',path:'C:\\Owned'}}}));await t.readState();assert.equal(e['game-source'].value,'C:\\Owned');e['game-source'].value='';await t.readState();assert.equal(e['game-source'].value,'','polling the same completed job must not reapply an old choice');
 t.setAPI(async()=>({...base,job:{id:11,kind:'browse',busy:false,result:{picker_kind:'game-folder',path:'C:\\Owned'}}}));await t.readState();assert.equal(e['game-source'].value,'C:\\Owned','identical new picker selection must be reapplied');
}
async function staleResolveAfterDisconnect(){
 const {e,t,base}=harness(),stale=deferred(),fresh=deferred();let connected=false,resolveCalls=0,posts=0;
 t.setAPI(path=>{if(path==='resolve')return (++resolveCalls===1?stale:fresh).promise;if(path==='state')return connected?Promise.resolve(base):Promise.reject(Error('connection lost'));if(path==='action'){posts++;return Promise.resolve({status:'started'})}throw Error(path)});
 const oldPreview=t.preview();const priorRevision=t.snapshot().previewRevision;await t.readState();
 assert.equal(t.snapshot().connectionFailed,true);assert.equal(e.launch.disabled,true);assert.equal(e['prepare-play'].disabled,true,'failed state poll must block preparation');assert.equal(e.quit.disabled,true,'failed state poll must block other mutation controls too');
 assert(t.snapshot().previewRevision>priorRevision,'disconnect must invalidate every outstanding preview');
 stale.resolve({fingerprint:'same',mods:[],automatic:[],warnings:[]});await oldPreview;
 assert.equal(t.snapshot().previewValid,false,'older resolve cannot restore readiness after disconnect');assert.equal(t.readiness().ready,false);assert.equal(e.launch.disabled,true);
 await t.action('launch');assert.equal(posts,0,'disconnected action guard must not send a mutation');
 connected=true;await t.readState();assert.equal(t.snapshot().connectionFailed,false);assert.equal(e.launch.disabled,true,'fresh state alone does not replace fresh profile resolution');
 fresh.resolve({fingerprint:'same',mods:[],automatic:[],warnings:[]});await new Promise(setImmediate);
 assert.equal(resolveCalls,2);assert.equal(t.snapshot().previewValid,true);assert.equal(t.readiness().ready,true);assert.equal(e.launch.disabled,false,'fresh state plus fresh matching resolution permits Play');
}
async function recoveryIgnoresOlderResolve(){
 const {t,base}=harness(),old=deferred(),fresh=deferred();let connected=false,resolveCalls=0;
 t.setAPI(path=>path==='state'?(connected?Promise.resolve(base):Promise.reject(Error('offline'))):(++resolveCalls===1?old:fresh).promise);
 const pending=t.preview();await t.readState();connected=true;await t.readState();
 fresh.resolve({fingerprint:'same',mods:[],automatic:[],warnings:[]});await new Promise(setImmediate);assert.equal(t.readiness().ready,true);
 old.resolve({fingerprint:'obsolete',mods:[],automatic:[],warnings:[]});await pending;assert.equal(t.snapshot().previewFingerprint,'same','pre-disconnect response cannot replace fresh post-reconnect resolution');assert.equal(t.readiness().ready,true);
}
async function disconnectWhileSelectionPending(){
 const {t}=harness(),selection=deferred();let posts=0;
 t.setSelectionQueue(selection.promise);t.setAPI(path=>{if(path==='state')return Promise.reject(Error('connection lost'));if(path==='action'){posts++;return Promise.resolve({status:'started'})}throw Error(path)});
 const action=t.action('launch');await t.readState();selection.resolve();await action;assert.equal(posts,0,'an action waiting for selection must recheck connection before sending');assert.equal(t.snapshot().requestPending,false);
}
(async()=>{const failures=[];for(const fn of [identicalRetry,staleResolveAfterDisconnect,recoveryIgnoresOlderResolve,disconnectWhileSelectionPending]){try{await fn();console.log('PASS:',fn.name)}catch(e){failures.push(fn.name+': '+e.message)}}if(failures.length)throw Error(failures.join('\n'));})().catch(e=>{console.error(e);process.exitCode=1});

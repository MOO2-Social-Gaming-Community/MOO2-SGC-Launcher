'use strict';
// Deterministic JavaScript control-flow checks. Not native browser/UI acceptance.
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const source=fs.readFileSync(__dirname+'/../manager/web/app.js','utf8');
const html=fs.readFileSync(__dirname+'/../manager/web/index.html','utf8');
function extract(name){
 const re=new RegExp('(?:async )?function '+name+'\\(');const match=re.exec(source);assert(match,'missing '+name);
 const start=match.index;const rest=source.slice(start+match[0].length);const end=/\n(?:async )?function /.exec(rest);
 return source.slice(start,end?start+match[0].length+end.index:source.length);
}
function elements(){const e={};for(const id of [...html.matchAll(/id="([^"]+)"/g)].map(x=>x[1]))e[id]={value:'',textContent:'',disabled:false,classList:{toggle(){},add(){},remove(){}}};e.engine.value='1.50.26';e.core.value='150';e.name.value='Community standard';return e;}
function readyContext(){const e=elements();return {e,$:id=>{assert(e[id],'missing DOM '+id);return e[id]},state:{active:{community:'g1'},prepared:{community:{fingerprint:'same'}},runtime_status:{ready:true},network_preflight:{community:{required:true,ok:true}}},currentID:'community',connectionFailed:false,previewValid:true,previewProfile:'community',previewFingerprint:'same'};}
function deferred(){let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject};}
(async()=>{
 const c=readyContext();vm.createContext(c);vm.runInContext(extract('readiness'),c);
 assert.equal(c.readiness().ready,true,'matching actual prepared generation may play');
 c.e['game-source'].value='C:\\Owned\\different.zip';assert.equal(c.readiness().ready,false,'pending source must not claim current game is it');c.e['game-source'].value='';
 c.previewFingerprint='changed';assert.equal(c.readiness().ready,false,'changed version/mods must Prepare');c.previewFingerprint='same';
 c.previewProfile='baseline';assert.equal(c.readiness().ready,false,'old profile preview must not enable new profile');c.previewProfile='community';
 c.previewValid=false;assert.equal(c.readiness().ready,false,'inflight/rejected resolution must disable Play');c.previewValid=true;
 c.state.runtime_status={ready:false,message:'Needs repair'};assert.equal(c.readiness().ready,false);c.state.runtime_status.ready=true;
 c.state.network_preflight.community.ok=false;assert.equal(c.readiness().ready,false);c.state.network_preflight.community.ok=true;
 delete c.state.prepared.community;assert.equal(c.readiness().ready,false,'active pointer alone is not prepared metadata');
 const e=elements();e['game-source'].value='previous';e['runtime-source'].value='prior runtime';let sourceChanged=0;
 const p={$:id=>e[id],sourceChoice:()=>sourceChanged++};vm.createContext(p);vm.runInContext(extract('applyPicker'),p);
 p.applyPicker({picker_kind:'game-folder',canceled:true});assert.equal(e['game-source'].value,'previous');assert.equal(sourceChanged,0);
 p.applyPicker({picker_kind:'game-zip',path:'C:\\Games\\星.zip'});assert.equal(e['game-source'].value,'C:\\Games\\星.zip');assert.equal(sourceChanged,1);
 p.applyPicker({picker_kind:'runtime-folder',path:'D:\\Runtime'});assert.equal(e['runtime-source'].value,'D:\\Runtime');assert.equal(e['game-source'].value,'C:\\Games\\星.zip');
 const requests=[];const pv={connectionFailed:false,previewRevision:0,previewValid:true,previewFingerprint:'old',previewProfile:'community',currentID:'community',state:{},$:id=>e[id],updateControls(){},updateCoreDesc(){},selected:()=>({id:pv.currentID,engine:'1.50.26'}),api:()=>{const d=deferred();requests.push(d);return d.promise}};
 vm.createContext(pv);vm.runInContext(extract('preview'),pv);
 const first=pv.preview();assert.equal(pv.previewValid,false,'invalidate synchronously before resolve');pv.currentID='baseline';const second=pv.preview();
 requests[1].resolve({fingerprint:'newest',mods:[],automatic:[],warnings:[]});await second;requests[0].resolve({fingerprint:'stale',mods:[],automatic:[],warnings:[]});await first;
 assert.equal(pv.previewFingerprint,'newest');assert.equal(pv.previewProfile,'baseline','out-of-order resolve cannot switch readiness');
 const failed=pv.preview();requests[2].reject(Error('conflicting mods'));await failed;assert.equal(pv.previewValid,false);
 let pollCalls=0;const gate=deferred();const pc={offline:false,pollPromise:null,readState:()=>{pollCalls++;return gate.promise}};vm.createContext(pc);vm.runInContext(extract('poll'),pc);
 const one=pc.poll(),two=pc.poll();assert.equal(one,two);assert.equal(pollCalls,1,'polls must not overlap');gate.resolve();await one;assert.equal(pc.pollPromise,null);
 const accepted=deferred();let postCount=0,refreshes=0;const ac={connectionFailed:false,requestPending:false,offline:false,pollPromise:null,selectionQueue:Promise.resolve(),clearError(){},updateControls(){},api:()=>{postCount++;return accepted.promise},poll:async()=>refreshes++,error(e){throw e},$:id=>e[id]};vm.createContext(ac);vm.runInContext(extract('action'),ac);
 const click1=ac.action('prepare-play'),click2=ac.action('prepare-play');await new Promise(setImmediate);assert.equal(postCount,1,'repeated click must not submit twice');accepted.resolve({status:'started'});await click1;await click2;assert.equal(refreshes,1);assert.equal(ac.requestPending,false);
 for(const id of ['prepare','prepare-play'])assert(source.includes("$('"+id+"').onclick=()=>action('prepare-play'"),'both Prepare controls use complete pipeline');
 for(const m of source.matchAll(/\$\(['"]([^'"]+)['"]\)/g))assert(html.includes('id="'+m[1]+'"'),'missing static DOM id '+m[1]);
 console.log('PASS: ready state, pending source, picker cancel/Unicode, stale resolve, conflicts, single-flight poll, repeated-click guard, complete Prepare pipeline, DOM IDs');
})().catch(e=>{console.error(e);process.exitCode=1});

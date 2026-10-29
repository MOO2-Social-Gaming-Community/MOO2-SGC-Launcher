'use strict';
// Focused deterministic event-flow tests; no claim of a native browser run.
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const source=fs.readFileSync(__dirname+'/../manager/web/app.js','utf8');
const start=source.indexOf("$('duplicate').onclick=");
const end=source.indexOf('\nif(!auth)',start);
assert(start>=0&&end>start,'duplicate event handler not found');
const helperStart=source.indexOf('function completeDuplicate(');
const helperEnd=source.indexOf('\nfunction ',helperStart+1);
const helper=helperStart>=0?source.slice(helperStart,helperEnd):'';
async function scenario(accepted=true){
 const button={},ctx={state:{profiles:[{id:'baseline',name:'Baseline'}]},currentID:'baseline',pendingDuplicate:null,prompt:()=> 'copy-test',selected:()=>({id:'baseline',name:'Baseline'}),error:()=>{},action:async()=>accepted?{status:'started'}:undefined,setTimeout:()=>{},poll:async()=>{},$:()=>button,loadProfile:(id)=>{ctx.currentID=id}};
 vm.createContext(ctx);vm.runInContext(helper+'\n'+source.slice(start,end),ctx);
 await button.onclick();return {ctx,button};
}
(async()=>{
 let {ctx}=await scenario(false);assert.equal(ctx.currentID,'baseline','rejected duplicate must retain current profile');assert.equal(ctx.pendingDuplicate,null);
 ({ctx}=await scenario());assert.equal(ctx.currentID,'baseline','accepted save must not switch before completion');
 ctx.completeDuplicate({kind:'save',result:{id:'old-save'},busy:false,error:''});assert(ctx.pendingDuplicate,'older completed save must not consume pending duplicate');ctx.state.profiles.push({id:'copy-test'});ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:true,error:''});assert.equal(ctx.currentID,'baseline');ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:false,error:''});assert.equal(ctx.currentID,'copy-test');
 ({ctx}=await scenario());ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:false,error:'invalid profile'});assert.equal(ctx.currentID,'baseline');assert.equal(ctx.pendingDuplicate,null);
 ({ctx}=await scenario());ctx.state.profiles.push({id:'copy-test'});ctx.currentID='community';ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:false,error:''});assert.equal(ctx.currentID,'community','delayed completion must preserve newer navigation');
 ({ctx}=await scenario());ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:false,error:''});assert.equal(ctx.currentID,'baseline','missing saved profile must not become selected');assert(ctx.pendingDuplicate,'successful result with stale profile list must wait for the next poll');ctx.state.profiles.push({id:'copy-test'});ctx.completeDuplicate({kind:'save',result:{id:'copy-test'},busy:false,error:''});assert.equal(ctx.currentID,'copy-test','later fresh profile list completes the pending selection');
 console.log('PASS: duplicate rejected, pending, successful, failed, newer navigation, and stale-profile-list recovery flows');
})().catch(e=>{console.error(e);process.exitCode=1});

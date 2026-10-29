'use strict';
// Deterministic DOM/event doubles, not native Windows or rendered-browser acceptance.
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const source=fs.readFileSync(__dirname+'/../manager/web/app.js','utf8');
const html=fs.readFileSync(__dirname+'/../manager/web/index.html','utf8');
const tokenPrefix=source.slice(0,source.indexOf("for(const b of document.querySelectorAll('button'))"));
function authCase(fragment,options={}){
 let stripped=false,stored='';
 const c={location:{hash:fragment,pathname:'/'},document:{},sessionStorage:{setItem(k,v){if(options.blockSet)throw Error('SecurityError');stored=v;},getItem(){if(options.blockGet)throw Error('SecurityError');return options.saved||'';}},history:{replaceState(){if(options.blockHistory)throw Error('SecurityError');stripped=true;}}};
 vm.createContext(c);vm.runInContext(tokenPrefix+'\nglobalThis.token=auth;',c);return {token:c.token,stripped,stored};
}
assert.deepEqual(authCase('#secret'),{token:'secret',stripped:true,stored:'secret'});
assert.deepEqual(authCase('#secret',{blockSet:true}),{token:'secret',stripped:false,stored:''},'blocked storage must retain usable token and refresh fragment');
assert.equal(authCase('',{saved:'prior'}).token,'prior');
assert.equal(authCase('',{blockGet:true}).token,'','no token uses visible reopen guidance, not a script exception');
assert.equal(authCase('#secret',{blockHistory:true}).token,'secret');
const ids=[...html.matchAll(/id="([^"]+)"/g)].map(x=>x[1]);assert.equal(new Set(ids).size,ids.length,'duplicate DOM id');
const elements={};for(const id of ids)elements[id]={value:'',checked:false,disabled:false,textContent:'',events:{},addEventListener(n,fn){this.events[n]=fn;},classList:{add(){},remove(){},toggle(){}},replaceChildren(){},append(){}};
let confirmations=[],allowDiscard=false,previews=0,actions=[];
const c={document:{getElementById:id=>{assert(elements[id],'missing '+id);return elements[id]},querySelectorAll:()=>[]},location:{hash:'',pathname:'/'},sessionStorage:{getItem:()=>''},history:{},confirm:message=>{confirmations.push(message);return allowDiscard},setInterval(){throw Error('unauthenticated page must not poll')},fetch(){throw Error('unexpected network')}};
vm.createContext(c);vm.runInContext(source+`\npreview=()=>{globalThis.onPreview()};modUI=()=>{};renderHistory=()=>{};networkUI=()=>{};
globalThis.test={presentationDraft,presentationKey,presentationDirty,loadPresentation,cancelPresentation,resetPresentation,acceptPresentationResult,loadProfile,setState(s){state=s;},setID(id){currentID=id;},getID(){return currentID;},setAction(fn){action=fn;},setAPI(fn){api=fn;}};`,c);
c.onPreview=()=>previews++;
const api=c.test;api.setAction((kind,payload)=>actions.push({kind,payload}));api.setAPI(async()=>({}));
const profiles=[{id:'community',name:'Community',engine:'1.50.26',role:'standalone',fullscreen:true,presentation:{window_size:'large',mute_audio:true,mute_inactive:true,pause_inactive:true}},{id:'baseline',name:'Legacy',engine:'1.40b23',role:'standalone',fullscreen:false}];
api.setState({profiles,history:{}});api.setID('community');api.loadPresentation(profiles[0]);
assert.equal(api.presentationDirty(),false);
api.resetPresentation();assert.equal(elements.fullscreen.checked,false);assert.equal(elements['window-size'].value,'800x600');assert.equal(api.presentationDirty(),true);assert.equal(profiles[0].presentation.window_size,'large','Defaults must not write saved state');
api.cancelPresentation();assert.equal(elements.fullscreen.checked,true);assert.equal(elements['window-size'].value,'large');assert.equal(api.presentationDirty(),false,'Cancel returns to saved settings');
elements['window-size'].value='small';assert.equal(elements['window-size'].events.change.name,'preview');elements.profiles.value='baseline';elements.profiles.events.change();assert.equal(api.getID(),'community');assert.equal(elements.profiles.value,'community');assert.equal(elements['window-size'].value,'small','declining navigation preserves draft');
allowDiscard=true;elements.profiles.value='baseline';elements.profiles.events.change();assert.equal(api.getID(),'baseline');assert.equal(elements['window-size'].value,'800x600','legacy absent preference reads original defaults');assert.equal(elements['mute-audio'].checked,false);
api.setID('community');api.loadPresentation(profiles[0]);elements['window-size'].value='medium';elements['save-presentation'].onclick();assert.equal(actions.at(-1).kind,'presentation-save');assert.equal(actions.at(-1).payload.id,'community');assert.equal(actions.at(-1).payload.presentation.window_size,'medium');assert.equal(actions.at(-1).payload.profile,undefined,'scoped settings save must not submit engine/mod draft');
api.acceptPresentationResult({kind:'presentation-save',error:'disk full',result:{id:'community',fullscreen:false,presentation:{window_size:'medium'}}});assert.equal(profiles[0].presentation.window_size,'large');assert.equal(api.presentationDirty(),true,'save failure keeps draft and dirty status');
// Completion can arrive with stale profile list. Retain response as saved metadata,
// but do not overwrite newer draft edits or navigate back to the saved profile.
elements['window-size'].value='1024x768';api.acceptPresentationResult({kind:'presentation-save',result:{id:'community',fullscreen:true,presentation:{window_size:'medium'}}});assert.equal(profiles[0].presentation.window_size,'medium');assert.equal(elements['window-size'].value,'1024x768');assert.equal(api.presentationDirty(),true);
api.setID('baseline');api.loadPresentation(profiles[1]);api.acceptPresentationResult({kind:'presentation-save',result:{id:'community',fullscreen:true,presentation:{window_size:'large'}}});assert.equal(api.getID(),'baseline');assert.equal(elements['window-size'].value,'800x600');assert.equal(api.presentationDirty(),false);
api.setID('community');api.loadPresentation(profiles[0]);assert.equal(elements['window-size'].value,'large','reopening saved profile restores latest confirmed preferences');
elements['find-after-install'].onclick();assert.equal(actions.at(-1).kind,'game-detect','store handoff returns to actual discovery');
for(const match of html.matchAll(/<a\s+[^>]*href="(https:[^"]+)"[^>]*>/g)){assert(/rel="noopener noreferrer"/.test(match[0]),'external anchor lacks privacy protection');assert(/target="_blank"/.test(match[0]));assert(!match[1].includes('#'));}
for(const expected of ['https://store.steampowered.com/app/410980/Master_of_Orion_2/','https://www.gog.com/en/game/master_of_orion_1_2'])assert(html.includes(expected));
assert(html.includes('GOG-specific editions are not yet validated'),'GOG edition caveat missing');
assert.equal(previews,3);assert.equal(confirmations.length,2);
console.log('PASS: token storage fallback; defaults/Cancel; unsaved navigation; legacy preferences; scoped Save; failed/stale save; profile reopen; actual discovery handoff; safe official links');

import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';
class Element {
 constructor(value=''){this.value=value;this.listeners=new Map();this.dataset={};this.disabled=false;this.textContent='';}
 addEventListener(type,fn){this.listeners.set(type,[...this.listeners.get(type)||[],fn]);}
 async fire(type){await Promise.all((this.listeners.get(type)||[]).map(fn=>fn({preventDefault(){}})));}
 querySelector(selector){return this.children?.[selector];}
}
function setup(status=200){
 const root=new Element(),upload=new Element(),remove=new Element(),dialog=new Element(),window=new Element(),document=new Element(),id=new Element(),name=new Element('My font'),file=new Element(),revision=new Element('3'),uploadMessage=new Element(),deleteMessage=new Element(),uploadButton=new Element(),deleteButton=new Element(),accept=new Element(),cancel=new Element();
 file.files=[{name:'font.woff2',size:100,lastModified:1}];
 upload.action='/presentation/fonts/upload';upload.fields={id,name,file};upload.elements={namedItem:key=>upload.fields[key]};upload.children={'[data-management-feedback]':uploadMessage,'button[type="submit"]':uploadButton};
 remove.action='/presentation/fonts/font:existing/delete';remove.fields={expectedRevision:revision};remove.children={'[data-management-feedback]':deleteMessage,'button[type="submit"]':deleteButton};
 root.children={'[data-presentation-upload]':upload};root.querySelectorAll=()=>[remove];dialog.id='presentation-font-confirm';dialog.children={'[data-font-confirm]':accept,'[data-font-cancel]':cancel};
 document.querySelector=()=>root;document.getElementById=()=>dialog;document.readyState='complete';
 const events=[],requests=[];let count=0,reloads=0;
 window.dispatchEvent=event=>{events.push(event);if(event.type==='modal:close')void dialog.fire('close');};
 class Data {constructor(form){this.values=new Map(Object.entries(form.fields).map(([key,input])=>[key,input.value]));}get(key){return this.values.get(key);}[Symbol.iterator](){return this.values.entries();}}
 runInNewContext(readFileSync(new URL('./presentation-fonts.js',import.meta.url),'utf8'),{document,window,FormData:Data,URLSearchParams,CustomEvent:class{constructor(type,init){this.type=type;this.detail=init?.detail;}},crypto:{randomUUID:()=>`command-${++count}`},location:{reload(){reloads++;}},fetch:async(url,options)=>{requests.push({url,body:options.body});if(status===0&&requests.length===1)throw Error('transport');return {ok:status===200||status===0,json:async()=>({message:'Font in use.',usedBy:['Ocean']})};}});
 return {upload,remove,dialog,accept,cancel,id,name,file,uploadMessage,deleteMessage,uploadButton,deleteButton,events,requests,reloads:()=>reloads};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('unknown upload outcome retries with the same identity and reloads after confirmation',async()=>{
 const s=setup(0);await s.upload.fire('change');const id=s.id.value;
 await s.upload.fire('submit');assert.equal(s.reloads(),0);assert.ok(s.uploadMessage.textContent.includes('Outcome unknown'));assert.equal(s.uploadButton.disabled,false);
 await s.upload.fire('submit');assert.equal(s.requests.length,2);assert.equal(s.requests[0].body.get('id'),id);assert.equal(s.requests[1].body.get('id'),id);assert.equal(s.reloads(),1);
});
test('changing an uncertain upload creates a new immutable font reference',async()=>{
 const s=setup(409);await s.upload.fire('change');const original=s.id.value;await s.upload.fire('submit');s.name.value='Other font';await s.upload.fire('change');assert.notEqual(s.id.value,original);await s.upload.fire('submit');assert.notEqual(s.requests[0].body.get('id'),s.requests[1].body.get('id'));
});
test('oversized font is rejected before upload',async()=>{
 const s=setup();s.file.files[0].size=10485761;await s.upload.fire('submit');assert.equal(s.requests.length,0);assert.equal(s.uploadMessage.textContent,'The file exceeds 10 MiB.');
});
test('font deletion requires the Goshtoso dialog and preserves server usage errors',async()=>{
 const s=setup(409);let deletion=s.remove.fire('submit');await tick();assert.equal(s.events.at(-1).type,'modal:open');await s.cancel.fire('click');await deletion;assert.equal(s.requests.length,0);
 deletion=s.remove.fire('submit');await tick();await s.dialog.fire('close');await deletion;assert.equal(s.requests.length,0);
 deletion=s.remove.fire('submit');await tick();await s.accept.fire('click');await deletion;
 assert.equal(s.requests[0].url,'/presentation/fonts/font:existing/delete');assert.equal(s.requests[0].body.get('expectedRevision'),'3');assert.equal(s.deleteMessage.textContent,'Font in use. Used by Ocean.');assert.equal(s.reloads(),0);assert.equal(s.deleteButton.disabled,false);
});

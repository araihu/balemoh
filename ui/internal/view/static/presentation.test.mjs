import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';
class Element {
 get value(){return this._value;}
 set value(v){this._value=v;if(v===''&&this.files)this.files=[];}
 constructor(value=''){this.value=value;this.listeners=new Map();this.dataset={};this.textContent='';this.disabled=false;this.hidden=false;this.classList={values:new Set(),add(name){this.values.add(name)},remove(name){this.values.delete(name)},contains(name){return this.values.has(name)}};}
 addEventListener(name,callback){const list=this.listeners.get(name)||[];list.push(callback);this.listeners.set(name,list);}
 async fire(type,extra={}){const event={type,target:this,preventDefault(){this.defaultPrevented=true;},...extra};await Promise.all((this.listeners.get(type)||[]).map(fn=>fn(event)));}
 dispatchEvent(event){for(const fn of this.listeners.get(event.type)||[])fn({...event,target:this});return true;}
 hasAttribute(name){return this.attributes?.includes(name)||false;}
 focus(){}
 setAttribute(name,value){this[name]=value;}
 removeAttribute(name){delete this[name];}
 querySelector(selector){return this.children?.[selector]||null;}
 querySelectorAll(){return [];}
}
const source=readFileSync(new URL('./presentation.js',import.meta.url),'utf8');
function setup(status=200,withBrand=false,uploadGate=Promise.resolve(),custom=false){
 const a={id:'builtin:goshtoso',name:'Goshtoso',revision:'',definition:{saved:'a'}},b={id:'builtin:arctic',name:'Arctic',revision:'',definition:{saved:'b'}};
 if(custom){a.id='theme:one';a.revision='4';}
 const deleteAction=new Element(),deleteFeedback=new Element();
 const root=new Element(),editor=new Element(),form=new Element(),dialog=new Element(),window=new Element(),document=new Element(),selection=new Element(a.id),mode=new Element('system'),name=new Element(),accept=new Element(),cancel=new Element(),feedback=new Element(),save=new Element(),base=new Element();dialog.id='presentation-confirm';
 form.elements={namedItem:()=>mode};
 root.dataset.presentationThemes=JSON.stringify({creating:!custom,recovery:custom==='recovery',selectedID:a.id,settingsRevision:'7',catalogRevision:'catalog-v1',items:[a,b],fonts:[]});
 const control={};for(const key of ['new','copy','reload','conflict','inspect'])control[key]=new Element();
 root.children={'[data-presentation-selection]':selection,'[data-presentation-name]':name,'[data-presentation-mode]':mode,'[data-presentation-feedback]':feedback,'[data-presentation-save]':save,'[data-presentation-new]':control.new,'[data-presentation-copy]':control.copy,'[data-presentation-reload]':control.reload,'[data-presentation-conflict]':control.conflict,'[data-presentation-inspect]':control.inspect};
 const brand={resetControl:new Element(),iconResetControl:new Element(),dismiss:new Element(),iconDismiss:new Element(),iconFile:new Element(),iconReset:new Element(),iconFeedback:new Element(),file:new Element(),reset:new Element(),feedback:new Element(),logo:new Element(),icon:new Element(),fallback:new Element()};brand.file.files=[];brand.iconFile.files=[];
 if(withBrand)Object.assign(root.children,{'[data-brand-reset-control]':brand.resetControl,'[data-brand-icon-reset-control]':brand.iconResetControl,'[data-brand-dismiss]':brand.dismiss,'[data-brand-icon-dismiss]':brand.iconDismiss,'[data-brand-icon-file]':brand.iconFile,'[data-brand-icon-reset]':brand.iconReset,'[data-brand-icon-feedback]':brand.iconFeedback,'[data-brand-file]':brand.file,'[data-brand-reset]':brand.reset,'[data-brand-feedback]':brand.feedback,'[data-brand-logo-preview]':brand.logo,'[data-brand-icon-preview]':brand.icon,'[data-brand-default-preview]':brand.fallback});
 const hidden=new Element(JSON.stringify(a.definition));editor.children={'[data-theme-definition]':hidden};dialog.children={'[data-presentation-confirm-accept]':accept,'[data-presentation-confirm-cancel]':cancel,'[data-presentation-confirm-message]':new Element()};
 document.querySelector=selector=>selector==='[data-theme-delete-feedback]'?deleteFeedback:root;document.getElementById=id=>({'theme-primary-delete':deleteAction,'presentation-theme-selection-trigger':base,'presentation-theme-editor':editor,'presentation-save':form,'presentation-confirm':dialog}[id]);document.readyState='complete';
 const events=[],requests=[],loads=[],history=[],assigned=[];let reloads=0,uuid=0;
 const originalDispatch=window.dispatchEvent.bind(window);window.dispatchEvent=event=>{events.push(event);if(event.type==='modal:close')dialog.dispatchEvent({type:'close'});return originalDispatch(event);};editor.addEventListener('themeeditor:load',event=>loads.push(event.detail));
 const context={document,window,JSON,URL,URLSearchParams,FormData:class {append(){}},matchMedia:()=>({matches:false}),Event:class{constructor(type){this.type=type;}},CustomEvent:class{constructor(type,init){this.type=type;this.detail=init?.detail;}},crypto:{randomUUID:()=>`0000000${++uuid}-0000-4000-8000-000000000000`},location:{href:'http://localhost/presentation#themes',origin:'http://localhost',pathname:'/presentation',search:'',reload(){reloads++;},assign(value){assigned.push(value);}},history:{replaceState(...args){history.push(args);}},fetch:async(url,options)=>{if(url==='/presentation/brand/upload'){requests.push({url});await uploadGate;return {ok:status===200,json:async()=>status===200?{id:requests.length===2?'upload:NEWICON':'upload:NEWLOGO'}:{code:'invalid_request',message:'Image rejected'}};}
 requests.push({url,body:options.body instanceof URLSearchParams?Object.fromEntries(options.body):JSON.parse(options.body)});if(status===0&&requests.length===1)throw Error("transport lost");const actual=status===0?200:status;return {ok:actual===200,status:actual,json:async()=>actual===200?{redirect:'/presentation#themes'}:{code:'conflict',message:'Saved state changed',conflict:{kind:'settings',currentRevision:'8'}}};}};
 runInNewContext(source,context);loads.length=0;return {deleteAction,deleteFeedback,window,brand,base,save,selection,name,mode,editor,form,dialog,accept,cancel,feedback,events,requests,loads,history,assigned,reloads:()=>reloads};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('switching themes waits for the Goshtoso dialog; Cancel and Escape keep the draft',async()=>{const s=setup();s.name.value='My draft';await s.name.fire('input');s.selection.value='builtin:arctic';let switching=s.selection.fire('change');await tick();assert.equal(s.events.at(-1).type,'modal:open');assert.equal(s.loads.length,0);await s.cancel.fire('click');await switching;assert.equal(s.selection.value,'builtin:goshtoso');assert.equal(s.name.value,'My draft');s.selection.value='builtin:arctic';switching=s.selection.fire('change');await tick();await s.dialog.fire('close');await switching;assert.equal(s.selection.value,'builtin:goshtoso');assert.equal(s.loads.length,0);});
test('Confirm replaces only the local draft after the modal closes',async()=>{const s=setup();await s.name.fire('input');s.selection.value='builtin:arctic';const switching=s.selection.fire('change');await tick();await s.accept.fire('click');await switching;assert.equal(s.loads.length,1);assert.equal(s.loads[0].definition.saved,'b');assert.equal(s.name.value,'Arctic custom');assert.equal(s.requests.length,0);});
test('successful Save forces a full reload even on the same URL',async()=>{const s=setup();await s.form.fire('submit');assert.equal(s.reloads(),1);assert.equal(s.assigned.length,0);assert.equal(s.history[0][2],'/presentation#themes');assert.equal(s.requests[0].body.expectedSettingsRevision,'7');});
test('conflict preserves the same command, original revision and draft on retry',async()=>{const s=setup(409);s.name.value='Preserved draft';await s.name.fire('input');await s.form.fire('submit');await s.form.fire('submit');assert.equal(s.reloads(),0);assert.equal(s.name.value,'Preserved draft');assert.deepEqual(s.requests[0].body,s.requests[1].body);assert.equal(s.requests[0].body.expectedSettingsRevision,'7');assert.equal(s.feedback.textContent,'Saved state changed');});

test('global mode follows preview until user explicitly inspects another mode',async()=>{const s=setup();s.mode.value='dark';await s.mode.fire('change');assert.equal(s.loads.at(-1).previewMode,'dark');s.editor.dispatchEvent({type:'themeeditor:change',detail:{id:'presentation-theme-editor',definition:{saved:'a'},previewMode:'dark',dirty:false,valid:true}});const count=s.loads.length;s.mode.value='light';await s.mode.fire('change');assert.equal(s.loads.length,count);});

test('unknown transport outcome retains exact command for a successful replay',async()=>{const s=setup(0);s.name.value='Uncertain draft';await s.name.fire('input');await s.form.fire('submit');assert.equal(s.reloads(),0);assert.equal(s.name.value,'Uncertain draft');await s.form.fire('submit');assert.equal(s.reloads(),1);assert.deepEqual(s.requests[0].body,s.requests[1].body);});
test('Save reads the native global mode radio value',async()=>{const s=setup();s.mode.value='dark';await s.mode.fire('change');await s.form.fire('submit');assert.equal(s.requests[0].body.colorMode,'dark');});

test('selecting a logo uploads immediately and blocks Save until the draft is ready',async()=>{
 let finish;const gate=new Promise(resolve=>finish=resolve),s=setup(200,true,gate);
 s.brand.file.files=[{size:100,name:'logo.png'}];const uploading=s.brand.file.fire('change');
 assert.equal(s.requests.length,1);assert.equal(s.requests[0].url,'/presentation/brand/upload');
 assert.equal(s.save.disabled,true);assert.equal(s.brand.reset.disabled,true);assert.equal(s.brand.file.disabled,true);assert.equal(s.base.disabled,true);
 await s.form.fire('submit');assert.equal(s.requests.length,1);
 finish();await uploading;
 assert.equal(s.reloads(),0);assert.equal(s.save.disabled,false);assert.equal(s.brand.reset.disabled,false);assert.equal(s.brand.file.disabled,false);assert.equal(s.base.disabled,false);
 assert.equal(s.brand.file.files.length,0);assert.equal(s.brand.logo.hidden,false);assert.ok(s.brand.logo.src.includes('upload%3ANEWLOGO'));
 await s.form.fire('submit');assert.equal(s.requests[1].body.selection.brandLogoId,'upload:NEWLOGO');assert.equal(s.reloads(),1);
});
test('failed or oversized logo keeps Save blocked until its failed attempt is discarded',async()=>{
 const s=setup(409,true);s.brand.file.files=[{size:2097153,name:'huge.png'}];await s.brand.file.fire('change');assert.equal(s.requests.length,0);assert.ok(s.brand.feedback.textContent.includes('2 MiB'));
 s.brand.file.files=[{size:100,name:'bad.svg'}];await s.brand.file.fire('change');assert.equal(s.reloads(),0);assert.equal(s.save.disabled,true);assert.equal(s.brand.feedback.textContent,'Image rejected');
 await s.form.fire('submit');assert.equal(s.requests.length,1);
 await s.brand.dismiss.fire('click');assert.equal(s.save.disabled,false);assert.equal(s.brand.logo.hidden,true);
 await s.form.fire('submit');assert.equal(s.requests.at(-1).body.selection.brandIconId,'');
});
test('logo errors notify once, mark the field and allow the same file to be selected again',async()=>{
 const s=setup(409,true),notices=()=>s.events.filter(e=>e.type==='notify');
 await s.brand.file.fire('change');assert.equal(notices().length,0);assert.equal(s.requests.length,0);
 s.brand.file.files=[{size:2097153,name:'huge.png'}];await s.brand.file.fire('change');
 assert.equal(notices().length,1);assert.equal(notices()[0].detail.tone,'danger');assert.equal(s.requests.length,0);
 assert.equal(s.brand.file['aria-invalid'],'true');assert.equal(s.brand.feedback.classList.contains('text-danger'),true);assert.equal(s.brand.file.files.length,0);
 const file={size:20,name:'bad.svg'};
 for(let i=0;i<2;i++){s.brand.file.files=[file];await s.brand.file.fire('change');assert.equal(s.brand.file.files.length,0);}
 assert.equal(notices().length,3);assert.equal(s.requests.length,2);assert.equal(notices().at(-1).detail.message,'Image rejected');assert.equal(s.brand.file['aria-invalid'],'true');assert.equal(s.save.disabled,true);
 await s.brand.dismiss.fire('click');assert.equal(s.brand.file['aria-invalid'],undefined);assert.equal(s.brand.feedback.classList.contains('text-danger'),false);assert.equal(s.save.disabled,false);
});
test('successful replacement clears the logo error state without applying the theme',async()=>{
 const s=setup(200,true);s.brand.file.files=[{size:2097153,name:'huge.png'}];await s.brand.file.fire('change');
 s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');
 assert.equal(s.brand.file['aria-invalid'],undefined);assert.equal(s.brand.feedback.classList.contains('text-danger'),false);assert.equal(s.save.disabled,false);assert.equal(s.reloads(),0);
});

test('logo and icon upload/reset independently, with errors blocking Save across both fields',async()=>{
 const s=setup(200,true);s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');
 assert.equal(s.brand.icon.src,'/ui/brand/favicon.svg');
 s.brand.iconFile.files=[{size:20,name:'icon.png'}];await s.brand.iconFile.fire('change');
 assert.ok(s.brand.logo.src.includes('upload%3ANEWLOGO'));assert.ok(s.brand.icon.src.includes('upload%3ANEWICON'));
 await s.brand.reset.fire('click');assert.equal(s.brand.logo.hidden,true);assert.ok(s.brand.icon.src.includes('upload%3ANEWICON'));
 s.brand.file.files=[{size:2097153,name:'huge.png'}];await s.brand.file.fire('change');
 await s.brand.iconReset.fire('click');assert.equal(s.save.disabled,true);assert.equal(s.brand.file['aria-invalid'],'true');assert.equal(s.brand.icon.src,'/ui/brand/favicon.svg');
 s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');assert.equal(s.save.disabled,false);
 await s.form.fire('submit');assert.equal(s.requests.at(-1).body.selection.brandLogoId,'upload:NEWLOGO');assert.equal(s.requests.at(-1).body.selection.brandIconId,'');
});
test('icon upload failures stay on the icon field when a custom logo is restored',async()=>{
 const s=setup(200,true);s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');
 s.brand.iconFile.files=[{size:2097153,name:'huge.png'}];await s.brand.iconFile.fire('change');
 assert.equal(s.brand.iconFile['aria-invalid'],'true');assert.equal(s.brand.file['aria-invalid'],undefined);assert.equal(s.events.filter(e=>e.type==='notify').at(-1).detail.title,'Icon upload failed');
 await s.brand.reset.fire('click');assert.equal(s.save.disabled,true);assert.ok(s.brand.iconFeedback.textContent.includes('2 MiB'));
 await s.brand.iconDismiss.fire('click');assert.equal(s.save.disabled,false);assert.equal(s.brand.iconFile['aria-invalid'],undefined);
});
test('new drafts have no deletion flow',async()=>{const s=setup();await s.window.fire('presentation-delete-theme');assert.equal(s.requests.length,0);assert.equal(s.events.length,0);});
test('deleting a saved theme waits for confirmation; Cancel and Escape preserve the draft',async()=>{
 const s=setup(200,false,Promise.resolve(),true);s.name.value='Unsaved name';await s.name.fire('input');
 for(const close of [()=>s.cancel.fire('click'),()=>s.dialog.fire('close')]){
  const deleting=s.window.fire('presentation-delete-theme');await tick();assert.equal(s.events.at(-1).type,'modal:open');await close();await deleting;
  assert.equal(s.requests.length,0);assert.equal(s.name.value,'Unsaved name');assert.equal(s.assigned.length,0);
 }
});
for(const recovery of [false,true])test(`confirmed theme deletion navigates to saved themes, recovery=${recovery}`,async()=>{
 const s=setup(200,false,Promise.resolve(),recovery?'recovery':true);s.name.value='Unsaved name';await s.name.fire('input');
 const deleting=s.window.fire('presentation-delete-theme');await tick();await s.accept.fire('click');await deleting;
 assert.equal(s.requests[0].url,'/presentation/themes/theme%3Aone/delete');assert.deepEqual(s.requests[0].body,{expectedRevision:'4'});
 assert.equal(s.assigned[0],recovery?'/presentation?appearance=recovery#themes':'/presentation#themes');
 let prevented=false;await s.window.fire('beforeunload',{preventDefault(){prevented=true;}});assert.equal(prevented,false);
});
test('deletion conflict preserves draft, selected revision and header feedback',async()=>{
 const s=setup(409,false,Promise.resolve(),true);s.name.value='Keep my draft';await s.name.fire('input');
 for(let i=0;i<2;i++){const deleting=s.window.fire('presentation-delete-theme');await tick();await s.accept.fire('click');await deleting;}
 assert.equal(s.assigned.length,0);assert.equal(s.name.value,'Keep my draft');assert.equal(s.deleteAction.disabled,false);assert.equal(s.deleteFeedback.textContent,'Saved state changed');assert.deepEqual(s.requests[0],s.requests[1]);
});

test('reset X controls follow only their own custom image reference',async()=>{
 const s=setup(200,true);assert.equal(s.brand.resetControl.hidden,true);assert.equal(s.brand.iconResetControl.hidden,true);
 s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');assert.equal(s.brand.resetControl.hidden,false);assert.equal(s.brand.iconResetControl.hidden,true);
 s.brand.iconFile.files=[{size:20,name:'icon.png'}];await s.brand.iconFile.fire('change');assert.equal(s.brand.iconResetControl.hidden,false);
 await s.brand.reset.fire('click');assert.equal(s.brand.resetControl.hidden,true);assert.equal(s.brand.iconResetControl.hidden,false);
 await s.brand.iconReset.fire('click');assert.equal(s.brand.iconResetControl.hidden,true);
});
test('default-image upload failure keeps X hidden and can be dismissed without a replacement',async()=>{
 const s=setup(409,true);s.brand.file.files=[{size:20,name:'bad.svg'}];await s.brand.file.fire('change');
 assert.equal(s.brand.resetControl.hidden,true);assert.equal(s.brand.dismiss.hidden,false);assert.equal(s.save.disabled,true);
 await s.brand.dismiss.fire('click');assert.equal(s.brand.resetControl.hidden,true);assert.equal(s.brand.dismiss.hidden,true);assert.equal(s.save.disabled,false);assert.equal(s.brand.file['aria-invalid'],undefined);
});
test('discarding a failed replacement preserves the previous custom image and its X',async()=>{
 const s=setup(200,true);s.brand.file.files=[{size:20,name:'logo.png'}];await s.brand.file.fire('change');const previous=s.brand.logo.src;
 s.brand.file.files=[{size:2097153,name:'huge.png'}];await s.brand.file.fire('change');assert.equal(s.save.disabled,true);
 await s.brand.dismiss.fire('click');assert.equal(s.brand.logo.src,previous);assert.equal(s.brand.resetControl.hidden,false);assert.equal(s.save.disabled,false);
});

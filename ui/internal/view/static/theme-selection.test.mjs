import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';
class Element {
 constructor(value=''){this.value=value;this.dataset={};this.listeners=new Map();this.hidden=false;this.disabled=false;}
 focus(){this.focused=true;}
 addEventListener(type,fn){this.listeners.set(type,[...this.listeners.get(type)||[],fn]);}
 async fire(type,extra={}){for(const fn of this.listeners.get(type)||[])await fn({preventDefault(){},target:this,...extra});}
 querySelector(selector){return this.children[selector];}
}
const source=readFileSync(new URL('./theme-selection.js',import.meta.url),'utf8');
function setup(status=200){
 const builtin={id:'builtin:goshtoso',revision:'',definition:{name:'builtin'}}, custom={id:'theme:one',revision:'4',definition:{name:'custom'},brandLogoId:'upload:LOGO',brandIconId:'upload:ICON'};
 const root=new Element(),form=new Element(),selection=new Element(builtin.id),mode=new Element('system'),frame=new Element(),window=new Element(),document=new Element(),system=new Element(),feedback=new Element(),apply=new Element(),edit=new Element(),editLink=new Element(),create=new Element(),createLink=new Element(),reload=new Element();
 edit.children={'a':editLink};create.children={'a':createLink};frame.id='presentation-theme-preview';form.elements={namedItem:()=>mode};
 const brandMessages=[],messages=[];frame.contentWindow={postMessage:(data,origin)=>{if(data.type==='goshtoso:theme-state')messages.push({data,origin});else brandMessages.push(data);}};system.matches=false;
 root.dataset.themeSelection=JSON.stringify({selectedID:builtin.id,settingsRevision:'7',catalogRevision:'catalog',recovery:true,items:[builtin,custom]});
 root.children={'[data-theme-apply]':form,'[data-presentation-selection]':selection,'[data-presentation-mode]':mode,'[data-theme-selection-feedback]':feedback,'[data-theme-apply-button]':apply,'[data-theme-edit]':edit,'[data-theme-create]':create,'[data-theme-create] a':createLink,'[data-theme-selection-reload]':reload};
 document.readyState='complete';document.querySelector=()=>root;document.getElementById=()=>frame;
 let reloads=0,uuid=0;const requests=[],history=[],assigned=[];
 apply.disabled=true;
 runInNewContext(source,{document,window,JSON,URL,matchMedia:()=>system,crypto:{randomUUID:()=>`command-${++uuid}`},location:{origin:'http://localhost',reload(){reloads++;},assign(url){assigned.push(url);}},history:{replaceState(...args){history.push(args);}},fetch:async(url,options)=>{requests.push({url,body:JSON.parse(options.body)});if(status===0&&requests.length===1)throw Error('Transport lost');return {ok:status===200||status===0,json:async()=>({code:'conflict',message:'Saved settings changed'})};}});
 assert.equal(apply.disabled,false);
 return {brandMessages,form,selection,mode,frame,window,system,feedback,apply,edit,editLink,create,createLink,reload,root,messages,requests,history,assigned,reloads:()=>reloads};
}
test('browsing saved themes and modes only previews; editor links carry selected theme and recovery',async()=>{
 const s=setup();assert.equal(s.requests.length,0);assert.equal(s.edit.hidden,true);assert.equal(s.create.hidden,false);assert.ok(s.createLink.href.includes('new=1'));
 s.selection.value='theme:one';await s.selection.fire('change');s.mode.value='dark';await s.mode.fire('change');
 assert.equal(s.requests.length,0);assert.equal(s.edit.hidden,false);assert.equal(s.create.hidden,true);assert.ok(s.editLink.href.includes('themeID=theme%3Aone'));assert.ok(s.editLink.href.includes('appearance=recovery'));assert.ok(!s.editLink.href.includes('new=1'));
 assert.equal(s.messages.at(-1).data.definition.name,'custom');assert.equal(s.messages.at(-1).data.previewMode,'dark');assert.equal(s.brandMessages.at(-1).logoID,'upload:LOGO');assert.equal(s.brandMessages.at(-1).iconID,'upload:ICON');
});
test('preview ready handshake rejects another origin, frame and instance',async()=>{
 const s=setup(),data={type:'goshtoso:theme-ready',version:1,instanceId:s.frame.id},before=s.messages.length;
 await s.window.fire('message',{origin:'http://evil.test',source:s.frame.contentWindow,data});
 await s.window.fire('message',{origin:'http://localhost',source:{},data});
 await s.window.fire('message',{origin:'http://localhost',source:s.frame.contentWindow,data:{...data,instanceId:'other'}});
 assert.equal(s.messages.length,before);
 await s.window.fire('message',{origin:'http://localhost',source:s.frame.contentWindow,data});assert.equal(s.messages.length,before+1);
});
test('Apply uses saved identity and revision without creating or editing a definition',async()=>{
 const s=setup();s.selection.value='theme:one';await s.form.fire('submit');const payload=s.requests[0].body;
 assert.deepEqual(payload.selection,{kind:'custom',id:'theme:one',expectedThemeRevision:'4'});assert.equal(payload.expectedSettingsRevision,'7');assert.equal(s.reloads(),1);assert.equal(s.history[0][2],'/presentation#themes');
});
for(const status of [409,0])test(`Apply retains exact command on conflict or uncertain response: ${status}`,async()=>{
 const s=setup(status);await s.form.fire('submit');assert.equal(s.reloads(),0);assert.equal(s.selection.value,'builtin:goshtoso');assert.equal(s.apply.disabled,false);assert.equal(s.reload.hidden,false);
 await s.form.fire('submit');assert.deepEqual(s.requests[0].body,s.requests[1].body);assert.equal(s.requests[0].body.selection.kind,'builtin');assert.equal(s.requests[0].body.selection.expectedCatalogRevision,'catalog');
});

test('secondary Create opens an independent copy of the currently selected custom theme',async()=>{
 const s=setup();s.selection.value='theme:one';await s.selection.fire('change');await s.root.fire('theme-create');
 const url=new URL(s.assigned[0],'http://localhost');assert.equal(url.pathname,'/presentation/themes/editor');assert.equal(url.searchParams.get('themeID'),'theme:one');assert.equal(url.searchParams.get('new'),'1');assert.equal(url.searchParams.get('appearance'),'recovery');assert.equal(s.requests.length,0);
});

test('apply failures focus stable feedback and retain selection',async()=>{const s=setup(409);s.selection.value='theme:one';await s.form.fire('submit');assert.equal(s.feedback.focused,true);assert.equal(s.selection.value,'theme:one');assert.equal(s.reloads(),0);});

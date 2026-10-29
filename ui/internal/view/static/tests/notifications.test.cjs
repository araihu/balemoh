const {test} = require('node:test');
const assert = require('node:assert/strict');
const {createScheduler} = require('../notifications.js');
function harness(load) {
 let hidden=false, next=0;
 const timers=new Map();
 const scheduler=createScheduler({load,hidden:()=>hidden,setTimer:(fn,ms)=>{timers.set(++next,{fn,ms});return next;},clearTimer:id=>timers.delete(id)});
 return {scheduler,timers,setHidden(value){hidden=value;scheduler.visibilityChanged();},async fire(){const [id,timer]=timers.entries().next().value;timers.delete(id);await timer.fn();},delay(){assert.equal(timers.size,1);return [...timers.values()][0].ms;}};
}
test('refresh backs off on failures and resets after recovery',async()=>{
 let succeeds=true,calls=0;
 const h=harness(async()=>{calls++;return succeeds;});
 await h.scheduler.refresh();assert.equal(h.delay(),15000);
 succeeds=false;await h.fire();assert.equal(h.delay(),30000);
 await h.fire();assert.equal(h.delay(),60000);
 await h.fire();assert.equal(h.delay(),60000);
 succeeds=true;await h.fire();assert.equal(h.delay(),15000);assert.equal(calls,5);
 h.scheduler.stop();assert.equal(h.timers.size,0);
});
test('hidden documents pause and visibility resumes without concurrent requests',async()=>{
 let calls=0,release;
 const h=harness(()=>{calls++;return new Promise(resolve=>{release=resolve;});});
 const first=h.scheduler.refresh();assert.equal(calls,1);
 h.setHidden(true);assert.equal(h.timers.size,0);
 h.setHidden(false);h.scheduler.refresh();assert.equal(calls,1);
 release(true);await first;assert.equal(calls,2);
 const secondRelease=release;h.setHidden(true);secondRelease(true);
 await new Promise(resolve=>setImmediate(resolve));assert.equal(h.timers.size,0);
 h.setHidden(false);assert.equal(calls,3);h.scheduler.stop();release(true);
 await new Promise(resolve=>setImmediate(resolve));assert.equal(h.timers.size,0);
});
test('rejections back off and initial hidden state never invokes the client',async()=>{
 let calls=0;const h=harness(async()=>{calls++;throw Error('offline');});
 h.setHidden(true);await h.scheduler.refresh();assert.equal(calls,0);
 h.setHidden(false);await new Promise(resolve=>setImmediate(resolve));
 assert.equal(calls,1);assert.equal(h.delay(),30000);h.scheduler.stop();
});

const fs = require('node:fs');
const vm = require('node:vm');
const runtime = fs.readFileSync(require.resolve('../notifications.js'), 'utf8');
const preferenceKey = 'balemoh.notifications.unreadOnly';
function browserHarness(storage, browser) {
 const events = new Map(), requests = [], links = [{}, {}];
 let observe;
 function element() {
  const attrs = new Map(), listeners = new Map();
  return {checked:false, addEventListener:(name,fn)=>listeners.set(name,fn),
   setAttribute:(name,value)=>attrs.set(name,value), getAttribute:name=>attrs.get(name),
   querySelector:()=>null, change(){listeners.get('change')?.();}};
 }
 const trigger=element(), summary=element(), tray=element(), filter=element();
 summary.querySelector=()=>({dataset:{notificationAvailable:'true',notificationCount:'0'}});
 const inboxLinks=links.map(attrs=>({setAttribute:(name,value)=>{attrs[name]=value;}}));
 const elements={'notification-trigger':trigger,'notification-summary':summary,'notification-tray':tray,'notification-only-unread':filter};
 const document={readyState:'complete',hidden:false,
  getElementById:id=>elements[id], querySelector:()=>null,
  querySelectorAll:selector=>selector==='[data-notification-inbox-link]'?inboxLinks:[],
  addEventListener(name,fn){if(!events.has(name))events.set(name,new Set());events.get(name).add(fn);},
  removeEventListener:(name,fn)=>events.get(name)?.delete(fn)};
 const context={document,BalemohBrowserNotifications:browser,location:{pathname:'/',search:''},URLSearchParams,
  setTimeout:()=>1,clearTimeout:()=>{},addEventListener:()=>{},
  MutationObserver:class {constructor(fn){observe=fn;}observe(){}disconnect(){}},
  htmx:{async ajax(method,url,{source}){
   requests.push({method,url});
   for(const fn of events.get('htmx:finally:request')||[])fn({detail:{ctx:{sourceElement:source,response:{status:200}}}});
  }}};
 Object.defineProperty(context,'localStorage',{get:()=>typeof storage==='function'?storage():storage});
 vm.runInNewContext(runtime,context);
 const settle=()=>new Promise(resolve=>setImmediate(resolve));
 return {filter,links,requests,async visibility(hidden){document.hidden=hidden;for(const fn of events.get('visibilitychange')||[])fn();await settle();},async open(){trigger.setAttribute('aria-expanded','true');observe();await settle();},
  async change(checked){filter.checked=checked;filter.change();await settle();}};
}
function memoryStorage(value) {
 const values=new Map(value===undefined?[]:[[preferenceKey,value]]);
 return {getItem:key=>values.get(key)??null,setItem:(key,value)=>values.set(key,value)};
}
test('popover remembers both filter values across page loads and restores inbox links',async()=>{
 const storage=memoryStorage();
 const first=browserHarness(storage);await first.open();
 assert.equal(first.filter.checked,false);assert.equal(first.requests.at(-1).url,'/notifications/tray');
 await first.change(true);assert.equal(storage.getItem(preferenceKey),'true');
 const restored=browserHarness(storage);
 assert.equal(restored.filter.checked,true);
 assert.ok(restored.links.every(link=>link.href==='/notifications?unread=true'));
 await restored.open();assert.equal(restored.requests.at(-1).url,'/notifications/tray?unread=true');
 await restored.change(false);assert.equal(storage.getItem(preferenceKey),'false');
 assert.ok(restored.links.every(link=>link.href==='/notifications'));
 const disabled=browserHarness(storage);await disabled.open();
 assert.equal(disabled.filter.checked,false);assert.equal(disabled.requests.at(-1).url,'/notifications/tray');
 assert.ok(first.requests.concat(restored.requests,disabled.requests).every(request=>request.method==='GET'));
});
test('invalid stored filter defaults off and blocked storage never disables filtering',async()=>{
 const invalid=browserHarness(memoryStorage('invalid'));assert.equal(invalid.filter.checked,false);
 const blocked=()=>{throw Error('storage unavailable');};
 for(const storage of [blocked,{getItem:blocked,setItem:blocked},{getItem:()=> 'true',setItem:blocked}]){
  const browser=browserHarness(storage);await browser.open();
  await browser.change(true);
  assert.equal(browser.filter.checked,true);
  assert.equal(browser.requests.at(-1).url,'/notifications/tray?unread=true');
  assert.ok(browser.links.every(link=>link.href==='/notifications?unread=true'));
  await browser.change(false);
  assert.equal(browser.requests.at(-1).url,'/notifications/tray');
  assert.ok(browser.links.every(link=>link.href==='/notifications'));
 }
});

const {test}=require('node:test');
const assert=require('node:assert/strict');
const {create}=require('../browser-notifications.js');
const fs=require('node:fs'),vm=require('node:vm');
const tokenKey='balemoh.notifications.pushToken';
function harness({permission='granted',enabled=true,storage=new Map(),subscription=null,subscribed=false}={}){
 const events={},requests=[],status={},button={setAttribute(name,value){this[name]=value;}};
 let asks=0,subscribes=0,unsubscribes=0,serverSubscribed=subscribed,failPost=false,failDelete=false;
 const key=Uint8Array.from([4,1,2,3]);const publicKey=Buffer.from(key).toString('base64url');
 function makeSubscription(){return {options:{applicationServerKey:key.buffer},toJSON:()=>({endpoint:'https://push.example/device',expirationTime:null,keys:{p256dh:'key',auth:'auth'}}),async unsubscribe(){unsubscribes++;subscription=null;return true;}};}
 if(subscription===true)subscription=makeSubscription();
 const worker={pushManager:{async getSubscription(){return subscription;},async subscribe(options){assert.equal(options.userVisibleOnly,true);subscribes++;subscription=makeSubscription();return subscription;}}};
 const env={isSecureContext:true,PushManager:function(){},crypto:require('node:crypto').webcrypto,setTimeout,clearTimeout,
  atob:s=>Buffer.from(s,'base64').toString('binary'),btoa:s=>Buffer.from(s,'binary').toString('base64'),
  Notification:{permission,async requestPermission(){asks++;return this.permission;}},
  navigator:{serviceWorker:{async register(){},ready:Promise.resolve(worker)}},
  localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value)},
  document:{querySelectorAll:()=>[{querySelector:selector=>selector.endsWith('toggle]')?button:status}],addEventListener(name,fn){events[name]=fn;}},
  addEventListener(name,fn){events[name]=fn;},
  async fetch(url,options){requests.push({url,...options});const method=options.method;
   if(method==='POST'){if(failPost)return {ok:false};serverSubscribed=true;}
   if(method==='DELETE'){if(failDelete)return {ok:false};serverSubscribed=false;}
   return {ok:true,json:async()=>({enabled,publicKey,subscribed:serverSubscribed})};
  }};
 const controller=create(env);
 return {env,controller,storage,requests,status,button,events,worker,counts:()=>({asks,subscribes,unsubscribes}),failPost(value){failPost=value;},failDelete(value){failDelete=value;}};
}
test('explicit opt-in subscribes and persists; reopening renews without asking again',async()=>{
 const h=harness();await h.controller.ready;assert.equal(h.controller.active(),false);assert.equal(h.counts().asks,0);
 await h.controller.toggle();assert.equal(h.controller.active(),true);assert.equal(h.storage.get(tokenKey).length,43);
 assert.equal(h.counts().asks,1);assert.equal(h.counts().subscribes,1);
 assert.match(h.status.textContent,/tab is closed/);
 const post=h.requests.find(r=>r.method==='POST');assert.equal(JSON.parse(post.body).token,h.storage.get(tokenKey));assert.equal('expirationTime' in JSON.parse(post.body).subscription,false);
 const restored=harness({storage:h.storage,subscription:true,subscribed:true});await restored.controller.ready;
 assert.equal(restored.controller.active(),true);assert.equal(restored.counts().asks,0);assert.equal(restored.counts().subscribes,0);
 assert.equal(restored.requests.filter(r=>r.method==='POST').length,1);
 await restored.controller.toggle();assert.equal(restored.controller.active(),false);assert.equal(restored.counts().unsubscribes,1);
});
test('denial, disabled server, unsupported browser and storage failure stay off',async()=>{
 const denied=harness({permission:'denied'});await denied.controller.ready;assert.equal(denied.button.disabled,true);
 const disabled=harness({enabled:false});await disabled.controller.ready;assert.equal(disabled.button.disabled,true);assert.match(disabled.status.textContent,/not configured/);
 const unsupported=harness();delete unsupported.env.PushManager;await unsupported.controller.ready;unsupported.controller.sync();assert.equal(unsupported.button.disabled,true);
 const blocked=harness();await blocked.controller.ready;Object.defineProperty(blocked.env,'localStorage',{get(){throw Error('blocked');}});
 await blocked.controller.toggle();assert.equal(blocked.counts().asks,0);assert.equal(blocked.counts().subscribes,0);assert.equal(blocked.controller.active(),false);
 const dismissed=harness({permission:'default'});await dismissed.controller.ready;await dismissed.controller.toggle();assert.equal(dismissed.counts().subscribes,0);
});
test('failed server persistence unsubscribes; failed cleanup can retry without resubscribing',async()=>{
 const h=harness();await h.controller.ready;h.failPost(true);await h.controller.toggle();
 assert.equal(h.controller.active(),false);assert.equal(h.counts().unsubscribes,1);
 h.failPost(false);await h.controller.toggle();assert.equal(h.controller.active(),true);
 h.failDelete(true);await h.controller.toggle();assert.equal(h.controller.active(),false);assert.match(h.button.textContent,/Retry disabling/);
 const subscriptions=h.counts().subscribes;h.failDelete(false);await h.controller.toggle();
 assert.equal(h.counts().subscribes,subscriptions);assert.equal(h.button.textContent,'Enable browser notifications');
});
test('revoked permission still permits removing the registered subscription',async()=>{
 const h=harness();await h.controller.ready;await h.controller.toggle();h.env.Notification.permission='denied';h.controller.sync();
 assert.equal(h.button.disabled,false);await h.controller.toggle();assert.equal(h.counts().unsubscribes,1);
});
test('push displays without any tab; malformed payload remains user-visible',async()=>{
 const handlers={},shown=[];
 const self={addEventListener(name,fn){handlers[name]=fn;},registration:{async showNotification(title,options){shown.push({title,options});}}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../notifications-worker.js'),'utf8'),{self,URL});
 let completion;
 handlers.push({data:{json:()=>({id:'one',title:'Backup failed',body:'Details'})},waitUntil(promise){completion=promise;}});await completion;
 assert.equal(shown[0].title,'Backup failed');assert.equal(shown[0].options.tag,'balemoh-one');
 handlers.push({data:{json(){throw Error('bad payload');}},waitUntil(promise){completion=promise;}});await completion;
 assert.equal(shown[1].title,'Balemoh');
});
test('notification click opens trusted inbox with no open tabs',async()=>{
 const handlers={};let destination,completion,closed=false;
 const self={location:{origin:'https://balemoh.example'},addEventListener(name,fn){handlers[name]=fn;},clients:{async matchAll(){return [];},async openWindow(url){destination=url;}}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../notifications-worker.js'),'utf8'),{self,URL});
 handlers.notificationclick({notification:{data:{url:'https://untrusted.example'},close(){closed=true;}},waitUntil(promise){completion=promise;}});await completion;
 assert.equal(destination,'https://balemoh.example/notifications');assert.equal(closed,true);
});

test('worker upgrade activates without waiting for an open old tab to close',async()=>{
 const handlers={};let skipped=false,completion;
 const self={addEventListener(name,fn){handlers[name]=fn;},async skipWaiting(){skipped=true;}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../notifications-worker.js'),'utf8'),{self,URL});
 handlers.install({waitUntil(promise){completion=promise;}});await completion;assert.equal(skipped,true);
});

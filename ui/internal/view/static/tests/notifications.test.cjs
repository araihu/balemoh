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

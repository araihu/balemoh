import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';
const source=readFileSync(new URL('./appearance.js',import.meta.url),'utf8');
test('font failure falls back after shell initialization without changing saved data',async()=>{
 const saved={id:'font:one',source:'upload',url:'/fonts/one',families:['Example']};
 const data={textContent:JSON.stringify([saved])},warning={hidden:true},classes=new Set(),listeners=new Map(),media={matches:true,addEventListener(name,handler){listeners.set('media:'+name,handler);}},root={dataset:{theme:'custom'},classList:{toggle(name,on){on?classes.add(name):classes.delete(name);}}};
 const document={documentElement:root,getElementById:id=>id==='balemoh-font-watch'?data:warning,fonts:{load:()=>Promise.reject(Error('unavailable'))},addEventListener:(name,handler)=>listeners.set(name,handler)};
 runInNewContext(source,{document,matchMedia:()=>media,JSON});await new Promise(resolve=>setImmediate(resolve));
 assert.equal(root.dataset.theme,'goshtoso');assert.equal(classes.has('dark'),true);assert.equal(warning.hidden,false);
 root.dataset.theme='custom';listeners.get('alpine:initialized')();assert.equal(root.dataset.theme,'goshtoso');
 media.matches=false;listeners.get('media:change')();assert.equal(classes.has('dark'),false);assert.deepEqual(JSON.parse(data.textContent),[saved]);
});

import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import assert from 'node:assert/strict';
import test from 'node:test';
test('real card links and refresh forms remain local in preview',()=>{
 const handlers=new Map();
 runInNewContext(readFileSync(new URL('./theme-preview.js',import.meta.url),'utf8'),{window:{addEventListener(){}},document:{addEventListener(type,handler,capture){assert.equal(capture,true);handlers.set(type,handler);}}});
 for(const [type,anchor,expected] of [['click',true,true],['click',false,false],['submit',false,true]]){
  let prevented=false,stopped=false;
  handlers.get(type)({target:{closest:()=>anchor?{}:null},preventDefault(){prevented=true;},stopImmediatePropagation(){stopped=true;}});
  assert.equal(prevented,expected);assert.equal(stopped,expected);
 }
});

test('brand preview rejects foreign windows, unsafe IDs and stale sequence numbers',()=>{
 const handlers=new Map(),parent={},custom={hidden:true,removeAttribute(key){delete this[key]}},fallback={hidden:false},favicon={},root={querySelector:key=>key.includes('custom')?custom:fallback};
 const context={parent,location:{origin:'http://localhost'},window:{addEventListener:(type,fn)=>handlers.set(type,fn)},document:{addEventListener(){},querySelector:key=>key.startsWith('link')?favicon:root}};
 runInNewContext(readFileSync(new URL('./theme-preview.js',import.meta.url),'utf8'),context);
 const send=handlers.get('message'),message={type:'balemoh:brand-preview',version:1,instanceId:'presentation-theme-preview',sequence:1,logoID:'upload:LOGO',iconID:'upload:ICON'};
 send({origin:'http://evil.test',source:parent,data:message});send({origin:'http://localhost',source:{},data:message});send({origin:'http://localhost',source:parent,data:{...message,iconID:'https://evil.test/image'}});assert.equal(custom.hidden,true);
 send({origin:'http://localhost',source:parent,data:message});assert.equal(custom.hidden,false);assert.equal(fallback.hidden,true);assert.equal(custom.src,'/icons/upload%3ALOGO/image');assert.equal(favicon.href,'/icons/upload%3AICON/image');
 send({origin:'http://localhost',source:parent,data:{...message,iconID:''}});assert.equal(custom.hidden,false);
 send({origin:'http://localhost',source:parent,data:{...message,sequence:2,logoID:'',iconID:'upload:ICON'}});assert.equal(custom.hidden,true);assert.equal(fallback.hidden,false);assert.equal(custom.src,undefined);assert.equal(favicon.href,'/icons/upload%3AICON/image');
 send({origin:'http://localhost',source:parent,data:{...message,sequence:3,iconID:''}});assert.equal(custom.hidden,false);assert.equal(favicon.href,'/ui/brand/favicon.svg');
});

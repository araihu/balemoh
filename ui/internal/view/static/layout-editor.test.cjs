const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { Draft, canonical } = require('./layout-editor.js');
const initial = JSON.parse(readFileSync(path.resolve(__dirname, '../../../../docs/specs/spec-001-homepage-layouts/example-layout.json')));
function setup() {
 const document = structuredClone(initial);
 document.groups[0].placements = [{ itemId: 'host-dormant' }];
 const preview = { document, requestId: 'initial', draftGeneration: 0, layoutRevision: '0', stale: false, resolvedGroups: document.groups.map(g => ({ groupId: g.id, title: g.title, displayLabel: g.title, members: g.id === 'builtin:hosts' ? [{ itemId: 'host-a', state: 'active' }, { itemId: 'host-dormant', state: 'dormant' }, { itemId: 'host-z', state: 'active' }] : [] })) };
 return new Draft({ state: { revision: '0', document }, preview, selection: { groupId: 'builtin:hosts', itemId: 'host-a' } });
}
function result(entry, model) { return { ...structuredClone(model.preview), requestId: entry.request.requestId, draftGeneration: entry.request.draftGeneration, document: structuredClone(model.document) }; }
test('late success and failure cannot replace a newer draft or enable Save', () => {
 const d = setup(); d.change(doc => { doc.homepage.width = 'contained'; }); const old = d.begin('old', 'old-digest');
 d.change(doc => { doc.groups[0].title = 'Latest input'; }); const latest = d.begin('latest', 'latest-digest');
 assert.equal(d.accept(old, result(old, d)), false); assert.equal(d.fail(old, 503), false); assert.equal(d.ready, false);
 assert.equal(d.accept(latest, result(latest, d)), true); assert.equal(d.group().title, 'Latest input'); assert.equal(d.ready, true);
});
test('movement includes dormant positions and changes only destination policy', () => {
 const d = setup(); assert.equal(d.orderItems('builtin:hosts', ['host-z', 'host-dormant', 'host-a']), true);
 assert.deepEqual(d.group().ordering.ids, ['host-z', 'host-dormant', 'host-a']); assert.equal(d.document.homepage.ordering.mode, 'alphabetical');
 assert.equal(d.move('host-a', 'builtin:widgets', null), false, 'pending retained preview must be inert');
 const p = d.begin('ordered', 'digest'); const response = result(p, d); response.resolvedGroups[0].members.reverse(); d.accept(p, response);
 assert.equal(d.move('host-a', 'builtin:widgets', null), true); assert.deepEqual(d.group('builtin:hosts').ordering.ids, ['host-z', 'host-dormant']); assert.equal(d.selection.groupId, 'builtin:widgets');
 assert.deepEqual(d.group('builtin:widgets').ordering.ids, ['host-a']); assert.equal(d.group('builtin:services').ordering.mode, 'alphabetical');
});
test('group deletion rehomes dormant overrides and source defaults without copying ranks', () => {
 const d = setup(); assert.equal(d.remove('builtin:hosts', 'builtin:services'), true); assert.equal(d.document.sourceGroups.host, 'builtin:services'); assert.deepEqual(d.group('builtin:services').placements, [{ itemId: 'host-dormant' }]); assert.equal(d.group('builtin:services').ordering.mode, 'alphabetical'); assert.equal(d.selection.groupId, 'builtin:services');
});
test('creating a group preserves page policy and uses reviewed defaults', () => {
 const d = setup(); assert.equal(d.create('11111111-1111-4111-8111-111111111111', 'Media'), true); const g = d.group(); assert.equal(d.document.homepage.ordering.mode, 'alphabetical'); assert.deepEqual(g.layout, { type: 'grid', columns: 2, gapPx: 16 }); assert.equal(g.showHeading, true); assert.equal(g.initiallyCollapsed, false);
});
test('uncertain Save confirms configuration only and never silently retries a newer revision', () => {
 const d = setup(); d.change(doc => { doc.homepage.width = 'contained'; }); const p = d.begin('preview', 'digest'); d.accept(p, result(p, d)); const attempt = d.startSave(); assert.ok(attempt); assert.equal(d.startSave(), null);
 d.state = 'outcome_unknown'; assert.equal(d.readback({ revision: '1', document: structuredClone(attempt.document) }), true); assert.equal(d.dirty, false); assert.equal(d.revision, '1');
 const other = setup(); other.change(doc => { doc.homepage.width = 'contained'; }); const e = other.begin('preview', 'digest'); other.accept(e, result(e, other)); other.startSave(); other.state = 'outcome_unknown'; assert.equal(other.readback({ revision: '9', document: initial }), false); assert.equal(other.state, 'conflict'); assert.equal(other.revision, '0'); assert.equal(other.document.homepage.width, 'contained');
});
test('canonical equality sorts configuration arrays and preserves custom sequence', () => {
 const a = structuredClone(initial), b = structuredClone(initial); b.groups.reverse(); b.homepage.placements.reverse(); assert.equal(canonical(a), canonical(b)); a.homepage.ordering = { mode: 'custom', ids: ['builtin:hosts', 'builtin:widgets'] }; b.homepage.ordering = { mode: 'custom', ids: ['builtin:widgets', 'builtin:hosts'] }; assert.notEqual(canonical(a), canonical(b));
});
test('capacity rejects materialization while retaining selected default members', () => {
 const d = setup(); d.preview.resolvedGroups[0].members = Array.from({ length: 513 }, (_, i) => ({ itemId: 'host-' + i, state: 'active' }));
 assert.equal(d.fitsOrder('builtin:hosts'), false); assert.equal(d.orderItems('builtin:hosts', d.preview.resolvedGroups[0].members.map(m => m.itemId)), false);
 assert.equal(d.document.groups[0].placements.length, 1); assert.equal(d.ready, true);
 assert.equal(d.move('host-0', 'builtin:widgets', null), true, 'one member can still move to a smaller group');
});
test('stale or revision-mismatched bootstrap is never ready', () => {
 const d=setup();const data={state:{revision:'0',document:d.saved},preview:{...d.preview,stale:true,layoutRevision:'1'},selection:d.selection};const stale=new Draft(data);assert.equal(stale.state,'conflict');assert.equal(stale.ready,false);
 data.preview.stale=false;const mismatch=new Draft(data);assert.equal(mismatch.ready,false);assert.equal(mismatch.startSave(),null);
});
test('overlapping live readbacks cannot regress a ready editor', async () => {
 const d=setup();let resolveFirst,resolveSecond;const firstResult=new Promise(resolve=>{resolveFirst=resolve;});const secondResult=new Promise(resolve=>{resolveSecond=resolve;});
 const first=d.beginLive('1');const firstApply=firstResult.then(state=>d.applyLive(first,state));const second=d.beginLive('2');const secondApply=secondResult.then(state=>d.applyLive(second,state));
 const one=structuredClone(d.document);one.groups[0].title='Revision one';const two=structuredClone(d.document);two.groups[0].title='Revision two';
 resolveSecond({revision:'2',document:two});assert.equal(await secondApply,true);const p=d.begin('live-two','digest');const current=result(p,d);current.layoutRevision='2';assert.equal(d.accept(p,current),true);assert.equal(d.ready,true);
 resolveFirst({revision:'1',document:one});assert.equal(await firstApply,false);assert.equal(d.revision,'2');assert.equal(d.group().title,'Revision two');assert.equal(d.ready,true);assert.equal(d.beginLive('1'),null);
});
test('rejected provisional ID recovery rewrites every group reference and retains draft fields', () => {
 const d=setup(),oldId='11111111-1111-4111-8111-111111111111',newId='22222222-2222-4222-8222-222222222222';assert.equal(d.create(oldId,'Keep this title'),true);
 const p=d.begin('created','digest');assert.equal(d.accept(p,result(p,d)),true);
 d.change(doc=>{doc.homepage.width='contained';doc.homepage.ordering={mode:'custom',ids:doc.groups.map(g=>g.id)};doc.sourceGroups.other=oldId;});const rejected=d.begin('rejected','digest');
 const at=rejected.request.document.groups.findIndex(g=>g.id===oldId),detail={fieldErrors:{['/document/groups/'+at+'/id']:'Use another UUID.'}};assert.equal(d.fail(rejected,422),true);const row=d.rejectedGroup(rejected,detail);assert.ok(row);
 assert.equal(d.regenerateGroup(row,newId),true);assert.equal(d.selection.groupId,newId);assert.equal(d.group().title,'Keep this title');assert.equal(d.document.homepage.width,'contained');assert.equal(d.document.sourceGroups.other,newId);assert.ok(d.document.homepage.ordering.ids.includes(newId));assert.ok(d.document.homepage.placements.some(p=>p.groupId===newId));assert.equal(JSON.stringify(d.document).includes(oldId),false);assert.equal(d.saved.groups.length,3);assert.equal(d.state,'preview_pending');
});
test('conflict stays frozen until explicit recovery even when draft equals old saved content', () => {
 const d=setup();d.state='conflict';assert.equal(d.dirty,false);assert.equal(d.followsLive,false);const entry=d.beginLive('1');const changed=structuredClone(d.document);changed.homepage.width='contained';assert.equal(d.applyLive(entry,{revision:'1',document:changed}),false);assert.equal(d.state,'conflict');assert.equal(d.revision,'0');
});
test('active item disappearance clears selection but preserves dormant membership and owning group', () => {
 const d=setup();const pending=d.begin('membership','digest');const preview=result(pending,d);preview.resolvedGroups[0].members=preview.resolvedGroups[0].members.map(m=>m.itemId==='host-a'?{...m,state:'dormant'}:m);assert.equal(d.accept(pending,preview),true);assert.equal(d.selection.itemId,'');assert.equal(d.selection.groupId,'builtin:hosts');assert.ok(d.preview.resolvedGroups[0].members.some(m=>m.itemId==='host-a'&&m.state==='dormant'));
});

test('malformed Save confirmations preserve the exact attempt until canonical readback', async t => {
 const cases=[null,{}, {revision:'1'}, {revision:'',document:initial}, {revision:'01',document:initial}, {revision:'9223372036854775808',document:initial}, {revision:'2',document:initial}, {revision:'1',document:null}, {revision:'1',document:{}}, {revision:'1',document:{...initial,schemaVersion:99}}];
 for (const [i,invalid] of cases.entries()) await t.test(String(i),()=>{
  const d=setup();d.change(doc=>{doc.homepage.width='contained';});const entry=d.begin('preview','digest');d.accept(entry,result(entry,d));const attempt=d.startSave(),bytes=JSON.stringify(attempt);
  assert.equal(d.savedResult(attempt,invalid),false);assert.equal(d.state,'outcome_unknown');assert.equal(JSON.stringify(d.saveAttempt),bytes);assert.deepEqual(d.document,attempt.document);assert.equal(d.revision,'0');
  assert.equal(d.readback({revision:'1',document:structuredClone(attempt.document)}),true);assert.equal(d.state,'saved');assert.equal(d.saveAttempt,null);
 });
});
test('invalid readback cannot replace an uncertain draft or its precondition',()=>{
 const d=setup();d.change(doc=>{doc.homepage.width='contained';});const entry=d.begin('preview','digest');d.accept(entry,result(entry,d));const attempt=d.startSave();d.state='outcome_unknown';
 for(const state of [null,{}, {revision:'1',document:null},{revision:'-1',document:attempt.document}]){assert.equal(d.readback(state),false);assert.equal(d.state,'outcome_unknown');assert.equal(d.saveAttempt,attempt);assert.deepEqual(d.document,attempt.document);assert.equal(d.revision,'0');}
});

test('a dirty Save cannot be confirmed at its old revision but canonical readback may confirm later',()=>{
 const d=setup();d.change(doc=>{doc.homepage.width='contained';});const p=d.begin('preview','digest');d.accept(p,result(p,d));const attempt=d.startSave();
 assert.equal(d.savedResult(attempt,{revision:'0',document:structuredClone(attempt.document)}),false);assert.equal(d.state,'outcome_unknown');assert.equal(d.saveAttempt,attempt);assert.equal(d.revision,'0');assert.equal(d.dirty,true);
 assert.equal(d.readback({revision:'1',document:structuredClone(attempt.document)}),true);assert.equal(d.state,'saved');assert.equal(d.revision,'1');
});

test('malformed current previews cannot clear ownership or replace the exact draft',async t=>{
 for(const mode of ['null','empty document','null document','missing groups','null members','foreign draft'])await t.test(mode,()=>{
  const d=setup();d.change(doc=>{doc.groups[0].title='Retain every character';});const entry=d.begin('current','digest'),before=JSON.stringify(d.document),selection=structuredClone(d.selection),lastPreview=d.preview;
  let response=result(entry,d);
  if(mode==='null')response=null;
  if(mode==='empty document')response.document={};
  if(mode==='null document')response.document=null;
  if(mode==='missing groups')delete response.resolvedGroups;
  if(mode==='null members')response.resolvedGroups[0].members=null;
  if(mode==='foreign draft')response.document.groups[0].title='Unrequested replacement';
  assert.equal(d.accept(entry,response),false);assert.equal(JSON.stringify(d.document),before);assert.deepEqual(d.selection,selection);assert.equal(d.preview,lastPreview);assert.equal(d.pending,entry);assert.equal(d.ready,false);
 });
});

test('preview title normalization matches Go Unicode White_Space rather than JS trim',()=>{
 const d=setup();d.change(doc=>{doc.groups[0].title='\u0085Host title\u0085';});const entry=d.begin('unicode-title','digest'),preview=result(entry,d);preview.document.groups[0].title='Host title';assert.equal(d.accept(entry,preview),true);assert.equal(d.group().title,'Host title');
 const other=setup();other.change(doc=>{doc.groups[0].title='\uFEFFRetain BOM\uFEFF';});const request=other.begin('bom-title','digest');assert.equal(other.accept(request,result(request,other)),true);assert.equal(other.group().title,'\uFEFFRetain BOM\uFEFF');
});

test('contextual movement disables boundaries and moving to the current group',()=>{
 const d=setup();assert.equal(d.movementAllowed('group-earlier'),false);assert.equal(d.movementAllowed('group-later'),true);assert.equal(d.movementAllowed('item-earlier'),false);assert.equal(d.movementAllowed('item-later'),true);assert.equal(d.movementAllowed('move-item','builtin:hosts'),false);assert.equal(d.movementAllowed('move-item','builtin:widgets'),true);
 d.selection.itemId='host-z';assert.equal(d.movementAllowed('item-later'),false);assert.equal(d.movementAllowed('item-earlier'),true);d.selection.itemId='';assert.equal(d.movementAllowed('move-item','builtin:widgets'),false);
});

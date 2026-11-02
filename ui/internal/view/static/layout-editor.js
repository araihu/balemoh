// Application drafts use public Layout events; Goshtoso owns geometry and pickup.
(function (global) {
  'use strict';
  const clone = value => JSON.parse(JSON.stringify(value));
  const trimTitle = value => value.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
  const key = id => [...new TextEncoder().encode(id)].map(b => b.toString(16).padStart(2, '0')).join('');
  function canonical(document) {
    const d = clone(document);
    d.groups.sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
    d.homepage.placements.sort((a, b) => a.groupId < b.groupId ? -1 : a.groupId > b.groupId ? 1 : 0);
    d.groups.forEach(g => g.placements.sort((a, b) => a.itemId < b.itemId ? -1 : a.itemId > b.itemId ? 1 : 0));
    function stable(v) { return Array.isArray(v) ? v.map(stable) : v && typeof v === 'object' ? Object.fromEntries(Object.keys(v).sort().map(k => [k, stable(v[k])])) : v; }
    return JSON.stringify(stable(d));
  }
  const validRevision = value => typeof value === 'string' && /^(0|[1-9][0-9]*)$/.test(value) && BigInt(value) <= 9223372036854775807n;
  class Draft {
    constructor(data) {
      this.revision = data.state.revision; this.saved = clone(data.state.document); this.document = clone(data.preview.document);
      this.preview = clone(data.preview); this.selection = clone(data.selection); this.generation = 0; this.state = data.preview.stale || data.preview.layoutRevision !== data.state.revision ? 'conflict' : 'preview_valid';this.liveSequence=0;this.liveRevision=this.revision; this.pending = null;this.saveAttempt = null;this.provisionalGroups=new Map();this.nextGroupRow=0;
      this.reconcileSelection();
    }
    beginLive(revision) { if (!/^(0|[1-9][0-9]*)$/.test(revision)||BigInt(revision)<BigInt(this.revision)||BigInt(revision)<BigInt(this.liveRevision)) return null;this.liveRevision=revision;return {sequence:++this.liveSequence,revision,generation:this.generation}; }
    ownsLive(entry) {return !!entry&&entry.sequence===this.liveSequence&&entry.revision===this.liveRevision;}
    applyLive(entry,state) {if(!this.ownsLive(entry)||state.revision!==entry.revision||entry.generation!==this.generation||this.dirty||this.saveAttempt||['saving','outcome_unknown','conflict'].includes(this.state))return false;this.saved=clone(state.document);this.document=clone(state.document);this.revision=state.revision;this.generation++;this.pending=null;this.reconcileSelection();this.state='preview_pending';return true;}
    referenceCount() { return this.document.groups.reduce((n,g)=>n+g.placements.length,0); }
    fitsOrder(groupId) { const g=this.group(groupId),r=this.resolved(groupId);return !!g&&!!r&&this.referenceCount()-g.placements.length+r.members.length<=512; }
    fitsMove(itemId,groupId) { const g=this.group(groupId),r=this.resolved(groupId);if(!g||!r)return false;const outside=this.document.groups.some(v=>v.id!==groupId&&v.placements.some(p=>p.itemId===itemId));const count=r.members.some(m=>m.itemId===itemId)?r.members.length:r.members.length+1;return this.referenceCount()-g.placements.length-(outside?1:0)+count<=512; }
    movementAllowed(action,destination) {
      if(action==='move-item')return !!this.selection.itemId&&destination!==this.selection.groupId&&this.fitsMove(this.selection.itemId,destination);
      if(action==='group-position')return this.preview.resolvedGroups.some(g=>g.groupId===this.selection.groupId);
      if(action==='item-position')return this.fitsOrder(this.selection.groupId)&&!!this.resolved()?.members.some(m=>m.itemId===this.selection.itemId);
      return true;
    }
    get dirty() { return canonical(this.saved) !== canonical(this.document); }
    get followsLive() {return !["saving","outcome_unknown","conflict"].includes(this.state);}
    get ready() { return this.state === 'preview_valid' && this.preview.draftGeneration === this.generation; }
    group(id = this.selection.groupId) { return this.document.groups.find(g => g.id === id); }
    resolved(id = this.selection.groupId) { return this.preview.resolvedGroups.find(g => g.groupId === id); }
    reconcileSelection() {
      if (!this.group()) { this.selection.groupId = this.preview.resolvedGroups.find(g => this.group(g.groupId))?.groupId || this.document.groups[0]?.id || ''; this.selection.itemId = ''; }
      if (this.selection.itemId) {
        const owner = this.preview.resolvedGroups.find(g => g.members.some(m => m.itemId === this.selection.itemId));
        if (owner && this.group(owner.groupId)) this.selection.groupId = owner.groupId; else this.selection.itemId = '';
      }
    }
    change(action, structural = false) {
      if (['saving', 'outcome_unknown', 'conflict'].includes(this.state) || structural && !this.ready) return false;
      action(this.document); this.generation++; this.state = 'preview_pending'; this.pending = null; return true;
    }
    begin(requestId, digest) {
      const request = { requestId, draftGeneration: this.generation, expectedRevision: this.revision, document: clone(this.document) };
      const entry = { request, digest,selection:clone(this.selection),bytes: JSON.stringify(this.document) }; this.pending = entry; this.state = 'preview_pending'; return entry;
    }
    owns(entry) { return this.pending === entry && entry.request.draftGeneration === this.generation && entry.request.expectedRevision === this.revision && entry.bytes === JSON.stringify(this.document); }
    accept(entry, result) {
      if (!this.owns(entry)) return false;
      try {
        if (result?.requestId !== entry.request.requestId || result.draftGeneration !== entry.request.draftGeneration) throw Error();
        if (!validRevision(result.layoutRevision) || typeof result.stale !== 'boolean' || !Array.isArray(result.resolvedGroups) || result.resolvedGroups.some(g => typeof g.groupId !== 'string' || !Array.isArray(g.members) || g.members.some(m => typeof m.itemId !== 'string' || !['active','dormant'].includes(m.state)))) throw Error();
        const proposed = clone(entry.request.document); proposed.groups.forEach(g => { g.title = trimTitle(g.title); });
        if (canonical(result.document) !== canonical(proposed)) throw Error();
        if (result.stale || result.layoutRevision !== this.revision) { this.state = 'conflict'; return false; }
        const staged = Object.assign(Object.create(Draft.prototype), {document:clone(result.document),preview:clone(result),selection:clone(this.selection)});
        const oldMember=this.preview.resolvedGroups.flatMap(g=>g.members).find(m=>m.itemId===this.selection.itemId),nextMember=staged.preview.resolvedGroups.flatMap(g=>g.members).find(m=>m.itemId===this.selection.itemId);
        if(oldMember?.state==='active'&&nextMember?.state!=='active')staged.selection.itemId='';
        staged.reconcileSelection();
        this.document=staged.document;this.preview=staged.preview;this.selection=staged.selection;this.pending=null;this.state='preview_valid';return true;
      } catch (_) { this.fail(entry,503); return false; }
    }
    fail(entry, status) { if (!this.owns(entry)) return false; this.state = status === 412 ? 'conflict' : status === 422 || status === 400 || status === 404 || status === 409 ? 'preview_invalid' : 'preview_error'; return true; }
    orderGroups(ids) { return this.change(d => { d.homepage.ordering = { mode: 'custom', ids: clone(ids) }; }, true); }
    orderItems(groupId, ids) { if (!this.fitsOrder(groupId)) return false; return this.change(d => { const g = d.groups.find(g => g.id === groupId); g.ordering = { mode: 'custom', ids: clone(ids) }; g.placements = ids.map(itemId => ({ itemId })); }, true); }
    setPosition(kind, value) {
      if (!this.ready || !['group','item'].includes(kind) || String(value).trim()==='') return false;
      const ids = kind==='group' ? this.preview.resolvedGroups.map(g=>g.groupId) : (this.resolved()?.members.map(m=>m.itemId)||[]);
      const id = kind==='group' ? this.selection.groupId : this.selection.itemId;
      const at = ids.indexOf(id), position = Number(value);
      if (at<0 || !Number.isSafeInteger(position) || position<1 || position>ids.length || at===position-1) return false;
      ids.splice(at,1); ids.splice(position-1,0,id);
      return kind==='group' ? this.orderGroups(ids) : this.orderItems(this.selection.groupId,ids);
    }
    move(itemId, groupId, beforeItemId) {
      const destination = this.resolved(groupId); if (!this.ready || !destination || !this.fitsMove(itemId,groupId) || !this.preview.resolvedGroups.some(g => g.members.some(m => m.itemId === itemId))) return false;
      const ids = destination.members.map(m => m.itemId).filter(id => id !== itemId); const at = beforeItemId === null ? ids.length : ids.indexOf(beforeItemId); if (at < 0) return false;
      return this.change(d => {
        d.groups.forEach(g => { g.placements = g.placements.filter(p => p.itemId !== itemId); g.ordering.ids = g.ordering.ids.filter(id => id !== itemId); });
        ids.splice(at, 0, itemId); const target = d.groups.find(g => g.id === groupId); target.placements = ids.map(itemId => ({ itemId })); target.ordering = { mode: 'custom', ids };
        this.selection = { groupId, itemId };
      }, true);
    }
    regenerateGroup(rowKey,newId) {
      const record=this.provisionalGroups.get(rowKey);if(this.state!=='preview_invalid'||!record||!this.group(record.id)||this.group(newId)||!/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(newId))return false;
      return this.change(d=>{const old=record.id;this.group(old).id=newId;d.homepage.placements.forEach(p=>{if(p.groupId===old)p.groupId=newId;});d.homepage.ordering.ids=d.homepage.ordering.ids.map(id=>id===old?newId:id);Object.keys(d.sourceGroups).forEach(k=>{if(d.sourceGroups[k]===old)d.sourceGroups[k]=newId;});if(this.selection.groupId===old)this.selection.groupId=newId;record.id=newId;});
    }
    rejectedGroup(entry,detail) {
      for(const pointer of Object.keys(detail.fieldErrors||{})){const match=pointer.match(/^\/document\/groups\/(\d+)\/id$/);if(!match)continue;const id=entry.request.document.groups[Number(match[1])]?.id;for(const [rowKey,record] of this.provisionalGroups)if(record.id===id&&this.group(id))return rowKey;}return null;
    }
    create(id, title) { if (this.document.groups.length >= 32) return false; return this.change(d => { d.groups.push({ id, title, layout: { type: 'grid', columns: 2, gapPx: 16 }, ordering: { mode: 'alphabetical', ids: [] }, showHeading: true, initiallyCollapsed: false, placements: [] }); d.homepage.placements.push({ groupId: id, span: 'auto' });this.provisionalGroups.set('local-group-'+(++this.nextGroupRow),{id});this.selection = { groupId: id, itemId: '' }; }, true); }
    remove(groupId, destination) {
      if (!this.ready || this.document.groups.length < 2 || destination === groupId || !this.group(destination)) return false;
      return this.change(d => { const source = this.group(groupId); const target = this.group(destination); target.placements.push(...source.placements); d.groups = d.groups.filter(g => g.id !== groupId); d.homepage.placements = d.homepage.placements.filter(p => p.groupId !== groupId); d.homepage.ordering.ids = d.homepage.ordering.ids.filter(id => id !== groupId); Object.keys(d.sourceGroups).forEach(kind => { if (d.sourceGroups[kind] === groupId) d.sourceGroups[kind] = destination; }); this.selection = { groupId: destination, itemId: '' }; }, true);
    }
    startSave() { if (!this.ready || !this.dirty) return null; this.saveAttempt = { expectedRevision: this.revision, document: clone(this.document), generation: this.generation }; this.state = 'saving'; return this.saveAttempt; }
    savedResult(attempt, state, readback = false) { if (this.saveAttempt !== attempt || this.generation !== attempt.generation) return false; try { if (!validRevision(state?.revision) || BigInt(state.revision) < BigInt(attempt.expectedRevision) || !readback && BigInt(state.revision) !== BigInt(attempt.expectedRevision) + 1n || canonical(state.document) !== canonical(attempt.document)) throw Error(); } catch (_) { this.state = 'outcome_unknown'; return false; } this.saved = clone(state.document); this.document = clone(state.document); this.revision = state.revision; this.saveAttempt = null;this.provisionalGroups.clear();this.pending = null;this.state = 'saved'; return true; }
    readback(state) { if (!this.saveAttempt) return false; try { if (!validRevision(state?.revision)) throw Error(); if (canonical(state.document) === canonical(this.saveAttempt.document)) return this.savedResult(this.saveAttempt, state, true); this.state = 'conflict'; } catch (_) { this.state = 'outcome_unknown'; } return false; }
  }
  global.BalemohLayoutDraft = { Draft, canonical, key };
  if (typeof module !== 'undefined') module.exports = global.BalemohLayoutDraft;
  if (typeof document === 'undefined') return;

  async function boot() {
    const root = document.querySelector('[data-layout-editor]'); if (!root || root.dataset.layoutInitialized) return; root.dataset.layoutInitialized = 'true';
    if (global.goshtosoDependencies?.ready) await global.goshtosoDependencies.ready;
    const model = new Draft(JSON.parse(root.dataset.layoutEditor));
    global.htmx.registerExtension('balemoh-layout-editor-disclosure',{htmx_before_morph_attr(element,detail){if(element.matches('details[data-editor-collapse]')&&detail.attrName==='open')return false;}});
    const status = document.getElementById('layout-editor-status'); const errors = document.getElementById('layout-editor-errors'); const frame = document.getElementById('layout-editor-frame');
    let timer, controller, liveController, restoreId, restoreElement, movedIdentity, lastSaveMessage, rejectedGroupRow, pointer = false, queuedPreview, queuedLive, navigation, createId, afterPreviewFocus, latestLiveIdentity, revealGroup, activeSettingsTab='page', requestedSettingsTab, settingsReturnId='layout-action-settings', settingsOpen=false, interactionEpoch=0, alive = true;
    const find = action => root.querySelector(`[data-layout-action="${action}"]`);
    const dispatchModal = (name, id) => global.dispatchEvent(new CustomEvent(`modal:${name}`, { detail: { id: `layout-${id}` } }));
    const announce = text => { status.textContent = text; };
    function settingsTab(tab) { return root.querySelector('[data-layout-settings-tab="'+tab+'"]')?.closest('button'); }
    function openSettings(tab,returnId) {
      settingsOpen=true;activeSettingsTab=tab; settingsReturnId=returnId||settingsReturnId;
      const button=settingsTab(tab); button?.click();
      global.dispatchEvent(new CustomEvent('drawer:open',{detail:{id:'layoutSettings'}}));
      global.Alpine.nextTick(()=>button?.focus({preventScroll:true}));
    }
    function closeSettings(event) {
      if(event.detail?.id!=='layoutSettings'||event.defaultPrevented)return;
      settingsOpen=false;requestedSettingsTab=null;
      global.Alpine.nextTick(()=>{const target=[document.getElementById(settingsReturnId),document.getElementById('edit-group-'+key(model.selection.groupId)),find('open-settings')].find(el=>el&&!el.matches(':disabled')&&!el.closest('#layout-editor-inspector')&&el.getClientRects().length);focusControl(target);});
    }
    global.addEventListener('drawer:close-request',closeSettings);
    const trackInteraction=event=>{if(event.isTrusted)interactionEpoch++;};
    root.addEventListener('pointerdown',trackInteraction,true);root.addEventListener('keydown',trackInteraction,true);
    function state(text) {
      if (text) announce(text);
      if(settingsOpen&&['preview_error','preview_invalid','conflict','outcome_unknown'].includes(model.state)){
        settingsOpen=false;requestedSettingsTab=null;global.dispatchEvent(new CustomEvent('drawer:close',{detail:{id:'layoutSettings'}}));
        global.Alpine.nextTick(()=>focusControl(errors.hidden?status:errors,true));
      }
      const frozen = ['saving', 'outcome_unknown', 'conflict'].includes(model.state);
      const focused = document.activeElement;
      const focusedStructural=[...frame.querySelectorAll('[data-layout-structural]')].some(el=>el===focused||el.contains(focused)||el.hidden&&document.getElementById(el.dataset.layoutFocus||el.id+'-trigger')===focused);
      if (!model.ready && frame.contains(focused) && (frozen || focused.closest('[data-layout-preview]') || focusedStructural)) { restoreId = focused.id;restoreElement=focused;if(!focused.closest('#layout-editor-inspector'))status.focus({ preventScroll: true }); }
      frame.setAttribute('aria-busy', String(model.state === 'preview_pending' || model.state === 'saving'));
      frame.querySelector('[data-layout-preview]').disabled = !model.ready;
      frame.querySelectorAll('[data-layout-structural]').forEach(el => { const capacityBlocked = el.dataset.layoutAction==='item-position'&&!model.fitsOrder(model.selection.groupId) || el.dataset.layoutAction==='item-order'&&model.group()?.ordering.mode==='alphabetical'&&!model.fitsOrder(model.selection.groupId) || el.dataset.layoutAction==='move-item'&&!model.fitsMove(model.selection.itemId,root.querySelector('[data-layout-destination]').value); const shouldDisable = capacityBlocked || !model.movementAllowed(el.dataset.layoutAction,root.querySelector('[data-layout-destination]')?.value) || !model.ready || el.dataset.layoutAction === 'create' && model.document.groups.length >= 32 || el.dataset.layoutAction === 'delete' && model.document.groups.length < 2 || ['move-item', 'item-position'].includes(el.dataset.layoutAction) && !model.selection.itemId; disable(el, shouldDisable); });
      frame.querySelectorAll('[data-layout-field]').forEach(el => { if (!el.hasAttribute('data-layout-structural')) disable(el, frozen || el.closest('[data-layout-group-fields-owner]')&&el.closest('[data-layout-group-fields-owner]').dataset.layoutGroupFieldsOwner!==model.selection.groupId || el.hasAttribute('data-layout-span') && model.document.homepage.layout.type === 'stack'); });
      find('save').disabled = !model.ready || !model.dirty;
      root.querySelectorAll('[data-layout-order-unavailable]').forEach(el=>{el.hidden=model.fitsOrder(model.selection.groupId);});
      const groupLimit=root.querySelector('[data-layout-group-unavailable]');if(groupLimit)groupLimit.hidden=model.document.groups.length<32;
      const moveNote=root.querySelector('[data-layout-move-unavailable]'),destination=root.querySelector('[data-layout-destination]')?.value;if(moveNote)moveNote.hidden=destination===model.selection.groupId||model.fitsMove(model.selection.itemId,destination);
      find('regenerate-provisional').hidden=model.state!=='preview_invalid'||!rejectedGroupRow;
      find('retry').hidden = model.state !== 'preview_error' && model.state !== 'preview_invalid';
      find('readback').hidden = model.state !== 'outcome_unknown';
      find('view-saved').hidden = model.state !== 'conflict'; find('reload').hidden = model.state !== 'conflict';
      root.querySelectorAll('[data-layout-create-form] button[type=submit]').forEach(el => { el.disabled = !model.ready; });
      if (model.ready && !afterPreviewFocus && document.activeElement === status) {const control=restoreElement?.isConnected?restoreElement:restoreId?document.getElementById(restoreId):null;control?.focus({preventScroll:true});restoreId=null;restoreElement=null;}
    }
    function disable(el, value) { if(el.hasAttribute('data-layout-position')){el.readOnly=value;el.setAttribute('aria-disabled',String(value));return;} el.disabled = value; if (el.hidden) { const target = el.dataset.layoutFocus || el.id + '-trigger'; const control = document.getElementById(target); if (control) control.disabled = value; } }
    function focusControl(el,scroll=false) { if(!el)return;
      const context=el.closest('[data-layout-settings-context]')?.dataset.layoutSettingsContext;
      if(context&&(!settingsOpen||context!==activeSettingsTab)){openSettings(context);global.Alpine.nextTick(()=>focusControl(el,scroll));return;}for(let parent=el.parentElement;parent;parent=parent.parentElement)if(parent.tagName==='DETAILS')parent.open=true;el.focus({preventScroll:true});const bounds=el.getBoundingClientRect(),bar=root.querySelector('.balemoh-layout-topbar').getBoundingClientRect();if(scroll||bounds.top<bar.bottom||bounds.bottom>innerHeight)el.scrollIntoView({block:scroll?'start':'nearest',behavior:'instant'}); }
    function clearErrors() { errors.replaceChildren(); errors.hidden = true; root.querySelectorAll('[data-layout-inline-error]').forEach(el => el.remove()); root.querySelectorAll('[aria-invalid="true"]').forEach(el => { el.removeAttribute('aria-invalid'); el.removeAttribute('aria-describedby'); }); }
    function targets() {
      const result = {}; const groupIndex = model.document.groups.findIndex(g => g.id === model.selection.groupId); const placementIndex = model.document.homepage.placements.findIndex(p => p.groupId === model.selection.groupId);
      frame.querySelectorAll('[data-layout-field]').forEach(el => { let path = el.dataset.layoutField; if (path === '/group/span') path = `/homepage/placements/${placementIndex}/span`; else if (path.startsWith('/group/')) path = `/groups/${groupIndex}/${path.slice(7)}`; const target = el.dataset.layoutFocus || (el.hidden ? el.id + '-trigger' : el.id); result['/document' + path] = target; result[path] = target; }); return result;
    }
    function showError(detail, map) {
      clearErrors(); errors.hidden = false; const title = document.createElement('p'); title.textContent = detail.message || 'Layout request failed. Your draft is retained.'; errors.append(title);
      for (const [path, message] of Object.entries(detail.fieldErrors || {})) { const id = map?.[path]; const control = id && document.getElementById(id); const row = document.createElement('p'); if (control) { const link = document.createElement('a'); link.href = '#' + id; link.textContent = message; link.addEventListener('click', event => { event.preventDefault(); focusControl(control,true); }); row.append(link); control.setAttribute('aria-invalid', 'true'); const inline = document.createElement('p'); inline.dataset.layoutInlineError = ''; inline.id = id + '-layout-error'; inline.textContent = message; control.setAttribute('aria-describedby', inline.id); control.after(inline); } else row.textContent = message; errors.append(row); }
      global.Alpine.nextTick(()=>focusControl(errors,true));
    }
    async function response(url, options = {}) { const r = await fetch(url, { cache: 'no-store', ...options }); const data = await r.json(); return { r, data }; }
    async function apply(entry, result) {
      if (!model.owns(entry)) return;
      if (pointer) { queuedPreview = [entry, result]; return; }
      if (typeof result.movementHtml!=='string'||typeof result.pageHtml!=='string'||typeof result.inspectorHtml!=='string'||typeof result.previewHtml!=='string'||typeof result.deleteHtml!=='string') {model.fail(entry,503);state('Preview response unavailable. Your draft is retained.');return;}
      if (!model.accept(entry, result.preview)) { state(model.state === 'conflict' ? 'Saved layout changed. Your draft is retained.' : 'Preview response invalid. Your draft is retained. Retry preview.'); return; }
      if(JSON.stringify(entry.selection)!==JSON.stringify(model.selection)){afterPreviewFocus='editor-group-heading-'+key(model.selection.groupId);model.state='preview_pending';state('Selection changed with current membership. Updating preview…');preview();return;}
      try {
      const focusEpoch=interactionEpoch;const focus = document.activeElement; const inSettings=!!focus?.closest('#layout-editor-inspector'); const id = focus?.id; const start = focus?.selectionStart; const end = focus?.selectionEnd;
      const openPanels=[...root.querySelectorAll('details[data-layout-panel][open]')].map(el=>el.dataset.layoutPanel);
      // Replace form controls without copying old attributes during HTMX settlement.
      await global.htmx.swap({sourceElement:frame,target:'#layout-editor-page-controls',swap:'innerHTML settle:0',text:result.pageHtml});
      const focusedTab=focus?.closest('[role=tab]');
      await global.htmx.swap({ sourceElement: frame, target: '#layout-editor-inspector', swap: 'innerHTML settle:0', text: result.inspectorHtml });
      const tab=settingsTab(requestedSettingsTab||activeSettingsTab);tab?.click();if(focusedTab&&focusEpoch===interactionEpoch)tab?.focus({preventScroll:true});
      await global.htmx.swap({ sourceElement: frame, target: '#layout-editor-preview-body', swap: 'innerMorph', text: result.previewHtml });
      frame.querySelectorAll('details[data-editor-collapse]').forEach(el=>{const group=model.resolved(el.dataset.editorCollapse);if(group?.groupId===model.selection.groupId&&group.members.some(m=>m.itemId===model.selection.itemId&&m.state==='active'))el.open=true;});
      if(root.querySelector('#layout-selected-movement'))await global.htmx.swap({sourceElement:frame,target:'#layout-selected-movement',swap:'innerHTML settle:0',text:result.movementHtml});
      await global.htmx.swap({ sourceElement: root, target: '#layout-delete-destinations', swap: 'innerHTML settle:0', text: result.deleteHtml });
      if (!alive) return;
      root.querySelectorAll('details[data-layout-panel]').forEach(el=>{el.open=openPanels.includes(el.dataset.layoutPanel);});
      if(revealGroup){document.getElementById('editor-group-'+key(revealGroup))?.querySelector('details[data-editor-collapse]')?.setAttribute('open','');revealGroup=null;}
      clearErrors(); let outcome = model.dirty ? 'Unsaved changes.' : lastSaveMessage || 'All changes saved.';
      if (movedIdentity) {
        if (movedIdentity.kind === 'group') { const at = model.preview.resolvedGroups.findIndex(g => g.groupId === movedIdentity.id); if (at >= 0) outcome = `${model.preview.resolvedGroups[at].displayLabel}, group ${at + 1} of ${model.preview.resolvedGroups.length}. ${outcome}`; }
        else { const owner = model.preview.resolvedGroups.find(g => g.members.some(m => m.itemId === movedIdentity.id)); if (owner) { const at = owner.members.findIndex(m => m.itemId === movedIdentity.id); outcome = `${owner.members[at].displayLabel}, position ${at + 1} of ${owner.members.length} in ${owner.displayLabel}. ${outcome}`; } }
        movedIdentity = null;
      }
      state(outcome);
      if (focusEpoch===interactionEpoch && id && (document.activeElement === document.body || inSettings)) { const next = document.getElementById(id); next?.focus({ preventScroll: true }); if (next?.setSelectionRange && start !== null) { try { next.setSelectionRange(start, end); } catch (_) {} } }
      if (focusEpoch===interactionEpoch && afterPreviewFocus && (document.activeElement === status || document.activeElement === document.body)) { const next=document.getElementById(afterPreviewFocus);focusControl(next,afterPreviewFocus==='layout-inspector-heading'&&matchMedia('(max-width:1099px)').matches); }
      if(focusEpoch===interactionEpoch&&inSettings&&document.activeElement===document.body)settingsTab(activeSettingsTab)?.focus({preventScroll:true});
      if(requestedSettingsTab&&focusEpoch===interactionEpoch)openSettings(requestedSettingsTab);requestedSettingsTab=null;
      afterPreviewFocus = null;restoreId=null;restoreElement=null;
      } catch (_) {model.state='preview_error';state('Preview could not be displayed. Your draft is retained. Retry preview.');}
    }
    async function preview() {
      clearTimeout(timer); controller?.abort(); controller = new AbortController();
      const bytes = JSON.stringify(model.document); const generation = model.generation; const revision = model.revision;
      const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(bytes)))].map(v => v.toString(16).padStart(2, '0')).join('');
      if (generation !== model.generation || revision !== model.revision || bytes !== JSON.stringify(model.document) || ['saving', 'outcome_unknown', 'conflict'].includes(model.state)) return;
      const entry = model.begin(crypto.randomUUID(), digest); entry.targets = targets(); state('Updating preview…');
      try { const { r, data } = await response('/settings/homepage/layout/preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ request: entry.request, selection: entry.selection }), signal: controller.signal });
        if (!model.owns(entry)) return;
        if (!r.ok) { if (model.fail(entry, r.status)) {rejectedGroupRow=r.status===422?model.rejectedGroup(entry,data):null;state(r.status === 412 ? 'Layout changed. Your draft is retained.' : 'Preview needs attention. Save remains disabled.'); showError(data, entry.targets); } return; }
        await apply(entry, data);
      } catch (error) { if (error.name !== 'AbortError' && model.fail(entry, 503)) { state('Preview unavailable. Your draft is retained.'); showError({ message: 'Preview unavailable. Retry when the connection returns.' }); } }
    }
    function changed(text) { lastSaveMessage=null;clearErrors(); state(text || 'Updating preview…'); clearTimeout(timer); timer = setTimeout(preview, 180); }
    function select(groupId, itemId = '', control) {
      if (!model.ready || !model.group(groupId)) return;
      model.selection={groupId,itemId};revealGroup=groupId;
      requestedSettingsTab=itemId?'item':'group';settingsReturnId=control?.closest('[data-layout-preview]')?control.id:itemId?'edit-item-'+key(itemId):'edit-group-'+key(groupId);
      model.state='preview_pending';changed('Updating selection…');
    }
    function applyPosition(el) {
      if(!model.ready)return;
      if(!el.checkValidity()){const note=document.getElementById(el.id+'-error');note.textContent='Enter a whole position between 1 and '+el.max+'. Position remains '+el.defaultValue+'.';note.hidden=false;el.value=el.defaultValue;state('Position unchanged.');return;}
      const kind=el.dataset.layoutAction==='group-position'?'group':'item';
      if(model.setPosition(kind,el.value)){movedIdentity={kind,id:kind==='group'?model.selection.groupId:model.selection.itemId};changed('Position changed. Updating preview…');}
      else state(model.dirty?'Unsaved changes.':'All changes saved.');
    }
    root.addEventListener('change', event => {
      const el = event.target; const action = el.dataset.layoutAction;
      if (el.hasAttribute('data-layout-destination')) return state();
      if(el.hasAttribute('data-layout-position'))return applyPosition(el);
      if (action === 'select-group') return select(el.value, '', el);
      if (action === 'select-item') return select(model.selection.groupId, el.value, el);
      if (action === 'page-order' || action === 'item-order') { if (!model.ready) return; if (action === 'page-order') { if (model.change(d => { d.homepage.ordering = { mode: el.value, ids: el.value === 'custom' ? model.preview.resolvedGroups.map(g => g.groupId) : [] }; }, true)) changed(); } else { const group = model.resolved(); if (el.value === 'custom') { if (model.orderItems(group.groupId, group.members.map(m => m.itemId))) changed(); else state('This order would exceed 512 explicit references. Choose a smaller group or reset alphabetical order.'); } else if (model.change(() => { model.group().ordering = { mode: 'alphabetical', ids: [] }; }, true)) changed(); } return; }
      if (el.hasAttribute('data-layout-field') && el.tagName !== 'TEXTAREA' && !(el.tagName === 'INPUT' && el.type === 'text' && !el.hidden)) editField(el);
    });
    function editField(el) {
      const path = el.dataset.layoutField; if (!path) return;const owner=el.closest('[data-layout-group-fields-owner]')?.dataset.layoutGroupFieldsOwner;if(path.startsWith('/group/')&&owner!==model.selection.groupId)return; const structural = el.hasAttribute('data-layout-structural'); const value = el.type === 'checkbox' ? el.checked : el.hasAttribute('data-layout-number') ? Number(el.value) : el.value;
      if (model.change(d => { if (path === '/group/span') { d.homepage.placements.find(p => p.groupId === model.selection.groupId).span = value; return; } const parts = path.split('/').slice(1); let target = parts[0] === 'group' ? model.group() : d; if (parts[0] === 'group') parts.shift(); const last = parts.pop(); parts.forEach(p => { target = target[p]; }); target[last] = value; }, structural)) changed();
    }
    root.addEventListener('input', event => { if(event.target.hasAttribute('data-layout-position')){document.getElementById(event.target.id+'-error').hidden=true;return state();} if (event.target.type === 'text' && !event.target.hidden && event.target.hasAttribute('data-layout-field')) editField(event.target); });
    root.addEventListener('click', async event => {
      const tab=event.target.closest('[role=tab]')?.querySelector('[data-layout-settings-tab]');if(tab){activeSettingsTab=tab.dataset.layoutSettingsTab;if(event.isTrusted)requestedSettingsTab=null;}
      const group = event.target.closest('[data-layout-select-group]'); if (group) return select(group.dataset.layoutSelectGroup, '', group);
      const item = event.target.closest('[data-layout-select-item]'); if (item) { const owner = model.preview.resolvedGroups.find(g => g.members.some(m => m.itemId === item.dataset.layoutSelectItem)); if (owner) select(owner.groupId, item.dataset.layoutSelectItem, item); return; }
      const button = event.target.closest('[data-layout-action]'); if (!button || button.disabled) return; const action = button.dataset.layoutAction;
      if (action === 'move-item') { const destination = root.querySelector('[data-layout-destination]').value; if (model.move(model.selection.itemId, destination, null)) {movedIdentity={kind:'item',id:model.selection.itemId};changed('Item moved. Updating preview…');} }
      else if (action === 'create' && model.ready) { createId = crypto.randomUUID(); root.querySelector('[data-layout-create-form]').reset(); root.querySelector('[data-layout-create-error]').hidden=true;find('regenerate').hidden=true; dispatchModal('open', 'create'); }
      else if(action==='regenerate-provisional'){const next=crypto.randomUUID();if(model.regenerateGroup(rejectedGroupRow,next)){rejectedGroupRow=null;afterPreviewFocus='editor-group-heading-'+key(next);status.focus({preventScroll:true});changed('Group ID replaced in draft. Updating preview…');}}
      else if (action === 'regenerate') {createId=crypto.randomUUID();root.querySelector('[data-layout-create-error]').hidden=true;find('regenerate').hidden=true;document.getElementById('layout-create-title').focus({preventScroll:true});}
      else if (action === 'delete' && model.ready && model.document.groups.length > 1) dispatchModal('open', 'delete');
      else if (action === 'confirm-delete') { const destination = document.getElementById('layout-delete-destination').value; if (model.remove(model.selection.groupId, destination)) { dispatchModal('close', 'delete');requestedSettingsTab='group';settingsReturnId='edit-group-'+key(destination); afterPreviewFocus = 'editor-group-heading-' + key(destination); status.focus({ preventScroll: true }); changed('Group deleted in draft. Updating preview…'); } else announce('Choose a different surviving group.'); }
      else if (action.startsWith('close-')) dispatchModal('close', action.slice(6));
      else if(action==='open-settings')openSettings(activeSettingsTab,'layout-action-settings');
            else if (action === 'retry') preview();
      else if (action === 'save') save();
      else if (action === 'readback') readback();
      else if (action === 'view-saved') viewSaved();
      else if (action === 'reload') { navigation = location.href; dispatchModal('open', 'discard'); }
      else if (action === 'cancel') { navigation = '/'; if (model.dirty) dispatchModal('open', 'discard'); else location.assign('/'); }
      else if (action === 'confirm-discard') { alive = false; controller?.abort(); live.close(); location.assign(navigation || '/'); }
    });
    root.querySelector('[data-layout-create-form]').addEventListener('submit', event => { event.preventDefault(); if (!model.ready || !event.target.reportValidity()) return; const title = trimTitle(event.target.elements.title.value); if (!title || [...title].length > 80) return; if (!/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(createId) || model.group(createId)) {const error=root.querySelector('[data-layout-create-error]');error.textContent='This group ID is invalid or already used. Generate another ID; your title is retained.';error.hidden=false;find('regenerate').hidden=false;find('regenerate').focus({preventScroll:true});return;} if (model.create(createId, title)) { dispatchModal('close', 'create');requestedSettingsTab='group';settingsReturnId='edit-group-'+key(createId); afterPreviewFocus = 'editor-group-heading-' + key(createId); status.focus({ preventScroll: true }); changed('Group created in draft. Updating preview…'); } });
    root.addEventListener('layout:before-move', event => { const d = event.detail; if (d && d.fromContainerId!=='layout-editor-groups') {const destination=model.document.groups.find(g=>'editor-items-'+key(g.id)===d.toContainerId);if(!destination||!model.fitsMove(d.itemId,destination.id)){event.preventDefault();announce('This move would exceed 512 explicit references.');return;}} if (!model.ready || model.pending || !d || !model.group() || d.fromContainerId === 'layout-editor-groups' && d.toContainerId !== 'layout-editor-groups') event.preventDefault(); });
    root.addEventListener('layout:move', event => { if (!model.ready) return; const d = event.detail; if (d.fromContainerId === 'layout-editor-groups') { const ids = model.preview.resolvedGroups.map(g => g.groupId).filter(id => id !== d.itemId); const at = d.beforeItemId === null ? ids.length : ids.indexOf(d.beforeItemId); if (at < 0) return; ids.splice(at, 0, d.itemId); if (model.orderGroups(ids)) {movedIdentity={kind:'group',id:d.itemId};changed('Group moved. Updating preview…');} } else { const destination = model.document.groups.find(g => 'editor-items-' + key(g.id) === d.toContainerId); if (destination && model.move(d.itemId, destination.id, d.beforeItemId)) {movedIdentity={kind:'item',id:d.itemId};changed('Item moved. Updating preview…');} } });
    root.addEventListener('toggle',event=>{const disclosure=event.target;if(!disclosure.matches?.('[data-editor-collapse]')||disclosure.open)return;const group=model.resolved(disclosure.dataset.editorCollapse);if(group?.groupId===model.selection.groupId&&group.members.some(m=>m.itemId===model.selection.itemId&&m.state==='active')){disclosure.open=true;announce('Selected item stays visible while editing. Saved collapse setting is unchanged.');}},true);
    root.addEventListener('pointerdown', event => { if (event.target.closest('[data-layout-preview]')) pointer = true; }, true);
    function releasePointer() { pointer = false; if (queuedLive) { const data = queuedLive; queuedLive = null; onLive(data); } if (queuedPreview) { const args = queuedPreview; queuedPreview = null; apply(...args); } }
    global.addEventListener('pointerup', releasePointer); global.addEventListener('pointercancel', releasePointer);
    root.addEventListener('keydown', event => { if (event.key === 'Escape') releasePointer(); if(event.key==='Enter'&&event.target.hasAttribute('data-layout-position')){event.preventDefault();applyPosition(event.target);} });
    async function save() {
      const attempt = model.startSave(); if (!attempt) return; controller?.abort(); clearTimeout(timer);if(document.activeElement===find('save'))status.focus({preventScroll:true});state('Saving layout…'); const map = targets();
      try { const { r, data } = await response('/settings/homepage/layout', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ expectedRevision: attempt.expectedRevision, document: attempt.document }) });
        if (model.saveAttempt !== attempt) return;
        if (r.ok && model.savedResult(attempt, data)) { lastSaveMessage='Layout saved.';announce(lastSaveMessage);await preview(); }
        else { model.state = r.ok || data.uncertain ? 'outcome_unknown' : r.status === 412 ? 'conflict' : 'preview_invalid'; state('Save was not confirmed. Your draft is retained.'); showError(data, map); if (model.state === 'outcome_unknown') readback(); }
      } catch (_) { if (model.saveAttempt === attempt) { model.state = 'outcome_unknown'; state('Save outcome unknown. Checking the saved layout…'); readback(); } }
    }
    async function readback() { if (model.state !== 'outcome_unknown') return; try { const { r, data } = await response('/settings/homepage/layout/saved'); if (!r.ok) throw Error(); if (model.readback(data)) { lastSaveMessage='Saved layout confirmed.';announce(lastSaveMessage);await preview(); } else state('Saved layout differs. Your draft is retained. Inspect the saved version before discarding.'); } catch (_) { state('Unable to confirm Save. Your exact draft and original revision are retained.'); } }
    async function viewSaved() {
      try {
        const {r,data}=await response('/settings/homepage/layout/compare',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({document:clone(model.document)})});
        if(!r.ok||typeof data.html!=='string')throw Error();
        await global.htmx.swap({sourceElement:root,target:'[data-layout-saved-summary]',swap:'innerHTML settle:0',text:data.html});dispatchModal('open','saved');
      } catch(_){announce('Unable to compare with the saved version. Your draft is retained.');}
    }

    document.addEventListener('click', event => { const link = event.target.closest('a[href]'); if (!model.dirty || !link || event.defaultPrevented || link.target || event.metaKey || event.ctrlKey || link.href.startsWith(location.href.split('#')[0] + '#') || link.closest('dialog') || link.closest('#layout-editor-errors')) return; if (new URL(link.href).origin === location.origin) { event.preventDefault(); navigation = link.href; dispatchModal('open', 'discard'); } });
    global.addEventListener('beforeunload', event => { if (alive && model.dirty) { event.preventDefault(); event.returnValue = ''; } });
    const live = new EventSource('/dashboard/events?layoutEditor=true');
    async function onLive(data) {
      if (pointer) { queuedLive = data; return; }
      const entry=model.beginLive(data.revision);if(!entry)return;
      if (data.revision !== model.revision && model.followsLive) {
        if (model.dirty) { controller?.abort(); model.pending = null; model.state = 'conflict'; state('Another client saved a layout. Your draft is retained.'); }
        else {
          liveController?.abort();liveController=new AbortController();controller?.abort();model.pending=null;model.state='preview_pending';state('Loading the saved layout…');
          try {
            const saved=await response('/settings/homepage/layout/saved',{signal:liveController.signal});
            if(!model.ownsLive(entry))return;
            if(saved.r.ok&&model.applyLive(entry,saved.data))await preview();
            else if(model.dirty){model.state='conflict';state('Saved layout changed while you edited. Your draft is retained.');}
            else if(!saved.r.ok){model.state='preview_error';state('Saved layout unavailable. Retry preview or inspect the saved version.');}
          } catch(error) {if(error.name!=='AbortError'&&model.ownsLive(entry)){model.state='preview_error';state('Saved layout unavailable. Your local values are retained.');}}
        }
      }
      if(!model.ownsLive(entry))return;
      if (data.identity !== latestLiveIdentity) { const initial = latestLiveIdentity === undefined; latestLiveIdentity = data.identity; if (!initial && !['conflict', 'saving', 'outcome_unknown'].includes(model.state)) preview(); }
      if (data.html && !pointer) await global.htmx.swap({ sourceElement: root, target: '#layout-editor-live', swap: 'none', text: data.html });
    }

    live.onmessage = event => { try { onLive(JSON.parse(event.data)).catch(()=>announce('Live update unavailable. Your draft is retained.')); } catch (_) { announce('Live updates could not be read.'); } };
    global.addEventListener('pagehide', () => { alive = false;controller?.abort();liveController?.abort();clearTimeout(timer);live.close(); global.removeEventListener('pointerup', releasePointer); global.removeEventListener('pointercancel', releasePointer);global.removeEventListener('drawer:close-request',closeSettings); }, { once: true });
    root.querySelector('[data-layout-controls]').disabled=false;
    state(model.ready?'All changes saved.':'Saved layout changed while loading. Your draft is retained.');
    root.balemohLayout = model;
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot, { once: true }); else boot();
})(globalThis);

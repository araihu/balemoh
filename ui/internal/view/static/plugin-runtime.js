// One host-owned observer manages contribution lifetimes and revision refreshes.
(() => {
 const active = new Map(), imports = new Map(), styles = new Set(), revisions = new WeakMap();
 let pending = false;
 const schedule = () => { if (!pending) { pending = true; queueMicrotask(reconcile); } };
 async function mount(root) {
  const record = { cleanups: [], removed: false }; active.set(root, record);
  let assets; try { assets = JSON.parse(root.dataset.pluginAssets || '[]'); } catch { return; }
  for (const asset of assets) {
   if (asset.kind === 'style') {
    if (!styles.has(asset.url)) { const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = asset.url; document.head.append(link); styles.add(asset.url); }
   } else {
    try {
     if (!imports.has(asset.url)) imports.set(asset.url, import(asset.url));
     const module = await imports.get(asset.url);
     if (record.removed || !root.isConnected) continue;
     const cleanup = await module.mount?.(root, { key: root.dataset.pluginKey });
     if (typeof cleanup === 'function') { if (record.removed || !root.isConnected) cleanup(); else record.cleanups.push(cleanup); }
     else if (typeof module.unmount === 'function') { const stop = () => module.unmount(root); if (record.removed) stop(); else record.cleanups.push(stop); }
    } catch { root.dataset.pluginAssetError = 'true'; }
   }
  }
 }
 function reconcile() {
  pending = false;
  for (const [root, record] of active) if (!root.isConnected) { record.removed = true; for (const stop of record.cleanups) { try { stop(); } catch {} } active.delete(root); }
  document.querySelectorAll('[data-plugin-mount]').forEach(root => { if (!active.has(root)) mount(root); });
  document.querySelectorAll('[data-plugin-revision]').forEach(signal => {
   const slot = document.getElementById(signal.dataset.pluginTarget); if (!slot) return;
   const previous = revisions.get(slot), revision = signal.dataset.pluginRevision;
   revisions.set(slot, revision); if (previous !== undefined && previous !== revision) slot.dispatchEvent(new Event('plugin-refresh'));
  });
 }
 new MutationObserver(schedule).observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-plugin-revision'] });
 window.addEventListener('pagehide', () => { for (const record of active.values()) { record.removed = true; for (const stop of record.cleanups) { try { stop(); } catch {} } } active.clear(); });
 window.addEventListener('pageshow', schedule);
 schedule();
})();

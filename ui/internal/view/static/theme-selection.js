(() => {
  function mount() {
    const root = document.querySelector('[data-theme-selection]');
    if (!root || root.dataset.mounted) return;
    root.dataset.mounted = 'true';
    const config = JSON.parse(root.dataset.themeSelection), form = root.querySelector('[data-theme-apply]'), selection = root.querySelector('[data-presentation-selection]'), mode = form.elements.namedItem('colorMode'), frame = document.getElementById('presentation-theme-preview'), feedback = root.querySelector('[data-theme-selection-feedback]'), apply = root.querySelector('[data-theme-apply-button]'), system = matchMedia('(prefers-color-scheme: dark)');
    let sequence = 0, pending = null;
    const current = () => config.items.find(item => item.id === selection.value);
    function preview() {
      const item = current();
      if (!item) return;
      const url = new URL('/presentation/themes/editor', location.origin);
      url.searchParams.set('themeID', item.id);
      if (config.recovery) url.searchParams.set('appearance', 'recovery');
      const edit = root.querySelector('[data-theme-edit]');
      edit.hidden = !item.revision;
      edit.querySelector('a').href = url.pathname + url.search;
      url.searchParams.set('new', '1');
      const create = root.querySelector('[data-theme-create]');
      create.hidden = !!item.revision;
      create.querySelector('a').href = url.pathname + url.search;
      frame.contentWindow?.postMessage({type:'goshtoso:theme-state',version:1,instanceId:frame.id,sequence:++sequence,definition:item.definition,previewMode:mode.value === 'system' ? (system.matches ? 'dark' : 'light') : mode.value}, location.origin);
      frame.contentWindow?.postMessage({type:'balemoh:brand-preview',version:1,instanceId:frame.id,sequence,logoID:item.brandLogoId || '',iconID:item.brandIconId || ''},location.origin);
    }
    root.addEventListener('theme-create', () => location.assign(root.querySelector('[data-theme-create] a').href));
    selection.addEventListener('change', preview);
    root.querySelector('[data-presentation-mode]').addEventListener('change', preview);
    system.addEventListener('change', () => { if (mode.value === 'system') preview(); });
    window.addEventListener('message', event => {
      const message = event.data;
      if (event.origin === location.origin && event.source === frame.contentWindow && message?.type === 'goshtoso:theme-ready' && message.version === 1 && message.instanceId === frame.id) preview();
    });
    preview();
    apply.disabled = false;
    form.addEventListener('submit', async event => {
      event.preventDefault();
      const item = current();
      if (!item) return;
      const selection = item.revision ? {kind:'custom',id:item.id,expectedThemeRevision:item.revision} : {kind:'builtin',key:item.id.slice('builtin:'.length),expectedCatalogRevision:config.catalogRevision};
      const payload = {expectedSettingsRevision:config.settingsRevision,colorMode:mode.value,selection}, serialized = JSON.stringify(payload);
      if (pending?.serialized !== serialized) pending = {serialized,command:{...payload,commandId:crypto.randomUUID()}};
      apply.disabled = true;
      feedback.textContent = 'Applying…';
      let confirmed = false;
      try {
        const response = await fetch('/presentation/save', {method:'POST',headers:{'Accept':'application/json','Content-Type':'application/json'},body:JSON.stringify(pending.command)});
        const body = await response.json();
        confirmed = true;
        if (!response.ok) throw body;
        history.replaceState(null, '', '/presentation#themes');
        location.reload();
      } catch (error) {
        feedback.textContent = confirmed ? (error?.message || 'The theme could not be applied.') : 'Outcome unknown. Retry the same selection or reload saved settings.';
        root.querySelector('[data-theme-selection-reload]').hidden = false;
        feedback.focus();
        apply.disabled = false;
      }
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, {once:true});
  else mount();
})();

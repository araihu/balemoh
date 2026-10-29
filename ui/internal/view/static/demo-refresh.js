// One bounded refresh loop per page. It fetches retained observations only.
(() => {
  if (window.balemohDemoRefresh) return;
  window.balemohDemoRefresh = true;
  let busy = false;
  async function refresh() {
    if (document.hidden || busy || !window.htmx) return;
    busy = true;
    try {
      const host = document.getElementById('host-detail');
      if (host) {
        await window.htmx.ajax('GET', location.pathname + location.search, {target: '#host-overview', swap: 'innerMorph'});
      } else {
        for (const widget of document.querySelectorAll('[data-dashboard-widget]')) {
          if (document.hidden) break;
          const id = widget.id.slice('widget-'.length);
          if (id.startsWith('host-')) {
            await window.htmx.ajax('GET', '/dashboard/widgets/host/' + encodeURIComponent(id.slice(5)), {target: '#' + widget.id, swap: 'innerMorph'});
          }
        }
      }
    } catch (_) { /* Existing values remain visible on network failure. */ }
    finally { busy = false; }
  }
  setTimeout(() => { refresh(); setInterval(refresh, 60000); }, 60000 + Math.random() * 15000);
})();

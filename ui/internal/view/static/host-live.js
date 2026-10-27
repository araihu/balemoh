// Connection and measurement freshness share one visible status indicator.
// Existing bookmarks retain their meaning after the two inventory tabs merge.
function normalizeApplicationHash() {
  if (['#services', '#workloads'].includes(location.hash)) {
    history.replaceState(null, '', `${location.pathname}${location.search}#applications`);
  }
}
normalizeApplicationHash();

function filterHostApplications() {
  const root = document.getElementById('host-applications');
  if (!root) return;
  const query = root.querySelector('[data-application-search]').value.trim().toLocaleLowerCase();
  const filter = root.querySelector('[data-application-filter]').value;
  const rows = root.querySelectorAll('[data-application-row]');
  const status = root.querySelector('#host-application-filter-status');
  // The server owns the source-empty state. Filter recovery only makes sense
  // when some received rows can be revealed by a different query or scope.
  if (rows.length === 0) {
    status.hidden = true;
    status.textContent = '';
    return;
  }
  let visible = 0;
  for (const row of rows) {
    const inScope = row.dataset.scope === (filter === 'missing' ? 'missing' : 'current');
    row.hidden = !(inScope && (filter !== 'attention' || row.dataset.attention === 'true') && row.dataset.search.includes(query));
    if (!row.hidden) visible++;
  }
  const message = visible === 0 ? 'No matching applications or workloads. Try another search or filter.' : `${visible} ${visible === 1 ? 'group' : 'groups'} shown`;
  status.hidden = false;
  if (status.textContent !== message) status.textContent = message;
}
document.addEventListener('input', event => {
  if (event.target.matches('[data-application-search], [data-application-filter]')) filterHostApplications();
});
document.addEventListener('change', event => {
  if (event.target.matches('[data-application-filter]')) filterHostApplications();
});
document.addEventListener('htmx:after:swap', filterHostApplications);
document.addEventListener('application-filter-changed', filterHostApplications);
document.addEventListener('DOMContentLoaded', filterHostApplications, { once: true });
function initializeHostApplicationMorphs() {
  window.htmx.registerExtension('balemoh-application-disclosures', {
    htmx_before_morph_attr(element, detail) {
      if (element.matches('[data-application-details]') && detail.attrName === 'open') return false;
    },
  });
  filterHostApplications();
}
// ConsoleShell's LocalRuntime loads HTMX before this script and does not
// create the optional head.Dependencies promise used by CDN-first consumers.
if (window.htmx) initializeHostApplicationMorphs();
else window.goshtosoDependencies?.ready.then(initializeHostApplicationMorphs);

function syncHostStatusDot() {
  const dot = document.getElementById('host-status-dot');
  const tooltip = document.getElementById('host-status-tooltip');
  const connection = document.querySelector('#host-live-status .balemoh-live-badge');
  if (!dot || !tooltip || !connection) return;
  const message = connection.textContent.trim();
  const label = `${dot.querySelector("[data-telemetry-status]").dataset.telemetryStatus}. ${message}`;
  tooltip.textContent = label;
  dot.setAttribute('aria-label', `${dot.dataset.hostName}: ${label}`);
  dot.classList.toggle('balemoh-host-connection-warning', message !== 'Live updates connected');
}
document.addEventListener('htmx:after:swap', syncHostStatusDot);

function hostLiveStatus(message) {
  const status = document.querySelector('#host-live-status .balemoh-live-badge, #dashboard-live-status .balemoh-live-badge');
  if (status) status.textContent = message;
  syncHostStatusDot();
}

document.addEventListener('htmx:sse:before:connection', (event) => {
  if (event.target.matches?.('[data-host-live], [data-dashboard-live]') && event.detail.connection.attempt > 0) {
    hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
  }
});
document.addEventListener('htmx:sse:after:connection', (event) => {
  if (!event.target.matches?.('[data-host-live], [data-dashboard-live]')) return;
  // hx-sse 4.0.0 aborts before cancelling its reader during morph cleanup.
  // Cancel first to avoid its documented unhandled AbortError.
  const connection = event.detail.connection;
  const controller = connection.abortController;
  const abort = controller.abort.bind(controller);
  controller.abort = () => {
    const reader = connection.reader;
    connection.reader = null;
    reader?.cancel().catch(error => {
      if (error.name !== 'AbortError') reportError(error);
    });
    abort();
  };
  hostLiveStatus('Live updates connected');
});
document.addEventListener('htmx:sse:error', (event) => {
  if (!event.target.matches?.('[data-host-live], [data-dashboard-live]')) return;
  hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
});
document.addEventListener('htmx:sse:close', (event) => {
  if (!event.target.matches?.('[data-host-live], [data-dashboard-live]')) return;
  hostLiveStatus('Live updates disconnected. Refresh to reconnect.');
});
document.addEventListener('visibilitychange', () => {
  if (document.hidden) hostLiveStatus('Live updates paused while this tab is in the background.');
});

// Ignore a queued message from the previous range while its stream closes.
document.addEventListener('htmx:sse:before:message', (event) => {
  if (!event.target.matches?.('[data-host-live], [data-dashboard-live]')) return;
  const current = event.target.getAttribute('hx-sse:connect');
  if (new URL(event.detail.connection.url, location.href).href !== new URL(current, location.href).href) {
    event.preventDefault();
  }
});

// Acknowledge explicit refreshes after the swap; background SSE stays quiet.
const widgetRefreshAnimations = new WeakMap();
document.addEventListener('htmx:after:swap', (event) => {
  if (!event.target.matches?.('[data-widget-refresh]')) return;
  const status = event.detail.ctx.response?.status;
  if (!(status >= 200 && status < 300) || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  const widget = event.target.closest('[data-dashboard-widget]');
  if (!widget) return;
  widgetRefreshAnimations.get(widget)?.cancel();
  widgetRefreshAnimations.set(widget, widget.animate([
    { opacity: 0.65, transform: 'translateY(2px)' },
    { opacity: 1, transform: 'translateY(0)' },
  ], { duration: 220, easing: 'ease-out' }));
});

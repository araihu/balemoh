function hostLiveStatus(message) {
  const status = document.querySelector('#host-live-status .balemoh-live-badge, #dashboard-live-status .balemoh-live-badge');
  if (status) status.textContent = message;
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

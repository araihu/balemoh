function hostLiveStatus(message) {
  const status = document.querySelector('#host-live-status .balemoh-live-badge');
  if (status) status.textContent = message;
}

document.addEventListener('htmx:sse:before:connection', (event) => {
  if (event.target.matches?.('[data-host-live]') && event.detail.connection.attempt > 0) {
    hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
  }
});
document.addEventListener('htmx:sse:after:connection', (event) => {
  if (!event.target.matches?.('[data-host-live]')) return;
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
  if (!event.target.matches?.('[data-host-live]')) return;
  hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
});
document.addEventListener('htmx:sse:close', (event) => {
  if (!event.target.matches?.('[data-host-live]')) return;
  hostLiveStatus('Live updates disconnected. Refresh to reconnect.');
});
document.addEventListener('visibilitychange', () => {
  if (document.hidden) hostLiveStatus('Live updates paused while this tab is in the background.');
});

// Ignore a queued message from the previous range while its stream closes.
document.addEventListener('htmx:sse:before:message', (event) => {
  if (!event.target.matches?.('[data-host-live]')) return;
  const current = event.target.getAttribute('hx-sse:connect');
  if (new URL(event.detail.connection.url, location.href).href !== new URL(current, location.href).href) {
    event.preventDefault();
  }
});

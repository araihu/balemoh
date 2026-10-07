function hostLiveStatus(message) {
  const status = document.querySelector('#host-live-status .balemoh-live-badge');
  if (status) status.textContent = message;
}

document.addEventListener('htmx:sse:before:connection', (event) => {
  if (event.target.id === 'host-live' && event.detail.connection.attempt > 0) {
    hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
  }
});
document.addEventListener('htmx:sse:after:connection', (event) => {
  if (event.target.id !== 'host-live') return;
  hostLiveStatus('Live updates connected');
});
document.addEventListener('htmx:sse:error', (event) => {
  if (event.target.id !== 'host-live') return;
  hostLiveStatus('Connection interrupted. Reconnecting; showing last received values.');
});
document.addEventListener('htmx:sse:close', (event) => {
  if (event.target.id !== 'host-live') return;
  hostLiveStatus('Live updates disconnected. Refresh to reconnect.');
});
document.addEventListener('visibilitychange', () => {
  if (document.hidden) hostLiveStatus('Live updates paused while this tab is in the background.');
});

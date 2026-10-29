// Push wakes this worker without an open tab. No fetch/cache interception.
self.addEventListener('install', event => event.waitUntil(self.skipWaiting()));
self.addEventListener('push', event => {
  let message;
  try { message = event.data?.json(); } catch (_) {}
  const title = typeof message?.title === 'string' ? message.title : 'Balemoh';
  const body = typeof message?.body === 'string' ? message.body : 'New notification in your inbox.';
  const options = {body,icon:'/ui/brand/icon-192.png',badge:'/ui/brand/icon-192.png'};
  if (typeof message?.id === 'string') options.tag = 'balemoh-' + message.id;
  event.waitUntil(self.registration.showNotification(title,options));
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const inbox = new URL('/notifications', self.location.origin).href;
    const windows = await self.clients.matchAll({type:'window',includeUncontrolled:true});
    for (const client of windows) {
      if (new URL(client.url).origin === self.location.origin) {
        await client.navigate(inbox); await client.focus(); return;
      }
    }
    await self.clients.openWindow(inbox);
  })());
});

(function (global) {
  'use strict';
  const tokenKey = 'balemoh.notifications.pushToken';
  function create(env) {
    let settings, subscription, token = '', pending = true, failure = '', cleanup = false;
    const supported = () => env.isSecureContext && env.Notification?.requestPermission && env.navigator.serviceWorker && env.PushManager;
    const registered = () => !!settings?.subscribed;
    const active = () => !!(settings?.enabled && registered() && subscription && env.Notification.permission === 'granted');
    function sync() {
      let status = active() ? 'Browser notifications are on, including when the tab is closed.' : 'Browser notifications are off.';
      if (!env.isSecureContext) status = 'Browser notifications require HTTPS or localhost.';
      else if (!supported()) status = 'Web Push unavailable in this browser. On iPhone or iPad, add Balemoh to your Home Screen first.';
      else if (env.Notification.permission === 'denied') status = 'Notifications blocked. Allow them in your browser site settings.';
      else if (settings && !settings.enabled) status = 'Web Push is not configured on this Balemoh server.';
      if (pending) status = 'Checking browser notifications…';
      if (failure) status = failure;
      env.document.querySelectorAll('[data-browser-notifications]').forEach(section => {
        section.hidden = false;
        const button = section.querySelector('[data-browser-notifications-toggle]');
        button.textContent = cleanup ? 'Retry disabling notifications' : registered() ? 'Disable browser notifications' : 'Enable browser notifications';
        button.disabled = pending || (!registered() && !cleanup && (!supported() || !settings?.enabled || env.Notification.permission === 'denied'));
        button.setAttribute('aria-pressed', String(active()));
        section.querySelector('[data-browser-notifications-status]').textContent = status;
      });
    }
    async function request(method, body) {
      const response = await env.fetch('/notifications/push', {method,cache:'no-store',
        headers:{'Content-Type':'application/json', ...(token ? {'X-Push-Token':token} : {})},
        ...(body ? {body:JSON.stringify(body)} : {}), signal:AbortSignal.timeout(6000)});
      if (!response.ok) throw Error('Web Push unavailable');
      return method === 'GET' ? response.json() : undefined;
    }
    async function registration() {
      let timeout;
      const register = async () => {
        const current=await env.navigator.serviceWorker.register('/notifications-worker.js');
        const next=current?.installing||current?.waiting;
        if (next && next.state!=='activated') await new Promise((resolve,reject)=>{
          const settled=()=>{if(next.state==='activated'){next.removeEventListener('statechange',settled);resolve();}else if(next.state==='redundant'){next.removeEventListener('statechange',settled);reject(Error('worker update failed'));}};
          next.addEventListener('statechange',settled);settled();
        });
        return env.navigator.serviceWorker.ready;
      };
      return Promise.race([register(),new Promise((_,reject)=>{timeout=env.setTimeout(()=>reject(Error('worker unavailable')),6000);})]).finally(()=>env.clearTimeout(timeout));
    }
    function keyBytes(key) {
      const encoded=key.replace(/-/g,'+').replace(/_/g,'/');
      return Uint8Array.from(env.atob(encoded+'='.repeat((4-encoded.length%4)%4)),c=>c.charCodeAt(0));
    }
    function payload(value) {
      const {endpoint,keys}=value.toJSON();
      return {endpoint,keys:{p256dh:keys.p256dh,auth:keys.auth}};
    }
    function newToken() {
      const bytes=env.crypto.getRandomValues(new Uint8Array(32));
      return env.btoa(String.fromCharCode(...bytes)).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
    }
    function storageToken() {
      let value=env.localStorage.getItem(tokenKey);
      if (!value) { value=newToken();env.localStorage.setItem(tokenKey,value); }
      if (env.localStorage.getItem(tokenKey)!==value) throw Error('storage unavailable');
      return value;
    }
    function locked(run) { return env.navigator.locks ? env.navigator.locks.request('balemoh-web-push',run) : run(); }
    async function restore() {
      if (!supported()) { pending=false;sync();return; }
      try {
        token=env.localStorage.getItem(tokenKey)||'';
        settings=await request('GET');
        if (token) {
          const worker=await registration(); subscription=await worker.pushManager.getSubscription();
          const key=subscription?.options?.applicationServerKey;
          if (settings.enabled && key && String(new Uint8Array(key))!==String(keyBytes(settings.publicKey))) subscription=null;
          // Renew the lease only for a confirmed, existing subscription.
          if (active()) await request('POST',{token,subscription:payload(subscription)});
        }
        failure='';
      } catch (_) { failure='Could not check Web Push. Reload to retry. Browser storage must be available.'; }
      finally { pending=false;sync(); }
    }
    async function toggle() {
      if (pending) return;
      pending=true;failure='';sync();
      try {
        if (registered() || cleanup) {
          await locked(async()=>{
            const worker=await registration(); const existing=await worker.pushManager.getSubscription();
            if (existing && !await existing.unsubscribe()) throw Error('unsubscribe failed');
            subscription=null;cleanup=true;settings.subscribed=false;
            await request('DELETE',{token});cleanup=false;
            try { env.localStorage.setItem('balemoh.notifications.pushChanged',String(Date.now())); } catch (_) {}
          });
        } else {
          token=storageToken();
          // Preserve user activation: request permission before awaiting registration.
          const permission=await env.Notification.requestPermission();
          if (permission!=='granted') return;
          await locked(async()=>{
            settings=await request('GET');
            if (!settings.enabled) throw Error('push disabled');
            const worker=await registration();let current=await worker.pushManager.getSubscription();
            const key=current?.options?.applicationServerKey;
            if (current && (!key || String(new Uint8Array(key))!==String(keyBytes(settings.publicKey)))) {
              if (!await current.unsubscribe()) throw Error('unsubscribe failed');current=null;
            }
            current=current||await worker.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:keyBytes(settings.publicKey)});
            try { await request('POST',{token,subscription:payload(current)}); }
            catch (error) { await current.unsubscribe();throw error; }
            subscription=current;settings.subscribed=true;
            try { env.localStorage.setItem('balemoh.notifications.pushChanged',String(Date.now())); } catch (_) {}
          });
        }
      } catch (_) { failure=cleanup ? 'Push disabled in this browser. Server cleanup failed; retry disabling.' : 'Could not change Web Push. Check browser storage and connection, then retry.'; }
      finally { pending=false;sync(); }
    }
    env.document.addEventListener('click',event=>{if(event.target.closest('[data-browser-notifications-toggle]'))toggle();});
    env.addEventListener('focus',()=>{if(!pending){pending=true;sync();locked(restore);}});
    env.addEventListener('storage',event=>{if((event.key===tokenKey||event.key==='balemoh.notifications.pushChanged')&&!pending){pending=true;sync();locked(restore);}});
    sync();const ready=locked(restore);
    return {active,sync,toggle,ready};
  }
  if(typeof module!=='undefined'&&module.exports)module.exports={create};
  else global.BalemohBrowserNotifications={create};
})(globalThis);

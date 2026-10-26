// Notification refresh owns its timer and targets, never the shell lifecycle.
(function (global) {
  'use strict';
  function createScheduler({load, hidden, setTimer = setTimeout, clearTimer = clearTimeout}) {
    let timer, busy = false, pending = false, stopped = false, delay = 15000;
    const clear = () => { if (timer !== undefined) clearTimer(timer); timer = undefined; };
    async function refresh() {
      clear();
      if (stopped || hidden()) return;
      if (busy) { pending = true; return; }
      busy = true;
      try { delay = await load() ? 15000 : Math.min(delay * 2, 60000); }
      catch (_) { delay = Math.min(delay * 2, 60000); }
      finally {
        busy = false;
        if (!stopped && !hidden()) {
          if (pending) { pending = false; refresh(); }
          else timer = setTimer(refresh, delay);
        }
      }
    }
    return {
      refresh,
      visibilityChanged() { if (hidden()) { clear(); pending = false; } else refresh(); },
      stop() { stopped = true; pending = false; clear(); },
    };
  }
  if (typeof module !== 'undefined' && module.exports) { module.exports = {createScheduler}; return; }
  function start() {
    const trigger = document.getElementById('notification-trigger');
    const summary = document.getElementById('notification-summary');
    const tray = document.getElementById('notification-tray');
    if (!trigger || !summary || !tray || !global.htmx) return;
    function syncLabel() {
      const state = summary.querySelector('[data-notification-available]');
      if (!state) return;
      const text = state.dataset.notificationAvailable === 'true'
        ? `Notifications, ${state.dataset.notificationCount} unread`
        : 'Notifications unavailable';
      trigger.setAttribute('aria-label', text);
      trigger.setAttribute('title', text);
    }
    function unavailable(target) {
      const state = document.createElement('span');
      state.dataset.notificationAvailable = 'false';
      state.textContent = '!';
      state.className = 'balemoh-notification-count balemoh-notification-count-warning';
      summary.replaceChildren(state);
      if (target === tray) {
        const message = document.createElement('p');
        message.className = 'balemoh-notification-message';
        message.setAttribute('role', 'status');
        message.textContent = 'Notifications unavailable. Try refreshing.';
        tray.replaceChildren(message);
      }
      syncLabel();
    }
    async function load() {
      const open = trigger.getAttribute('aria-expanded') === 'true';
      const target = open ? tray : summary;
      let success = false;
      const completed = event => {
        if (event.detail?.ctx?.sourceElement === target) {
          const status = event.detail.ctx.response?.status;
          success = status >= 200 && status < 300;
        }
      };
      document.addEventListener('htmx:finally:request', completed);
      try {
        await global.htmx.ajax('GET', (open ? '/notifications/tray' : '/notifications/summary') + (global.location.pathname === '/notifications' ? '?inbox=true' : ''), {
          source: target, target, swap: 'innerMorph',
        });
      } catch (_) { success = false; }
      finally { document.removeEventListener('htmx:finally:request', completed); }
      if (!success) unavailable(target);
      syncLabel();
      return success;
    }
    const scheduler = createScheduler({load, hidden: () => document.hidden});
    let wasOpen = trigger.getAttribute('aria-expanded') === 'true';
    const observer = new MutationObserver(() => {
      const open = trigger.getAttribute('aria-expanded') === 'true';
      if (open && !wasOpen) scheduler.refresh();
      wasOpen = open;
    });
    observer.observe(trigger, {attributes: true, attributeFilter: ['aria-expanded']});
    const visibilityChanged = () => scheduler.visibilityChanged();
    document.addEventListener('visibilitychange', visibilityChanged);
    document.getElementById('notification-refresh')?.addEventListener('click', () => scheduler.refresh());
    document.addEventListener('htmx:after:settle', syncLabel);
    const readFocus = new WeakMap();
    // HTMX may replace ctx.sourceElement with the swap target after removing
    // the form. Keep focus state on the request context, which stays stable.
    document.addEventListener('htmx:config:request', event => {
      const ctx = event.detail?.ctx;
      const form = ctx?.sourceElement?.closest?.('form[action^="/notifications/"]');
      if (form?.contains(document.activeElement)) {
        readFocus.set(ctx, {previous: document.activeElement, target: form.getAttribute('hx-target')});
      }
    });
    document.addEventListener('htmx:finally:request', event => {
      const ctx = event.detail?.ctx;
      const saved = ctx && readFocus.get(ctx);
      if (!saved) return;
      readFocus.delete(ctx);
      // Respect an operator who moved focus while the write was pending.
      if (!saved.previous.isConnected && document.activeElement === document.body) {
        const selector = ctx.hx?.retarget || saved.target;
        const target = selector && document.querySelector(selector);
        if (target) target.focus({preventScroll:true});
      }
    });
    global.addEventListener('pagehide', () => {
      scheduler.stop(); observer.disconnect(); document.removeEventListener('visibilitychange', visibilityChanged);
    }, {once: true});
    global.addEventListener('pageshow', event => { if (event.persisted) global.location.reload(); });
    scheduler.refresh();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, {once:true});
  else start();
})(globalThis);

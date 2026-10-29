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
    const browser = global.BalemohBrowserNotifications?.create(global);
    const trigger = document.getElementById('notification-trigger');
    const summary = document.getElementById('notification-summary');
    const tray = document.getElementById('notification-tray');
    const unreadFilter = document.getElementById('notification-only-unread');
    const unreadPreferenceKey = 'balemoh.notifications.unreadOnly';
    try { if (unreadFilter) unreadFilter.checked = global.localStorage.getItem(unreadPreferenceKey) === 'true'; } catch (_) {}
    const returnPage = document.getElementById('notification-return-page');
    if (returnPage && global.location.pathname === '/notifications') returnPage.value = global.location.pathname + global.location.search;
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
        const query = new URLSearchParams();
        if (global.location.pathname === '/notifications') query.set('inbox','true');
        if (open && unreadFilter?.checked) query.set('unread','true');
        if (open) tray.setAttribute('aria-busy','true');
        await global.htmx.ajax('GET', (open ? '/notifications/tray' : '/notifications/summary') + (query.size ? '?' + query.toString() : ''), {
          source: target, target, swap: 'innerMorph',
        });
      } catch (_) { success = false; }
      finally { document.removeEventListener('htmx:finally:request', completed); if (open) tray.setAttribute('aria-busy','false'); }
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
    const visibilityChanged = () => { browser?.sync(); scheduler.visibilityChanged(); };
    document.addEventListener('visibilitychange', visibilityChanged);
    document.getElementById('notification-refresh')?.addEventListener('click', () => scheduler.refresh());
    global.addEventListener('notifications:refresh', () => scheduler.refresh());
    function syncUnreadLinks() {
      const path = unreadFilter?.checked ? '/notifications?unread=true' : '/notifications';
      document.querySelectorAll('[data-notification-inbox-link]').forEach(link => link.setAttribute('href',path));
    }
    syncUnreadLinks();
    unreadFilter?.addEventListener('change', () => {
      try { global.localStorage.setItem(unreadPreferenceKey, String(unreadFilter.checked)); } catch (_) {}
      syncUnreadLinks();
      scheduler.refresh();
    });
    document.addEventListener('htmx:after:settle', () => {
      browser?.sync();
      syncLabel();
      if (returnPage && global.location.pathname === '/notifications') returnPage.value = global.location.pathname + global.location.search;
      const cleared = document.querySelector('[data-notifications-cleared]');
      if (cleared) {
        cleared.remove();
        document.querySelector('form[action="/notifications/clear"]')?.reset();
        const failure = document.querySelector('[data-notification-clear-error]');
        if (failure) { failure.hidden = true; failure.textContent = ''; }
        global.dispatchEvent(new CustomEvent('notifications-cleared'));
        scheduler.refresh();
      }
    });
    const readFocus = new WeakMap();
    const clearRequests = new WeakSet();
    const pageRequests = new WeakSet();
    const bulkRequests = new WeakMap();
    let bulkPending = false;
    function bulkControls(disabled) {
      document.querySelectorAll('#notification-mark-all, form[action="/notifications/read-all"] button[type="submit"]').forEach(button => {
        button.disabled = disabled;
        button.setAttribute('aria-disabled', String(disabled));
      });
    }
    function admitBulk(event) {
      const source = event.type === 'submit'
        ? event.target.closest('form[action="/notifications/read-all"]')
        : event.target.closest('#notification-mark-all');
      if (!source) return;
      if (bulkPending) { event.preventDefault(); event.stopImmediatePropagation(); return; }
      bulkPending = true;
      const status = document.querySelector(event.type === 'submit' ? '#notification-page-read-status' : '#notification-read-all-status');
      if (status) { status.textContent = 'Marking read…'; status.setAttribute('role','status'); }
      // Run HTMX admission first, then lock controls even while its request waits
      // behind a refresh. Request-time hx-disable starts too late for that queue.
      queueMicrotask(() => bulkControls(true));
    }
    document.addEventListener('click', admitBulk, true);
    document.addEventListener('submit', admitBulk, true);
    // HTMX may replace ctx.sourceElement with the swap target after removing
    // the form. Keep focus state on the request context, which stays stable.
    document.addEventListener('htmx:config:request', event => {
      const ctx = event.detail?.ctx;
      const form = ctx?.sourceElement?.closest?.('form[action^="/notifications/"]');
      if (ctx?.sourceElement?.closest?.('#notification-page-filter, [data-notification-page-refresh]')) pageRequests.add(ctx);
      if (ctx?.sourceElement?.id === 'notification-mark-all' || form?.getAttribute('action') === '/notifications/read-all') {
        bulkRequests.set(ctx, {target:form ? '#notification-page-read-status' : '#notification-read-all-status',tray:!form});
      }
      if (form?.getAttribute('action') === '/notifications/clear') clearRequests.add(ctx);
      if (form?.contains(document.activeElement)) {
        readFocus.set(ctx, {previous: document.activeElement, target: form.getAttribute('hx-target')});
      }
    });
    document.addEventListener('htmx:finally:request', event => {
      const ctx = event.detail?.ctx;
      if (ctx && pageRequests.has(ctx)) {
        pageRequests.delete(ctx);
        if (!(ctx.response?.status >= 200 && ctx.response.status < 300)) {
          const filter = document.getElementById('notification-page-only-unread');
          if (filter) filter.checked = new URLSearchParams(global.location.search).get('unread') === 'true';
          const status = document.getElementById('notification-page-read-status');
          if (status) { status.textContent = 'Notifications could not refresh. Try again.'; status.setAttribute('role','alert'); }
        }
      }
      const bulk = ctx && bulkRequests.get(ctx);
      if (bulk) {
        bulkRequests.delete(ctx);
        bulkPending = false;
        bulkControls(false);
        const status = ctx.response?.status;
        if (!status) {
          const target = document.querySelector(bulk.target);
          if (target) {
            target.textContent = 'Read state not confirmed. Refresh before trying again.';
            target.setAttribute('role','alert');
          }
        }
        if (status >= 200 && status < 300) {
          const target = document.querySelector(bulk.target);
          if (target) { target.textContent = ''; target.removeAttribute('role'); }
          scheduler.refresh();
        }
        if (document.activeElement === document.body) {
          const target = document.querySelector(!status || status >= 400 ? bulk.target : bulk.tray ? '#notification-actions-trigger' : '#notification-inbox');
          // HTMX reenables hx-disable targets after its finally event. Wait one
          // frame so the menu trigger can accept focus, without overriding a
          // focus move made while the response was settling.
          requestAnimationFrame(() => {
            if (target && document.activeElement === document.body && (!bulk.tray || trigger.getAttribute('aria-expanded') === 'true')) target.focus({preventScroll:true});
          });
        }
      }
      if (ctx && clearRequests.has(ctx)) {
        clearRequests.delete(ctx);
        if (!ctx.response?.status) {
          const failure = document.querySelector('[data-notification-clear-error]');
          if (failure) {
            failure.textContent = 'Clear not confirmed. Refresh the inbox before trying again.';
            failure.hidden = false;
            if (document.activeElement === document.body && failure.closest('dialog')?.open) {
              failure.focus({preventScroll:true});
            }
          }
        }
      }
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

/* Hydrate before UI startup. Import/reset epochs reject writes from stale tabs. */
;(() => {
  'use strict';
  const ENDPOINT = '/app-data/items';
  const KEEP = /^pinless-(?!wall-snapshot-v1$)[A-Za-z0-9._-]{1,120}$/;
  const OWN = /^(?:pinless-|pinful-|app-data-)/;
  const BROWSER_KEYS = [], BROWSER_STATE = 'pt-browser-settings-v1';
  const REV = 'app-data-revisions-v1';
  let revisions = {}, issue = null;
  const PENDING = 'app-data-pending-v1', SYNCED = 'app-data-synced-v1', EPOCH = 'app-data-epoch-v1';
  let storage;
  try { storage = window.localStorage; storage.getItem(PENDING); } catch { return; }
  if (window.__appDataSync) return;
  window.__appDataSync = true;
  const proto = Storage.prototype, rawGet = proto.getItem, rawSet = proto.setItem, rawRemove = proto.removeItem;
  const keep = key => typeof key === 'string' && KEEP.test(key);
  const get = key => rawGet.call(storage, key);
  function browserState() {
    const values = {};
    for (const key of BROWSER_KEYS) values[key] = get(key);
    return values;
  }
  function applyBrowserState() {
    if (!BROWSER_KEYS.length || !get(BROWSER_STATE)) return;
    try {
      const values = JSON.parse(get(BROWSER_STATE));
      for (const key of BROWSER_KEYS) {
        if (typeof values[key] === 'string') rawSet.call(storage, key, values[key]);
        else rawRemove.call(storage, key);
      }
    } catch {}
  }
  const readPending = () => { try { const p = JSON.parse(get(PENDING) || '{}'); return p && typeof p === 'object' && !Array.isArray(p) ? p : {}; } catch { return {}; } };
  const writePending = pending => { Object.keys(pending).length ? rawSet.call(storage, PENDING, JSON.stringify(pending)) : rawRemove.call(storage, PENDING); };
  function clearOwned(target) {
    if (!target) return;
    const keys = [];
    for (let i = 0; i < target.length; i++) if (OWN.test(target.key(i))) keys.push(target.key(i));
    for (const key of keys) rawRemove.call(target, key);
  }
  try { revisions = JSON.parse(get(REV) || '{}'); } catch {}
  const saveRevisions = () => rawSet.call(storage, REV, JSON.stringify(revisions));
  function problem(value) { issue = value; suspended = true; clearTimeout(timer); timer = 0; }
  let epoch = get(EPOCH) || '', suspended = false, chain = Promise.resolve(), timer = 0, serial = 0;
  function stale() {
    suspended = true;
    clearTimeout(timer); timer = 0;
    try { clearOwned(window.sessionStorage); } catch {}
    if (window.location) window.location.reload();
  }
  function send(items, backup) {
    if (suspended || (!Object.keys(items).length && !Object.keys(backup || {}).length)) return chain;
    const sent = readPending(), sentEpoch = epoch;

    chain = chain.then(async () => {
      if (suspended || epoch !== sentEpoch) return;
      const base = {}; for (const key of Object.keys(items)) base[key] = revisions[key] || '';
      const body = JSON.stringify({items, ...(backup ? {backup} : {}), epoch: sentEpoch, base});
      const response = await fetch(ENDPOINT, {method: 'POST', credentials: 'same-origin', keepalive: body.length < 60000,
        headers: {'Content-Type': 'application/json'}, body});
      if (response.status === 409) {
        let data; try { data = await response.json(); } catch {}
        if (data?.code === 'conflict') problem(data); else stale();
        throw Error('Data changed in another browser.');
      }
      if (!response.ok) throw Error('Save failed.');
      let result; try { result = await response.json(); } catch {}
      for (const key of Object.keys(items)) if (result?.revisions) revisions[key] = result.revisions[key] || '';
      saveRevisions();
      const pending = readPending();
      for (const key of Object.keys(items)) if (pending[key] === sent[key]) delete pending[key];
      writePending(pending);
    }).catch(() => {}); // Keep unconfirmed changes for an explicit retry.
    return chain;
  }
  let server;
  try {
    const request = new XMLHttpRequest();
    request.open('GET', ENDPOINT, false); request.send();
    if (request.status === 200) server = JSON.parse(request.responseText);
    else problem({code: 'recovery', message: 'Saved data is unavailable. Saving is paused. Open Menu to recover saved data.'});
  } catch { problem({code: 'unavailable', message: 'The local app could not be reached. Reload to retry saving.'}); }
  if (server && server.items) {
    const changed = (server.epoch || '') !== epoch;
    if (changed) { clearOwned(storage); try { clearOwned(window.sessionStorage); } catch {} }
    epoch = server.epoch || '';
    if (BROWSER_KEYS.length && !get(BROWSER_STATE) && !(BROWSER_STATE in server.items) && BROWSER_KEYS.some(key => get(key) !== null)) {
      rawSet.call(storage, BROWSER_STATE, JSON.stringify(browserState()));
    }
    const pending = readPending();
    if (changed) revisions = {};
    for (const key of new Set([...Object.keys(revisions), ...Object.keys(server.items)])) {
      if (!(key in pending)) revisions[key] = server.revisions?.[key] || '';
      else if (!(key in revisions)) revisions[key] = '__unknown__';
    }
    saveRevisions();
    const firstTime = get(SYNCED) === null, backup = {}, upload = {};
    for (let i = 0; i < storage.length; i++) {
      const key = storage.key(i); if (!keep(key)) continue;
      const local = get(key);
      if (key in pending || !(key in server.items)) upload[key] = local;
      else if (firstTime && server.items[key] !== local) backup[key] = local;
    }
    for (const key of Object.keys(pending)) if (keep(key)) upload[key] = get(key);
    try {
      for (const [key, value] of Object.entries(server.items)) {
        if (!keep(key) || key in pending) continue;
        value === null ? rawRemove.call(storage, key) : rawSet.call(storage, key, value);
      }
      rawSet.call(storage, EPOCH, epoch); rawSet.call(storage, SYNCED, '1');
      applyBrowserState();
    } catch {}
    send(upload, backup);
  }
  function flush() {
    clearTimeout(timer); timer = 0;
    const items = {};
    for (const key of Object.keys(readPending())) if (keep(key)) items[key] = get(key);
    return send(items);
  }
  function queue(key) {
    const pending = readPending();
    pending[key] = Date.now() + ':' + (++serial) + ':' + Math.random();
    writePending(pending);
    if (!timer) timer = setTimeout(flush, 120);
  }
  proto.setItem = function (key, value) {
    if (suspended && OWN.test(String(key))) return;
    rawSet.call(this, key, value);
    if (this === storage && keep(String(key))) queue(String(key));
    if (this === storage && BROWSER_KEYS.includes(String(key))) {
      rawSet.call(storage, BROWSER_STATE, JSON.stringify(browserState())); queue(BROWSER_STATE);
    }
  };
  proto.removeItem = function (key) {
    if (suspended && OWN.test(String(key))) return;
    rawRemove.call(this, key);
    if (this === storage && keep(String(key))) queue(String(key));
    if (this === storage && BROWSER_KEYS.includes(String(key))) {
      rawSet.call(storage, BROWSER_STATE, JSON.stringify(browserState())); queue(BROWSER_STATE);
    }
  };
  addEventListener('pagehide', () => { if (timer && !suspended) flush(); });
  addEventListener('storage', event => { if (event.key === EPOCH && event.newValue !== epoch) stale(); });
  async function checkSavedState() {
    if (suspended) return;
    try { const response = await fetch(ENDPOINT, {cache: 'no-store'}); if (response.ok) { const current = await response.json(); if ((current.epoch || '') !== epoch) stale(); else if (!Object.keys(readPending()).length && JSON.stringify(current.revisions || {}) !== JSON.stringify(revisions)) stale(); } } catch {}
  }
  addEventListener('focus', checkSavedState);
  addEventListener('pageshow', event => { if (event.persisted) checkSavedState(); });
  if (BROWSER_KEYS.length && window.fetch && typeof document !== 'undefined') {
    const originalFetch = window.fetch;
    window.fetch = function(input, init = {}) {
      const url = new URL(typeof input === 'string' ? input : input.url, location.href);
      if (url.origin === location.origin && !url.pathname.startsWith('/app-data/')) {
        const headers = new Headers(init.headers || (typeof input === 'object' && input.headers) || {});
        headers.set('X-App-Epoch', epoch); init = {...init, headers};
      }
      return originalFetch.call(this, input, init);
    };
    const open = XMLHttpRequest.prototype.open, sendXHR = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function(method, url, ...args) { this.appURL = new URL(url, location.href); return open.call(this, method, url, ...args); };
    XMLHttpRequest.prototype.send = function(...args) {
      if (this.appURL?.origin === location.origin && !this.appURL.pathname.startsWith('/app-data/')) this.setRequestHeader('X-App-Epoch', epoch);
      return sendXHR.apply(this, args);
    };
    document.addEventListener('submit', event => {
      const form = event.target;
      if (!(form instanceof HTMLFormElement) || new URL(form.action).origin !== location.origin) return;
      let input = form.querySelector('[name="_app_epoch"]');
      if (!input) { input = document.createElement('input'); input.type = 'hidden'; input.name = '_app_epoch'; form.append(input); }
      input.value = epoch;
    }, true);
    document.addEventListener('click', event => {
      const link = event.target.closest?.('a[href]'); if (!link) return;
      const url = new URL(link.href);
      if (url.origin === location.origin && ['/toggle_theme', '/subscribe_playlist', '/modify_notifications'].includes(url.pathname)) {
        url.searchParams.set('_app_epoch', epoch); link.href = url.href;
      }
    }, true);
  }
  window.appData = {
    get epoch() { return epoch; },
    get issue() { return issue; },
    localCopy() { const items = {}; for (let i = 0; i < storage.length; i++) { const key = storage.key(i); if (keep(key)) items[key] = get(key); } return {format: 'myapps-conflict', version: 1, epoch, items}; },
    async resolve(useLocal) {
      if (issue?.code !== 'conflict') throw Error('Reload before continuing.');
      const response = await fetch(ENDPOINT, {cache: 'no-store'});
      if (!response.ok) throw Error('Saved data is unavailable.');
      const current = await response.json();
      if ((current.epoch || '') !== epoch) { stale(); return; }
      if (!useLocal) { this.accept(current); window.location.reload(); return; }
      revisions = current.revisions || {}; saveRevisions(); issue = null; suspended = false;
      await this.flush(); window.location.reload();
    },
    async flush() {
      if (suspended) throw Error('Data changed. Reload this page.');
      await flush();
      if (suspended || Object.keys(readPending()).some(keep)) throw Error('Some changes could not be saved. Check the local app and try again.');
    },
    async pause() { await this.flush(); suspended = true; },
    resume() { if (!issue) suspended = false; },
    accept(data) {
      suspended = true; clearTimeout(timer); timer = 0;
      clearOwned(storage); try { clearOwned(window.sessionStorage); } catch {}
      for (const [key, value] of Object.entries(data.items || {})) if (keep(key) && value !== null) rawSet.call(storage, key, value);
      applyBrowserState();
      revisions = data.revisions || {}; saveRevisions();
      epoch = data.epoch || '';
      rawSet.call(storage, SYNCED, '1'); rawSet.call(storage, EPOCH, epoch);
    }
  };
})();

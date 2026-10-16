/* App data on the server. The app keeps using browser storage as before; this script mirrors the
   important keys (playlists, history, follows, search groups...) to the app's own server, so
   clearing the browser or switching browsers no longer loses them.
   - Page start: the server copy is loaded first (a tiny local request) and written into browser
     storage before any other script reads it. The server copy wins, except for changes this
     browser made that the server has not confirmed yet.
   - First start: anything the server has never seen is uploaded from this browser. If this
     browser's copy differed from the server's, its copy is also kept on the server as a backup.
   - Every later change is sent to the server right away (one request at a time, in order). */
;(() => {
  'use strict';
  const ENDPOINT = '/app-data/items';
  const KEEP = /^pinless-(?!wall-snapshot-v1$)[A-Za-z0-9._-]{1,120}$/;
  const PENDING = 'app-data-pending-v1';  // keys changed here and not yet confirmed by the server
  const SYNCED = 'app-data-synced-v1';    // this browser has connected to the server before
  let storage;
  try { storage = window.localStorage; storage.getItem(PENDING); } catch (_) { return; }
  if (window.__appDataSync) return;
  window.__appDataSync = true;
  const proto = Storage.prototype, rawGet = proto.getItem, rawSet = proto.setItem, rawRemove = proto.removeItem;
  const keep = key => typeof key === 'string' && KEEP.test(key);
  const get = key => rawGet.call(storage, key);
  const readPending = () => { try { const p = JSON.parse(get(PENDING) || '{}'); return p && typeof p === 'object' ? p : {}; } catch (_) { return {}; } };
  const writePending = pending => { try { Object.keys(pending).length ? rawSet.call(storage, PENDING, JSON.stringify(pending)) : rawRemove.call(storage, PENDING); } catch (_) {} };

  let chain = Promise.resolve();
  function send(items, backup) {
    const keys = Object.keys(items);
    if (!keys.length && !(backup && Object.keys(backup).length)) return chain;
    const sent = Date.now();
    const body = JSON.stringify(backup ? {items, backup} : {items});
    // keepalive lets the last change survive closing the tab; browsers cap it at 64 KB.
    chain = chain.then(() => fetch(ENDPOINT, {method: 'POST', credentials: 'same-origin', keepalive: body.length < 60000,
      headers: {'Content-Type': 'application/json'}, body}).then(response => {
      if (!response.ok) throw new Error('save failed');
      const pending = readPending();
      for (const key of keys) if (pending[key] !== undefined && pending[key] <= sent) delete pending[key];
      writePending(pending);
    })).catch(() => {}); // still pending: retried on the next change or page load
    return chain;
  }

  // 1. Load the server copy before the app's own scripts run.
  let server = null;
  try {
    const request = new XMLHttpRequest();
    request.open('GET', ENDPOINT, false);
    request.send();
    if (request.status === 200) server = JSON.parse(request.responseText).items || {};
  } catch (_) { server = null; }

  if (server) {
    const pending = readPending(), firstTime = get(SYNCED) === null;
    const localKeys = [];
    for (let i = 0; i < storage.length; i++) { const key = storage.key(i); if (keep(key)) localKeys.push(key); }
    const backup = {}, upload = {};
    for (const key of localKeys) {
      const local = get(key);
      if (key in pending || !(key in server)) upload[key] = local;
      else if (firstTime && server[key] !== local) backup[key] = local;
    }
    for (const key of Object.keys(pending)) if (keep(key) && !(key in upload)) upload[key] = get(key);
    try {
      for (const [key, value] of Object.entries(server)) {
        if (!keep(key) || key in pending) continue;
        if (value === null) { if (get(key) !== null) rawRemove.call(storage, key); }
        else if (get(key) !== value) rawSet.call(storage, key, value);
      }
      rawSet.call(storage, SYNCED, String(Date.now()));
    } catch (_) {}
    send(upload, Object.keys(backup).length ? backup : null);
  }

  // 2. Mirror every later change.
  let outbox = {}, timer = 0;
  function flush() {
    clearTimeout(timer); timer = 0;
    const items = outbox; outbox = {};
    send(items);
  }
  function queue(key, value) {
    const pending = readPending(); pending[key] = Date.now(); writePending(pending);
    outbox[key] = value;
    if (!timer) timer = setTimeout(flush, 120);
  }
  proto.setItem = function (key, value) {
    rawSet.call(this, key, value);
    if (this === storage && keep(String(key))) queue(String(key), String(value));
  };
  proto.removeItem = function (key) {
    rawRemove.call(this, key);
    if (this === storage && keep(String(key))) queue(String(key), null);
  };
  addEventListener('pagehide', () => { if (timer) flush(); });
})();

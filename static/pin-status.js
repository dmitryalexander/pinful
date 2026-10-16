(() => {
  'use strict';
  const cache = new Map(), known = new WeakSet(), visible = new Set();
  const queue = [];
  let revision = 0;
  let active = false, timer = null, stopped = false, controller = null;
  const idFor = tile => {
    const card = tile.closest('[data-pin-id], [data-featured-pin]');
    const id = card?.dataset.pinId || card?.dataset.featuredPin || tile.querySelector('a[href^="/pin/"]')?.getAttribute('href')?.match(/^\/pin\/([A-Za-z0-9_-]+)(?=[/?#]|$)/)?.[1];
    return /^(?:\d{1,24}|e1_[A-Za-z0-9_-]{1,240})$/.test(id || '') ? id : null;
  };
  const paint = (tile, state) => {
    tile.classList.toggle('pin-no-related', state === 'unresolved' || state === 'empty');
    tile.dataset.recommendationStatus = state;
    if (state === 'unresolved') tile.title = 'Unresolved pin link — related pins unavailable';
    else if (state === 'empty') tile.title = 'No related pins returned by Pinterest';
    else if (tile.title === 'Unresolved pin link — related pins unavailable' || tile.title === 'No related pins returned by Pinterest') tile.removeAttribute('title');
  };
  const schedule = (delay = 1100) => {
    if (!timer && !stopped) timer = setTimeout(() => { timer = null; run(); }, delay);
  };
  const enqueue = (tile, priority = false) => {
	if (tile.closest('[data-lens-fallback]')) { paint(tile, 'unresolved'); return; }
    const id = idFor(tile);
    if (!id || tile.closest('[hidden]')) return;
    const entry = cache.get(id);
    if (entry && entry.expires > Date.now()) { paint(tile, entry.state); return; }
    // Expired checks lose their purple border until checked again.
    paint(tile, 'unknown');
    const index = queue.indexOf(tile);
    if (index >= 0) { if (!priority) return; queue.splice(index, 1); }
    if (priority) queue.unshift(tile); else queue.push(tile);
    schedule();
  };
  async function run() {
    if (active || stopped || document.hidden) return;
    let tile;
    while (queue.length) {
      const candidate = queue.shift();
      if (candidate.isConnected && visible.has(candidate) && !candidate.closest('[hidden]')) { tile = candidate; break; }
    }
    if (!tile) return;
    const id = idFor(tile);
    const entry = cache.get(id);
    if (entry && entry.expires > Date.now()) { paint(tile, entry.state); schedule(); return; }
    const requestRevision = revision;
    active = true;
    controller = new AbortController();
    const timeout = setTimeout(() => controller?.abort(), 22000);
    let delay = 1100;
    try {
      const response = await fetch('/pin-status?id=' + encodeURIComponent(id), {signal: controller.signal, credentials: 'same-origin', cache: 'no-store'});
      if (response.status === 429) { delay = 10000; queue.unshift(tile); return; }
      if (!response.ok) throw new Error('Status check unavailable');
      const result = await response.json();
      if (requestRevision !== revision) return;
      const state = ['available', 'empty', 'unresolved', 'unknown'].includes(result.state) ? result.state : 'unknown';
      if (cache.size >= 2048) cache.delete(cache.keys().next().value);
      cache.set(id, {state, expires: Date.now() + 120000});
      for (const other of document.querySelectorAll('.pin-video-tile')) if (idFor(other) === id) paint(other, state);
    } catch (_) { if(requestRevision === revision) paint(tile, 'unknown'); }
    finally { clearTimeout(timeout); controller = null; active = false; schedule(delay); }
  }
  const observer = typeof IntersectionObserver === 'function' ? new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (entry.isIntersecting) { visible.add(entry.target); enqueue(entry.target); }
      else visible.delete(entry.target);
    }
  }) : null;
  const scan = () => {
    for (const tile of document.querySelectorAll('.pin-video-tile')) {
      if (known.has(tile)) continue;
      known.add(tile);
      if (observer) observer.observe(tile);
    }
  };
  // Delegated focus/hover also covers saved walls and dynamically added cards.
  for (const name of ['pointerover', 'focusin']) document.addEventListener(name, event => {
    const tile = event.target.closest?.('.pin-video-tile');
    if (tile) { visible.add(tile); enqueue(tile, true); }
  }, {passive: true});
  document.addEventListener('visibilitychange', () => { if (!document.hidden) schedule(); });
  let scanPending = false;
  new MutationObserver(() => {
    if (scanPending) return;
    scanPending = true;
    queueMicrotask(() => { scanPending = false; scan(); });
  }).observe(document.body, {childList: true, subtree: true});
  function connectionChanged() {
    revision++;
    controller?.abort();
    cache.clear();
    queue.length = 0;
    for(const tile of document.querySelectorAll('.pin-video-tile')) {
      paint(tile,'unknown');
      if(visible.has(tile)) enqueue(tile);
    }
  }
  document.addEventListener('pinterest-session-changed',connectionChanged);
  window.addEventListener('storage',event=>{
    if(event.key==='pinful-pinterest-revision') connectionChanged();
  });
  scan();
  window.addEventListener('pagehide', () => { stopped = true; clearTimeout(timer); controller?.abort(); observer?.disconnect(); });
})();

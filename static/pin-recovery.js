/* Retain already-received media when Pinterest rejects encrypted detail IDs. */
(() => {
  'use strict';
  const pendingKey = 'pinless-open-pin-preview-v1';
  const pinID = href => {
    try {
      const u = new URL(href, location.href);
      return u.origin === location.origin ? u.pathname.match(/^\/pin\/((?:\d+|e1_[A-Za-z0-9_-]+))\/?$/)?.[1] : null;
    } catch (_) { return null; }
  };
  const safeMedia = src => {
    if (!src || typeof src !== 'string') return '';
    try {
      const u = new URL(src, location.href);
      if (u.origin === location.origin && ['/image', '/video', '/hls'].includes(u.pathname)) {
        const target = new URL(u.searchParams.get('url'));
        if (target.protocol !== 'https:' || target.username || target.password || !/(^|\.)pinimg\.com$/.test(target.hostname)) return '';
        return u.pathname + u.search;
      }
      return u.protocol === 'https:' && !u.username && !u.password && /(^|\.)pinimg\.com$/.test(u.hostname) ? u.href : '';
    } catch (_) { return ''; }
  };
  function preview(value, id) {
    if (!value || value.id !== id) return null;
    const image = safeMedia(value.image), video = safeMedia(value.video);
    if (!image && !video) return null;
    const size = v => Math.min(10000, Math.max(1, Number(v) || 500));
    return {id, image, video, title: String(value.title || 'Saved pin').slice(0, 500), width: size(value.width), height: size(value.height)};
  }
  // A single tab-local handoff also covers unsaved pins opened from a wall.
  document.addEventListener('click', event => {
    const link = event.target.closest?.('a[href]');
    const id = link && pinID(link.getAttribute('href'));
    if (!id) return;
    const card = link.closest('[data-featured-pin], [data-pin-id], .pin-video-tile') || link.parentElement;
    const img = card.querySelector('img'), video = card.querySelector('video');
    const value = preview({id, image: img?.getAttribute('src') || video?.getAttribute('poster'), video: video?.dataset.originalSource || video?.getAttribute('src'), title: img?.alt || video?.getAttribute('aria-label'), width: img?.getAttribute('width'), height: img?.getAttribute('height')}, id);
    if (value) { try { sessionStorage.setItem(pendingKey, JSON.stringify(value)); } catch (_) {} }
  }, true);
  const id = pinID(location.href);
  const error = document.querySelector('.pin-end-of-line');
  if (!id || !error || error.querySelector('[data-confirmed-missing]')) return;
  let saved;
  try { saved = preview(JSON.parse(sessionStorage.getItem(pendingKey)), id); } catch (_) {}
  for (const [store, key] of [[localStorage, 'pinless-playlists-v1'], [localStorage, 'pinless-history-v1'], [sessionStorage, 'pinless-wall-snapshot-v1']]) {
    if (saved) break;
    try {
      const stack = [JSON.parse(store.getItem(key))];
      let visited = 0;
      while (stack.length && visited++ < 20000 && !saved) {
        const value = stack.pop();
        if (!value || typeof value !== 'object') continue;
        saved = preview(value, id);
        if (!saved) stack.push(...Object.values(value).filter(v => v && typeof v === 'object'));
      }
    } catch (_) {}
  }
  if (!saved) return;
  const wall = document.createElement('div'); wall.className = 'grid-container pin-detail-wall';
  const card = document.createElement('article'); card.className = 'pin-featured'; card.dataset.featuredPin = id;
  const box = document.createElement('div'); box.className = 'pin-container';
  if (saved.video) {
    const video = document.createElement('video'); video.controls = true; video.playsInline = true; video.preload = 'none'; video.src = saved.video; video.poster = saved.image; video.setAttribute('aria-label', saved.title); box.append(video);
  } else {
    const link = document.createElement('a'); link.href = location.pathname;
    const img = document.createElement('img'); img.src = saved.image; img.alt = saved.title; img.width = saved.width; img.height = saved.height; link.append(img); box.append(link);
  }
  card.append(box); wall.append(card);
  error.after(wall);
  const note = document.createElement('p'); note.textContent = 'Showing your saved image. Pinterest could not provide this pin’s details or related pins.';
  const retry = document.createElement('a'); retry.href = ''; retry.className = 'pin-retry'; retry.textContent = 'Retry details';
  error.className = 'source-error pin-preview-note'; error.style.cssText = 'text-align:center;padding:16px'; error.replaceChildren(note, retry);
  document.dispatchEvent(new CustomEvent('continuous:appended'));
})();

(() => {
  const icons = {
    play: '<path d="m9 5 11 7-11 7z" fill="currentColor" stroke="none"/>',
    download: '<path d="M12 3v12m-5-5 5 5 5-5M5 16v5h14v-5"/>',
    open: '<path d="M14 4h6v6m0-6L10 14M10 4H4v16h16v-6"/>',
    cancel: '<path d="m6 6 12 12M18 6 6 18"/>'
  };
  function icon(element, name, label) {
    element.innerHTML = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + icons[name] + '</svg>';
    if (element.classList.contains('pin-video-download')) {
      const caption = document.createElement('span');
      caption.textContent = name === 'cancel' ? 'Cancel download' : 'Download';
      element.append(caption);
    }
    element.setAttribute('aria-label', label);
    if (name === 'open') element.title = label;
    if (element.classList.contains('pin-video-download')) {
      element.title = label + ' · D';
      element.setAttribute('aria-keyshortcuts', 'D');
    }
    if (element.classList.contains('pin-video-play')) {
      element.title = label + ' · Space';
      element.setAttribute('aria-keyshortcuts', 'Space');
    }
  }
  function addDownload(video, source, tile, notify) {
    const url = new URL(source, location.href);
    if (url.origin !== location.origin || !['/video', '/hls', '/image'].includes(url.pathname)) return;
    const isImage = url.pathname === '/image';
    const downloadLabel = isImage ? 'Download image' : 'Download video';
    const target = url.searchParams.get('url');
    if (!target) return;
    video.setAttribute('controlslist', 'nodownload');
    const actions = document.createElement('div');
    actions.className = 'pin-video-actions';
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'pin-video-action pin-video-download';
    icon(button, 'download', downloadLabel);
    actions.append(button);
    tile.append(actions);
    const pinLink = tile.nextElementSibling;
    if (pinLink?.matches('a') && pinLink.textContent.trim() === 'Open pin') {
      pinLink.className = 'pin-media-open';
      pinLink.textContent = '';
      pinLink.setAttribute('aria-label', 'Open pin');
      pinLink.title = 'Open pin · O';
      pinLink.setAttribute('aria-keyshortcuts', 'O');
      tile.append(pinLink);
    }
    let controller;
    actions.addEventListener('click', event => event.stopPropagation());
    if (isImage) {
      // Images and GIFs: the top strip opens the full-size image in a new tab (videos still download).
      icon(button, 'open', 'Open full-size image in a new tab');
      button.title = 'Open full-size image · D';
      button.addEventListener('click', async event => {
        event.preventDefault();
        const proxy = u => '/image?url=' + encodeURIComponent(u);
        const candidates = [];
        try {
          const t = new URL(target);
          if (/(^|\.)pinimg\.com$/.test(t.hostname) && /\/\d+x\d*\//.test(t.pathname)) {
            const big = new URL(t.href); big.pathname = big.pathname.replace(/\/\d+x\d*\//, '/originals/'); candidates.push(big.href);
          }
        } catch (_) {}
        candidates.push(target);
        const tab = window.open('about:blank', '_blank');
        if (tab) tab.opener = null;
        let chosen = candidates[candidates.length - 1];
        for (const candidate of candidates) {
          const probe = new AbortController();
          try {
            const response = await fetch(proxy(candidate), { signal: probe.signal, cache: 'force-cache' });
            const ok = response.ok && /^image\//i.test(response.headers.get('Content-Type') || '');
            probe.abort();
            if (ok) { chosen = candidate; break; }
          } catch (_) { probe.abort(); }
        }
        if (tab) tab.location.href = proxy(chosen); else window.open(proxy(chosen), '_blank', 'noopener');
      });
      return;
    }
    button.addEventListener('click', async event => {
      event.preventDefault();
      if (controller) { controller.abort(); return; }
      controller = new AbortController();
      icon(button, 'cancel', 'Cancel download');
      tile.classList.add('is-downloading');
      notify(url.pathname === '/hls' ? 'Preparing video… Click the top edge to cancel.' : 'Downloading… Click the top edge to cancel.', true);
      try {
        const response = await fetch('/download?url=' + encodeURIComponent(target) + (isImage ? '&kind=image' : ''), { signal: controller.signal, cache: 'no-store' });
        if (!response.ok) {
          const message = await response.text();
          throw new Error(message && message.length < 250 ? message : 'Download failed. Please try again.');
        }
        const contentType = response.headers.get('Content-Type') || '';
        if (!(isImage ? /^image\/(?!svg\+xml)/i : /^video\/(mp4|webm)(;|$)/i).test(contentType)) throw new Error('The server did not return a media file.');
        notify('Downloading… Click the top edge to cancel.', true);
        const blob = await response.blob();
        const filename = /filename="?([^";]+)"?/i.exec(response.headers.get('Content-Disposition') || '')?.[1] || (blob.type.includes('webm') ? 'pinless-video.webm' : 'pinless-video.mp4');
        const objectURL = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = objectURL;
        link.download = filename;
        document.body.append(link);
        link.click();
        link.remove();
        setTimeout(() => URL.revokeObjectURL(objectURL), 60000);
        notify(isImage ? 'Image ready to save.' : 'Video ready to save.');
      } catch (error) {
        notify(error.name === 'AbortError' ? 'Download cancelled.' : error.message);
      } finally {
        controller = null;
        icon(button, 'download', downloadLabel);
        tile.classList.remove('is-downloading');
      }
    });
    window.addEventListener('pagehide', () => controller?.abort(), { once: true });
  }
  // Blue strip (10-20%) on images and GIFs: search this image with Google Lens in a new tab.
  function addLens(tile, source) {
    let target;
    try {
      const u = new URL(source, location.href);
      if (u.origin !== location.origin || u.pathname !== '/image') return;
      target = u.searchParams.get('url');
    } catch (_) { return; }
    if (!target) return;
    try {
      const t = new URL(target);
      if (t.protocol !== 'https:') return;
      if (/(^|\.)pinimg\.com$/.test(t.hostname)) t.pathname = t.pathname.replace(/\/\d+x\d*\//, '/736x/');
      target = t.href;
    } catch (_) { return; }
    const lens = document.createElement('a');
    lens.className = 'pin-lens-open';
    lens.href = 'https://lens.google.com/uploadbyurl?url=' + encodeURIComponent(target);
    lens.target = '_blank';
    lens.rel = 'noopener noreferrer';
    lens.setAttribute('aria-label', 'Search this image with Google Lens (opens a new tab)');
    lens.title = 'Google Lens · G';
    lens.setAttribute('aria-keyshortcuts', 'G');
    lens.addEventListener('click', event => event.stopPropagation());
    tile.append(lens);
  }
  let library;
  function loadPlayer() {
    if (!library) {
      library = new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = '/static/hls-1.7.3.min.js';
        script.onload = resolve;
        script.onerror = () => { script.remove(); library = null; reject(new Error('Player could not load')); };
        document.head.append(script);
      });
    }
    return library;
  }
  // Hover previews (October 14, 2026), like the YouTube app: rest the mouse on the play area of a video
  // (its lower 80%) for a third of a second and it plays silently in the tile, from about 15% in.
  // Moving away stops it and returns the cover picture. One preview at a time; the download and
  // open-pin strips never preview, and touch screens never do. Menu > Hover previews turns them off.
  const PREVIEW_KEY = 'pinless-hover-previews';
  const PREVIEW_DELAY = 333;
  const previewStart = length => (isFinite(length) && length >= 8 ? Math.min(length * 0.15, 60) : 0);
  let hovering = null;
  function previewsWanted() {
    try { return localStorage.getItem(PREVIEW_KEY) !== 'off'; } catch (_) { return true; }
  }
  function loader(tile, mode) {
    let ring = tile.querySelector(':scope > .pin-video-loader');
    if (!mode) { ring?.remove(); return; }
    if (!ring) {
      ring = document.createElement('span');
      ring.className = 'pin-video-loader';
      ring.setAttribute('role', 'status');
      tile.append(ring);
    }
    ring.classList.toggle('pin-video-wait', mode === 'wait');
    ring.setAttribute('aria-label', mode === 'wait' ? 'Waiting to preview' : 'Loading video');
  }
  document.addEventListener('visibilitychange', () => { if (document.hidden) hovering?.stop(); });
  function mediaUnavailable(tile, media) {
    if (tile.querySelector('.pin-media-unavailable')) return;
    tile.classList.add('has-media-error');
    const width = Number(media.getAttribute('width')), height = Number(media.getAttribute('height'));
    tile.style.aspectRatio = width > 0 && height > 0 ? `${width} / ${height}` : '1';
    const message = document.createElement('div');
    message.className = 'pin-media-unavailable'; message.setAttribute('role', 'status');
    message.setAttribute('aria-label', '404 — Pin media unavailable');
    const art = document.createElement('img'); art.src = '/static/warning.png'; art.alt = '';
    const code = document.createElement('strong'); code.textContent = '404';
    message.append(art, code); tile.append(message);
  }
  function clearMediaError(tile) {
    tile.classList.remove('has-media-error'); tile.style.removeProperty('aspect-ratio');
    tile.querySelector('.pin-media-unavailable')?.remove();
  }
  function initialize(root) { root.querySelectorAll('video').forEach(video => {
    if (video.dataset.hlsReady) return;
    const source = video.getAttribute('src');
    if (!source) return;
    video.dataset.hlsReady = 'true';
    video.dataset.originalSource = source;
    const isHLS = source.startsWith('/hls?');
    const tile = document.createElement('div');
    tile.className = 'pin-video-tile';
    video.before(tile);
    tile.append(video);
    video.controls = false;
    video.title = 'Play or pause video · Space';
    video.setAttribute('aria-keyshortcuts', 'Space');
    if (isHLS) { video.removeAttribute('src'); video.load(); }
    const status = document.createElement('span');
    status.className = 'pin-video-status';
    status.setAttribute('role', 'status');
    status.hidden = true;
    tile.append(status);
    let statusTimer;
    function notify(message, persistent = false) {
      clearTimeout(statusTimer);
      status.textContent = message;
      status.hidden = !message;
      if (!persistent && message) statusTimer = setTimeout(() => { status.hidden = true; }, 7000);
    }
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'pin-video-play';
    icon(button, 'play', 'Play video');
    tile.append(button);
    addDownload(video, source, tile, notify);
    const frame = document.createElement('div');
    frame.className = 'pin-video-frame';
    tile.before(frame);
    frame.append(tile);
    let player, ready = false, starting = false, attaching = null, attempt = null, previewing = false, hoverTimer = 0;
    // Attaches the stream (HLS through hls.js) once; clicked playback and previews share it.
    function attach() {
      if (!isHLS) {
        if (video.getAttribute('src') !== source) video.src = source;
        ready = true;
        return Promise.resolve();
      }
      if (ready) return Promise.resolve();
      if (attaching) return attaching;
      const current = attempt = { cancelled: false, reject: null };
      attaching = (async () => {
        if (video.canPlayType('application/vnd.apple.mpegurl')) { video.src = source; ready = true; return; }
        await loadPlayer();
        if (current.cancelled) throw new Error('cancelled');
        if (!window.Hls || !Hls.isSupported()) throw new Error('This browser cannot play this video.');
        await new Promise((resolve, reject) => {
          current.reject = reject;
          const hls = player = new Hls({ enableWorker: false, maxBufferLength: 30 });
          hls.on(Hls.Events.MANIFEST_PARSED, () => { if (player === hls) { ready = true; resolve(); } });
          hls.on(Hls.Events.ERROR, (_, data) => {
            if (!data.fatal || player !== hls) return;
            if (!ready) reject(new Error('Video could not load. Click to retry.'));
            else if (previewing) endPreview();
            else fail('Video could not load. Click to retry.');
          });
          hls.loadSource(source);
          hls.attachMedia(video);
        });
      })().finally(() => { if (attempt === current) { attaching = null; attempt = null; } });
      return attaching;
    }
    function endPreview() {
      clearTimeout(hoverTimer);
      hoverTimer = 0;
      if (hovering?.video === video) hovering = null;
      loader(tile, null);
      tile.classList.remove('is-previewing');
      if (!previewing) return;
      previewing = false;
      video.pause();
      // A stream still being attached for this preview is abandoned.
      if (attempt) { attempt.cancelled = true; attempt.reject?.(new Error('cancelled')); attaching = null; attempt = null; }
      // Back to the cover picture, and stop downloading.
      if (player) { player.destroy(); player = null; }
      ready = false;
      video.removeAttribute('src');
      video.load();
      video.muted = false;
      video.loop = false;
      delete video.dataset.hoverPreview;
    }
    async function startPreview() {
      hoverTimer = 0;
      if (tile.classList.contains('is-playing') || starting) { loader(tile, null); return; }
      if (hovering && hovering.video !== video) hovering.stop();
      hovering = { video, stop: endPreview };
      previewing = true;
      video.dataset.hoverPreview = 'true'; // not a real play: no history, no Z target
      video.muted = true;
      video.loop = true;
      loader(tile, 'load');
      try {
        await attach();
        if (!previewing) return;
        const seek = () => { const start = previewStart(video.duration); if (start) video.currentTime = start; };
        if (video.readyState >= 1) seek(); else video.addEventListener('loadedmetadata', seek, { once: true });
        await video.play();
      } catch (_) {
        if (previewing) endPreview();
      }
    }
    video.addEventListener('playing', () => {
      clearMediaError(tile);
      if (previewing) { loader(tile, null); tile.classList.add('is-previewing'); }
    });
    video.addEventListener('waiting', () => { if (previewing) loader(tile, 'load'); });
    button.addEventListener('pointerenter', event => {
      if (event.pointerType !== 'mouse' || previewing || hoverTimer || starting || tile.classList.contains('is-playing') || !previewsWanted()) return;
      if (hovering && hovering.video !== video) hovering.stop();
      hovering = { video, stop: endPreview };
      loader(tile, 'wait');
      hoverTimer = setTimeout(startPreview, PREVIEW_DELAY);
    });
    button.addEventListener('pointerleave', () => { if (previewing || hoverTimer) endPreview(); });
    const fail = message => {
      if (player) { player.destroy(); player = null; }
      ready = false;
      starting = false;
      previewing = false;
      delete video.dataset.hoverPreview;
      loader(tile, null);
      tile.classList.remove('is-previewing');
      video.controls = false;
      button.hidden = false;
      button.disabled = false;
      tile.classList.remove('is-loading');
      tile.classList.remove('is-playing');
      icon(button, 'play', 'Retry video');
      mediaUnavailable(tile, video);
      notify(message);
    };
    const play = async () => {
      ready = true;
      video.controls = true;
      tile.classList.add('is-playing');
      button.hidden = true;
      button.disabled = false;
      starting = false;
      tile.classList.remove('is-loading');
      if (!tile.classList.contains('is-downloading')) notify('');
      try { await video.play(); }
      catch (_) {
        tile.classList.remove('is-playing');
        button.hidden = false;
        icon(button, 'play', 'Play video');
        notify('Click to play.');
      }
    };
    button.addEventListener('click', async () => {
      if (starting) return;
      clearMediaError(tile);
      starting = true;
      button.disabled = true;
      // A running preview becomes the real thing: sound on, from the start.
      const wasPreview = previewing;
      clearTimeout(hoverTimer);
      hoverTimer = 0;
      if (hovering?.video === video) hovering = null;
      previewing = false;
      loader(tile, null);
      tile.classList.remove('is-previewing');
      if (wasPreview) {
        video.pause();
        delete video.dataset.hoverPreview;
        video.muted = false;
        video.loop = false;
        if (ready) video.currentTime = 0;
      }
      tile.classList.add('is-loading');
      if (!tile.classList.contains('is-downloading')) notify('Loading video…', true);
      try {
        await attach();
        if (wasPreview && video.currentTime) video.currentTime = 0;
        await play();
      } catch (error) { fail(error?.message || 'Video could not load. Click to retry.'); }
    });
    video.addEventListener('ended', () => { tile.classList.remove('is-playing'); video.controls = false; button.hidden = false; icon(button, 'play', 'Replay video'); });
    video.addEventListener('error', () => {
      if (previewing) { endPreview(); return; }
      if (video.getAttribute('src') === source) fail('Video could not load. Click to retry.');
    });
    window.addEventListener('pagehide', () => { clearTimeout(statusTimer); endPreview(); if (player) player.destroy(); }, { once: true });
  });
    root.querySelectorAll('a[href^="/pin/"] > img, .pin-container > img').forEach(img => {
      if (img.dataset.downloadReady) return;
      img.dataset.downloadReady = 'true';
      const source = img.getAttribute('src');
      if (!source) return;
      const link = img.parentElement.matches('a') ? img.parentElement : null;
      const tile = document.createElement('div');
      tile.className = 'pin-video-tile pin-image-tile';
      const content = link || img;
      content.before(tile);
      tile.append(content);
      img.addEventListener('error', () => mediaUnavailable(tile, img));
      img.addEventListener('load', () => clearMediaError(tile));
      if (img.complete && !img.naturalWidth) mediaUnavailable(tile, img);
      if (link) {
        link.classList.add('pin-image-link');
        link.title = 'Open pin · O';
        link.setAttribute('aria-keyshortcuts', 'O');
      }
      const status = document.createElement('span');
      status.className = 'pin-video-status';
      status.setAttribute('role', 'status');
      status.hidden = true;
      tile.append(status);
      let timer;
      const notify = (message, persistent = false) => {
        clearTimeout(timer);
        status.textContent = message;
        status.hidden = !message;
        if (message && !persistent) timer = setTimeout(() => { status.hidden = true; }, 7000);
      };
      addDownload(img, source, tile, notify);
      addLens(tile, source);
      window.addEventListener('pagehide', () => clearTimeout(timer), {once:true});
    });
  }
  initialize(document);
  document.addEventListener('continuous:appended', () => initialize(document));
})();

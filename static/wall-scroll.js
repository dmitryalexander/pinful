/* Continuous browsing for the local apps. Source links remain usable without JS. */
(() => {
  'use strict';
  const APP = 'pinless';
  const q = (root, selector) => root.querySelector(selector);
  const all = (root, selector) => [...root.querySelectorAll(selector)];
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  function localURL(link, base) {
    if (!link) return null;
    const url = new URL(link.getAttribute('href'), base);
    url.hash = '';
    return url.origin === location.origin ? url.href : null;
  }
  function key(item) {
    if (item.dataset.featuredPin) return '/pin/' + item.dataset.featuredPin;
    const post = item.matches('[data-post-id]') ? item : q(item, '[data-post-id]');
    if (post) return post.dataset.postId;
    const link = item.matches('a[href]') ? item : q(item, 'a[href^="/pin/"],a[href^="/p/"],a[href^="/watch?"],a[href^="/playlist?"],a[href^="/channel/"]');
    if (link) {
      const u = new URL(link.getAttribute('href'), location.href);
      return u.pathname + (u.pathname === '/watch' ? '?v=' + u.searchParams.get('v') : u.pathname === '/playlist' ? '?list=' + u.searchParams.get('list') : '');
    }
    return item.textContent.trim();
  }
  function masonry(root) {
    root.classList.add('media-wall');
    let frame;
    function layout() {
      frame = null;
      const width = root.clientWidth;
      if (!width) return;
      const gap = 4;
      const pinDetail = root.classList.contains('pin-detail-wall');
      const columns = Math.max(1, Math.floor((width + gap) / ((pinDetail ? 230 : 260) + gap)));
      const cell = (width - gap * (columns - 1)) / columns;
      const heights = Array(columns).fill(0);
      const items = [...root.children].filter(item => !item.hidden).sort((a,b)=>{const rank=i=>i.classList.contains('pin-featured')?-2:i.dataset.creatorSide?-1:0;return rank(a)-rank(b);});
      root.classList.add('wall-ready');
      const span = item => pinDetail && (!root.hasAttribute('data-lens-page') || root.hasAttribute('data-previous-wall')) && item.classList.contains('pin-featured') ? Math.min(2, columns) : 1;
      items.forEach(item => { item.style.width = (cell * span(item) + gap * (span(item) - 1)) + 'px'; });
      items.forEach(item => {
        const size = span(item);
        const levels = heights.slice(0, columns - size + 1).map((_, col) => Math.max(...heights.slice(col, col + size)));
        const col = item.classList.contains('pin-featured') || item.dataset.creatorSide === 'left' ? 0 : item.dataset.creatorSide === 'after-pin' && columns > 2 ? 2 : levels.indexOf(Math.min(...levels));
        item.style.left = (col * (cell + gap)) + 'px';
        item.style.top = levels[col] + 'px';
        for (let offset = 0; offset < size; offset++) heights[col + offset] = levels[col] + item.offsetHeight + gap;
      });
      root.style.height = Math.max(0, Math.max(...heights) - gap) + 'px';
    }
    function schedule() { if (!frame) frame = requestAnimationFrame(layout); }
    const observer = new ResizeObserver(schedule);
    observer.observe(root);
    const observed = new Set();
    function register(items) {
      for (const item of observed) if (!root.contains(item)) {observer.unobserve(item); observed.delete(item);}
      items.forEach(item => {observer.observe(item); observed.add(item);});
      schedule();
    }
    register([...root.children]);
    root.addEventListener('load', schedule, true);
    root.addEventListener('loadedmetadata', schedule, true);
    return register;
  }
  window.PinlessMasonry = masonry;
  const videos = new IntersectionObserver(entries => {
    entries.forEach(({ target: video, isIntersecting }) => {
      if (isIntersecting && !reducedMotion && !document.hidden) video.play().catch(() => {});
      else video.pause();
    });
  }, { threshold: 0.15 });
  function enhance(item, wall) {
    all(item, 'img').forEach(img => { img.loading = 'lazy'; img.decoding = 'async'; });
    if (wall && APP !== 'pinless') {
      all(item, 'video').forEach(video => {
        video.removeAttribute('autoplay');
        video.muted = true;
        video.loop = true;
        video.playsInline = true;
        video.controls = true;
        video.preload = 'none';
        videos.observe(video);
      });
    }
    if (APP === 'tumblr' && !q(item, '.post-body img:not(.avatar), .post-body video, .post-body iframe')) {
      const post = item.matches('.post') ? item : q(item, '.post');
      const permalink = q(item, '.interaction-buttons a[href^="/"]');
      if (post && permalink && !q(item, '.wall-read-post')) {
        post.classList.add('wall-text-post');
        const link = document.createElement('a');
        link.className = 'wall-read-post';
        link.href = permalink.getAttribute('href');
        link.textContent = 'Read post';
        q(post, '.post-footer')?.before(link);
      }
    }
    if (APP === 'instagram') {
      const caption = q(item, '.caption');
      if (caption && (caption.textContent.trim().length > 100 || caption.textContent.trim().split('\n').length > 3) && !caption.parentElement.matches('details')) {
        const details = document.createElement('details');
        details.className = 'wall-caption';
        const summary = document.createElement('summary');
        summary.textContent = 'Read caption';
        caption.before(details);
        details.append(summary, caption);
      }
    }
  }
  function connect(config) {
    const root = config.root;
    const initial = config.items(document);
    if (!root || !initial.length) return;
    const sources = config.sources ? config.sources() : [];
    let archive = false;
    let next = config.next(document, location.href);
    if (APP === 'pinless') window.PinlessContinuation = () => next ? [{url:next}] : [];
    if (!next && sources.length) { next = sources.shift(); archive = true; }
    let busy = false, stopped = false, failed = false, repeats = 0;
    const seen = new Set(initial.map(key));
    const pages = new Set();
    const repeatedSources = new Map();
    initial.forEach(item => enhance(item, config.wall));
    let mediaOnly = APP === 'tumblr';
    if (mediaOnly) {
      try { mediaOnly = localStorage.getItem('tumblr-wall-view') !== 'all'; } catch (_) {}
    }
    function filterItems(items) {
      if (APP === 'tumblr') {
        root.classList.toggle('media-only', mediaOnly);
        items.forEach(item => {
          item.hidden = mediaOnly && (item.matches('.wall-text-post') || !!q(item, '.wall-text-post'));
        });
      }
    }
    filterItems(initial);
    const register = config.wall ? masonry(root) : () => {};
    document.addEventListener('pinless:feed-layout', () => register([...root.children]));
    if (APP === 'tumblr') {
      const toolbar = document.createElement('div');
      toolbar.className = 'wall-view-options';
      toolbar.setAttribute('role', 'group');
      toolbar.setAttribute('aria-label', 'Posts to show');
      ['Images & GIFs', 'All posts'].forEach((label, index) => {
        const choice = document.createElement('button');
        choice.type = 'button';
        choice.textContent = label;
        choice.setAttribute('aria-pressed', String(mediaOnly === (index === 0)));
        choice.addEventListener('click', () => {
          mediaOnly = index === 0;
          try { localStorage.setItem('tumblr-wall-view', mediaOnly ? 'media' : 'all'); } catch (_) {}
          [...toolbar.children].forEach((button, i) => button.setAttribute('aria-pressed', String(mediaOnly === (i === 0))));
          filterItems([...root.children]);
          register([]);
        });
        toolbar.append(choice);
      });
      root.before(toolbar);
    }
    const bar = document.createElement('div');
    bar.className = 'continuous-status';
    const status = document.createElement('span');
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = 'Load more';
    const skip = document.createElement('button');
    skip.type = 'button';
    skip.textContent = 'Skip this account';
    skip.hidden = true;
    bar.append(status, button, skip);
    config.hide();
    root.after(bar);
    const observer = new IntersectionObserver(entries => {
      if (entries.some(e => e.isIntersecting) && !failed) load();
    }, { rootMargin: '700px 0px' });
    function finish() {
      stopped = true;
      observer.disconnect();
      button.hidden = true;
      skip.hidden = true;
      status.textContent = APP === 'tumblr' || APP === 'instagram' ? 'You’ve reached the end of the available posts.' : 'You’ve reached the end of the results.';
    }
    async function load() {
      if (busy || stopped) return;
      if (!next) { finish(); return; }
      if (pages.has(next)) { finish(); return; }
      busy = true;
      skip.hidden = true;
      button.disabled = true;
      status.textContent = 'Loading more…';
      root.setAttribute('aria-busy', 'true');
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 45000);
      const requested = next;
      try {
        const response = await fetch(requested, { credentials: 'same-origin', signal: controller.signal });
        if (!response.ok) throw new Error('HTTP ' + response.status);
        if (!response.headers.get('content-type')?.includes('text/html')) throw new Error('Unexpected response');
        const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
        if (doc.querySelector('.source-error') || !config.valid(doc)) throw new Error('The source did not return a feed');
        const items = config.items(doc);
        const following = config.next(doc, response.url);
        const added = [];
        for (const item of items) {
          const id = key(item);
          if (!id || seen.has(id)) continue;
          seen.add(id);
          // Scripts from fetched pages are never executed. App setup runs explicitly below.
          all(item, 'script').forEach(script => script.remove());
          const copy = document.importNode(item, true);
          enhance(copy, config.wall);
          filterItems([copy]);
          root.append(copy);
          added.push(copy);
        }
        register(added);
        document.dispatchEvent(new CustomEvent('continuous:appended', { detail: { items: added } }));
        pages.add(requested);
        const sourcePath = new URL(requested).pathname;
        const repeatCount = added.length ? 0 : (repeatedSources.get(sourcePath) || 0) + 1;
        repeatedSources.set(sourcePath, repeatCount);
        if (archive) {
          if (following && !pages.has(following)) sources.push(following);
          next = sources.shift() || null;
        } else {
          next = following;
          if ((!next || pages.has(next)) && sources.length) { next = sources.shift(); archive = true; }
        }
        repeats = added.length ? 0 : repeats + 1;
        failed = false;
        status.textContent = '';
        button.textContent = 'Load more';
        if (!next || pages.has(next)) finish();
        else if ((!archive && repeats >= 3) || repeatCount >= 3) {
          failed = true;
          status.textContent = 'The source is returning repeated results. You can try again.';
          button.textContent = 'Try again';
        }
      } catch (_) {
        failed = true;
        status.textContent = 'More posts couldn’t load. Your place is saved here.';
        button.textContent = 'Retry';
        skip.hidden = !archive || !sources.length;
      } finally {
        clearTimeout(timeout);
        busy = false;
        root.removeAttribute('aria-busy');
        button.disabled = false;
        // A short page may leave the sentinel in view without a new observer event.
        if (!stopped && !failed) setTimeout(() => {
          if (bar.getBoundingClientRect().top < innerHeight + 700) load();
        }, 500);
      }
    }
    button.addEventListener('click', () => { failed = false; repeats = 0; repeatedSources.clear(); load(); });
    skip.addEventListener('click', () => { next = sources.shift() || null; failed = false; repeats = 0; load(); });
    if (next) observer.observe(bar); else finish();
  }
  function start() {
    const homeFeed = q(document, '[data-home-feed], [data-profile-feed]');
    if (homeFeed) {
      const register = masonry(homeFeed);
      document.addEventListener('pinless:feed-layout', () => register([...homeFeed.children]));
      return;
    }
    if (APP === 'tumblr') {
      const selector = '#m.timeline, #m.blog-posts';
      const root = q(document, selector);
      if (!root) return;
      document.body.classList.add('tumblr-wall');
      connect({ root, wall: true,
        sources: () => location.pathname === '/' ? all(document, '.pb-chip:not(.pb-add)[href]').map(e => localURL(e, location.href)).filter(Boolean) : [],
        items: doc => [...(q(doc, selector)?.children || [])],
        next: (doc, base) => localURL(q(doc, '.pb-pagination a.pb-button, .paging a.primary.next-page'), base),
        valid: doc => !!q(doc, selector),
        hide: () => all(document, '.pb-pagination, .paging').forEach(e => { e.hidden = true; })
      });
    } else if (APP === 'instagram') {
      const selector = '.kg-wall, .user-posts, .tile-container, #main > .grid';
      const root = q(document, selector);
      if (!root) return;
      document.body.classList.add('instagram-wall');
      connect({ root, wall: true,
        sources: () => location.pathname === '/' ? all(document, '.kg-chip:not(.kg-add)[href]').map(e => { const url = new URL(e.href); url.searchParams.set('view', 'timeline'); return url.href; }) : [],
        items: doc => [...(q(doc, selector)?.children || [])],
        next: (doc, base) => localURL(q(doc, '.kg-pagination a.kg-button, #main > a.next-button'), base),
        valid: doc => !!q(doc, selector) || !!q(doc, '[data-wall-empty]'),
        hide: () => all(document, '.kg-pagination, #main > a.next-button').forEach(e => { e.hidden = true; })
      });
    } else if (APP === 'pinless') {
      ['.grid-container', '.pins-grid', '.boards-grid'].forEach(selector => {
        const root = q(document, selector);
        if (!root) return;
        // The pin template owns failed/empty related results. Do not turn an
        // unavailable source or a missing featured image into an end-of-feed.
        if (root.matches('.pin-detail-wall') && q(document, '.source-error, .no-related')) {
          masonry(root);
          return;
        }
        const paging = node => {
          const sibling = node?.nextElementSibling;
          return sibling?.matches('.pagination-container') ? sibling : null;
        };
        connect({ root, wall: true,
          items: doc => [...(q(doc, selector)?.children || [])].filter(item => doc === document || !item.classList.contains('pin-featured')),
          next: (doc, base) => localURL(q(paging(q(doc, selector)) || doc.createElement('div'), 'a'), base),
          valid: doc => !!q(doc, selector) || (!!q(doc, '.search-container') && !q(doc, '.error')),
          hide: () => { const nav = paging(root); if (nav) nav.hidden = true; }
        });
      });
    } else if (APP === 'youtube') {
      // Watch pages have independent recommendation/player behavior.
      if (!/^\/(search|feed\/|channel\/|playlist|hashtag\/)/.test(location.pathname)) return;
      const cards = doc => [...new Set(all(doc, '.video-card-row').map(e => e.closest('[class*="pure-u-"]')).filter(Boolean))];
      const initial = cards(document);
      if (!initial.length) return;
      const root = initial[0].parentElement;
      connect({ root, wall: false, items: cards,
        next: (doc, base) => {
          const link = q(doc, '.page-next-container a');
          if (link) return localURL(link, base);
          // Invidious search can return a short page and omit a valid continuation.
          const u = new URL(base);
          if (u.pathname === '/search' && cards(doc).length) {
            u.searchParams.set('page', String((parseInt(u.searchParams.get('page'), 10) || 1) + 1));
            return u.href;
          }
          return null;
        },
        valid: doc => !!q(doc, '#contents') && !q(doc, '#error_message'),
        hide: () => all(document, '.page-next-container, .page-prev-container').forEach(e => { e.hidden = true; })
      });
    }
  }
  document.addEventListener('visibilitychange', () => {
    if (APP !== 'tumblr' && APP !== 'instagram') return;
    all(document, '.media-wall video').forEach(video => {
      const rect = video.getBoundingClientRect();
      if (document.hidden) video.pause();
      else if (!reducedMotion && rect.bottom > 0 && rect.top < innerHeight && rect.height) video.play().catch(() => {});
    });
  });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, { once: true });
  else start();
})();

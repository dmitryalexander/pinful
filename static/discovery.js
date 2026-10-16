/* Shared header and the profile page feed. */
(() => {
  'use strict';
  // A board link on a profile page ("/user/board/") becomes one more source for the profile feed.
  function boardSource(pathname) {
    const parts = pathname.split('/').filter(Boolean);
    if (parts.length !== 2 || !parts.every(part => /^[a-z0-9_-]+$/i.test(part)) || /^(pin|search|ideas|settings|business|about|static)$/i.test(parts[0]) || ['_saved', '_created'].includes(parts[1])) return null;
    return {path:'/' + parts.map(part => part.toLowerCase()).join('/') + '/', label:parts.join(' / '), board:true};
  }
  function start() {
    const form = document.querySelector('form[action="/search/pins/"]');
    if (!form) return;
    document.body.classList.add('pt-youtube', 'pinless-discovery');
    const oldHeader = form.closest('.search-container, .container');
    const header = document.createElement('header');
    header.className = 'pt-header';
    const home = document.createElement('a');
    home.className = 'pt-brand'; home.href = '/'; home.textContent = 'P';
    home.title = 'Pinful home'; home.setAttribute('aria-label', 'Pinful home');
    const innerSearch = form.querySelector('.search');
    if (innerSearch) form.replaceChildren(...innerSearch.childNodes);
    form.className = 'pt-search';
    form.querySelector('input').setAttribute('aria-label', 'Search Pinterest');
    form.querySelector('button').setAttribute('aria-label', 'Search');
    const nav = document.createElement('div'); nav.className = 'pt-nav';
    const menu = document.createElement('details'); menu.className = 'pt-menu';
    const summary = document.createElement('summary');
    summary.textContent = 'M'; summary.title = 'Menu'; summary.setAttribute('aria-label', 'Menu');
    const panel = document.createElement('div'); panel.className = 'quiet-menu-panel';
    for (const [label, href] of [['Home feed','/'],['About','/about'],['Source','https://github.com/bunk-im/pinless']]) {
      const link = document.createElement('a');
      link.className = 'pt-menu-item'; link.textContent = label; link.href = href; panel.append(link);
    }
    const refresh = document.createElement('button');
    refresh.type = 'button'; refresh.className = 'pt-menu-item'; refresh.textContent = 'Refresh feed';
    refresh.addEventListener('click', () => {
      menu.open = false;
      if (document.querySelector('[data-profile-feed]')) document.dispatchEvent(new Event('pinless:refresh-feed'));
      else location.href = '/';
    });
    panel.prepend(refresh);
    menu.append(summary, panel); nav.append(menu); header.append(home, form, nav);
    if (oldHeader) oldHeader.replaceWith(header); else document.body.prepend(header);
    document.addEventListener('click', event => {if (!menu.contains(event.target)) menu.open = false;});
    document.addEventListener('keydown', event => {
      if (event.key === 'Escape' && menu.open) {menu.open = false; summary.focus();}
    });

    const wall = document.querySelector('[data-profile-feed]');
    if (!wall || new URLSearchParams(location.search).has('pt_page')) return;
    const profileFeed = wall.hasAttribute('data-profile-feed');
    const feedSources = () => [{term:document.querySelector('.profile-name')?.textContent || 'Saved pins',path:location.pathname,profile:true}];
    const warning = null;
    const status = document.querySelector('#feed-status');
    const more = document.querySelector('#feed-more');
    const seen = new Set();
    const knownSources = new Set();
    let pool = [], queue = [], retries = [], generation = 0, busy = false, failed = false, controller;
    window.PinlessContinuation = () => [...queue,...retries].map(source => ({url:source.path + '?' + new URLSearchParams(source.profile && !source.profileBoards ? {pins_next:'',pins_cursor:source.cursor} : {next:'',cursor:source.cursor}), profile:source.profile}));
    const shuffle = terms => terms.map(term => ({term, sort:Math.random()})).sort((a,b) => a.sort-b.sort).map(item => item.term);
    function reset(terms) {
      pool = [...terms]; generation++;
      if (controller) controller.abort();
      queue = shuffle(pool).map(source => ({...source, cursor:'', pages:new Set()}));
      retries = []; knownSources.clear(); queue.forEach(source => knownSources.add(source.path));
      seen.clear(); wall.replaceChildren(); wall.setAttribute('aria-busy', 'false'); busy = false; failed = false;
      more.hidden = !queue.length; more.disabled = false; more.textContent = 'Load more';
      status.textContent = queue.length ? 'Loading your feed…' : 'Nothing to load here.';
      document.dispatchEvent(new Event('pinless:feed-layout'));
      if(warning) warning.hidden=!queue.length;
      if (queue.length) load();
    }
    async function load() {
      if (busy || !queue.length) return;
      busy = true; failed = false; more.disabled = true;
      wall.setAttribute('aria-busy', 'true');
      const current = generation;
      const batch = [];
      try {
        const requests = Math.min(3, queue.length);
        for (let i = 0; i < requests; i++) {
          const source = queue.shift();
          if (!source) break;
          status.textContent = 'Loading ' + source.term + '…';
          const requestController = new AbortController(); controller = requestController;
          const timeout = setTimeout(() => requestController.abort(), 65000);
          let response;
          try {
            response = await fetch(source.path + '?' + new URLSearchParams(source.profile && !source.profileBoards ? {pins_next:'', pins_cursor:source.cursor, cursor:''} : {next:'', cursor:source.cursor, pins_cursor:''}), {credentials:'same-origin', signal:requestController.signal});
            if (!response.ok || !response.headers.get('content-type')?.includes('text/html')) throw new Error('Source unavailable');
            const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
            if (current !== generation) return;
            let results = doc.querySelector(source.profile ? '.pins-grid' : '.grid-container');
            if (source.profile) {
              const boards = doc.querySelector('.boards-grid');
              for (const link of boards?.querySelectorAll('a[href]') || []) {
                const url = new URL(link.getAttribute('href'), location.origin);
                if (url.origin !== location.origin || knownSources.has(url.pathname)) continue;
                const board = boardSource(url.pathname); if (!board) continue;
                knownSources.add(board.path);
                queue.push({term:board.label,path:board.path,profile:false,cursor:'',pages:new Set()});
              }
              const nextBoardLink=boards?.nextElementSibling?.querySelector('a[href]');
              if (!source.profileBoards && nextBoardLink) {
                const u=new URL(nextBoardLink.getAttribute('href'),location.origin),cursor=u.searchParams.get('cursor');
                const key='boards:'+source.path+':'+cursor;
                if(cursor&&!knownSources.has(key)){knownSources.add(key);queue.push({...source,profileBoards:true,cursor,pages:new Set()});}
              }
              if(source.profileBoards)results=boards;
            }
            if (doc.querySelector('.source-error') || (!results && !doc.querySelector(source.profile ? '.profile-header' : '.board-header'))) throw new Error('Source unavailable');
            const items = [...(results?.children || [])].filter(item => {
              const link = item.querySelector('a[href^="/pin/"]');
              const id = link && new URL(link.getAttribute('href'), location.origin).pathname;
              if (!id || seen.has(id)) return false;
              seen.add(id); return true;
            });
            batch.push(items);
            source.pages.add(source.cursor);
            const nextLink = results?.nextElementSibling?.matches('.pagination-container') ? results.nextElementSibling.querySelector('a') : null;
            const nextURL = nextLink && new URL(nextLink.getAttribute('href'), location.origin);
            const cursor = nextURL?.searchParams.get(source.profile && !source.profileBoards ? 'pins_cursor' : 'cursor');
            if (cursor && source.pages.has(cursor)) retries.push(source);
            if (cursor && !source.pages.has(cursor) && nextURL.origin === location.origin && nextURL.pathname === source.path) {
              source.cursor = cursor; queue.push(source);
            }
          } catch (_) {
            if (current !== generation) return;
            retries.push(source);
          } finally { clearTimeout(timeout); }
        }
        if (current !== generation) return;
        failed = !queue.length && retries.length > 0;
        status.textContent = retries.length ? retries.length + ' sources could not load. Other sources continue; use Retry to try those again.' : queue.length ? '' : seen.size ? 'All currently accessible public pins and boards have been checked.' : 'No public pins were returned. Private or unavailable boards cannot be shown here.';
        more.textContent = failed ? 'Retry' : 'Load more';
        more.hidden = !queue.length && !retries.length;
      } catch (_) {
        if (current !== generation) return;
        failed = true;
        status.textContent = 'A source could not load. Loaded pins are kept. Retry continues with the next source.';
        more.textContent = 'Retry'; more.hidden = false;
      } finally {
        if (current === generation) {
          // Mix each fetched topic into the same wall instead of grouping by topic.
          // Completed results survive if a later topic fails.
          const fragment = document.createDocumentFragment();
          while (batch.some(items => items.length)) {
            for (const items of batch) if (items.length) fragment.append(items.shift());
          }
          wall.append(fragment);
          document.dispatchEvent(new Event('continuous:appended'));
          document.dispatchEvent(new Event('pinless:feed-layout'));
          busy = false; more.disabled = false; wall.setAttribute('aria-busy', 'false');
          if(warning)warning.hidden=wall.children.length>0 || !queue.length || failed;
          if (!failed && queue.length && document.querySelector('.continuous-status').getBoundingClientRect().top < innerHeight + 500) {
            setTimeout(() => {if (current === generation) load();}, 300);
          }
        }
      }
    }
    more.addEventListener('click', () => {if (!queue.length && retries.length) {queue = retries; retries = [];} more.textContent = 'Load more'; load();});
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting) && !failed) load();
    }, {rootMargin:'500px 0px'});
    observer.observe(document.querySelector('.continuous-status'));
    document.addEventListener('pinless:refresh-feed', () => reset(feedSources()));
    reset(feedSources());
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, {once:true});
  else start();
})();

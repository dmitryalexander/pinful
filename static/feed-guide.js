/* Feed guide (October 15, 2026): a boxed section on the Menu page, right under Favorites, that explains the Feed
   (the Discover wall of pins) and the keys that work on it. Text only; the behavior lives in modern.js
   (loadDiscovery(), shuffleWall(), the keydown handler, snapshot() for Restore) and player.js (hover previews). */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-feed-guide{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 0;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-feed-guide h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-feed-guide h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-feed-guide h3{margin:18px 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-feed-guide p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-feed-guide .pt-feed-guide-note{margin:0 0 4px;font-size:12px;color:#8f9188}
.pn-panel .pt-feed-guide-row{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}
.pn-panel .pt-feed-guide-card{box-sizing:border-box;flex:1 1 220px;max-width:300px;padding:10px 12px;background:#222;border:1px solid #45453e;border-radius:10px}
.pn-panel .pt-feed-guide-card[data-zone=rose]{border-left:4px solid #b96170}
.pn-panel .pt-feed-guide-card[data-zone=blue]{border-left:4px solid #608faf}
.pn-panel .pt-feed-guide-card[data-zone=gray]{border-left:4px solid #929292}
.pn-panel .pt-feed-guide-card[data-zone=green]{border-left:4px solid #5aa36a}
.pn-panel .pt-feed-guide-card strong{display:block;margin:6px 0 2px;font:600 13px/1.35 system-ui;color:#eee}
.pn-panel .pt-feed-guide-card[data-zone] strong{margin-top:0}
.pn-panel .pt-feed-guide-card span{display:block;font:12px/1.5 system-ui;color:#a9aba1}
.pn-panel .pt-feed-guide kbd{display:inline-block;min-width:20px;padding:1px 7px;border:1px solid #6a6a60;border-bottom-width:2px;border-radius:5px;background:#2c2c28;color:#eee;font:600 12px/1.5 system-ui;text-align:center}
.pn-panel .pt-feed-guide button.pt-feed-guide-try{box-sizing:border-box;display:inline-block;width:auto;margin:10px 8px 0 0;padding:8px 16px;background:#222;color:#eee;border:1px solid #45453e;border-radius:22px;font:13px system-ui;cursor:pointer}
.pn-panel .pt-feed-guide button.pt-feed-guide-try:hover,.pn-panel .pt-feed-guide button.pt-feed-guide-try:focus-visible{border-color:#e9b344;color:#e9b344;outline:none}`;
  document.head.append(style);

  const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text) node.textContent = text; return node; };
  const keys = names => { const box = document.createDocumentFragment(); names.forEach((name, i) => { if (i) box.append(' + '); box.append(el('kbd', '', name)); }); return box; };
  function card(keyNames, title, text) {
    const box = el('div', 'pt-feed-guide-card'), top = el('div');
    top.append(keys(keyNames));
    box.append(top, el('strong', '', title), el('span', '', text));
    return box;
  }
  function zone(color, title, text) {
    const box = el('div', 'pt-feed-guide-card');
    box.dataset.zone = color;
    box.append(el('strong', '', title), el('span', '', text));
    return box;
  }
  function row(...cards) { const r = el('div', 'pt-feed-guide-row'); r.append(...cards); return r; }

  function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-feed-guide')) return;
    // Wait (up to four seconds) for the Favorites box so the Feed box always lands right under it.
    if (!panel.querySelector('.pt-fav-guide') && tries < 40) { setTimeout(() => build(tries + 1), 100); return; }
    const section = el('section', 'pt-feed-guide');
    section.setAttribute('aria-labelledby', 'pt-feed-guide-title');
    const title = el('h2'); title.id = 'pt-feed-guide-title';
    // The same magnifying glass as the Feed button in the top bar.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5.5 5.5"/></svg>';
    title.append('Feed');
    section.append(title,
      el('p', '', 'The Feed is the wall of pins you browse: a mix drawn from the search terms in your active search group. Open it with the magnifying glass at the top left, next to the search group’s name. Opening it again deals a fresh mix.'));

    section.append(el('h3', '', 'How the wall fills up'),
      el('p', '', 'Pinful takes a few of your terms at a time and weaves their results together, so no single term crowds out the others. Scroll down and more pins load by themselves, or press Load more at the bottom. If some searches fail you get a Retry button, and the pins you already have stay put. When a term runs out of results the wall simply ends. A group with no terms shows a note instead: point at the search bar at the top and press A to add one.'),
      el('p', '', 'Pins you hid with X never come back on a wall, and pins already saved in a playlist hover green.'));

    section.append(el('h3', '', 'What a click does'),
      el('p', 'pt-feed-guide-note', 'Point at a pin and a coloured border shows what a click will do:'));
    section.append(row(
      zone('rose', 'Top strip (rose)', 'On an image or GIF, click to open the full-size image in a new tab. On a video, click to download it; click again to cancel. D over the tile does the same.'),
      zone('blue', 'Second strip (blue)', 'On an image or GIF, click to search that image with Google Lens in a new tab (or hover and press G). On a video, click to open the pin’s own page, with its details and related pins. O opens the pin page for either kind.'),
      zone('gray', 'The rest (gray)', 'An image opens its pin page. A video plays right in its tile; Space over it plays or pauses. Rest the mouse on its lower part for a moment and a silent preview starts (the Hover previews button above turns this off).'),
      zone('green', 'Green border', 'The pin is already in one of your playlists.')));

    section.append(el('h3', '', 'Keys on the wall'),
      el('p', 'pt-feed-guide-note', 'Keep the mouse over the pin or the wall and press a key. Keys are ignored while you type in a box or a pop-up is open, and they don’t work over the top bar.'));
    section.append(row(
      card(['G'], 'Google Lens', 'Point at an image or GIF and press G to search it with Google Lens. This is the image’s blue-strip action; it does not run on videos.'),
      card(['O'], 'Open pin', 'Point at an image or video and press O to open its pin page, with details and related pins.'),
      card(['D'], 'Full-size image or video download', 'Point at an image or GIF and press D to open the full-size image. On a video, D downloads it, or cancels the current download.'),
      card(['Space'], 'Play or pause video', 'Point at a video and press Space. A silent hover preview becomes normal playback; Space again pauses or resumes. Over an image, Space keeps its normal scrolling behavior.'),
      card(['A'], 'Save or unsave', 'Adds the pin you are pointing at to your active playlist (its name is next to the star). Press A again and it comes back out.'),
      card(['X'], 'Hide a pin', 'Press X, then X again within a few seconds. The pin disappears from every wall and search. Esc or moving away cancels the first press. Nothing is deleted from the web; it is only hidden in this browser.'),
      card(['S'], 'Shuffle', 'Mixes the order of the pins already loaded on the wall. Nothing new is fetched.'),
      card(['Z'], 'Fullscreen', 'For a video you clicked to play. Press Z again to leave fullscreen.'),
      card(['Esc'], 'Cancel', 'Cancels a half-finished X and leaves fullscreen.')));

    section.append(el('h3', '', 'Bravely companion for Google Lens'),
      el('p', '', 'Pinful can send an image to Google Lens using the blue strip or G. Bravely is the optional Brave extension companion: it gives Lens visual matches the same quiet image-wall experience, and adds matching controls on Pinterest, DeviantArt and pixiv. Pinful works without it. Bravely’s extra website and local-app access start off and can be enabled in its options when needed.'));
    const companion = el('a', '', 'Get Bravely on GitHub');
    companion.href = 'https://github.com/dmitryalexander/bravely';
    companion.target = '_blank'; companion.rel = 'noopener noreferrer';
    section.append(companion);

    section.append(el('h3', '', 'Getting back to your wall'),
      el('p', '', 'The diamond button at the top right (Restore wall) brings back the last wall you were looking at in this browser tab, with the same pins and the same scroll position. Use it after opening a pin or wandering off, so you don’t lose your place in a mix you can’t easily get back.'));

    section.append(el('h3', '', 'Good to know'),
      el('p', '', 'The Feed is only as good as your search group. Change what shows up by switching group (click its name at the top) or by adding and removing terms. The Search groups box below explains the keys for that, and the Search tags box shows how to narrow results to gifs, videos or tall images. Playing or opening a pin adds it to History; scrolling past it does not.'));
    const open = el('button', 'pt-feed-guide-try', 'Open the Feed');
    open.type = 'button';
    open.addEventListener('click', () => { location.href = '/?pt_page=discover'; });
    section.append(open);

    // Order on the Menu: Favorites, Feed, Search tags, Search groups, History.
    const next = panel.querySelector('.pt-search-tags, .pt-group-guide, .pt-history-guide');
    if (next) panel.insertBefore(section, next); else panel.append(section);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

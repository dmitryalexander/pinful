/* Favorites guide (October 15, 2026): the first boxed section on the Menu page. It explains the Favorites feed
   (the Playlists page, opened with the star): saving pins with A, choosing the active playlist, and the keys on the
   page. Text only; the behavior lives in modern.js (lists(), playlistPicker(), updateSaved(), movePin()). */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-fav-guide{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 0;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-fav-guide h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-fav-guide h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-fav-guide h3{margin:18px 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-fav-guide p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-fav-guide .pt-fav-guide-note{margin:0 0 4px;font-size:12px;color:#8f9188}
.pn-panel .pt-fav-guide-row{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}
.pn-panel .pt-fav-guide-card{box-sizing:border-box;flex:1 1 220px;max-width:300px;padding:10px 12px;background:#222;border:1px solid #45453e;border-radius:10px}
.pn-panel .pt-fav-guide-card strong{display:block;margin:6px 0 2px;font:600 13px/1.35 system-ui;color:#eee}
.pn-panel .pt-fav-guide-card span{display:block;font:12px/1.5 system-ui;color:#a9aba1}
.pn-panel .pt-fav-guide kbd{display:inline-block;min-width:20px;padding:1px 7px;border:1px solid #6a6a60;border-bottom-width:2px;border-radius:5px;background:#2c2c28;color:#eee;font:600 12px/1.5 system-ui;text-align:center}
.pn-panel .pt-fav-guide button.pt-fav-guide-try{display:inline-block;margin:10px 8px 0 0;padding:8px 16px;background:#222;border:1px solid #45453e;border-radius:22px;color:#eee;font:13px system-ui;cursor:pointer}
.pn-panel .pt-fav-guide button.pt-fav-guide-try:hover,.pn-panel .pt-fav-guide button.pt-fav-guide-try:focus-visible{border-color:#e9b344;color:#e9b344;outline:none}`;
  document.head.append(style);

  const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text) node.textContent = text; return node; };
  const keys = names => { const box = document.createDocumentFragment(); names.forEach((name, i) => { if (i) box.append(' + '); box.append(el('kbd', '', name)); }); return box; };
  function card(keyNames, title, text) {
    const box = el('div', 'pt-fav-guide-card'), top = el('div');
    top.append(keys(keyNames));
    box.append(top, el('strong', '', title), el('span', '', text));
    return box;
  }
  function row(...cards) { const r = el('div', 'pt-fav-guide-row'); r.append(...cards); return r; }

  function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-fav-guide')) return;
    const section = el('section', 'pt-fav-guide');
    section.setAttribute('aria-labelledby', 'pt-fav-guide-title');
    const title = el('h2'); title.id = 'pt-fav-guide-title';
    // The same star icon as the Favorites button in the top bar.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z"/></svg>';
    title.append('Favorites');
    section.append(title,
      el('p', '', 'Favorites are the pins you keep. They live in playlists: you start with “Save for later” and “Favorites”, and you can make as many more as you like. Open them with the star at the top left. One playlist is always the active one; its name sits right next to the star.'));

    section.append(el('h3', '', 'Saving a pin'),
      el('p', '', 'Point at any pin in a wall (Discover, a search, History, a profile) and press A. It goes into the active playlist and a quick flash confirms it. Press A again and it comes back out. A pin can be in several playlists at once.'),
      el('p', '', 'Once a pin is saved anywhere, pointing at it shows a green border instead of the usual gray one, so you can tell at a glance what you already have. New pins are added at the end of the playlist.'));

    section.append(el('h3', '', 'Choosing and managing playlists'),
      el('p', 'pt-fav-guide-note', 'Click the playlist name next to the star (or Tab to it and press Enter, Space or ↓). Click a playlist, or use ↑ ↓ Home End and Enter, to make it the active one. Esc closes the list.'));
    section.append(row(
      card(['A'], 'New playlist', 'With the list open, press A (or press A while pointing at the closed name). Type a name, Enter to save, Esc to cancel. The new playlist is empty and becomes the active one.'),
      card(['R'], 'Rename', 'Point at a playlist in the open list, or reach it with the arrow keys, and press R. Enter saves, Esc cancels.'),
      card(['X'], 'Delete', 'Point at a playlist and press X, then X again within a few seconds. The last playlist can’t be deleted, so one is always active.')));
    section.append(el('p', 'pt-fav-guide-note', 'Keep the mouse on the list while you press a key: if you move it onto the page, the list closes and the key goes to the page instead. Playlist names must be different (capitals don’t matter).'));
    const label = document.querySelector('.pt-active-playlist');
    if (label) {
      const tryIt = el('button', 'pt-fav-guide-try', 'Open the playlist list to try it'); tryIt.type = 'button';
      tryIt.addEventListener('click', () => label.click());
      section.append(tryIt);
    }

    section.append(el('h3', '', 'On the Favorites page'),
      el('p', 'pt-fav-guide-note', 'It shows the pins of the active playlist, in the order you saved them. Point at a pin and press a key:'));
    section.append(row(
      card(['A'], 'Move to another playlist', 'Picks the pin up and opens the list; click a playlist to move the pin there. A again, or Esc, cancels.'),
      card(['X'], 'Remove from this playlist', 'Press X, then X again. The pin leaves only this playlist; other playlists that hold it are untouched.'),
      card(['S'], 'Shuffle', 'Mixes the order of the pins and keeps the new order. The Shuffle pins button under the wall does the same. Each shuffle replaces the last.')));

    section.append(el('h3', '', 'Good to know'),
      el('p', '', 'Playlists stay in this browser and are also saved in the app’s data, so they survive restarts.'));

    const next = panel.querySelector('.pt-feed-guide, .pt-search-tags, .pt-group-guide, .pt-history-guide');
    if (next) panel.insertBefore(section, next); else panel.append(section);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

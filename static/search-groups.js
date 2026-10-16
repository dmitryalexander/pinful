/* Search groups guide (October 15, 2026): the Menu explains the search group list (the name left of
   the search box) and its A / R / X keys, plus the keys for the search terms inside a group.
   The behavior itself lives in modern.js (playlistPicker) and interest-bar.js; this file only explains it. */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-group-guide{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 0;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-group-guide h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-group-guide h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-group-guide h3{margin:22px 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-group-guide p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-group-guide .pt-group-guide-note{margin:0 0 8px;font-size:12px;color:#8f9188}
.pn-panel .pt-group-guide .pt-group-guide-tip{color:#e9b344}
.pn-panel .pt-group-guide-row{display:flex;flex-wrap:wrap;gap:6px}
.pn-panel .pt-group-guide-card{box-sizing:border-box;flex:1 1 230px;max-width:320px;padding:10px 12px;background:#222;border:1px solid #45453e;border-radius:10px}
.pn-panel .pt-group-guide-card strong{display:block;margin:6px 0 2px;font:600 13px/1.35 system-ui;color:#eee}
.pn-panel .pt-group-guide-card span{display:block;font:12px/1.5 system-ui;color:#a9aba1}
.pn-panel .pt-group-guide kbd{display:inline-block;min-width:20px;padding:1px 7px;border:1px solid #6a6a60;border-bottom-width:2px;border-radius:5px;background:#2c2c28;color:#eee;font:600 12px/1.5 system-ui;text-align:center}
.pn-panel .pt-group-guide button.pt-group-guide-try{display:inline-block;margin:10px 0 0;padding:8px 16px;background:#222;border:1px solid #45453e;border-radius:22px;color:#eee;font:13px system-ui;cursor:pointer}
.pn-panel .pt-group-guide button.pt-group-guide-try:hover,.pn-panel .pt-group-guide button.pt-group-guide-try:focus-visible{border-color:#e9b344;color:#e9b344;outline:none}`;
  document.head.append(style);

  const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text) node.textContent = text; return node; };
  const keys = (...names) => { const box = document.createDocumentFragment(); names.forEach((name, i) => { if (i) box.append(' + '); box.append(el('kbd', '', name)); }); return box; };
  function card(keyNames, title, text) {
    const box = el('div', 'pt-group-guide-card');
    const top = el('div'); top.append(keys(...keyNames));
    box.append(top, el('strong', '', title), el('span', '', text));
    return box;
  }
  function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-group-guide')) return;
    const section = el('section', 'pt-group-guide');
    section.setAttribute('aria-labelledby', 'pt-group-guide-title');
    const title = el('h2'); title.id = 'pt-group-guide-title';
    // Stacked layers: a group of searches.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m12 3 9 5-9 5-9-5Z"/><path d="m3 13 9 5 9-5"/></svg>';
    title.append('Search groups');
    section.append(title,
      el('p', '', 'A search group is a named set of search terms. The chips in the top bar are the terms of the active group, and Discover mixes searches from them. The group name sits just left of the search box; the magnifying glass beside it opens that group’s feed.'));

    section.append(el('h3', '', 'The group list'),
      el('p', 'pt-group-guide-note', 'Click the group name (or Tab to it and press Enter, Space or ↓) to open the list. Click a group, or use ↑ ↓ Home End and Enter, to switch to it. Esc closes the list.'));
    const groupKeys = el('div', 'pt-group-guide-row');
    groupKeys.append(
      card(['A'], 'New group', 'With the list open, press A. It also works while you point at the closed group name. Type a name, press Enter to save or Esc to cancel. The new group starts empty and becomes the active one.'),
      card(['R'], 'Rename', 'Point at a group in the open list (or reach it with the arrow keys) and press R. Change the name, Enter to save, Esc to cancel.'),
      card(['X'], 'Delete', 'Point at a group and press X. The row turns red and asks you to press X again; do it within a few seconds to delete. Anything else cancels it. The last group can’t be deleted, so one is always active.'));
    section.append(groupKeys,
      el('p', 'pt-group-guide-tip', 'Keep the mouse on the list while you press a key. If the mouse has moved off the list onto the page, the list closes and the key goes to the page instead. Group names must be different (capitals don’t matter).'));
    // The group name in the top bar is built by interest-bar.js, possibly after this box, so look it up on click.
    const tryIt = el('button', 'pt-group-guide-try', 'Open the group list to try it');
    tryIt.type = 'button';
    tryIt.addEventListener('click', () => document.querySelector('.pt-active-group')?.click());
    section.append(tryIt);

    section.append(el('h3', '', 'Search terms inside a group'),
      el('p', 'pt-group-guide-note', 'Point the mouse at the search term chips in the top bar and press a key. These don’t work while you are typing in the search box.'));
    const termKeys = el('div', 'pt-group-guide-row');
    termKeys.append(
      card(['A'], 'Add a term', 'A small box opens. Type a search, then Save.'),
      card(['I'], 'Import or edit all', 'Opens the full list of terms in this group, to paste in many at once or fix them.'),
      card(['X'], 'Remove a term', 'Point at a chip and press X: it is marked. Press X again to delete it. Esc cancels.'),
      card(['Ctrl', 'Shift', 'X'], 'Delete all terms', 'Press it twice to empty the active group. Esc cancels.'),
      card(['S'], 'Shuffle', 'Mixes the order of the chips.'));
    section.append(termKeys);

    // Order on the Menu: Favorites, Search tags, Search groups, History.
    const later = panel.querySelector('.pt-history-guide');
    if (later) panel.insertBefore(section, later); else panel.append(section);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

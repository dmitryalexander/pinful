/* Hidden pins guide (October 15, 2026): a boxed section on the Menu page, just above History, that explains hiding
   pins with X and the Hidden pins button/page (restore one with X twice, restore all with Ctrl+Shift+X twice).
   Text only; the behavior lives in modern.js (confirmAction(), applyCards(), the keydown handler, the hidden page). */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-hidden-guide{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 0;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-hidden-guide h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-hidden-guide h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-hidden-guide h3{margin:18px 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-hidden-guide p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-hidden-guide .pt-hidden-guide-note{margin:0 0 4px;font-size:12px;color:#8f9188}
.pn-panel .pt-hidden-guide-row{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}
.pn-panel .pt-hidden-guide-card{box-sizing:border-box;flex:1 1 220px;max-width:300px;padding:10px 12px;background:#222;border:1px solid #45453e;border-radius:10px}
.pn-panel .pt-hidden-guide-card[data-zone=rose]{border-left:4px solid #b96170}
.pn-panel .pt-hidden-guide-card[data-zone=blue]{border-left:4px solid #608faf}
.pn-panel .pt-hidden-guide-card[data-zone=gray]{border-left:4px solid #929292}
.pn-panel .pt-hidden-guide-card[data-zone=green]{border-left:4px solid #5aa36a}
.pn-panel .pt-hidden-guide-card strong{display:block;margin:6px 0 2px;font:600 13px/1.35 system-ui;color:#eee}
.pn-panel .pt-hidden-guide-card[data-zone] strong{margin-top:0}
.pn-panel .pt-hidden-guide-card span{display:block;font:12px/1.5 system-ui;color:#a9aba1}
.pn-panel .pt-hidden-guide kbd{display:inline-block;min-width:20px;padding:1px 7px;border:1px solid #6a6a60;border-bottom-width:2px;border-radius:5px;background:#2c2c28;color:#eee;font:600 12px/1.5 system-ui;text-align:center}
.pn-panel .pt-hidden-guide button.pt-hidden-guide-try{box-sizing:border-box;display:inline-block;width:auto;margin:10px 8px 0 0;padding:8px 16px;background:#222;color:#eee;border:1px solid #45453e;border-radius:22px;font:13px system-ui;cursor:pointer}
.pn-panel .pt-hidden-guide button.pt-hidden-guide-try:hover,.pn-panel .pt-hidden-guide button.pt-hidden-guide-try:focus-visible{border-color:#e9b344;color:#e9b344;outline:none}`;
  document.head.append(style);

  const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text) node.textContent = text; return node; };
  const keys = names => { const box = document.createDocumentFragment(); names.forEach((name, i) => { if (i) box.append(' + '); box.append(el('kbd', '', name)); }); return box; };
  function card(keyNames, title, text) {
    const box = el('div', 'pt-hidden-guide-card'), top = el('div');
    top.append(keys(keyNames));
    box.append(top, el('strong', '', title), el('span', '', text));
    return box;
  }
  function zone(color, title, text) {
    const box = el('div', 'pt-hidden-guide-card');
    box.dataset.zone = color;
    box.append(el('strong', '', title), el('span', '', text));
    return box;
  }
  function row(...cards) { const r = el('div', 'pt-hidden-guide-row'); r.append(...cards); return r; }

  function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-hidden-guide')) return;
    // Wait (up to four seconds) for the Search groups box so this one always lands right under it.
    if (!panel.querySelector('.pt-group-guide') && tries < 40) { setTimeout(() => build(tries + 1), 100); return; }
    const section = el('section', 'pt-hidden-guide');
    section.setAttribute('aria-labelledby', 'pt-hidden-guide-title');
    const title = el('h2'); title.id = 'pt-hidden-guide-title';
    // The same crossed-out eye as the Hidden pins button above.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m3 3 18 18M2 12s4-7 10-7c6 0 10 7 10 7s-4 7-10 7S2 12 2 12Z"/></svg>';
    title.append('Hidden pins');
    section.append(title,
      el('p', '', 'Hiding is how you say “not this one”. Point at a pin on any wall (the Feed, a search, a profile) and press X, then X again: it turns red after the first press (Esc cancels) and disappears from every wall and every search after the second. Nothing is deleted from the web; the pin is only hidden here. The round Hidden pins button at the top of this page opens everything you have hidden.'));

    section.append(el('h3', '', 'The Hidden pins page'),
      el('p', 'pt-hidden-guide-note', 'Click the round Hidden pins button above. Every hidden pin shows up as a wall. Point at a pin and press a key:'));
    section.append(row(
      card(['X'], 'Bring one back', 'Press X, then X again. The pin leaves this list and shows up on walls and searches again.'),
      card(['Ctrl', 'Shift', 'X'], 'Bring them all back', 'Press it twice anywhere on the page. The whole wall turns red and a note asks you to confirm; Esc cancels.'),
      card(['A'], 'Save the pin', 'Adds it to your active playlist while you are looking at it, like on any wall.')));

    section.append(el('h3', '', 'Good to know'),
      el('p', '', 'Hidden pins are saved in this browser and in the app’s data, so they survive restarts, like your playlists and History. Hiding is separate from History and Favorites: a hidden pin can still be in a playlist, and a pin in History is not hidden. Use it for pins you are tired of seeing or never want to run into again.'));

    // Order on the Menu: Favorites, Feed, Search tags, Search groups, Hidden pins, History.
    const next = panel.querySelector('.pt-history-guide');
    if (next) panel.insertBefore(section, next); else panel.append(section);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

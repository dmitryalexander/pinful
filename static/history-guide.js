/* History guide (October 15, 2026): a boxed section on the Menu page that explains what History is and how it
   works. Text only; the behavior lives in modern.js (history(), showHistory(), histMore()). */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-history-guide{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 48px;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-history-guide h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-history-guide h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-history-guide h3{margin:18px 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-history-guide p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-history-guide-row{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}
.pn-panel .pt-history-guide-card{box-sizing:border-box;flex:1 1 220px;max-width:300px;padding:10px 12px;background:#222;border:1px solid #45453e;border-radius:10px}
.pn-panel .pt-history-guide-card strong{display:block;margin:6px 0 2px;font:600 13px/1.35 system-ui;color:#eee}
.pn-panel .pt-history-guide-card span{display:block;font:12px/1.5 system-ui;color:#a9aba1}
.pn-panel .pt-history-guide kbd{display:inline-block;min-width:20px;padding:1px 7px;border:1px solid #6a6a60;border-bottom-width:2px;border-radius:5px;background:#2c2c28;color:#eee;font:600 12px/1.5 system-ui;text-align:center}`;
  document.head.append(style);

  const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text) node.textContent = text; return node; };
  const keys = names => { const box = document.createDocumentFragment(); names.forEach((name, i) => { if (i) box.append(' + '); box.append(el('kbd', '', name)); }); return box; };
  function card(keyNames, title, text) {
    const box = el('div', 'pt-history-guide-card'), top = el('div');
    top.append(keys(keyNames));
    box.append(top, el('strong', '', title), el('span', '', text));
    return box;
  }
  function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-history-guide')) return;
    const section = el('section', 'pt-history-guide');
    section.setAttribute('aria-labelledby', 'pt-history-guide-title');
    const title = el('h2'); title.id = 'pt-history-guide-title';
    // The same book icon as the History button in the top bar.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 5v16M12 5C9 3 5 3 2 4v15c3-1 7-1 10 2 3-3 7-3 10-2V4c-3-1-7-1-10 1Z"/></svg>';
    title.append('History');
    section.append(title,
      el('p', '', 'History is your trail of pins: the ones you opened or played, newest first, so you can find something you saw earlier. Open it with the History icon (the open book) at the top right of every page.'));

    section.append(el('h3', '', 'What gets added'),
      el('p', '', 'A pin goes into History when you open it (click a pin to open its page, or land on a pin page directly) or when you click a video to play it. Scrolling past a pin, pointing at it, or watching a silent hover preview adds nothing.'),
      el('p', '', 'Opening the same pin again moves it back to the top instead of listing it twice. History keeps your latest 1000 pins; the oldest ones drop off the end.'));

    section.append(el('h3', '', 'Using it'),
      el('p', '', 'The History page opens with the newest 40 pins and adds more as you scroll down, so even a long history opens quickly. Point at a pin and press a key:'));
    const row = el('div', 'pt-history-guide-row');
    row.append(
      card(['A'], 'Save the pin', 'Adds it to your active playlist, or takes it out if it is already there. Saved pins hover green.'),
      card(['X'], 'Remove from History', 'Press X, then X again. Only the History entry goes; the pin itself and your playlists are untouched. Esc or moving away cancels.'),
      card(['Ctrl', 'Shift', 'X'], 'Clear all History', 'Press it twice anywhere on the History page. Esc cancels. This cannot be undone.'));
    section.append(row);

    section.append(el('h3', '', 'Good to know'),
      el('p', '', 'History stays in this browser and is also saved in the app’s data, like your playlists, so it survives restarts. Pins you hid with X on a wall don’t show in History either.'));

    section.append(el('h3', '', 'Pausing it'),
      el('p', '', 'The round Watch history button at the top of this page turns recording off: while it is off, pins you open or play are not added to History. What is already there stays, and you can still remove it with X.'));

    panel.append(section); // last on the Menu: Favorites, Feed, Search tags, Search groups, Hidden pins, History
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

/* Search tags guide (October 14, 2026): the Menu lists every #tag the search box understands,
   grouped, with what it does. Click one to add it to the search box, then press Enter.
   The tags themselves are read by the server (tags.go); /search-tags.json lists them. */
(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const style = document.createElement('style');
  style.textContent = `
.pn-panel .pt-search-tags{box-sizing:border-box;width:min(1000px,100%);margin:32px auto 0;padding:18px 22px 20px;color:#ddd;text-align:left;background:#1b1b1a;border:1px solid #55554b;border-radius:14px}
.pn-panel .pt-search-tags h2{display:flex;align-items:center;gap:10px;margin:0 0 8px;font:500 20px/1.3 system-ui;color:#eee}
.pn-panel .pt-search-tags h2 svg{flex:0 0 auto;color:#e9b344}
.pn-panel .pt-search-tags p{margin:6px 0;font:13px/1.6 system-ui;color:#b9bbb2}
.pn-panel .pt-search-tags .pt-search-tags-hint{color:#e9b344}
.pn-panel .pt-search-tags-group{margin-top:20px}
.pn-panel .pt-search-tags h3{margin:0 0 2px;font:600 14px/1.4 system-ui;color:#e0d9c7}
.pn-panel .pt-search-tags .pt-search-tags-note{margin:0 0 8px;font-size:12px;color:#8f9188}
.pn-panel .pt-search-tags-row{display:flex;flex-wrap:wrap;gap:6px}
.pn-panel .pt-search-tags button.pt-search-tag{display:flex;flex-direction:column;align-items:flex-start;gap:2px;min-width:140px;max-width:260px;margin:0;padding:8px 12px;background:#222;border:1px solid #45453e;border-radius:10px;color:#ccc;font:12px/1.35 system-ui;text-align:left;cursor:pointer}
.pn-panel .pt-search-tags button.pt-search-tag strong{font-size:13px;color:#eee}
.pn-panel .pt-search-tags button.pt-search-tag small{font-size:11px;color:#8f9188}
.pn-panel .pt-search-tags button.pt-search-tag:hover,.pn-panel .pt-search-tags button.pt-search-tag:focus-visible{border-color:#e9b344;outline:none}
.pn-panel .pt-search-tags button.pt-search-tag:hover strong,.pn-panel .pt-search-tags button.pt-search-tag:focus-visible strong{color:#e9b344}`;
  document.head.append(style);

  function input() {
    return document.querySelector('.pt-search input[name="q"]') || document.querySelector('form[action="/search/pins/"] input[name="q"]');
  }
  function add(name) {
    const box = input();
    if (!box) return;
    const words = box.value.trim().split(/\s+/).filter(Boolean);
    if (!words.some(word => word.toLowerCase() === '#' + name)) words.push('#' + name);
    box.value = words.join(' ');
    box.dispatchEvent(new Event('input', { bubbles: true }));
    box.focus();
    box.setSelectionRange(box.value.length, box.value.length);
  }
  async function build(tries = 0) {
    const panel = document.querySelector('.pn-panel');
    if (!panel) { if (tries < 40) setTimeout(() => build(tries + 1), 100); return; }
    if (panel.querySelector('.pt-search-tags')) return;
    const section = document.createElement('section');
    section.className = 'pt-search-tags';
    section.setAttribute('aria-labelledby', 'pt-search-tags-title');
    const title = document.createElement('h2');
    title.id = 'pt-search-tags-title';
    // A hash sign, like a tag.
    title.innerHTML = '<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 9h16M4 15h16M10 3 8 21M16 3l-2 18"/></svg>';
    title.append('Search tags');
    section.append(title);
    for (const text of [
      'Type tags into the search box with your words, like tiktok dance #videos #tall. Capitals don’t matter.',
      'Tags in the same group are alternatives: #gifs #videos shows GIFs or videos. Different groups narrow each other: #videos #tall shows tall videos.',
      'Tags work in search groups and search terms too. Unknown tags stay part of the search; put a tag in quotes, like "#gifs", to search for the word itself.',
      'Examples: sunset #gifs (GIFs only) · city skyline #images #wide (wide pictures only) · dance #videos #tall (tall videos only).'
    ]) { const p = document.createElement('p'); p.textContent = text; section.append(p); }
    const hint = document.createElement('p');
    hint.className = 'pt-search-tags-hint';
    hint.textContent = 'Click a tag to add it to the search box, then press Enter.';
    section.append(hint);
    // Order on the Menu: Favorites, Search tags, Search groups, History.
    const later = panel.querySelector('.pt-group-guide, .pt-history-guide');
    if (later) panel.insertBefore(section, later); else panel.append(section);
    let groups = [];
    try {
      const response = await fetch('/search-tags.json', { cache: 'no-store' });
      groups = (await response.json()).groups || [];
    } catch (_) {
      hint.textContent = 'The tag list could not load. Reload the page to try again.';
      return;
    }
    for (const group of groups) {
      const block = document.createElement('div');
      block.className = 'pt-search-tags-group';
      const heading = document.createElement('h3');
      heading.textContent = group.name;
      const note = document.createElement('p');
      note.className = 'pt-search-tags-note';
      note.textContent = group.help;
      const row = document.createElement('div');
      row.className = 'pt-search-tags-row';
      for (const tag of group.tags || []) {
        const chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'pt-search-tag';
        const label = document.createElement('strong');
        label.textContent = '#' + tag.name;
        const what = document.createElement('span');
        what.textContent = tag.help;
        chip.append(label, what);
        if (tag.aliases && tag.aliases.length) {
          const also = document.createElement('small');
          also.textContent = 'also ' + tag.aliases.map(alias => '#' + alias).join(' ');
          chip.append(also);
        }
        chip.title = 'Add #' + tag.name + ' to the search box';
        chip.addEventListener('click', () => add(tag.name));
        row.append(chip);
      }
      block.append(heading, note, row);
      section.append(block);
    }
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => build(), { once: true });
  else build();
})();

/* Shared tutorial presentation: input badge, quiet title, then explanatory copy. */
;(() => {
  'use strict';
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  const guides = '.pt-guide,.pt-feed-guide,.pt-fav-guide,.pt-group-guide,.pt-history-guide,.pt-hidden-guide';
  const cards = '.pt-hotkeys-card,.pt-feed-guide-card,.pt-fav-guide-card,.pt-group-guide-card,.pt-history-guide-card,.pt-hidden-guide-card';
  function clickCap(text = 'CLICK') {
    const cap = document.createElement('span'); cap.className = 'pt-input-cap'; cap.textContent = text; return cap;
  }
  function format(guide) {
    guide.classList.add('pt-tutorial');
    guide.querySelectorAll(cards).forEach(card => {
      if (card.classList.contains('pt-tutorial-card')) return;
      const title = [...card.children].find(node => node.tagName === 'STRONG');
      if (!title) return;
      let inputs = [...card.children].find(node => node.tagName === 'DIV' && node.querySelector('kbd,svg'));
      if (!inputs && card.dataset.zone && card.dataset.zone !== 'green') {
        inputs = document.createElement('div'); inputs.append(clickCap()); card.prepend(inputs);
      }
      if (inputs) {
        inputs.classList.add('pt-tutorial-keys');
        if (card.classList.contains('pt-hotkeys-wide')) inputs.append(' + ', clickCap());
        card.classList.add('pt-tutorial-with-input');
      }
      title.classList.add('pt-tutorial-title'); card.classList.add('pt-tutorial-card');
    });
    const walker = document.createTreeWalker(guide, NodeFilter.SHOW_TEXT), texts = [];
    while (walker.nextNode()) {
      const text = walker.currentNode;
      if (!text.parentElement.closest('kbd,.pt-input-cap,.pt-tutorial-keys,strong,h1,h2,h3,button,a,script,style,code,pre') && /\bclick\b/i.test(text.data)) texts.push(text);
    }
    for (const text of texts) {
      const fragment = document.createDocumentFragment(); let at = 0;
      for (const match of text.data.matchAll(/\bclick\b/gi)) {
        fragment.append(text.data.slice(at, match.index), clickCap(match[0])); at = match.index + match[0].length;
      }
      fragment.append(text.data.slice(at)); text.replaceWith(fragment);
    }
  }
  function start() {
    const root = document.querySelector('#contents') || document.body;
    let pending = false;
    const scan = () => { pending = false; root.querySelectorAll(guides).forEach(format); };
    scan();
    new MutationObserver(() => {
      if (pending) return; pending = true; requestAnimationFrame(scan);
    }).observe(root, {childList:true,subtree:true});
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, {once:true}); else start();
})();

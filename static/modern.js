/* Pinful navigation, local collections and discovery. No Pinterest account required. */
(() => {
'use strict';
// The search group dropdown (interest-bar.js) reuses the playlist picker.
window.ptMenuPicker = (label, opts) => playlistPicker(label, opts);
const $ = (s,r=document)=>r.querySelector(s), $$=(s,r=document)=>[...r.querySelectorAll(s)];
const KEYS={lists:'pinless-playlists-v1',history:'pinless-history-v1',hidden:'pinless-hidden-v1',snapshot:'pinless-wall-snapshot-v1',filter:'pinless-media-filter-v1'};
const svg=p=>'<svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+p+'</svg>';
const icons={restore:'<path d="m12 2 10 10-10 10L2 12Z"/><circle cx="12" cy="12" r="3"/>',filters:'<path d="M3 5h18l-9 15Z"/>',discover:'<path d="m12 2 3 7 7 3-7 3-3 7-3-7-7-3 7-3Z"/>',lists:'<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z"/>',history:'<path d="M12 5v16M12 5C9 3 5 3 2 4v15c3-1 7-1 10 2 3-3 7-3 10-2V4c-3-1-7-1-10 1Z"/>',menu:'<path d="M4 6h16M4 12h16M4 18h16"/>'};
const shuffle=a=>{a=[...a];for(let i=a.length-1;i>0;i--){const j=Math.floor(Math.random()*(i+1));[a[i],a[j]]=[a[j],a[i]];}return a;};
const read=(key,fallback)=>{const raw=localStorage.getItem(key);return raw===null?fallback:JSON.parse(raw);};
let notice;
function report(message){if(!notice){notice=document.createElement('div');notice.className='pn-notice';notice.setAttribute('role','status');document.body.append(notice);}notice.textContent=message;notice.hidden=false;clearTimeout(report.timer);report.timer=setTimeout(()=>notice.hidden=true,5000);}
function write(key,value){try{localStorage.setItem(key,JSON.stringify(value));return true;}catch(_){report('Could not save. Browser storage may be full or unavailable.');return false;}}
function lists(){const d=read(KEYS.lists,{activeId:'later',lists:[{id:'later',name:'Save for later',items:[]},{id:'favorites',name:'Favorites',items:[]}]});if(!d.lists?.length||!d.lists.some(l=>l.id===d.activeId)||!d.lists.every(l=>typeof l.name==='string'&&Array.isArray(l.items)))throw Error('Your playlists could not be read. Export a backup before resetting browser data.');return d;}
const validPinId=id=>/^(?:\d+|e1_[A-Za-z0-9_-]+)$/.test(String(id||''));
function record(card){const link=$('a[href^="/pin/"]',card),img=$('img',card),video=$('video',card);const id=card.dataset.featuredPin || (link && new URL(link.getAttribute("href"),location.origin).pathname.match(/^\/pin\/([^/]+)\/?$/)?.[1]);if(!validPinId(id))return null;return {id,title:card.dataset.title||img?.alt||video?.getAttribute('aria-label')||'Pin',image:img?.getAttribute('src')||video?.poster||'',width:Number(img?.getAttribute('width'))||400,height:Number(img?.getAttribute('height'))||500,video:video?.dataset.originalSource||video?.getAttribute('src')||''};}
function safeMedia(src){try{const u=new URL(src,location.origin);return u.origin===location.origin&&['/image','/video','/hls'].includes(u.pathname)||u.protocol==='https:'&&/(^|\.)pinimg\.com$/.test(u.hostname);}catch(_){return false;}}
function cardFor(item){const card=document.createElement('div');card.dataset.title=String(item.title||'Pin');const link=document.createElement('a');link.href='/pin/'+encodeURIComponent(item.id);if(item.video&&safeMedia(item.video)){const v=document.createElement('video');v.controls=true;v.playsInline=true;v.preload='none';v.src=item.video;if(safeMedia(item.image))v.poster=item.image;v.setAttribute('aria-label',item.title||'Pin video');card.append(v);link.textContent='Open pin';card.append(link);}else{const img=document.createElement('img');if(safeMedia(item.image))img.src=item.image;img.alt=item.title||'Saved pin';img.width=item.width||400;img.height=item.height||500;link.append(img);card.append(link);}return card;}
  /* Playlist picker: click the playlist name next to the star (top left, every page, the Playlists
     page included) to choose which playlist A saves to. Arrow keys move, Enter picks, Escape closes.
     In the open menu, point at (or focus) a playlist and press A for a new playlist, R to rename it,
     or X twice to delete it. The last playlist can't be deleted, so one is always active.
     A while pointing at the closed name also starts a new playlist.
     opts: read() -> data, save(data) -> bool, refresh(), select(id), here, move() -> function(id) or null */
  function playlistPicker(label, opts) {
    // Search groups pass noun, nouns, menuLabel and blank(); the playlists use the defaults.
    const NOUN = opts.noun || 'playlist', NOUNS = opts.nouns || 'playlists', MENU = opts.menuLabel || 'Save to playlist';
    const blank = opts.blank || (() => ({items: []}));
    if (!label || label.dataset.picker) return;
    if (opts.here) label.classList.add('pt-playlist-here');
    label.dataset.picker = '1';
    label.setAttribute('role', 'button'); label.tabIndex = 0;
    label.setAttribute('aria-haspopup', 'menu'); label.setAttribute('aria-expanded', 'false');
    const menu = document.createElement('div');
    menu.className = 'pt-playlist-picker'; if (opts.menuClass) menu.classList.add(opts.menuClass); menu.setAttribute('role', 'menu'); menu.setAttribute('aria-label', MENU); menu.hidden = true;
    const hint = document.createElement('p'); hint.className = 'pt-picker-hint'; hint.setAttribute('role', 'status');
    document.body.append(menu);
    const HINT = '', EDIT_HINT = '', MOVE_HINT = 'Pick a playlist to move the pin to';
    let hovered = null, inMenu = false, overLabel = false, movedSinceOpen = false, armed = null, armTimer = 0, noteTimer = 0, editing = null;
    const moving = () => opts.move ? opts.move() : null;
    const items = () => [...menu.querySelectorAll('button[data-list-id]')];
    const baseHint = () => editing ? EDIT_HINT : moving() ? MOVE_HINT : HINT;
    function showHint(text, warn) {
      clearTimeout(noteTimer); hint.textContent = text || baseHint(); hint.classList.toggle('pt-picker-warn', !!warn);
      if (warn) noteTimer = setTimeout(() => showHint(), 2600);
    }
    function disarm() {
      clearTimeout(armTimer);
      if (!armed) return;
      armed.classList.remove('pt-list-delete-armed'); armed = null; showHint();
    }
    function close(focusLabel, quiet) {
      if (menu.hidden) return;
      menu.hidden = true; label.setAttribute('aria-expanded', 'false');
      clearTimeout(armTimer); armed = null; editing = null; hovered = null; inMenu = false;
      removeEventListener('keydown', keys, true);
      if (!quiet) label.dispatchEvent(new CustomEvent('pt-picker-close'));
      if (focusLabel) label.focus({preventScroll: true});
    }
    function render(focusId) {
      let data; try { data = opts.read(); } catch (_) { return false; }
      clearTimeout(armTimer); armed = null; editing = null; hovered = null;
      menu.classList.toggle('pt-move-targets', !!moving());
      menu.setAttribute('aria-label', moving() ? 'Move pin to playlist' : MENU);
      menu.replaceChildren(...data.lists.map(list => {
        const item = document.createElement('button'); item.type = 'button'; item.dataset.listId = list.id;
        item.setAttribute('role', 'menuitemradio'); item.setAttribute('aria-checked', String(list.id === data.activeId));
        const name = document.createElement('span'); name.className = 'pt-picker-name'; name.textContent = list.name;
        item.append(name); item.title = list.name;
        item.addEventListener('click', event => {
          event.stopPropagation();
          const move = moving();
          if (move) { move(list.id); close(true); return; }
          close(true);
          if (list.id !== data.activeId) opts.select(list.id);
        });
        return item;
      }), hint);
      showHint();
      const focus = (focusId && items().find(item => item.dataset.listId === focusId)) || menu.querySelector('[aria-checked="true"]') || items()[0];
      focus?.focus({preventScroll: true}); focus?.scrollIntoView({block: 'nearest'});
      return true;
    }
    function position() {
      const box = label.getBoundingClientRect();
      menu.style.top = Math.round(box.bottom + 6) + 'px';
      menu.style.left = Math.round(Math.max(8, Math.min(box.left, innerWidth - menu.offsetWidth - 8))) + 'px';
    }
    function open() {
      if (!menu.hidden) { render(); return; }
      menu.hidden = false;
      if (!render()) { menu.hidden = true; return; }
      label.setAttribute('aria-expanded', 'true'); movedSinceOpen = false;
      position();
      addEventListener('keydown', keys, true);
    }
    function startEdit(list) {
      if (menu.hidden) open();
      if (editing) render();
      disarm();
      const row = document.createElement('div'); row.className = 'pt-picker-edit';
      const input = document.createElement('input'); input.maxLength = 80; input.autocomplete = 'off'; input.spellcheck = false;
      input.placeholder = list ? 'Rename ' + NOUN : 'New ' + NOUN + ' name'; input.setAttribute('aria-label', input.placeholder);
      input.value = list ? list.name : '';
      row.append(input);
      const at = list && items().find(item => item.dataset.listId === list.id);
      if (at) at.replaceWith(row); else menu.insertBefore(row, hint);
      editing = {id: list ? list.id : null, input};
      showHint();
      input.focus({preventScroll: true}); input.select(); row.scrollIntoView({block: 'nearest'});
      // Clicking elsewhere in the menu abandons the edit.
      input.addEventListener('blur', () => setTimeout(() => { if (editing && editing.input === input && !menu.hidden && document.activeElement !== input) render(editing.id); }, 0));
    }
    function commitEdit() {
      const {id, input} = editing, name = input.value.replace(/\s+/g, ' ').trim();
      if (!name) { input.focus(); return; }
      try {
        const data = opts.read();
        if (data.lists.some(list => list.id !== id && list.name.toLowerCase() === name.toLowerCase())) { showHint('A ' + NOUN + ' with that name already exists.', true); return; }
        let focusId = id;
        if (id) { const list = data.lists.find(list => list.id === id); if (!list) throw new Error('Playlist unavailable'); list.name = name; }
        else { focusId = crypto.randomUUID(); data.lists.push({id: focusId, name, ...blank()}); data.activeId = focusId; }
        if (!opts.save(data)) { showHint('Could not save. Check browser storage and try again.', true); return; }
        render(focusId); opts.refresh();
      } catch (_) { showHint('Could not save. Check browser storage and try again.', true); }
    }
    function remove(item) {
      const id = item.dataset.listId;
      let data; try { data = opts.read(); } catch (_) { showHint('Could not read ' + NOUNS + '. Check browser storage.', true); return; }
      const index = data.lists.findIndex(list => list.id === id);
      if (index < 0) { render(); return; }
      if (data.lists.length < 2) { disarm(); showHint('The last ' + NOUN + ' can’t be deleted.', true); return; }
      if (armed !== item) {
        disarm(); armed = item; item.classList.add('pt-list-delete-armed');
        showHint('Press X again to delete “' + data.lists[index].name + '”', true);
        clearTimeout(noteTimer); armTimer = setTimeout(disarm, 4000); return;
      }
      data.lists.splice(index, 1);
      const next = data.lists[Math.min(index, data.lists.length - 1)].id;
      if (data.activeId === id) data.activeId = next;
      if (!opts.save(data)) { disarm(); showHint('Could not delete. Check browser storage and try again.', true); return; }
      render(next); opts.refresh();
    }
    // While open, the menu owns the keyboard so page shortcuts (A, S, X...) never fire behind it,
    // unless the pointer has moved off to the page: then the key goes to whatever it points at.
    function keys(event) {
      if (editing && document.activeElement === editing.input) {
        if (event.key === 'Enter') { event.preventDefault(); commitEdit(); }
        else if (event.key === 'Escape') { event.preventDefault(); render(editing.id); }
        event.stopPropagation(); event.stopImmediatePropagation(); return;
      }
      const key = event.key.toLowerCase(), list = items(), at = list.indexOf(document.activeElement);
      const letter = /^[a-z]$/.test(key);
      if (letter && movedSinceOpen && !inMenu && !overLabel) { close(false, true); return; }
      const plain = !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey && !event.isComposing && !event.repeat;
      const target = (inMenu && hovered && hovered.isConnected) ? hovered : (at >= 0 ? list[at] : null);
      const free = !moving();
      if (event.key === 'Escape') { event.preventDefault(); if (armed) disarm(); else close(true); }
      else if (event.key === 'Tab') close(false);
      else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault(); disarm();
        list[(at + (event.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length]?.focus();
      } else if (event.key === 'Home' || event.key === 'End') { event.preventDefault(); disarm(); list[event.key === 'Home' ? 0 : list.length - 1]?.focus(); }
      else if (plain && key === 'a' && free) { event.preventDefault(); startEdit(null); }
      else if (plain && key === 'r' && target && free) {
        event.preventDefault();
        try { startEdit(opts.read().lists.find(item => item.id === target.dataset.listId)); } catch (_) {}
      }
      else if (plain && key === 'x' && target && free) { event.preventDefault(); remove(target); }
      else if (key !== 'x') disarm();
      event.stopPropagation(); event.stopImmediatePropagation();
    }
    menu.addEventListener('pointermove', event => {
      inMenu = true;
      const item = event.target.closest('button[data-list-id]');
      if (item !== hovered) { if (armed && item !== armed) disarm(); hovered = item; }
    }, {passive: true});
    menu.addEventListener('pointerleave', () => { inMenu = false; hovered = null; disarm(); });
    label.addEventListener('pointerenter', () => { overLabel = true; });
    label.addEventListener('pointerleave', () => { overLabel = false; });
    document.addEventListener('pointermove', () => { if (!menu.hidden) movedSinceOpen = true; }, {passive: true});
    label.addEventListener('click', event => { event.preventDefault(); event.stopPropagation(); menu.hidden ? open() : close(true); });
    label.addEventListener('keydown', event => {
      if (!['Enter', ' ', 'ArrowDown'].includes(event.key) || event.ctrlKey || event.altKey || event.metaKey) return;
      event.preventDefault(); event.stopPropagation(); open();
    });
    // A while pointing at the closed playlist name: new playlist.
    addEventListener('keydown', event => {
      if (!menu.hidden || !overLabel || event.key.toLowerCase() !== 'a' || event.repeat || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || event.isComposing) return;
      if (event.target.closest?.('input,textarea,select,[contenteditable]:not([contenteditable="false"])') || document.querySelector('dialog[open]')) return;
      event.preventDefault(); event.stopPropagation(); event.stopImmediatePropagation();
      startEdit(null);
    }, true);
    label.addEventListener('pt-picker-open', () => open());
    label.addEventListener('pt-picker-hide', () => close(false, true));
    document.addEventListener('pointerdown', event => { if (!menu.hidden && !menu.contains(event.target) && !label.contains(event.target)) close(false); }, true);
    addEventListener('resize', () => close(false));
    addEventListener('scroll', () => { if (menu.hidden) return; if (label.getBoundingClientRect().bottom < 0) close(false); else position(); }, {passive: true});
    addEventListener('blur', () => close(false));
  }
function start(){
 if(location.pathname==='/'&&!location.search){location.replace('/?pt_page=lists');return;}
 if((location.pathname==='/'&&new URLSearchParams(location.search).has('feed'))||new URLSearchParams(location.search).get('pt_page')==='following'){location.replace('/?pt_page=discover');return;}
 if(new URLSearchParams(location.search).get('pt_page')==='filters'){location.replace('/');return;}
 const header=$('.pt-header');if(!header)return;
 const page=new URLSearchParams(location.search).get('pt_page')||'',nav=$('.pt-nav',header),form=$('.pt-search',header),home=$('.pt-brand',header);
 let wall=$('[data-profile-feed]')||$('.grid-container,.pins-grid'),pointer=null,armed=null,armTimer,queue=[],busy=false,loaded=new Set(),restored=false,moving=null,lastPlayed=null;
 const managed=!!page, status=$('#feed-status'),more=$('#feed-more');
 const notifyLayout=()=>document.dispatchEvent(new Event('pinless:feed-layout'));
 const refresh=()=>{document.dispatchEvent(new Event('continuous:appended'));notifyLayout();};
 const circle=(name,label,href)=>{const a=document.createElement('a');a.className='pt-circle pn-'+name;a.href=href;a.innerHTML=svg(icons[name]);a.title=label;a.setAttribute('aria-label',label);if(page===name)a.setAttribute('aria-current','page');return a;};
 home.remove();// The image button was the Following feed; Discover has its own magnifier button in the header.
 const restore=circle('restore','Restore wall','/?pt_page=restore');nav.before(restore);// Header order (2026-10-11): the image (Following feed) button was removed on 2026-10-14; Restore stays after the search terms, just before History and Menu.
 const star=circle('lists','Playlists','/?pt_page=lists'),activeLabel=document.createElement('span');activeLabel.className='pt-active-playlist';
 const favorites=document.createElement("div");favorites.className="pt-favorites";favorites.append(star,activeLabel);header.prepend(favorites);const refreshLists=()=>{if(page==='lists')renderLists();else updateSaved();};playlistPicker(activeLabel,{read:lists,save:d=>write(KEYS.lists,d),refresh:refreshLists,here:page==='lists',move:()=>moving?movePin:null,select:id=>{try{const d=lists();if(!d.lists.some(l=>l.id===id))return;d.activeId=id;if(write(KEYS.lists,d))refreshLists();}catch(e){report(e.message);}}});activeLabel.addEventListener('pt-picker-close',()=>{if(moving)cancel();});const menu=$('.pt-menu',nav);menu.before(circle('history','History','/?pt_page=history'));
 // Keep the menu panel available to the shared term manager, but use a dedicated page.
 const summary=$('summary',menu);summary.innerHTML=svg(icons.menu);summary.addEventListener('click',e=>{e.preventDefault();location.href='/?pt_page=menu';});if(page==='menu')summary.setAttribute('aria-current','page');
 function flash(card,removed){const tile=$('.pin-video-tile',card)||card;$('.pn-list-flash',tile)?.remove();const overlay=document.createElement('span');overlay.className='pn-list-flash '+(removed?'pn-flash-remove':'pn-flash-save');overlay.setAttribute('aria-hidden','true');tile.append(overlay);overlay.addEventListener('animationend',()=>overlay.remove(),{once:true});setTimeout(()=>overlay.remove(),600);}
 function updateSaved(){try{const d=lists(),active=d.lists.find(l=>l.id===d.activeId);activeLabel.textContent=active.name;activeLabel.title='Saving to '+active.name+' · click to change · A for a new playlist';const ids=new Set(active.items.map(i=>i.id)),anyIds=new Set(d.lists.flatMap(l=>l.items.map(i=>i.id)));if(wall)for(const card of wall.children){const id=record(card)?.id;card.classList.toggle('pn-saved',ids.has(id));card.classList.toggle('pn-in-list',anyIds.has(id));}}catch(e){report(e.message);}}
 function applyCards(){if(!wall)return;let hidden,filter;try{hidden=new Set(read(KEYS.hidden,[]));filter='all';}catch(e){report('Could not read saved display settings.');return;}
 for(const card of wall.children){const item=record(card);if(!item)continue;card.dataset.pinId=item.id;card.tabIndex=0;card.hidden=(page!=='hidden'&&hidden.has(item.id))||(filter==='images'&&!!item.video)||(filter==='videos'&&!item.video);card.setAttribute('aria-label',item.title+' · A to save · X twice to hide');}
 updateSaved();notifyLayout();}
 document.addEventListener('continuous:appended',applyCards);
 const panel=document.createElement('section');panel.className='pn-panel';
 function setupPanel(title){if(status)status.textContent='';if(more)more.hidden=true;panel.setAttribute('aria-label',title);const heading=document.createElement('h1');heading.textContent=title;panel.append(heading);wall?.before(panel);document.title=title+' · Pinful';}
 function button(label,fn,parent=panel){const b=document.createElement('button');b.type='button';b.textContent=label;b.onclick=()=>{try{fn();}catch(e){report(e.message);}};parent.append(b);return b;}
 function hint(text){const p=document.createElement('p');p.className='pn-hint';p.textContent=text;panel.append(p);return p;}
 const HIST_STEP=40;let hist={items:[],shown:0},histTimer;
 // History loads 40 pins at a time and adds more as you scroll toward the bottom (the list can hold 1000).
 function histMore(count=HIST_STEP){const next=hist.items.slice(hist.shown,hist.shown+count);if(next.length){hist.shown+=next.length;wall.append(...next.map(cardFor));refresh();}const left=hist.items.length-hist.shown;if(more){more.hidden=left<=0;more.textContent='Load more';}if(status)status.textContent=hist.items.length?'':'Nothing here yet.';clearTimeout(histTimer);if(left>0&&more&&more.getBoundingClientRect().top<innerHeight+600)histTimer=setTimeout(histMore,250);}
 function showHistory(items,keep){const had=keep?hist.shown:0;hist={items:items.filter(i=>validPinId(i.id)),shown:0};wall.replaceChildren();histMore(Math.max(had,HIST_STEP));}
 function dropFromHistory(card,id){hist.items=hist.items.filter(i=>i.id!==id);if(card.isConnected){card.remove();hist.shown=Math.max(0,hist.shown-1);}refresh();histMore(0);}
 function showItems(items){wall.replaceChildren(...items.filter(i=>validPinId(i.id)).map(cardFor));refresh();if(status)status.textContent=items.length?'':'Nothing here yet.';}
 function nameDialog(title,initial,submit){const d=document.createElement('dialog');d.className='pn-dialog';d.innerHTML='<form><h2></h2><label>Name<input required maxlength="80" aria-label="Playlist name"></label><p role="alert"></p><button type="submit">Save</button><button type="button">Cancel</button></form>';$('h2',d).textContent=title;$('input',d).value=initial||'';$('[type=button]',d).onclick=()=>d.close();$('form',d).onsubmit=e=>{e.preventDefault();try{const value=$('input',d).value.trim();if(value&&submit(value)!==false)d.close();}catch(e){$('[role=alert]',d).textContent=e.message;}};d.onclose=()=>d.remove();document.body.append(d);d.showModal();$('input',d).focus();}
 let renderLists=()=>{};
 const addPlaylist=()=>nameDialog('New playlist','',name=>{const d=lists();if(d.lists.some(l=>l.name.toLowerCase()===name.toLowerCase()))throw Error('A playlist with this name already exists.');const id=crypto.randomUUID();d.lists.push({id,name,items:[]});d.activeId=id;if(!write(KEYS.lists,d))return false;renderLists();});
 if(page==='terms'){
 location.replace('/?pt_page=discover');if(wall)wall.hidden=true;
 }else if(page==='lists'){
 setupPanel('Playlists');panel.remove();document.body.classList.add('pn-playlists-page');wall.setAttribute('aria-label','Playlist pins');
 // Playlists are switched, added, renamed and deleted in the dropdown by the star; the header middle keeps the search term chips.
 renderLists=()=>{const d=lists();window.scrollTo(0,0);showHistory(d.lists.find(l=>l.id===d.activeId).items);updateSaved();};renderLists();
 if(more){more.onclick=()=>histMore();new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)&&!more.hidden)histMore();},{rootMargin:'600px'}).observe(more);}
 }else if(page==='history'||page==='hidden'){
 setupPanel(page==='history'?'History':'Hidden pins');panel.remove();wall.setAttribute('aria-label',page==='history'?'History':'Hidden pins');
 if(page==='history'){showHistory(read(KEYS.history,[]));if(more){more.onclick=()=>histMore();new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)&&!more.hidden)histMore();},{rootMargin:'600px'}).observe(more);}}else showItems(read('pinless-hidden-records-v1',[]));
 }else if(page==='menu'){
 setupPanel('Menu');panel.querySelector('h1')?.remove();wall.hidden=true;
 {const row=document.createElement('div');row.className='pn-switches';panel.append(row);
 const ICONS={loop:'<path d="M17 2l4 4-4 4"/><path d="M3 11V9a3 3 0 0 1 3-3h15"/><path d="M7 22l-4-4 4-4"/><path d="M21 13v2a3 3 0 0 1-3 3H3"/>',history:'<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1L3.5 8.5M3.5 4v4.5H8"/><path d="M12 7.5V12l3 2"/>',preview:'<path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>',hidden:'<path d="m3 3 18 18M2 12s4-7 10-7c6 0 10 7 10 7s-4 7-10 7S2 12 2 12Z"/>'};
 const make=(tag,icon,label,title)=>{const b=document.createElement(tag);b.className='pn-switch';if(tag==='button')b.type='button';if(title)b.title=title;const i=document.createElement('span');i.className='pn-switch-icon';i.innerHTML='<svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'+ICONS[icon]+'</svg>';const t=document.createElement('span');t.className='pn-switch-label';t.textContent=label;b.append(i,t);row.append(b);return {b,t};};
 const toggle=(icon,label,key,def,title)=>{const {b,t}=make('button',icon,label,title);const get=()=>{try{const v=localStorage.getItem(key);return v===null?def:v==='on';}catch(_){return def;}};const paint=()=>{const on=get();t.textContent=label+': '+(on?'on':'off');b.setAttribute('aria-pressed',on?'true':'false');};b.onclick=()=>{try{localStorage.setItem(key,get()?'off':'on');}catch(_){report('Could not save. Allow browser storage and try again.');}if(key==='pinless-loop-video')for(const video of document.querySelectorAll('video:not([data-hover-preview])'))video.loop=get();paint();};paint();};
 toggle('loop','Loop video','pinless-loop-video',false,'Loop video: a video you play starts over when it ends.');
 toggle('history','Watch history','pinless-watch-history',true,'Watch history: when off, pins you open or play are not added to History.');
 toggle('preview','Hover previews','pinless-hover-previews',true,'Hover previews: rest the mouse on a video for a silent preview.');
 const hid=make('a','hidden','Hidden pins','Pins you hid with X. Open the list to bring them back.');hid.b.href='/?pt_page=hidden';}
 }
 // Discover progresses through the active term group, keeping per-search pagination.
 async function loadDiscovery(){if(busy||!queue.length)return;busy=true;if(more){more.disabled=true;more.hidden=false;}let failures=[];const groups=[];const batch=queue.splice(0,4);for(const source of batch){try{const response=await fetch(source.url,{signal:AbortSignal.timeout(65000)});if(!response.ok)throw Error('Search unavailable');const doc=new DOMParser().parseFromString(await response.text(),'text/html');const root=$('.grid-container,.pins-grid,.boards-grid',doc);if(!root)throw Error('Search unavailable');if(root.matches('.boards-grid'))for(const a of $$('a[href]',root)){const u=new URL(a.getAttribute('href'),location.origin);if(u.origin===location.origin&&!queue.some(s=>s.url===u.pathname))queue.push({url:u.pathname});}const items=[...root.children].map(record).filter(Boolean);groups.push(shuffle(items));const a=root.nextElementSibling?.querySelector('a[href]');if(a){const u=new URL(a.getAttribute('href'),location.origin);if(u.origin===location.origin&&u.pathname===new URL(source.url,location.origin).pathname&&u.href!==new URL(source.url,location.origin).href)queue.push({url:u.pathname+u.search});}}catch(_){failures.push(source);}}
 const picks=[];while(groups.some(g=>g.length))for(const group of groups){const item=group.shift();if(item&&!loaded.has(item.id)){loaded.add(item.id);picks.push(cardFor(item));}}wall.append(...picks);
 queue.push(...failures);busy=false;refresh();if(status)status.textContent=failures.length?'Some searches could not load. Retry keeps your current pins.':!wall.children.length?'No pins found. Try adding more search terms.':queue.length?'':'End of the available results.';if(more){more.disabled=false;more.hidden=!queue.length;more.textContent=failures.length?'Retry':'Load more';}snapshot();if(!failures.length&&queue.length&&more&&more.getBoundingClientRect().top<innerHeight+300)setTimeout(loadDiscovery,300);}
 if(page==='discover'){
  if(status)status.textContent='Finding pins from your search groups…';if(more){more.hidden=true;more.onclick=loadDiscovery;}
  (async()=>{try{const d=read('pinless-search-term-lists-v1',null);let terms=d?d.lists.find(l=>l.id===d.activeId).terms:read('pinless-recommended-searches-v1',[]);queue=shuffle(terms).map(term=>({url:'/search/pins/?q='+encodeURIComponent(term)}));if(!queue.length){status.textContent='This search group has no terms yet. Point at the search bar and press A to add one.';return;}await loadDiscovery();}catch(e){status.textContent=e.message;}})();
 }
 if(['discover','restore'].includes(page)&&more){const observer=new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)&&more.textContent!=='Retry')loadDiscovery();},{rootMargin:'400px'});observer.observe(more);}
 function snapshot(){if(!wall||!['','discover','restore'].includes(page)||wall.hidden)return;const items=[...wall.children].map(record).filter(Boolean);if(!items.length)return;try{sessionStorage.setItem(KEYS.snapshot,JSON.stringify({items,queue:window.PinlessContinuation?window.PinlessContinuation():queue,source:page==='restore'?'restore':location.pathname+location.search,scroll:scrollY}));}catch(_){report('This wall is too large to save for Restore.');}}
 if(page==='restore'){
  try{const data=JSON.parse(sessionStorage.getItem(KEYS.snapshot)||'null');if(data){restored=true;queue=data.queue||[];showItems(data.items||[]);loaded=new Set((data.items||[]).map(i=>i.id));if(more){more.hidden=!queue.length;more.onclick=loadDiscovery;}requestAnimationFrame(()=>requestAnimationFrame(()=>scrollTo(0,data.scroll||0)));if(status)status.textContent=queue.length?'':'Restored your last wall.';}else{if(status)status.textContent='Browse or search for pins first, then use Restore to return to that wall.';if(more)more.hidden=true;}}catch(_){report('The saved wall could not be restored.');}
 }
 function shuffleWall(){if(!wall||wall.hidden)return;if(page==='lists'){try{const d=lists(),active=d.lists.find(l=>l.id===d.activeId);if(active.items.length<2)return;const previous=active.items.map(i=>i.id);active.items=shuffle(active.items);if(active.items.every((i,n)=>i.id===previous[n]))active.items.push(active.items.shift());if(write(KEYS.lists,d)){renderLists();report('Shuffled '+active.name+'. The new order is saved.');}}catch(e){report(e.message);}return;}const children=[...wall.children].filter(c=>!c.dataset.creatorSide);wall.append(...shuffle(children));notifyLayout();snapshot();report('Shuffled '+children.length+' loaded pins.');}
 if(wall&&!wall.hidden&&page!=='terms'){
 const actions=document.createElement('div');actions.className='pn-wall-actions';wall.after(actions);button('Shuffle pins',shuffleWall,actions);
 const updateShuffle=()=>{actions.hidden=page!=='lists'&&[...wall.children].filter(c=>!c.hidden&&!c.dataset.creatorSide).length<2;};updateShuffle();document.addEventListener('continuous:appended',updateShuffle);
 }
 function movePin(destId){const pending=moving;if(!pending)return;try{const d=lists(),source=d.lists.find(x=>x.id===pending.sourceId),dest=d.lists.find(x=>x.id===destId),item=source?.items.find(i=>i.id===pending.id);if(item&&dest&&source!==dest){if(!dest.items.some(i=>i.id===item.id))dest.items.push(item);source.items=source.items.filter(i=>i.id!==item.id);if(write(KEYS.lists,d)){cancel();renderLists();return;}}}catch(e){report(e.message);}cancel();}
 function cancel(){moving=null;$$('.pn-moving').forEach(c=>c.classList.remove('pn-moving'));clearTimeout(armTimer);armed=null;$$('.pn-armed').forEach(el=>el.classList.remove('pn-armed'));document.body.classList.remove('pn-clear-armed');}
 function confirmAction(id,element,action){if(armed===id){cancel();action();}else{cancel();armed=id;element.classList.add('pn-armed');report('Press X again to confirm · Escape cancels');armTimer=setTimeout(cancel,4000);}}
 document.addEventListener('pointermove',e=>{pointer={x:e.clientX,y:e.clientY};if(armed&&!e.target.closest('.pn-armed')&&!document.body.classList.contains('pn-clear-armed'))cancel();},{passive:true});window.addEventListener('blur',()=>{pointer=null;cancel();});
 function history(item){if(!item)return;try{if(localStorage.getItem('pinless-watch-history')==='off')return;const items=read(KEYS.history,[]);write(KEYS.history,[item,...items.filter(i=>i.id!==item.id)].slice(0,1000));}catch(_){report('Could not save history.');}}
 // Dynamic walls carry their public-media context into same-tab and new-tab navigation.
 if(wall && !/^\/pin\//.test(location.pathname)) {
  function sourcePins(target) {
   const id=target.pathname.match(/^\/pin\/([^/]+)\/?$/)?.[1];if(!validPinId(id))return null;
   let items;
   if(page==='lists') {const data=lists();items=data.lists.find(l=>l.id===data.activeId)?.items||[];}
   else items=[...wall.children].filter(card=>!card.hidden&&!card.dataset.creatorSide).map(record).filter(Boolean);
   const selected=items.find(i=>i.id===id);
   return selected?[selected,...items.filter(i=>i.id!==id)].slice(0,80):null;
  }
  async function navigatePin(target,pins,newTab) {
   // Open during the click so the browser permits the new tab before the fetch.
   const tab=newTab?window.open('about:blank','_blank'):null;
   if(newTab&&!tab)return;
   if(tab)tab.opener=null;
   try {
    const response=await fetch('/saved-wall-context',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pins})});
    if(!response.ok)throw Error('Previous wall unavailable');
    const result=await response.json();if(!/^[a-f0-9]{32}$/.test(result.context||''))throw Error('Invalid wall context');
    target.searchParams.set('wall_context',result.context);
   }catch(_){}finally{if(tab)tab.location.replace(target.href);else location.assign(target.href);}
  }
  document.addEventListener('click',e=>{
   if(e.defaultPrevented||e.button!==0||e.altKey||(e.shiftKey&&!e.ctrlKey&&!e.metaKey))return;
   const link=e.target.closest('a[href^="/pin/"]');if(!link)return;
   const target=new URL(link.getAttribute('href'),location.origin);
   if(target.searchParams.has('wall_context'))return;
   const pins=sourcePins(target);if(!pins)return;
   e.preventDefault();return navigatePin(target,pins,!!(e.ctrlKey||e.metaKey));
  },true);
 }
 document.addEventListener('click',e=>{const card=e.target.closest('[data-pin-id]');if(card&&e.target.closest('a[href^="/pin/"]')){const item=record(card);history(item);}},true);
 window.addEventListener('storage',e=>{if(e.key==='pinless-loop-video'||e.key===null)for(const video of document.querySelectorAll('video:not([data-hover-preview])'))video.loop=localStorage.getItem('pinless-loop-video')==='on';});
 document.addEventListener('play',e=>{if(e.target.matches('video')&&!e.target.dataset.hoverPreview){try{e.target.loop=localStorage.getItem('pinless-loop-video')==='on';}catch(_){}lastPlayed=e.target;history(record(e.target.closest('[data-pin-id]')||e.target.parentElement));}},true);
 if(/^\/pin\/\d+/.test(location.pathname)){const featured=$('.pin-featured');if(featured)history(record(featured));}
 document.addEventListener('keydown',e=>{
 const key=e.key.toLowerCase();if(key==='escape'){cancel();return;}if(e.defaultPrevented||e.repeat||e.altKey||e.metaKey||e.isComposing||e.target.closest('input,textarea,select,[contenteditable]:not([contenteditable=false])')||$('dialog[open]'))return;
 const target=pointer?document.elementFromPoint(pointer.x,pointer.y):document.activeElement;
 if((page==='history'||page==='hidden')&&key==='x'&&e.ctrlKey&&e.shiftKey){e.preventDefault();if(armed==='clear'){cancel();if(page==='history'){if(write(KEYS.history,[]))showHistory([]);}else if(write(KEYS.hidden,[])&&write('pinless-hidden-records-v1',[])){showItems([]);report('Hidden pins restored.');}}else{cancel();armed='clear';document.body.classList.add('pn-clear-armed');report(page==='history'?'Press Ctrl+Shift+X again to clear history · Escape cancels':'Press Ctrl+Shift+X again to restore all hidden pins · Escape cancels');}return;}if(e.ctrlKey||e.shiftKey)return;
 if(header.contains(target)||page==='terms')return;
 if(key==='s'){e.preventDefault();shuffleWall();return;}
 if(key==='z'){const playing=lastPlayed?.isConnected?lastPlayed:null;if(playing){e.preventDefault();const promise=document.fullscreenElement?document.exitFullscreen():playing.requestFullscreen();promise?.catch(()=>report('Fullscreen is unavailable.'));}return;}
 const card=target?.closest('[data-pin-id]');if(!card)return;const item=record(card);if(!item)return;
 if(key==='a'&&page==='lists'){e.preventDefault();if(moving?.id===item.id){cancel();activeLabel.dispatchEvent(new CustomEvent('pt-picker-hide'));return;}cancel();moving={id:item.id,sourceId:lists().activeId};card.classList.add('pn-moving');activeLabel.dispatchEvent(new CustomEvent('pt-picker-open'));return;}
 if(key==='a'){e.preventDefault();try{const d=lists(),active=d.lists.find(l=>l.id===d.activeId),exists=active.items.some(i=>i.id===item.id);active.items=exists?active.items.filter(i=>i.id!==item.id):[...active.items,item];if(write(KEYS.lists,d)){flash(card,exists);updateSaved();if(page==='lists'&&exists)setTimeout(renderLists,250);}}catch(e){report(e.message);}}
 if(key==='x'){e.preventDefault();confirmAction('pin:'+item.id,card,()=>{if(page==='lists'){const d=lists();d.lists.find(l=>l.id===d.activeId).items=d.lists.find(l=>l.id===d.activeId).items.filter(i=>i.id!==item.id);if(write(KEYS.lists,d)){flash(card,true);setTimeout(renderLists,260);}}else if(page==='history'){const next=read(KEYS.history,[]).filter(i=>i.id!==item.id);if(write(KEYS.history,next))dropFromHistory(card,item.id);}else if(page==='hidden'){if(write(KEYS.hidden,read(KEYS.hidden,[]).filter(id=>id!==item.id))&&write('pinless-hidden-records-v1',read('pinless-hidden-records-v1',[]).filter(i=>i.id!==item.id)))showItems(read('pinless-hidden-records-v1',[]));}else{const ids=read(KEYS.hidden,[]),records=read('pinless-hidden-records-v1',[]);if(write(KEYS.hidden,[...new Set([...ids,item.id])])&&write('pinless-hidden-records-v1',[item,...records.filter(i=>i.id!==item.id)])){applyCards();snapshot();}}});}
 });
 window.addEventListener('pagehide',snapshot);document.addEventListener('continuous:appended',()=>{clearTimeout(snapshot.timer);snapshot.timer=setTimeout(snapshot,200);});
 window.addEventListener('storage',e=>{if(e.key===KEYS.lists){if(page==='lists')renderLists();else updateSaved();}if(e.key===KEYS.hidden||e.key===KEYS.filter)applyCards();if(e.key===KEYS.history&&page==='history')showHistory(read(KEYS.history,[]),true);});
 applyCards();updateSaved();
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',()=>{try{start();}catch(e){console.error(e);report(e.message);}}, {once:true});else start();
})();

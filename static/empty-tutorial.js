/* Page-specific introductions; saved data and populated walls are untouched. */
;(() => {
 'use strict';
 const page = new URLSearchParams(location.search).get('pt_page') || '';
 const guides = {
  lists:['welcome','A home for the pins you keep.','This playlist is empty. Save images, GIFs and videos you want to return to.','Hover a pin anywhere and press A to save it to the playlist named beside the star. Choose another playlist with that name’s menu.','Find a pin','search'],
  history:['history','Your travels leave a trail.','Pins you open or play appear here so you can find your way back. Scrolling past a pin adds nothing.','Search for a pin and open it to begin. The Watch history switch in Menu pauses recording. Removing history never removes a saved pin.','Find something to explore','search'],
  hidden:['hidden','A little room for a second chance.','There are no hidden pins here. This page holds pins you have set aside from your walls.','On a pin, press X twice to hide it. Here, X twice brings it back. Your playlists and history are kept separately.','Explore your search groups','discover'],
  discover:['thinking','Give this feed a few interests.','Your search group mixes pins from the terms you choose. There are no pins to show here yet.','Point at the dotted interests bar and press A to add a term, or I to edit several. If a search could not load, its Retry control stays below.','Find a starting pin','search'],
  terms:['terms','Plant a few ideas.','This search group has no terms yet. Give it a hobby, a mood, a creator or a place you would like to explore.','Point over this page and press A to add one term, or I to paste a list. The magnifying glass opens a feed made from those interests.','Open your search-group feed','discover'],
  restore:['restore-meme','Pick up where you left off.','There is no wall saved in this browser tab yet. Restore brings back your last browsing wall with its pins and scroll position.','Browse a search or a search-group feed, open a pin, then use the diamond to return. This is a temporary bookmark, separate from your saved playlists.','Start exploring','discover'],
  search:['search','Try another trail.','This search has no pins to show right now. Try fewer words, a different phrase, or fewer filter tags.','Change the words in Search and press Enter. Any source error or Retry control remains visible below; an unavailable search does not change your saved pins.','Try another search','search'],
  profile:['following-trophy-down','A creator’s corner, waiting to fill.','There are no accessible pins on this creator page right now. Pinterest may not provide every profile or private board.','Try another creator or search for a topic. Opening this empty page does not change any of your saved information.','Search for pins','search'],
  board:['ideas','Room for this board’s pins.','This board has no accessible pins to show right now. It may be empty, private, or temporarily unavailable.','Any loading or source message remains below. You can explore another search while keeping your saved collection.','Explore another search','search'],
  pin:['search','This pin is not available here yet.','No image or video is available on this page right now. The source may be unavailable or the pin may have been removed.','Keep any source message below in mind and try another search. Your saved pins and login connection stay as they are.','Find another pin','search']
 };
 function chooseKind(path, selected, profile, board) {
  if (selected === 'menu') return null;
  if (guides[selected]) return selected;
  if (path.startsWith('/pin/')) return 'pin';
  if (path.startsWith('/search/')) return 'search';
  if (profile) return 'profile';
  if (board) return 'board';
  return selected || path !== '/' ? null : 'discover';
 }
 function sync() {
  const kind=chooseKind(location.pathname,page,!!document.querySelector('.profile-header,.pn-profile-hero'),!!document.querySelector('.board-header'));
  const old=document.getElementById('pn-empty-guide');
  if (!kind) { old?.remove(); return; }
  let wall=document.querySelector(kind==='terms'?'.pt-term-wall':'[data-profile-feed],.grid-container,.pins-grid,.pn-empty-page-anchor');
  if (!wall && ['pin','profile','board'].includes(kind)) { wall=document.createElement('div');wall.className='pn-empty-page-anchor';(document.querySelector('main')||document.body).append(wall); }
  if (!wall || wall.hidden) { old?.remove(); return; }
  const status=document.querySelector('#feed-status');
  const loading=wall.getAttribute('aria-busy')==='true' || /^(?:Loading|Finding)/i.test(status?.textContent || '');
  const hasContent=[...wall.querySelectorAll(kind==='terms'?'.pt-term-tile':'[data-pin-id],a[href^="/pin/"],video,img')].some(el=>!el.closest('[hidden],[data-creator-side]'));
  const featured=kind==='pin' && document.querySelector('.pin-featured img,.pin-featured video');
  if (loading || hasContent || featured) { old?.remove(); return; }
  if (old?.dataset.kind===kind) return;
  old?.remove();
  const [art,title,intro,detail,action,target]=guides[kind];
  const section=document.createElement('section');section.id='pn-empty-guide';section.className='pn-empty-guide';section.dataset.kind=kind;section.setAttribute('aria-labelledby','pn-empty-title');
  const img=document.createElement('img');img.src='/static/tutorial-'+art+'.png';img.alt='';img.width=240;img.height=420;
  const artLink=document.createElement('a');artLink.className='pn-empty-art';artLink.href='https://ynforma.art';artLink.target='_blank';artLink.rel='noopener noreferrer';artLink.title='ynforma.art';artLink.append(img);
  const copy=document.createElement('div'),eyebrow=document.createElement('p'),heading=document.createElement('h1'),body=document.createElement('p'),note=document.createElement('p');
  eyebrow.className='pn-empty-eyebrow';eyebrow.textContent='A little help getting started · '+(kind==='lists'?'Playlists':kind==='discover'?'Search groups':kind);
  heading.id='pn-empty-title';heading.textContent=title;body.textContent=intro;note.textContent=detail;
  const button=document.createElement(target==='search'?'button':'a');button.textContent=action;button.className='pn-empty-action';
  if(target==='search'){button.type='button';button.onclick=()=>{const input=document.querySelector('.pt-search input,#search');input?.focus();input?.select();};}else button.href='/?pt_page='+target;
  copy.append(eyebrow,heading,body,note,button);section.append(artLink,copy);wall.before(section);
 }
 window.pinfulEmptyGuides={sync,chooseKind};
 function start(){sync();let scheduled=false;new MutationObserver(()=>{if(scheduled)return;scheduled=true;requestAnimationFrame(()=>{scheduled=false;sync();});}).observe(document.body,{childList:true,subtree:true,characterData:true,attributes:true,attributeFilter:['hidden','aria-busy']});}
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start();
})();

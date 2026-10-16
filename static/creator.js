window.PinlessLoadingWarning = wall => {const warning=document.createElement('section');warning.className='pt-loading-warning';warning.setAttribute('aria-label','Image browsing advisory');warning.innerHTML="<div class=\"pt-warning-art\"><img src=\"/static/warning.png\" alt=\"Skull and crossbones cassette warning sticker\"></div><h1>WARNING</h1><div class=\"pt-warning-rating\"><div class=\"pt-warning-grade\"><strong>AA</strong><span>seriously addicting</span></div><strong class=\"pt-warning-heading\">COMPULSIVE IMAGE BROWSING CAN DISRUPT DAILY LIFE</strong><div class=\"pt-warning-body\"><p>Repeated, prolonged image browsing can crowd out sleep, focused work, physical activity and time with other people. Image recommendations and endless scrolling make it easy to continue browsing beyond the time you intended. When browsing becomes difficult to control, it can interfere with responsibilities, relationships and emotional well-being.</p><p>Using image feeds to repeatedly avoid stress or difficult feelings can leave the underlying problems unresolved. Watch for lost sleep, neglected commitments, reduced concentration and unsuccessful attempts to cut back. Pay attention to the effect on your everyday life and the time you are choosing to spend here.</p></div><p class=\"pt-warning-fine\">SET A TIME LIMIT BEFORE YOU BROWSE. PAUSE SCROLLING. TAKE REGULAR BREAKS. IF BROWSING IS DIFFICULT TO CONTROL OR CAUSES DISTRESS, CONSIDER SEEKING PROFESSIONAL SUPPORT.</p></div>";wall.before(warning);return warning;};
(() => {
  function start(){
    const profile=document.querySelector('.profile-header'),credit=document.querySelector('.pin-creator-strip');
    if(!profile&&!credit)return;
    if(credit){credit.remove();return;}// Pin pages: no creator name; the search term buttons stay in the top bar.
    const wall=(document.querySelector('[data-profile-feed]') || document.querySelector('.pin-detail-wall,.pins-grid')) || (()=>{const w=document.createElement('div');w.className='pins-grid';document.body.append(w);return w})();
    const link=credit?.querySelector('a');
    const name=(profile?.querySelector('.profile-name')?.textContent || link?.textContent || credit?.textContent || 'Profile').trim();
    const href=link?.getAttribute('href') || location.pathname.replace(/_(saved|created)\/$/,'');
    const tile=document.createElement('article');tile.className='pn-creator-card';tile.dataset.creatorSide=profile?'left':'after-pin';
    const avatar=document.createElement('div');avatar.className='pn-creator-avatar';avatar.textContent=name[0]||'?';
    const label=document.createElement(credit?'a':'span');if(credit)label.href=href;else label.style.pointerEvents='none';label.textContent=name;label.className='pn-creator-name';
    const portrait=document.createElement('div');portrait.className='pn-creator-portrait';portrait.append(avatar,label);tile.append(portrait);

    const setImage=img=>{if(!img)return;const copy=img.cloneNode(true);copy.removeAttribute('style');copy.alt=name;copy.loading='eager';avatar.replaceChildren(copy);};
    if(profile)setImage(profile.querySelector('img.profile-avatar'));
    else if(link)fetch(href,{credentials:'same-origin'}).then(r=>{if(!r.ok)throw Error();return r.text()}).then(html=>setImage(new DOMParser().parseFromString(html,'text/html').querySelector('img.profile-avatar'))).catch(()=>{});
    const hero=document.createElement('section');hero.className='pn-profile-hero';hero.setAttribute('aria-label',name);
    const banner=document.createElement('div');banner.className='pn-profile-banner creator-name-banner';
    const fallback=()=>{const title=document.createElement('span');title.textContent=name;banner.replaceChildren(title);banner.classList.add('creator-name-banner');};fallback();
    if(profile.dataset.coverUrl){const cover=new Image();cover.alt=name+' cover';cover.onload=()=>{banner.classList.remove('creator-name-banner');banner.replaceChildren(cover);};cover.onerror=fallback;cover.src=profile.dataset.coverUrl;}
    hero.append(tile,banner);
    function mount(){if(!hero.isConnected){document.querySelector('.pt-header').after(hero);document.dispatchEvent(new Event('pinless:feed-layout'));}}
    if(!wall.classList.contains('wall-ready')&&window.PinlessMasonry){const register=window.PinlessMasonry(wall);document.addEventListener('pinless:feed-layout',()=>register([...wall.children]));}
    mount();document.addEventListener('continuous:appended',mount);new MutationObserver(mount).observe(wall,{childList:true});
    if(credit)credit.remove();
    if(profile){
      document.body.classList.add('pn-profile-layout');profile.hidden=true;
      const header=document.querySelector('.pt-header'),tabs=document.querySelector('.tabs');if(tabs){tabs.classList.add('pn-profile-tabs');header.querySelector('.pt-nav').before(tabs);}
      const input=header.querySelector('input[name=q]');if(input){input.dataset.channelName=name;input.placeholder=name;input.setAttribute('aria-label','Search Pinterest');}
      const sourceError=document.querySelector('.source-error');if(sourceError){const retry=document.createElement('button');retry.textContent='Retry';retry.onclick=()=>location.reload();sourceError.append(retry);}
    }
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start();
})();

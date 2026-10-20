/* Menu-only, explicit downloads. Hashes never enter localStorage or info exports. */
;(() => {
  'use strict';
  const APP = 'pinful', NAME = APP === 'pinful' ? 'Pinful' : 'Individuous';
  window.appDataControlsVersion = 2;

  const el = (tag, text, cls) => { const node = document.createElement(tag); if (text) node.textContent = text; if (cls) node.className = cls; return node; };

  // Explicit allowlists: never treat saved information or synchronization state as cache.
  function temporaryEntries() {
    const result = [];
    const local = APP === 'individuous' ? /^(?:pt-shapes|pt-channel-avatar:UC[\w-]{22})$/ : /^$/;
    const session = APP === 'individuous' ? /^(?:pt-sandbox-wall-v1|pt-sandbox-search-v2)$/ : /^(?:pinless-wall-snapshot-v1|pinless-open-pin-preview-v1)$/;
    for (const [storage, pattern] of [[window.localStorage,local],[window.sessionStorage,session]]) {
      for (let i = 0; i < storage.length; i++) {
        const key = storage.key(i);
        if (pattern.test(key)) result.push({storage,key,bytes:2*(key.length+(storage.getItem(key)||'').length)});
      }
    }
    return result;
  }
  window.appTemporaryStorage = {
    measure: () => temporaryEntries().reduce((total,entry) => total + entry.bytes,0),
    clear: ({includeRestore = true} = {}) => { for (const {storage,key} of temporaryEntries()) if (includeRestore || storage !== window.sessionStorage) storage.removeItem(key); }
  };
  const AUTO_CLEANUP = 'pinless-auto-cleanup', AUTO_LAST = APP + '-auto-cleanup-last';
  let automaticWork;
  async function automaticCleanup() {
    if (automaticWork) return automaticWork;
    if (window.localStorage.getItem(AUTO_CLEANUP) !== 'daily') return;
    const last = Number(window.localStorage.getItem(AUTO_LAST)) || 0;
    if (Date.now() - last < 86400000) return;
    automaticWork = (async () => {
      await request('/app-data/temporary',{clear:true,epoch:window.appData?.epoch || ''});
      // Automatic cleanup keeps the current tab's Restore Feed and active media.
      window.appTemporaryStorage.clear({includeRestore:false});
      window.localStorage.setItem(AUTO_LAST,String(Date.now()));
    })();
    try { await automaticWork; } finally { automaticWork = null; }
  }
  window.appTemporaryStorage.automaticCleanup = automaticCleanup;
  function storageSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    return bytes < 1024*1024 ? (bytes/1024).toFixed(1)+' KiB' : (bytes/1024/1024).toFixed(1)+' MiB';
  }

  async function request(url, body = {}) {
    const response = await fetch(url, {method: 'POST', credentials: 'same-origin', cache: 'no-store',
      headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)});
    let result;
    try { result = await response.json(); } catch { throw Error('The local app did not return data. Reload and try again.'); }
    if (!response.ok) throw Error(result.message || 'The operation could not finish.');
    return result;
  }
  function download(data, kind) {
    const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], {type: 'application/json'}));
    const link = el('a'); link.href = url; link.download = APP + '-' + kind + '-' + new Date().toISOString().slice(0, 10) + '.json';
    document.body.append(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  async function readFile(file, kind) {
    if (!file || file.size > (kind === 'hash' ? 40000 : 100 * 1024 * 1024)) throw Error('Choose a smaller ' + kind + ' JSON file.');
    let data; try { data = JSON.parse(await file.text()); } catch { throw Error('This file is not valid JSON. Nothing was changed.'); }
    if (!data || data.format !== 'myapps-' + kind || data.version !== 1 || data.app !== APP)
      throw Error('Choose a ' + NAME + ' ' + kind + ' export (version 1). Nothing was changed.');
    if (kind === 'hash') {
      const c = data.credentials;
      if (!c || typeof c !== 'object' || Array.isArray(c) ||
          (APP === 'pinful' ? typeof c.session !== 'string' || !c.session : typeof c.cookie !== 'string' || !c.cookie))
        throw Error('This file does not contain a saved hash.');
    } else {
      if (!data.items || typeof data.items !== 'object' || Array.isArray(data.items)) throw Error('The info file has no valid app data.');
      const pattern = APP === 'pinful' ? /^pinless-[\w.-]{1,120}$/ : /^pt-[\w.-]{1,120}$/;
      for (const [key, value] of Object.entries(data.items)) {
        if (!pattern.test(key) || (value !== null && typeof value !== 'string')) throw Error('The info file contains an invalid item.');
        if (value === null || !/-v1(?:\.|$)/.test(key)) continue;
        let parsed; try { parsed = JSON.parse(value); } catch { throw Error('The info file contains unreadable saved data: ' + key); }
        if (/(?:playlists|video-lists|search-term-lists|following-groups)-v1$/.test(key)) {
          const group = /search-term/.test(key) ? 'terms' : /following/.test(key) ? 'channels' : 'items';
          if (!parsed || !Array.isArray(parsed.lists) ||
              !parsed.lists.every(list => list && typeof list.id === 'string' && typeof list.name === 'string' && Array.isArray(list[group])) ||
              !parsed.lists.some(list => list.id === parsed.activeId) && !['all-groups', '__all__'].includes(parsed.activeId))
            throw Error('The info file contains an invalid group: ' + key);
        }
      }
      if (APP === 'individuous' && (!data.account || typeof data.account !== 'object')) throw Error('The info file is missing account data.');
    }
    return data;
  }
  function confirmation(title, description, typed = false, label = 'Import info') {
    return new Promise(resolve => {
      const dialog = el('dialog', '', 'app-data-dialog'), form = el('form');
      form.method = 'dialog';
      const heading = el('h2', title); heading.id = 'app-data-confirm-title'; dialog.setAttribute('aria-labelledby', heading.id);
      form.append(heading, el('p', description));
      let input;
      if (typed) {
        const label = el('label', 'Type CLEAR EVERYTHING to reset this app');
        input = el('input'); input.autocomplete = 'off'; input.spellcheck = false; label.append(input); form.append(label);
      }
      const actions = el('div', '', 'app-data-actions'), cancel = el('button', 'Cancel'), yes = el('button', typed ? 'Clear everything' : label);
      cancel.type = 'button'; yes.type = 'submit'; yes.className = 'app-data-danger';
      yes.disabled = typed; if (input) input.oninput = () => { yes.disabled = input.value !== 'CLEAR EVERYTHING'; };
      cancel.onclick = () => dialog.close('cancel');
      form.onsubmit = event => { event.preventDefault(); if (!yes.disabled) dialog.close('yes'); };
      actions.append(cancel, yes); form.append(actions); dialog.append(form); document.body.append(dialog);
      dialog.addEventListener('close', () => { const accepted = dialog.returnValue === 'yes'; dialog.remove(); resolve(accepted); }, {once: true});
      dialog.showModal(); if (input) input.focus(); else cancel.focus();
    });
  }
  function mount() {
    if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return true;
    const host = document.querySelector(APP === 'pinful' ? '.pn-panel' : '.pt-navigation-page');
    const connection = host?.querySelector(APP === 'pinful' ? '.pn-pinterest-session' : '.pt-youtube-connection');
    if (!host || !connection) return false;
    if (host.querySelector('.app-data-controls')) return true;
    const section = el('section', '', 'app-data-controls');
    section.append(el('h2', 'Your information'), el('p', 'Export your collections, search groups, follows, history and settings to a JSON file. Import replaces the information in this app. Save your login hash separately above.'));
    const actions = el('div', '', 'app-data-actions');
    const exportButton = el('button', 'Export info'), importButton = el('button', 'Import info'), clear = el('button', 'Clear everything', 'app-data-danger');
    const picker = el('input'); picker.type = 'file'; picker.accept = '.json,application/json'; picker.hidden = true; picker.setAttribute('aria-label', 'Choose info export');
    const status = el('p'); status.setAttribute('role', 'status');
    actions.append(exportButton, importButton, clear); section.append(actions, picker, status);
    connection.after(section);
    const hashActions = el('div', '', 'app-data-actions app-hash-actions');
    const saveHash = el('button', 'Save hash'), loadHash = el('button', 'Load hash'), hashPicker = el('input');
    saveHash.type = loadHash.type = 'button'; hashPicker.type = 'file'; hashPicker.accept = '.json,application/json'; hashPicker.hidden = true; hashPicker.setAttribute('aria-label', 'Choose saved hash');
    const hashStatus = el('p'); hashStatus.setAttribute('role', 'status');
    hashActions.append(saveHash, loadHash, hashPicker);
    connection.append(hashActions, el('p', 'Save hash downloads a reusable login file. Keep it private: it grants account access. Load hash restores it here; expired sessions still need a fresh hash.'), hashStatus);
    let busy = false;
    const controls = [exportButton, importButton, clear, saveHash, loadHash];
    async function run(output, action) {
      if (busy) return; busy = true; controls.forEach(b => { b.disabled = true; }); output.textContent = 'Working…';
      try { await action(); } catch (error) { output.textContent = error.message || 'The operation could not finish.'; window.appData?.resume(); }
      finally { busy = false; controls.forEach(b => { b.disabled = false; }); }
    }
    function sync() { if (!window.appData) throw Error('App storage is unavailable. Enable browser storage and reload.'); return window.appData; }
    const recover = el('button', 'Recover saved data'); actions.append(recover); controls.push(recover);
    recover.onclick = () => run(status, async () => {
      const response = await fetch('/app-data/recovery', {cache: 'no-store'});
      if (!response.ok) throw Error('Recovery is unavailable. Restart the app and retry.');
      const data = await response.json();
      if (!data.copies.length) throw Error('No automatic recovery copies are available. Your current file is preserved. Restore a known-good store.json from your private backup and restart the app.');
      const dialog = el('dialog', '', 'app-data-dialog'), select = el('select');
      dialog.append(el('h2', 'Recover saved data'), el('p', 'Select a saved copy. This replaces collection data and keeps your hash. First-sync copies can contain only part of a browser’s data.'));
      for (const copy of data.copies.sort((a, b) => b.id.localeCompare(a.id))) { const option = el('option', copy.id + ' (' + copy.count + ' items)'); option.value = copy.id; select.append(option); }
      const yes = el('button', 'Use selected copy'), cancel = el('button', 'Cancel');
      dialog.append(select, yes, cancel); document.body.append(dialog);
      const selected = await new Promise(resolve => {
        yes.onclick = () => dialog.close('yes'); cancel.onclick = () => dialog.close('cancel');
        dialog.addEventListener('close', () => { resolve(dialog.returnValue === 'yes' ? select.value : null); dialog.remove(); }, {once: true});
        dialog.showModal(); cancel.focus();
      });
      if (!selected) { status.textContent = 'Recovery cancelled.'; return; }
      const result = await request('/app-data/recover', {id: selected, confirm: 'RECOVER'});
      sync().accept(result); location.reload();
    });
    saveHash.onclick = () => run(hashStatus, async () => {
      const data = await request(APP === 'pinful' ? '/pinterest-session/export' : '/youtube-account/export');
      download(data, 'hash'); hashStatus.textContent = 'Hash file downloaded. Keep it somewhere private for reuse.';
    });
    loadHash.onclick = () => { hashPicker.value = ''; hashPicker.click(); };
    hashPicker.onchange = () => { const file = hashPicker.files[0]; if (!file) return; void run(hashStatus, async () => {
      const data = await readFile(file, 'hash');
      await request(APP === 'pinful' ? '/pinterest-session/restore' : '/youtube-account/restore', data.credentials);
      hashStatus.textContent = 'Hash restored. Reloading to check the connection…'; location.reload();
    }); };
    exportButton.onclick = () => run(status, async () => {
      await sync().flush();
      download(await request('/app-data/export', {epoch: sync().epoch}), 'info');
      status.textContent = 'Info file downloaded. Use Save hash above to keep your login too.';
    });
    importButton.onclick = () => { picker.value = ''; picker.click(); };
    picker.onchange = () => { const file = picker.files[0]; if (!file) return; void run(status, async () => {
      const backup = await readFile(file, 'info');
      if (!await confirmation('Import info into ' + NAME + '?', 'This replaces current collections, groups, follows, history and settings with “' + file.name + '”. Export your current info first if you want to keep it. Your saved hash stays as it is.')) { status.textContent = 'Import cancelled.'; return; }
      await sync().pause();
      const result = await request('/app-data/import', {epoch: sync().epoch, backup});
      sync().accept(result); status.textContent = 'Info imported. Reloading…'; location.reload();
    }); };
    clear.onclick = () => run(status, async () => {
      if (!await confirmation('Start ' + NAME + ' from scratch?', 'This removes this app’s collections, groups, follows, history, settings, saved hash and automatic app backups. Export info and Save hash first to keep copies. Downloaded media files are kept.', true)) { status.textContent = 'Reset cancelled.'; return; }
      await sync().pause();
      const result = await request('/app-data/reset', {epoch: sync().epoch, confirm: 'CLEAR EVERYTHING'});
      sync().accept(result);
      // The local automatic-sign-in account remains usable; browser preferences reset.
      if (APP === 'individuous') document.cookie = 'PREFS=; Max-Age=0; path=/; SameSite=Lax';
      status.textContent = 'Everything cleared. Opening a fresh app…';
      location.assign(APP === 'pinful' ? '/' : '/star.html');
    });
    const temporary = el('section','','app-data-controls app-temporary-storage');
    temporary.append(el('h2','Temporary storage'),el('p','Clear reusable app data without removing collections, search groups, follows, history, settings or login hashes. Cached details may load again. Restore Feed data in this tab will be cleared.'));
    const usage = el('p'), tempStatus = el('p'); tempStatus.setAttribute('role','status');
    const tempActions = el('div','','app-data-actions'), refresh = el('button','Refresh storage usage'), clearTemp = el('button','Clear temporary data');
    refresh.type = clearTemp.type = 'button'; tempActions.append(refresh,clearTemp); temporary.append(usage,tempActions,tempStatus);
    temporary.append(el('p','Browser image/video cache and downloaded files in your Downloads folder are not included. Manage those in your browser or file manager. Browser usage shown covers this browser and this tab’s Restore Feed; other tabs may keep their own copy.'));
    section.after(temporary); controls.push(refresh,clearTemp);
    const autoLabel = el('label'), autoClear = el('input'); autoClear.type = 'checkbox';
    autoClear.checked = window.localStorage.getItem(AUTO_CLEANUP) === 'daily';
    autoLabel.append(autoClear,' Automatically clear temporary data daily');
    temporary.append(autoLabel,el('p','Runs when the app is open, at most once a day in this browser. Keeps your current Restore Feed, saved information and login hashes. Active downloads are left alone.'));
    controls.push(autoClear);
    autoClear.onchange = () => run(tempStatus,async () => {
      window.localStorage.setItem(AUTO_CLEANUP,autoClear.checked ? 'daily' : 'off');
      if (autoClear.checked) await automaticCleanup();
      await storageInfo();
      tempStatus.textContent = autoClear.checked ? 'Daily cleanup is on. Your current Restore Feed is kept.' : 'Automatic cleanup is off.';
    });
    async function storageInfo(clear = false) {
      const result = await request('/app-data/temporary',{clear,epoch:sync().epoch});
      if (clear) window.appTemporaryStorage.clear();
      usage.textContent = 'Reusable browser data: approximately '+storageSize(window.appTemporaryStorage.measure())+' · '+result.label+': '+storageSize(result.bytes)+(result.count === undefined ? '' : ' ('+result.count+' videos)')+(result.activeDownloads ? ' · '+result.activeDownloads+' active downloads' : '');
      tempStatus.textContent = (clear ? 'Temporary app data cleared. ' : '')+(result.note||'');
    }
    refresh.onclick = () => run(tempStatus,() => storageInfo());
    clearTemp.onclick = () => run(tempStatus,async () => {
      if (!await confirmation('Clear temporary data?','This clears reusable app cache and this tab’s Restore Feed. Saved information and login hashes stay as they are.',false,'Clear temporary data')) { tempStatus.textContent = 'Cleanup cancelled.'; return; }
      await storageInfo(true);
    });
    void run(tempStatus,() => storageInfo());
    return true;
  }
  function showIssue() {
    const issue = window.appData?.issue;
    let banner = document.querySelector('.app-data-issue');
    if (!issue || banner || !document.body) return;
    banner = el('aside', '', 'app-data-issue'); banner.setAttribute('role', 'alert');
    banner.append(el('p', issue.message || 'Saving paused: another browser changed this collection.'));
    if (issue.code === 'conflict') {
      const save = el('button', 'Download my unsaved copy'), server = el('button', 'Use saved version'), local = el('button', 'Replace with my version');
      save.onclick = () => download(window.appData.localCopy(), 'conflict');
      const resolve = async useLocal => {
        if (!await confirmation('Resolve conflicting changes?', useLocal ? 'Replace the latest saved collection with your unsaved version? Download your unsaved copy first if you want to keep both.' : 'Discard your unsaved changes and load the saved version? Download your unsaved copy first if you want to keep both.', false, useLocal ? 'Replace with my version' : 'Use saved version')) return;
        try { await window.appData.resolve(useLocal); } catch (error) { banner.querySelector('p').textContent = error.message; }
      };
      server.onclick = () => resolve(false); local.onclick = () => resolve(true); banner.append(save, server, local);
    } else {
      const menu = el('a', 'Open recovery in Menu'); menu.href = APP === 'pinful' ? '/?pt_page=menu' : '/feed/subscriptions?pt_page=menu'; banner.append(menu);
    }
    document.body.append(banner);
  }
  setInterval(showIssue, 500);
  setInterval(() => { void automaticCleanup().catch(() => {}); },3600000);
  function start() {
    void automaticCleanup().catch(() => {});
    showIssue();
    if (mount()) return;
    const observer = new MutationObserver(() => { if (mount()) observer.disconnect(); });
    observer.observe(document.documentElement, {childList: true, subtree: true});
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, {once: true}); else start();
})();

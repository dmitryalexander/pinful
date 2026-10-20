(() => {
  'use strict';
  let indicatorRevision = 0;
  function indicator(data) {
    indicatorRevision++;
    document.documentElement.setAttribute('data-pinterest-authenticated', String(data?.connected === true && data?.authenticated === true));
  }
  async function checkIndicator() {
    indicator(null);
    const at = indicatorRevision;
    try {
      const response = await fetch('/pinterest-session', {method:'GET', credentials:'same-origin', cache:'no-store'});
      const data = await response.json();
      if (at === indicatorRevision) indicator(response.ok ? data : null);
      return response.ok ? data : null;
    } catch { if (at === indicatorRevision) indicator(null); return null; }
  }
  const initialStatus = checkIndicator();
  window.addEventListener('storage', event => {
    if (event.key === 'pinful-pinterest-revision') void checkIndicator();
  });
  if (new URLSearchParams(location.search).get('pt_page') !== 'menu') return;
  function build() {
    const panel = document.querySelector('.pn-panel');
    if (!panel || panel.querySelector('.pn-pinterest-session')) return;
    const el = (tag, text) => { const node = document.createElement(tag); if (text) node.textContent = text; return node; };
    const section = el('section'); section.className = 'pn-pinterest-session';
    section.append(el('h2', 'Pinterest connection'));
    section.append(el('p', 'Use your Pinterest session for searches and related pins. This is experimental: connecting does not guarantee that Pinterest will provide recommendations.'));
    const status = el('p', 'Checking connection…'); status.setAttribute('role', 'status');
    const login = el('a', 'Log in on Pinterest'); login.href = 'https://www.pinterest.com/login/'; login.target = '_blank'; login.rel = 'noopener noreferrer';
    const details = el('details'); details.append(el('summary', 'How to connect your session'));
    const steps = el('ol');
    for (const text of [
      'Log in on Pinterest in your browser. Keep your password on Pinterest.',
      'On your Pinterest tab, press F12. Choose Application in the top menu bar (look under » if it is hidden). Under Storage in the left sidebar, expand Cookies and click https://www.pinterest.com. In Firefox, use Storage → Cookies.',
      'Type sess in the filter and select _pinterest_sess. In the lower Cookie Value preview, copy the entire long value. Scroll down to reach the end and make sure you copy the whole thing. Paste it into the Pinterest session field below, without the cookie name or the shortened text ending in dots.',
      'Click Connect and check. Then run a fresh search in Pinful and open a pin to test related recommendations.'
    ]) steps.append(el('li', text));
    details.append(steps, el('p', 'A session cookie grants access to your Pinterest account. Paste it only here on your own local Pinful. Do not share it in chat, screenshots or bug reports.'));
    const help = el('img'); help.src = '/static/pinterest-session-help.png'; help.alt = 'Application → Cookies → Pinterest: filter sess, select _pinterest_sess and copy the full Cookie Value below. Example value obscured.'; help.loading = 'lazy';
    const helpLink = el('a'); helpLink.className = 'pn-help-image-link'; helpLink.href = help.src; helpLink.target = '_blank'; helpLink.rel = 'noopener noreferrer'; helpLink.title = 'Open helper image in a new tab';
    helpLink.append(help); details.append(helpLink);
    const form = el('form'); form.autocomplete = 'off';
    function field(label, name, required) {
      const wrap = el('label', label), input = el('input');
      input.type = 'password'; input.name = name; input.autocomplete = 'off'; input.spellcheck = false; input.maxLength = 8192; input.required = required;
      wrap.append(input); form.append(wrap); return input;
    }
    const session = field('Pinterest session (_pinterest_sess value)', 'session', true);
    const connect = el('button', 'Connect and check'); connect.type = 'submit';
    const disconnect = el('button', 'Clear hash'); disconnect.type = 'button'; disconnect.hidden = true;
    const check = el('button', 'Check connection'); check.type = 'button'; check.hidden = true;
    form.append(connect, check, disconnect);
    function replacementState() {
      connect.disabled = busy || (session.dataset.sessionSaved === 'true' && !session.value.trim());
    }
    session.addEventListener('input', replacementState);
    const note = el('p', 'The last verified session is saved privately in this installation and reused after restart. Opening this Menu checks it with Pinterest. Clear hash removes the saved session without logging you out of Pinterest or deleting saved pins.');
    section.append(login, details, status, form, note);
    const switches = panel.querySelector('.pn-switches');
    if (switches) switches.after(section); else panel.prepend(section);
    async function request(method, body, checking = false) {
      const response = await fetch('/pinterest-session' + (checking ? '/check' : ''), {method, credentials:'same-origin', cache:'no-store',
        ...(method === 'GET' ? {} : {headers:{'Content-Type':'application/json'}}),
        ...(body ? {body:JSON.stringify(body)} : {})});
      let data; try { data = await response.json(); } catch (_) { throw new Error('Connection service unavailable. Reload Pinful and try again.'); }
      if (!response.ok) throw new Error(data.message || 'Could not update the connection.');
      return data;
    }
    function paint(data) {
      const authenticated = data.connected === true && data.authenticated === true;
      const saved = data.has_session === true;
      session.dataset.sessionSaved = String(saved);
      session.placeholder = saved ? 'h a s h - l o a d e d' : 'Paste your Pinterest session value';
      section.setAttribute('data-connected', String(authenticated));
      indicator(data);
      disconnect.hidden = check.hidden = !saved;
      connect.textContent = saved ? 'Replace and check session' : 'Connect and check';
      replacementState();
      status.textContent = authenticated
        ? 'Connected. Pinterest verified this session. Recommendations are not verified; try a fresh search and open a pin.'
        : saved ? 'Not connected. Pinterest could not verify this saved session. It may have expired, or Pinterest may be unavailable. Check again, replace it or Clear hash to forget it.'
        : 'Not connected. No session saved. Pinful is browsing anonymously.';
    }
    let busy = false, revision = 0;
    async function update(method, body, checking = false) {
      if (busy) return;
      revision++;
      busy = true; connect.disabled = disconnect.disabled = check.disabled = true;
      if (checking || method === 'DELETE') { section.setAttribute('data-connected', 'false'); indicator(null); }
      status.textContent = method === 'POST' ? 'Checking the session with Pinterest…' : 'Clearing saved hash…';
      try {
        paint(await request(method, body, checking));
        // Only a random revision is shared, never session credentials.
        if (!checking) {
          try { localStorage.setItem('pinful-pinterest-revision', crypto.randomUUID()); } catch (_) {}
          document.dispatchEvent(new Event('pinterest-session-changed'));
        }
      }
      catch (error) { if (checking || method === 'DELETE') { section.setAttribute('data-connected', 'false'); indicator(null); } status.textContent = error.message; }
      finally { busy = false; disconnect.disabled = check.disabled = false; replacementState(); }
    }
    form.addEventListener('submit', event => {
      event.preventDefault();
      const body = {session:session.value.trim()};
      session.value = '';
      void update('POST', body);
    });
    disconnect.addEventListener('click', () => { session.value = ''; void update('DELETE'); });
    check.addEventListener('click', () => { void update('POST', {}, true); });
    section.setAttribute('data-connected', 'false');
    initialStatus.then(data => { if (!revision) { if (data) paint(data); else status.textContent = 'Connection service unavailable. Reload Pinful and try again.'; } });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', build, {once:true});
  else build();
})();

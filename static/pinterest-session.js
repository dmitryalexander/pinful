(() => {
  'use strict';
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
    details.append(help);
    const form = el('form'); form.autocomplete = 'off';
    function field(label, name, required) {
      const wrap = el('label', label), input = el('input');
      input.type = 'password'; input.name = name; input.autocomplete = 'off'; input.spellcheck = false; input.maxLength = 8192; input.required = required;
      wrap.append(input); form.append(wrap); return input;
    }
    const session = field('Pinterest session (_pinterest_sess value)', 'session', true);
    const connect = el('button', 'Connect and check'); connect.type = 'submit';
    const disconnect = el('button', 'Disconnect'); disconnect.type = 'button'; disconnect.hidden = true;
    form.append(connect, disconnect);
    const note = el('p', 'The connection is shared by tabs of this local installation and kept only in server memory. Restarting Pinful disconnects it. Disconnect does not log you out of Pinterest or delete saved pins.');
    section.append(login, details, status, form, note);
    const switches = panel.querySelector('.pn-switches');
    if (switches) switches.after(section); else panel.prepend(section);
    async function request(method, body) {
      const response = await fetch('/pinterest-session', {method, credentials:'same-origin', cache:'no-store',
        ...(method === 'GET' ? {} : {headers:{'Content-Type':'application/json'}}),
        ...(body ? {body:JSON.stringify(body)} : {})});
      let data; try { data = await response.json(); } catch (_) { throw new Error('Connection service unavailable. Reload Pinful and try again.'); }
      if (!response.ok) throw new Error(data.message || 'Could not update the connection.');
      return data;
    }
    function paint(data) {
      section.setAttribute('data-connected', String(!!data.connected));
      disconnect.hidden = !data.connected;
      connect.textContent = data.connected ? 'Replace and check session' : 'Connect and check';
      status.textContent = data.connected
        ? 'Session connected. Recommendations are not verified; try a fresh search and open a pin.'
        : 'Not connected. Pinful is browsing anonymously.';
    }
    let busy = false, revision = 0;
    async function update(method, body) {
      if (busy) return;
      revision++;
      busy = true; connect.disabled = disconnect.disabled = true;
      status.textContent = method === 'POST' ? 'Checking the session with Pinterest…' : 'Disconnecting…';
      try {
        paint(await request(method, body));
        // Only a random revision is shared, never session credentials.
        try { localStorage.setItem('pinful-pinterest-revision', crypto.randomUUID()); } catch (_) {}
        document.dispatchEvent(new Event('pinterest-session-changed'));
      }
      catch (error) { status.textContent = error.message; }
      finally { busy = false; connect.disabled = disconnect.disabled = false; }
    }
    form.addEventListener('submit', event => {
      event.preventDefault();
      const body = {session:session.value.trim()};
      session.value = '';
      void update('POST', body);
    });
    disconnect.addEventListener('click', () => { session.value = ''; void update('DELETE'); });
    request('GET').then(data => { if (!revision) paint(data); }).catch(error => { if (!revision) status.textContent = error.message; });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', build, {once:true});
  else build();
})();

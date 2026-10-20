const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '../static/pinterest-session.js'), 'utf8');
const nodes = [], calls = [], notifications = [];
function element(tag) {
  const node = {tag, children:[], listeners:{}, dataset:{}, value:'',
    append(...children) { this.children.push(...children); },
    setAttribute(key, value) {this[key] = value;},
    addEventListener(key, fn) {this.listeners[key] = fn;},
    after(child) {this.afterNode = child;},
  };
  nodes.push(node); return node;
}
const switches = element('div');
const panel = {querySelector:s => s === '.pn-switches' ? switches : null, prepend() {throw Error('Should follow controls');}};
let connected = false, reject = false, authenticated = true;
const fetch = async (url, opts) => {
  assert.ok(['/pinterest-session','/pinterest-session/check'].includes(url));
  assert.equal(opts.cache, 'no-store');
  calls.push({url, ...opts});
  if (opts.method === 'POST' && !reject && url === '/pinterest-session') connected = true;
  if (opts.method === 'DELETE') connected = false;
  return {ok:!reject, json:async () => reject ? {message:'Session rejected'} : {connected:connected&&authenticated,authenticated:connected&&authenticated,has_session:connected}};
};
vm.runInNewContext(script, {
  window:{addEventListener(){}},
  document:{documentElement:{setAttribute(){}},readyState:'complete',querySelector:() => panel,createElement:element,dispatchEvent:e=>notifications.push(e.type)},
  location:{search:'?pt_page=menu'}, URLSearchParams, fetch, Event,
  localStorage:{setItem:(key,value)=>{assert.equal(key,'pinful-pinterest-revision');assert.equal(value,'revision-only');}},crypto:{randomUUID:()=> 'revision-only'},
});
const settle = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  await settle();
  const form = nodes.find(n => n.tag === 'form');
  const inputs = nodes.filter(n => n.tag === 'input');
  const status = nodes.find(n => n.role === 'status');
  const disconnect = nodes.find(n => n.textContent === 'Clear hash');
  const link = nodes.find(n => n.tag === 'a');
  assert.equal(link.href, 'https://www.pinterest.com/login/');
  assert.equal(link.rel, 'noopener noreferrer');
  assert.ok(inputs.every(n => n.type === 'password' && n.autocomplete === 'off'));
  assert.equal(disconnect.hidden, true);
  assert.equal(inputs.length, 1);
  inputs[0].value = ' synthetic-session ';
  form.listeners.submit({preventDefault(){}});
  assert.ok(inputs.every(n => n.value === ''), 'clear inputs immediately');
  await settle();
  assert.deepEqual(JSON.parse(calls.find(c => c.method === 'POST').body), {session:'synthetic-session'});
  assert.deepEqual(notifications,['pinterest-session-changed']);
  assert.equal(disconnect.hidden, false);
  assert.match(status.textContent, /Recommendations are not verified/);
  const section = nodes.find(n => n.tag === 'section');
  assert.equal(section['data-connected'],'true');
  const check = nodes.find(n => n.textContent === 'Check connection');
  check.listeners.click(); await settle();
  assert.equal(calls.at(-1).url,'/pinterest-session/check');
  assert.match(status.textContent,/Pinterest verified/);
  authenticated=false;check.listeners.click();await settle();
  assert.match(status.textContent,/could not verify/);
  assert.equal(section['data-connected'],'false');
  assert.match(status.textContent,/^Not connected/);
  assert.equal(disconnect.hidden,false,'saved invalid session can still be forgotten');
  assert.equal(notifications.length,1,'checking does not broadcast a connection replacement');
  reject = true; inputs[0].value = 'bad'; form.listeners.submit({preventDefault(){}});
  await settle(); assert.equal(status.textContent, 'Session rejected'); assert.equal(disconnect.hidden, false);
  reject = false; disconnect.listeners.click(); await settle();
  assert.equal(disconnect.hidden, true); assert.match(status.textContent, /anonymously/);
  assert.equal(calls.at(-1).method, 'DELETE');
  assert.equal(notifications.length,2,'only successful connection changes broadcast');
  console.log('PASS: session form submits locally, clears secrets, reports failures/unverified recommendations and disconnects.');
})().catch(error => {console.error(error); process.exitCode = 1;});

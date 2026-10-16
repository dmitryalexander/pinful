// Node-only regression check; does not read or change installed user data.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(process.argv[2] || path.join(__dirname, '../static/modern.js'), 'utf8');
const line = prefix => {
  const found = source.split('\n').find(text => text.trim().startsWith(prefix));
  assert.ok(found, `Missing ${prefix}`);
  return found.trim();
};
const opaque = 'e1_AfqPShx46h7NbnTxYpEgecPh13liq72yeWnDbYFrb_rCsNV0nDjD';
function fixture(id) {
  const link = {getAttribute: () => `/pin/${id}?q=Titanfall`};
  const img = {alt: 'Titanfall', getAttribute: name => ({src: '/image?url=fixture', width: '400', height: '500'})[name]};
  return {dataset: {}, querySelector: selector => selector.startsWith('a[') ? link : selector === 'img' ? img : null};
}
const wall = {children: [], replaceChildren(...items) {this.children = items;}, append(...items) {this.children.push(...items);}};
const root = {children: [fixture(opaque), fixture('123456789')], matches: () => false, nextElementSibling: null};
const context = vm.createContext({
  URL, AbortSignal, location: {origin: 'http://localhost:3001'},
  $: (selector, target) => target.querySelector(selector),
  wall, status: {}, more: {getBoundingClientRect: () => ({top: 5000})},
  DOMParser: class {parseFromString() {return {querySelector: () => root};}},
  fetch: async () => ({ok: true, text: async () => 'fixture'}),
  setTimeout: () => {throw Error('Unexpected automatic load');},
  innerHeight: 800, cardFor: item => item, shuffle: items => items,
  refresh: () => {}, snapshot: () => {},
});
const validation = source.split('\n').find(text => text.startsWith('const validPinId=')) || '';
vm.runInContext(`${validation}\n${line('function record(')}\n${line('function showItems(')}\n${line('function showHistory(')}
  let hist={items:[],shown:0}; const HIST_STEP=40;
  function histMore(){ wall.append(...hist.items); hist.shown=hist.items.length; }
  let busy=false,queue=[{url:'/search/pins/?q=Titanfall'},{url:'/search/pins/?q=Mecha'}],loaded=new Set();`, context);
context.input = fixture(opaque);
assert.equal(vm.runInContext('record(input)?.id', context), opaque, 'group feed must accept Pinterest opaque IDs');
context.input = fixture('123456789');
assert.equal(vm.runInContext('record(input)?.id', context), '123456789', 'numeric IDs remain supported');
context.input = fixture('invalid!');
assert.equal(vm.runInContext('record(input)', context), null, 'invalid IDs are rejected');
context.items = [{id: opaque}, {id: '123456789'}, {id: '../outside'}, {id: 'e1_'}];
vm.runInContext('showItems(items)', context);
assert.deepEqual(wall.children.map(item => item.id), [opaque, '123456789'], 'saved and hidden walls retain valid opaque pins');
vm.runInContext('showHistory(items)', context);
assert.deepEqual(wall.children.map(item => item.id), [opaque, '123456789'], 'history retains valid opaque pins');
const start = source.indexOf('async function loadDiscovery()');
const end = source.indexOf("if(page==='discover')", start);
assert.ok(start >= 0 && end > start);
vm.runInContext(source.slice(start, end), context);
wall.replaceChildren();
(async () => {
  await vm.runInContext('loadDiscovery()', context);
  assert.deepEqual(wall.children.map(item => item.id).sort(), ['123456789', opaque].sort(), 'group searches render opaque results and deduplicate across terms');
  assert.equal(context.status.textContent, 'End of the available results.');
  assert.equal(context.more.hidden, true);
  console.log('Opaque and numeric IDs: group discovery, deduplication, saved/hidden walls and history pass.');
})().catch(error => {console.error(error); process.exitCode = 1;});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const script = fs.readFileSync(path.join(__dirname, '../static/pin-recovery.js'), 'utf8');
function run(image, {confirmed = false, pending = false, snapshot = false, video = ''} = {}) {
  const id = 'e1_saved';
  const value = {id, image, video, title: '<script>text stays text</script>', width: 600, height: 800};
  const created = [], listeners = {};
  const node = tag => ({tag, dataset: {}, style: {}, children: [], className: '',
    append(...items) {this.children.push(...items);}, setAttribute(k, v) {this[k] = v;}});
  const error = {...node('section'), querySelector: () => confirmed ? {} : null,
    after(wall) {this.wall = wall;}, replaceChildren(...items) {this.children = items;}};
  const document = {
    addEventListener(k, fn) {listeners[k] = fn;},
    querySelector: () => error,
    createElement(tag) {const n = node(tag); created.push(n); return n;},
    dispatchEvent() {},
  };
  vm.runInNewContext(script, {document, URL, location: {href:'http://localhost:3001/pin/'+id, origin:'http://localhost:3001', pathname:'/pin/'+id},
    sessionStorage: {getItem: key => key === 'pinless-open-pin-preview-v1' && pending ? JSON.stringify(value) : key === 'pinless-wall-snapshot-v1' && snapshot ? JSON.stringify({items:[value]}) : null},
    localStorage: {getItem: key => !snapshot && key === 'pinless-playlists-v1' ? JSON.stringify({lists:[{items:[value]}]}) : null},
    CustomEvent: class {},
  });
  return {created, error, listeners};
}
const valid = '/image?url=https%3A%2F%2Fi.pinimg.com%2Foriginals%2Fexample.jpg';
for (const pending of [false, true]) {
  const result = run(valid, {pending});
  assert.ok(result.error.wall, 'Recover media from saved records or tab handoff');
  assert.equal(result.created.find(n => n.tag === 'img').src, valid);
  assert.equal(result.created.find(n => n.tag === 'img').alt, '<script>text stays text</script>');
  assert.ok(result.error.children[0].textContent.includes('saved image'));
}
for (const unsafe of ['javascript:alert(1)', 'https://attacker.example/file', '/app-data/items', '/image?url=http%3A%2F%2Flocalhost%2Fprivate', '/image?url=https%3A%2F%2Fpinimg.com.attacker.example%2Fx']) {
  assert.equal(run(unsafe).created.length, 0, 'Reject unsafe saved media: '+unsafe);
}
assert.equal(run(valid, {confirmed:true}).created.length, 0, 'Do not disguise a confirmed numeric not-found state');
assert.ok(run(valid, {snapshot:true}).error.wall, 'Recover existing Restore snapshots from session storage');
assert.ok(run(valid, {video:'/hls?url=https%3A%2F%2Fv1.pinimg.com%2Fexample.m3u8'}).created.some(n => n.tag === 'video'));
console.log('PASS: saved/tab media recovery, safe image/video URLs, text-only titles and confirmed-not-found guard.');

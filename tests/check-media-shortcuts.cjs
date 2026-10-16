// Exercises the real shortcut listener without opening websites, downloading or changing saved data.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const root = path.join(__dirname, '../static');
const handlers = {}, windowHandlers = {}, counts = {lens: 0, open: 0, download: 0, start: 0, pause: 0, resume: 0};
let hit = null, popup = false;
const action = name => ({disabled: false, click() {counts[name]++;}});
function tile(video = false) {
  const controls = {'.pin-lens-open': video ? null : action('lens'),
    'a.pin-image-link[href^="/pin/"], a.pin-media-open[href^="/pin/"]': action('open'),
    '.pin-video-download': action('download'), '.pin-video-play': video ? action('start') : null};
  const media = video ? {paused: false, pause() {this.paused = true; counts.pause++;}, play() {this.paused = false; counts.resume++; return Promise.resolve();}} : null;
  return {playing: false, hidden: false, classList: {contains(name) {return name === 'is-playing' && this.owner.playing;}},
    querySelector: selector => selector === 'video' ? media : controls[selector],
    closest(selector) {return selector === '.pin-video-tile' ? this : selector === '[hidden]' && this.hidden ? this : null;},
    contains(element) {return element === this;}};
}
const image = tile(), video = tile(true);
image.classList.owner = image; video.classList.owner = video;
const document = {activeElement: null, addEventListener: (name, fn) => handlers[name] = fn,
  querySelector: () => popup ? {} : null, elementFromPoint: () => hit};
vm.runInNewContext(fs.readFileSync(path.join(root, 'media-shortcuts.js'), 'utf8'), {document, window: {addEventListener: (name, fn) => windowHandlers[name] = fn}});
const body = {matches: () => false, closest: () => null};
function key(key, overrides = {}) {
  const event = {key, target: body, prevented: false, composedPath() {return [this.target];}, preventDefault() {this.prevented = true;}, ...overrides};
  handlers.keydown(event); return event;
}
handlers.pointermove({clientX: 100, clientY: 200}); hit = image;
assert.equal(key('g').prevented, true); assert.equal(counts.lens, 1);
key('o'); key('d'); assert.equal(counts.open, 1); assert.equal(counts.download, 1);
assert.equal(key(' ').prevented, false, 'Space over images retains page scrolling');
hit = video;
assert.equal(key('g').prevented, false, 'G never runs on videos');
key(' '); assert.equal(counts.start, 1, 'Uses existing play handler, including hover-preview promotion');
video.playing = true; key(' '); key(' ');
assert.equal(counts.pause, 1); assert.equal(counts.resume, 1);
key('d'); key('d'); assert.equal(counts.download, 3, 'Download/cancel uses the same click control');
hit = image;
const before = counts.lens;
for (const guard of ['repeat', 'ctrlKey', 'shiftKey', 'altKey', 'metaKey', 'isComposing', 'defaultPrevented']) key('g', {[guard]: true});
key('g', {target: {matches: () => true}});
key('g', {target: {isContentEditable: true}});
popup = true; key('g'); popup = false;
image.hidden = true; key('g'); image.hidden = false;
assert.equal(counts.lens, before, 'Typing, modifier, repeated, modal and hidden-tile events are ignored');
hit = null; assert.equal(key('g').prevented, false, 'Hit-tests the current layout rather than a stale tile');
handlers.pointerout({relatedTarget: null}); hit = image; key('g');
assert.equal(counts.lens, before, 'Leaving the document clears hover');
document.activeElement = image; key('g'); assert.equal(counts.lens, before + 1, 'Focused tiles remain accessible without a pointer');
document.activeElement = null; handlers.pointermove({clientX: 100, clientY: 200}); hit = video;
assert.equal(key(' ', {target: {matches: () => false, closest: () => ({})}}).prevented, false, 'Space on an unrelated focused control keeps its native action');
windowHandlers.blur(); assert.equal(key('g').prevented, false);
// Verify that image initialization still builds a real Lens link, shared by G and the blue strip.
let lens;
const player = fs.readFileSync(path.join(root, 'player.js'), 'utf8');
const start = player.indexOf('function addLens('), end = player.indexOf('  let library;', start);
vm.runInNewContext(player.slice(start, end) + '\naddLens(tile, source);', {
  URL, location: {href: 'http://localhost:3001/', origin: 'http://localhost:3001'}, source: '/image?url=' + encodeURIComponent('https://i.pinimg.com/originals/test.jpg'),
  tile: {append(value) {lens = value;}}, document: {createElement() {return {setAttribute() {}, addEventListener() {}};}}
});
assert.ok(lens.href.startsWith('https://lens.google.com/uploadbyurl?url='));
assert.match(lens.title, /G/);
console.log('PASS: image G/Lens, O/open, D/action, Space/video, input guards, hit-testing and keyboard focus.');

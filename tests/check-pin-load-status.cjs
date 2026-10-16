// Run the real wall script with failed/empty pin-page DOMs. Neither may create
// a pagination status claiming that a failed request reached the end.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
function run(source, marker) {
  const classes = new Set(['pin-detail-wall']);
  const root = {
    children: [{dataset: {featuredPin: 'e1_example'}}],
    classList: {add: name => classes.add(name), contains: name => classes.has(name)},
    matches: selector => selector === '.pin-detail-wall',
    addEventListener() {},
  };
  const document = {
    readyState: 'complete', addEventListener() {},
    querySelector(selector) {
      if (selector === '.grid-container') return root;
      if (selector === '.source-error, .no-related') return {className: marker};
      return null;
    },
    createElement() { throw new Error('Unexpected pagination status on a failed/empty pin page'); },
  };
  let intersectionObservers = 0;
  vm.runInNewContext(source, {
    document, window: {}, matchMedia: () => ({matches: false}),
    requestAnimationFrame: () => 1,
    ResizeObserver: class {observe() {} unobserve() {}},
    IntersectionObserver: class {constructor() {intersectionObservers++;} observe() {}},
    location: {href: 'http://localhost/pin/e1_example'},
  });
  assert.equal(intersectionObservers, 1, 'Only the shared video observer should exist');
  assert.ok(classes.has('media-wall'), 'Keep masonry for any successfully loaded pin');
}
const current = fs.readFileSync(path.join(__dirname, '../static/wall-scroll.js'), 'utf8');
for (const marker of ['source-error', 'no-related']) {
  run(current, marker);
}
console.log('PASS: failed and empty pin pages retain layout without misleading end-of-results status.');

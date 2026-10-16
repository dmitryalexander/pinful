const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const source = fs.readFileSync('static/pin-status.js', 'utf8');
async function check(state, purple, responseStatus = 200) {
  const classes = new Set();
  const tile = {isConnected: true, dataset: {}, title: '',
    closest: selector => selector === '[data-pin-id], [data-featured-pin]' ? {dataset: {pinId: 'e1_test'}} : null,
    classList: {toggle(name, on) {if(on) classes.add(name); else classes.delete(name);}},
    removeAttribute(name) {if(name === 'title') this.title = '';}};
  let observer, requestCount = 0;
  const listeners = {}, windows = {}, timers = [];
  const document = {hidden:false, body:{}, querySelectorAll:()=>[tile], addEventListener:(name,fn)=>listeners[name]=fn};
  const context = {document, window:{addEventListener:(name,fn)=>windows[name]=fn}, IntersectionObserver:class {constructor(fn){observer=fn;} observe(){} disconnect(){}}, MutationObserver:class{observe(){}}, queueMicrotask, AbortController,
    setTimeout(fn, delay){timers.push({fn,delay});return timers.length;}, clearTimeout(){},
    fetch:async(url,options)=>{requestCount++;assert.equal(url,'/pin-status?id=e1_test');assert.equal(options.credentials,'same-origin');return {ok:responseStatus===200,status:responseStatus,json:async()=>({state})};}};
  vm.runInNewContext(source,context);
  assert.equal(classes.has('pin-no-related'),false,'unchecked tiles must not be purple');
  observer([{target:tile,isIntersecting:true}]);
  await timers.shift().fn();
  for(let i=0;i<12;i++) await Promise.resolve();
  assert.equal(classes.has('pin-no-related'),purple,state);
  assert.equal(requestCount,1);
  if(responseStatus===200){listeners.pointerover({target:{closest:()=>tile}});assert.equal(requestCount,1,'hover must reuse cached status');}
  if(responseStatus===429) assert.ok(timers.some(t=>t.delay===10000),'rate limits need backoff');
  if(purple) {
    assert.ok(tile.title.includes(state==='empty'?'No related pins':'Unresolved'));
    listeners['pinterest-session-changed']();
    assert.equal(classes.has('pin-no-related'),false,'connecting clears purple immediately');
    await timers.find(t=>t.delay===1100).fn();
    for(let i=0;i<12;i++) await Promise.resolve();
    assert.equal(requestCount,2,'old cached result must be rechecked');
    windows.storage({key:'pinful-pinterest-revision'});
    assert.equal(classes.has('pin-no-related'),false,'other tabs clear purple on connection changes');
  }
}
(async()=>{
  await check('unresolved',true);await check('empty',true);await check('available',false);await check('unknown',false);await check('empty',false,429);
  console.log('PASS: purple borders only for checked unresolved/empty pins; cache, tooltips and rate-limit backoff.');
})().catch(error=>{console.error(error);process.exit(1);});

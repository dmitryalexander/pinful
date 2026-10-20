const test = require('node:test'), assert = require('node:assert/strict'), vm = require('node:vm'), fs = require('node:fs'), path = require('node:path');
function fixture({local = {}, server = {items:{},epoch:''}, fetcher}={}) {
  class Storage {
    constructor(values={}) { this.values = new Map(Object.entries(values)); }
    get length() { return this.values.size; }
    key(i) { return [...this.values.keys()][i]; }
    getItem(k) { return this.values.get(k) ?? null; }
    setItem(k,v) { this.values.set(k,String(v)); }
    removeItem(k) { this.values.delete(k); }
  }
  const localStorage=new Storage(local),sessionStorage=new Storage({'pinless-wall-state':'stale'}),events={},requests=[],timers=[];
  let reloads=0;
  class XMLHttpRequest { open(){} send(){this.status=200;this.responseText=JSON.stringify(server);} }
  const window={localStorage,sessionStorage,location:{reload(){reloads++;}}};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../static/app-data.js'),'utf8'),{
    window,Storage,XMLHttpRequest,addEventListener:(key,fn)=>{events[key]=fn;},setTimeout:fn=>(timers.push(fn),timers.length),clearTimeout(){},
    fetch:async(url,init)=>{requests.push(init?.body ? JSON.parse(init.body) : null);return fetcher ? fetcher(url,init) : {ok:true,status:200,json:async()=>server};}
  });
  return {window,localStorage,sessionStorage,events,requests,timers,get reloads(){return reloads;}};
}
test('reset epoch discards stale local data and pending changes before the UI reads them',async()=>{
  const f=fixture({local:{'app-data-epoch-v1':'old','app-data-pending-v1':'{"pinless-cache-test-v1":123}','pinless-cache-test-v1':'old data','pinless-volume-v1':'0.4',unrelated:'keep'},
    server:{items:{'pinless-cache-test-v1':'new data'},epoch:'new'}});
  await f.window.appData.flush();
  assert.equal(f.localStorage.getItem('pinless-cache-test-v1'),'new data');
  assert.equal(f.localStorage.getItem('pinless-volume-v1'),null);
  assert.equal(f.sessionStorage.getItem('pinless-wall-state'),null);
  assert.equal(f.localStorage.getItem('unrelated'),'keep');
  assert.equal(f.requests.length,0);
});
test('a failed save stays pending and prevents an incomplete export',async()=>{
  const f=fixture({fetcher:async()=>({ok:false,status:503})});
  f.localStorage.setItem('pinless-cache-test-v1','changes');
  await assert.rejects(f.window.appData.flush(),/could not be saved/);
  assert(f.localStorage.getItem('app-data-pending-v1'));
});
test('reset in another browser rejects the stale save and reloads without retrying it',async()=>{
  const f=fixture({fetcher:async()=>({ok:false,status:409})});
  f.localStorage.setItem('pinless-cache-test-v1','stale');
  await assert.rejects(f.window.appData.flush());
  assert.equal(f.reloads,1);
  const count=f.requests.length;
  f.localStorage.setItem('pinless-cache-test-v1','resurrection');
  assert.equal(f.localStorage.getItem('pinless-cache-test-v1'),'stale');
  assert.equal(f.requests.length,count);
});
test('same-tab reset blocks pagehide snapshots and keeps unrelated origin keys',async()=>{
  const f=fixture({local:{unrelated:'keep','dark_mode':'dark','save_player_pos':'old'}});
  await f.window.appData.pause();
  f.window.appData.accept({items:{},epoch:'reset'});
  f.sessionStorage.setItem('pinless-wall-state','old wall');
  f.localStorage.setItem('pinless-cache-test-v1','old lists');
  assert.equal(f.sessionStorage.getItem('pinless-wall-state'),null);
  assert.equal(f.localStorage.getItem('pinless-cache-test-v1'),null);
  assert.equal(f.localStorage.getItem('dark_mode'),'dark');
  assert.equal(f.localStorage.getItem('save_player_pos'),'old');
  assert.equal(f.localStorage.getItem('unrelated'),'keep');
  assert.equal(f.localStorage.getItem('app-data-epoch-v1'),'reset');
});

test('collection conflict preserves unsaved data and requires an explicit choice',async()=>{
  const f=fixture({server:{epoch:'',items:{'pinless-cache-test-v1':'original'},revisions:{'pinless-cache-test-v1':'r1'}},
    fetcher:async()=>({ok:false,status:409,json:async()=>({code:'conflict',epoch:'',items:{'pinless-cache-test-v1':'remote'},revisions:{'pinless-cache-test-v1':'r2'}})})});
  f.localStorage.setItem('pinless-cache-test-v1','my edit');
  await assert.rejects(f.window.appData.flush());
  assert.equal(f.requests[0].base['pinless-cache-test-v1'],'r1');
  assert.equal(f.window.appData.issue.code,'conflict');
  assert.equal(f.localStorage.getItem('pinless-cache-test-v1'),'my edit');
  assert.equal(f.reloads,0);
  f.window.appData.resume();
  f.localStorage.setItem('pinless-cache-test-v1','must not replace conflict');
  assert.equal(f.window.appData.localCopy().items['pinless-cache-test-v1'],'my edit');
});

test('offline edits retain their original revision across reloads',async()=>{
  const f=fixture({local:{'pinless-cache-test-v1':'offline edit','app-data-pending-v1':'{"pinless-cache-test-v1":"token"}','app-data-revisions-v1':'{"pinless-cache-test-v1":"old"}'},
    server:{items:{'pinless-cache-test-v1':'remote'},epoch:'',revisions:{'pinless-cache-test-v1':'new'}}});
  await f.window.appData.flush();
  assert.equal(f.requests[0].base['pinless-cache-test-v1'],'old');
});


test('Back/Forward restore rechecks reset epochs before reusing stale collections',async()=>{
 const server={items:{},epoch:'old'},f=fixture({server});server.epoch='new';
 f.events.pageshow({persisted:true});await new Promise(resolve=>setImmediate(resolve));assert.equal(f.reloads,1);
});
test('Back/Forward restore reloads changed server collections without pending edits',async()=>{
 const server={items:{},epoch:'',revisions:{}},f=fixture({server});server.revisions={'pinless-cache-test-v1':'new'};
 f.events.pageshow({persisted:true});await new Promise(resolve=>setImmediate(resolve));assert.equal(f.reloads,1);
});
test('Back/Forward restore preserves unsaved edits for normal conflict resolution',async()=>{
 const server={items:{},epoch:'',revisions:{}},f=fixture({server});f.localStorage.setItem('pinless-cache-test-v1','my edit');server.revisions={'pinless-cache-test-v1':'new'};
 f.events.pageshow({persisted:true});await new Promise(resolve=>setImmediate(resolve));assert.equal(f.reloads,0);assert.equal(f.localStorage.getItem('pinless-cache-test-v1'),'my edit');
});

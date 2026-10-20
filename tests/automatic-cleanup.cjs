const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
class Memory {constructor(values){this.data=new Map(Object.entries(values));}get length(){return this.data.size}key(i){return [...this.data.keys()][i]??null}getItem(k){return this.data.get(k)??null}setItem(k,v){this.data.set(k,v)}removeItem(k){this.data.delete(k)}}
test('daily cleanup is opt-in, coalesced and preserves Restore, saved info and login state',async()=>{
 const localStorage=new Memory({'pt-shapes':'cache','pinless-playlists-v1':'saved','app-data-pending-v1':'unsaved','login-fixture':'private'}),sessionStorage=new Memory({'pt-sandbox-wall-v1':'restore','pinless-wall-snapshot-v1':'restore'}),calls=[];
 const window={localStorage,sessionStorage,appData:{epoch:'fixture'}},document={readyState:'loading',addEventListener(){}};let fail=false;
 vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../static/data-controls.js'),'utf8'),{window,document,setInterval(){},fetch:async(url,options)=>{calls.push(JSON.parse(options.body));if(fail)throw Error('offline');return {ok:true,json:async()=>({})}}});
 const run=window.appTemporaryStorage.automaticCleanup;
 await run();assert.equal(calls.length,0);
 localStorage.setItem('pinless-auto-cleanup','daily');await Promise.all([run(),run()]);assert.equal(calls.length,1);assert.equal(calls[0].clear,true);assert.equal(calls[0].epoch,'fixture');
 assert.equal(localStorage.getItem('pinless-playlists-v1'),'saved');assert.equal(localStorage.getItem('app-data-pending-v1'),'unsaved');assert.equal(localStorage.getItem('login-fixture'),'private');assert.equal(sessionStorage.getItem('pt-sandbox-wall-v1'),'restore');assert.equal(sessionStorage.getItem('pinless-wall-snapshot-v1'),'restore');
 await run();assert.equal(calls.length,1);
 const last=[...localStorage.data.keys()].find(k=>k.endsWith('-auto-cleanup-last'));localStorage.removeItem(last);fail=true;await assert.rejects(run(),/offline/);assert.equal(localStorage.getItem(last),null);
 fail=false;await run();assert.equal(calls.length,3);localStorage.removeItem(last);localStorage.setItem('pinless-auto-cleanup','off');await run();assert.equal(calls.length,3);
});

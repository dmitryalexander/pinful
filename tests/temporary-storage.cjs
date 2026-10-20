const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
class Memory {
 constructor(data){this.data=new Map(Object.entries(data));} get length(){return this.data.size} key(i){return [...this.data.keys()][i]??null} getItem(k){return this.data.get(k)??null} removeItem(k){this.data.delete(k)}
}
const local=new Memory({'pt-shapes':'cached',['pt-channel-avatar:UC'+'a'.repeat(22)]:'avatar','pt-video-lists-v1':'saved','pt-following-groups-v1':'saved','pinless-playlists-v1':'saved','app-data-pending-v1':'unsaved','foreign':'keep'});
const session=new Memory({'pt-sandbox-wall-v1':'wall','pt-sandbox-search-v2':'search','pinless-wall-snapshot-v1':'wall','pinless-open-pin-preview-v1':'preview','app-data-epoch-v1':'keep','foreign':'keep'});
const window={localStorage:local,sessionStorage:session};
vm.runInNewContext(fs.readFileSync(require('node:path').join(__dirname,'../static/data-controls.js'),'utf8'),{window,document:{readyState:'loading',addEventListener(){}},setInterval(){}});
const total=window.appTemporaryStorage.measure();assert(total>0);window.appTemporaryStorage.clear();assert.equal(window.appTemporaryStorage.measure(),0);
for(const key of ['pt-video-lists-v1','pt-following-groups-v1','pinless-playlists-v1','app-data-pending-v1','foreign']) assert(local.getItem(key));
assert(session.getItem('app-data-epoch-v1'));assert(session.getItem('foreign'));assert.equal(window.appTemporaryStorage.measure(),0);console.log('Temporary cache allowlist preserves saved data, pending edits and unrelated storage.');

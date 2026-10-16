const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const source=fs.readFileSync('static/wall-scroll.js','utf8');
const masonry=source.slice(source.indexOf('  function masonry(root)'),source.indexOf('  window.PinlessMasonry'));
for(const mode of ['lens','normal','previous']) {
 const single=mode==='lens';
 const frames=[];
 const item=(featured,height)=>({hidden:false,dataset:{},style:{},offsetHeight:height,classList:{contains:c=>featured&&c==='pin-featured'}});
 const hero=item(true,300),related=item(false,150);
 const root={clientWidth:1000,children:[hero,related],style:{},classList:{add(){},contains:c=>c==='pin-detail-wall'},hasAttribute:name=>name==='data-lens-page'?mode!=='normal':name==='data-previous-wall'&&mode==='previous',addEventListener(){},contains:()=>true};
 vm.runInNewContext(masonry+'\nmasonry(root);',{root,ResizeObserver:class{observe(){}unobserve(){}},requestAnimationFrame:fn=>{frames.push(fn);return 1;}});
 frames.shift()();
 assert.equal(hero.style.left,'0px');assert.equal(hero.style.top,'0px');
 assert.equal(hero.style.width,single?'247px':'498px');
 assert.equal(related.style.left,single?'251px':'502px');assert.equal(related.style.top,'0px');
}
console.log('PASS: Lens recovery hero stays in top-left single cell; recommendations fill adjacent cells; ordinary and inherited detail use a two-column hero.');

const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const text=fs.readFileSync('static/modern.js','utf8');
const start=text.indexOf(' // Dynamic walls carry');
const source=text.slice(start,text.indexOf(" document.addEventListener('click',e=>{const card=",start));
async function scenario(modified=false,fail=false,page='lists',contextual=false){
 let click,navigated,tabNavigated,opened=0,requested,prevented=false;
 const items=Array.from({length:100},(_,i)=>({id:String(i+1),image:'/image?url=https%3A%2F%2Fi.pinimg.com%2F'+i+'.jpg'}));
 const context={window:{open:()=>{opened++;return {location:{replace:url=>tabNavigated=url}};}},page,wall:{children:items.map(item=>({item,dataset:{}}))},record:card=>card.item,document:{addEventListener:(name,fn)=>click=fn},location:{pathname:'/',origin:'http://localhost:3001',assign:url=>navigated=url},URL,
 lists:()=>{assert.equal(page,'lists','feed context must come from the visible feed, not the active playlist');return {activeId:'active',lists:[{id:'active',items}]};},validPinId:id=>/^\d+$/.test(id),fetch:async(url,opts)=>{assert.equal(url,'/saved-wall-context');requested=JSON.parse(opts.body);return {ok:!fail,json:async()=>({context:'a'.repeat(32)})};}};
 vm.runInNewContext(source,context);
 await click({button:0,ctrlKey:modified,target:{closest:()=>({getAttribute:()=>'/pin/100'+(contextual?'?wall_context=existing':'')})},preventDefault:()=>prevented=true});
 if(contextual){assert.equal(prevented,false,'existing local context should use native new-tab navigation');assert.equal(requested,undefined);assert.equal(opened,0);return;}
 assert.equal(prevented,true);assert.equal(requested.pins.length,80);assert.equal(requested.pins[0].id,'100');assert.equal(items.length,100,'playlist unchanged');
 const destination=modified?tabNavigated:navigated;
 assert.ok(destination.startsWith('http://localhost:3001/pin/100'));
 assert.equal(destination.includes('wall_context='),!fail);
 if(modified){assert.equal(opened,1);assert.equal(navigated,undefined,'Ctrl-click must not navigate the original tab');}
}
(async()=>{await scenario();await scenario(true);await scenario(false,true);await scenario(false,false,'discover');await scenario(false,false,'restore');await scenario(false,false,'history');await scenario(true,false,'lists',true);console.log('PASS: playlist and dynamic feed clicks transfer their own bounded wall context with selected image first, keep Ctrl-clicks in a new local tab and do not change saved items.');})().catch(e=>{console.error(e);process.exit(1)});

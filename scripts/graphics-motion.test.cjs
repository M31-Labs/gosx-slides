const test = require('node:test'), assert = require('node:assert/strict'), fs = require('node:fs'), vm = require('node:vm');
const source = fs.readFileSync(require.resolve('../assets/graphics-motion.js'),'utf8');
const turn = () => new Promise(resolve => setImmediate(resolve));
test('cue adapter delegates preparation and sampling, retains late addresses and waits for GoSX',async()=>{
 const events = new Map(), calls = [], plans = [], labels = {};
 const frames = [{commands:[{kind:5,data:{z:10}}],label:'Start'},{commands:[{kind:5,data:{z:6}}],label:'Arrive',durationMs:1000,easing:'linear'}];
 const slide={dataset:{activeStep:'1'},classList:{contains:()=>true}};
 const mount={dataset:{slideSteps:JSON.stringify({version:1,frames})},closest:()=>slide,setAttribute:(key,value)=>labels[key]=value,removeAttribute:key=>delete labels[key]};
 const listen=(name,fn)=>events.set(name,fn), deck={querySelectorAll:()=>[mount],addEventListener:listen};
 const document={readyState:'complete',querySelector:selector=>selector==='main.deck'?deck:{},addEventListener:listen};
 const window={addEventListener:listen}; let pending=Promise.resolve();
 vm.runInNewContext(source,{document,window,console});
 window.SlidesGraphicsMotion.seek(500);
 assert.equal(mount.dataset.appliedStep,undefined,'late features cannot falsely mark a pose as applied');
 window.__gosx={motion:{
  prepareSceneTransition(from,to){const plan={from,to};plans.push(plan);return plan;},
  createEasing(value){assert.equal(value,'linear');return t=>t;},
  bindSceneTransition(_mount,plan){return {
   setTransition(next){plan=next;},sample(progress){calls.push({plan,progress});},
   settled:()=>pending,dispose(){calls.push('dispose');}
  };}
 }};
 events.get('gosx:ready')(); await turn();
 assert.equal(plans.length,1); assert.equal(calls.at(-1).progress,.5); assert.equal(mount.dataset.appliedStep,'1');assert.equal(labels['aria-label'],'Arrive');
 window.SlidesGraphicsMotion.seek(1000);window.SlidesGraphicsMotion.seek(500);assert.equal(plans.length,1,'prepare each cue once');
 let finish;pending=new Promise(resolve=>finish=resolve);let done=false;const waiting=window.SlidesGraphicsMotion.settled().then(()=>done=true);await turn();assert.equal(done,false);finish();await waiting;
 const edited=JSON.parse(JSON.stringify({version:1,frames}));edited.frames[1].commands[0].data.z=4;
 window.SlidesGraphicsMotion.replace(mount,edited);await turn();assert.equal(plans.length,2);assert.equal(calls.at(-1).plan.to[0].data.z,4);
 assert.throws(()=>window.SlidesGraphicsMotion.replace(mount,{version:1,frames:[]}),/Cue count/);
 events.get('pagehide')({persisted:true});assert.notEqual(calls.at(-1),'dispose');events.get('pagehide')({persisted:false});assert.equal(calls.at(-1),'dispose');
});

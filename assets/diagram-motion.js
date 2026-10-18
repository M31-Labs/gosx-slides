(function(){
  'use strict';const deck=document.querySelector('main.deck');if(!deck||!window.SlidesNav)return;
  const reduced=matchMedia('(prefers-reduced-motion: reduce)'), records=new Map();
  function active(){return deck.querySelector('.slide.deck-active');}
  function render(record,time){record.time=Math.max(0,Math.min(record.duration,time));record.transition?.sample(record.duration?record.time/record.duration:1);}
  function cancel(record){record.playback?.dispose();record.playback=null;record.paused=true;}
  function start(record,direction=1){
    cancel(record);
    if ((direction>0&&record.time>=record.duration)||(direction<0&&record.time<=0)) return;
    const motion=window.__gosx?.motion;
    if(!motion?.createTransport){render(record,record.duration);return;}
    const time=record.time, playback=record.playback=motion.createTransport({durationMs:record.duration});
    playback.attach({sample(ms,state){render(record,ms);record.direction=state.direction;record.paused=state.paused||(state.direction>0?ms>=record.duration:ms<=0);}});
    playback.restore({time,direction,paused:false});
  }
  function switchState(group,index,animate){let record=records.get(group);const figures=Array.from(group.children).filter(el=>el.matches('figure'));if(!record){record={templates:figures.map(el=>el.querySelector('svg').cloneNode(true)),index:-1,playback:null};records.set(group,record);}if(record.index===index){render(record,record.duration);record.paused=true;return;}cancel(record);const old=record.svg?.cloneNode(true);record.index=index;const next=record.templates[index].cloneNode(true);figures.forEach((el,i)=>{el.hidden=i!==index;el.inert=i!==index;el.setAttribute('aria-hidden',String(i!==index));});figures[index].replaceChildren(next);record.transition?.dispose();record.transition=null;record.svg=next;record.duration=Math.max(0,Math.min(10000,Number.isFinite(Number(group.dataset.diagramDuration))?Number(group.dataset.diagramDuration):700));record.time=0;
    record.direction=1;
    const morph=window.__gosx?.motion?.createSVGMorph;
    if(old&&animate&&!reduced.matches&&morph){record.transition=morph(old,next,{easing:group.dataset.diagramEasing||'ease-in-out'});render(record,0);if(!window.SlidesMotion)start(record);}else{render(record,record.duration);record.paused=true;}
  }
  function sync(){records.forEach(cancel);active()?.querySelectorAll('.slides-diagram-morph').forEach(group=>{const n=group.children.length;switchState(group,Math.min(n-1,SlidesNav.step()),records.has(group));});}
  function selected(){return Array.from(records).filter(([group])=>active()?.contains(group)).map(([,record])=>record);}
  window.SlidesDiagramMotion={replay(){active()?.querySelectorAll('.slides-diagram-morph').forEach(group=>{const r=records.get(group);if(!r)return;const index=r.index;r.index=-1;r.svg=r.templates[Math.max(0,index-1)].cloneNode(true);switchState(group,index,true);});},pause(){selected().forEach(r=>{cancel(r);r.paused=true;});},play(){selected().forEach(r=>start(r,r.direction||1));},seek(ms){selected().forEach(r=>{cancel(r);r.paused=true;render(r,reduced.matches?r.duration:ms);});},reverse(){selected().forEach(r=>{if(r.time===0)render(r,r.duration);start(r,-1);});},duration(){return Math.max(0,...selected().map(r=>r.duration));},state(){return {time:Math.max(0,...selected().map(r=>r.time)),paused:selected().every(r=>r.paused)};}};
  deck.addEventListener('slides:change',sync);deck.addEventListener('slides:before-change',()=>records.forEach(cancel));sync();
})();

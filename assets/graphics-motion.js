(function () {
  'use strict';
  const discrete = new Set(['id', 'kind', 'type', 'text', 'count', 'lineSegments', 'indices']);
  function blend(a, b, t, key) {
    if (t === 0) return a;
    if (t === 1) return b;
    if (discrete.has(key)) return a;
    if (typeof a === 'number' && typeof b === 'number') return a + (b - a) * t;
    if (key === 'color' && /^#[\da-f]{6}$/i.test(a) && /^#[\da-f]{6}$/i.test(b)) {
      let color = '#';
      for (let i = 1; i < 7; i += 2) color += Math.round(parseInt(a.slice(i,i+2),16) + (parseInt(b.slice(i,i+2),16)-parseInt(a.slice(i,i+2),16))*t).toString(16).padStart(2,'0');
      return color;
    }
    if (Array.isArray(a) && Array.isArray(b) && a.length === b.length) return b.map((v,i) => blend(a[i],v,t,key));
    if (a && b && typeof a === 'object' && typeof b === 'object' && !Array.isArray(a) && !Array.isArray(b)) {
      const out = {}; for (const k of new Set([...Object.keys(a),...Object.keys(b)])) out[k] = blend(a[k],b[k],t,k); return out;
    }
    return a;
  }
  // Visibility commands are discrete. Remove followed by create is a retained
  // upsert, rather than two destructive operations on every sampled frame.
  function absoluteCommand(c) {
    if (!c.data) return c;
    if (c.kind === 2 || c.kind === 5) return {...c, data:{x:0,y:0,rotationX:0,rotationY:0,rotationZ:0,...(c.kind === 2 ? {z:0,scaleX:1,scaleY:1,scaleZ:1} : {}),...c.data}};
    if (c.kind !== 0 || !c.data.props) return c;
    const props = {...c.data.props};
    // IR omits zero coordinates. Upserts must carry those zeros explicitly so
    // a previous nonzero label/point cannot leak into a backward seek.
    if (c.data.kind === 'label') Object.assign(props,{x:props.x ?? 0,y:props.y ?? 0,z:props.z ?? 0});
    if (Array.isArray(props.points)) props.points = props.points.map(p => p && typeof p === 'object' && !Array.isArray(p) ? {x:0,y:0,z:0,...p} : p);
    return {...c,data:{...c.data,props}};
  }
  function commandMap(commands) {
    const out = new Map();
    for (const raw of commands) {
      const c = absoluteCommand(raw);
      if (c.kind === 0 || c.kind === 1) out.set('object:' + c.objectId, c);
      else out.set(c.kind + ':' + (c.objectId || ''), c);
    }
    return out;
  }
  function sampleCommands(from, to, progress) {
    const t = Math.max(0, Math.min(1, progress)), a = commandMap(from), b = commandMap(to), out = [];
    for (const key of new Set([...a.keys(), ...b.keys()])) {
      const old = a.get(key), next = b.get(key);
      if (!next) { if (old) out.push(old); continue; }
      if (!old || old.kind !== next.kind || old.kind === 1) { out.push(t < 1 && old ? old : next); continue; }
      out.push({...next, data: blend(old.data, next.data, t)});
    }
    // Creating a restored object must precede transforms/material updates.
    return out.sort((a,b) => (a.kind === 0 ? -1 : 0) - (b.kind === 0 ? -1 : 0));
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = {sampleCommands};
  if (typeof document === 'undefined') return;
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  function start() {
    const deck = document.querySelector('main.deck');
    if (!deck || !document.querySelector('#gosx-manifest')) return;
    const controllers = Array.from(deck.querySelectorAll('.slide-graphic[data-slide-steps]')).map(mount => {
      let frames = JSON.parse(mount.dataset.slideSteps).frames;
      const slide = mount.closest('[data-slide]');
      let index = 0, desired = frames[0].commands, pending = null, timer = null, attempts = 0, revision = 0, owner = null, last = new Map(), error = null;
      const active = () => slide?.classList.contains('deck-active');
      const length = () => index === 0 ? 0 : frames[index].durationMs ?? 600;
      const easing = frames => frames.map(frame => {
        const effect = new KeyframeEffect(null,[{}],{duration:1,fill:'both',easing:frame.easing || 'ease-in-out'}), animation = new Animation(effect);
        return t => { animation.currentTime = t; return effect.getComputedTiming().progress ?? t; };
      });
      let ease = easing(frames);
      function pump() {
        timer = null;
        if (!active() || pending) return;
        const handle = mount.__gosxScene3DHandle;
        if (!handle?.__gosxScene3DCommandReady) { if (++attempts <= 100) timer = setTimeout(pump,100); return; }
        if (owner !== handle) { owner = handle; last = new Map(); }
        attempts = 0;
        const target = desired, targetIndex = index, version = revision, next = commandMap(target), changes = [];
        for (const [key,c] of next) if (last.get(key) !== JSON.stringify(c)) changes.push(c);
        if (!changes.length) { mount.dataset.appliedStep = String(targetIndex); mount.setAttribute('aria-label',frames[targetIndex].label || 'Slide graphic'); return; }
        pending = Promise.resolve().then(() => handle.applyCommands(changes)).then(() => {
          last = new Map(Array.from(next,([key,c]) => [key,JSON.stringify(c)])); error = null;
          mount.dataset.appliedStep = String(targetIndex); mount.setAttribute('aria-label',frames[targetIndex].label || 'Slide graphic'); mount.removeAttribute('data-slide-step-error');
        }).catch(e => { error = e; mount.dataset.slideStepError = String(e.message || e); }).finally(() => { pending = null; if (revision !== version) pump(); });
      }
      function seek(ms) {
        const duration = length(), t = reduced.matches ? 1 : duration ? Math.max(0,Math.min(1,ms/duration)) : 1;
        desired = sampleCommands(frames[Math.max(0,index-1)].commands,frames[index].commands,ease[index](t)); revision++; pump();
      }
      return {mount, active, length, seek,
        frames: () => frames,
        inspect(frame,progress) {
          const target = Math.max(0,Math.min(frames.length-1,frame));
          desired = sampleCommands(frames[Math.max(0,target-1)].commands,frames[target].commands,ease[target](progress)); revision++; pump();
        },
        replace(timeline) {
          if (timeline.version !== 1 || timeline.frames?.length !== frames.length) throw new Error('Cue count changed; reload the deck.');
          frames = timeline.frames; ease = easing(frames); this.sync();
        },
        sync() { if (timer) clearTimeout(timer); timer = null; index = Math.max(0,Math.min(frames.length-1,Number(slide?.dataset.activeStep)||0)); desired = frames[index].commands; revision++; attempts = 0; pump(); },
        async settled() { while (pending) await pending; if (error) throw error; if (active() && !owner) throw new Error('Graphic surface is not ready'); }
      };
    });
    window.SlidesGraphicsMotion = {
      current() { return controllers.filter(c=>c.active()).map(c=>({mount:c.mount,frames:c.frames()})); },
      inspect(frame,progress) { controllers.filter(c=>c.active()).forEach(c=>c.inspect(frame,progress)); },
      replace(mount,timeline) { const controller = controllers.find(c=>c.mount===mount); if (!controller) throw new Error('Scene is unavailable'); controller.replace(timeline); },
      seek(ms) { controllers.filter(c=>c.active()).forEach(c=>c.seek(ms)); },
      duration() { return Math.max(0,...controllers.filter(c=>c.active()).map(c=>c.length())); },
      async settled() { await Promise.all(controllers.filter(c=>c.active()).map(c=>c.settled())); }
    };
    function sync() { controllers.forEach(c=>c.sync()); }
    deck.addEventListener('slides:change',sync); sync();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded',start); else start();
})();

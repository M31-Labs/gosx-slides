// Slides owns cue addresses and timing. GoSX owns pose preparation, readiness,
// command coalescing and absolute sampling; this adapter contains no renderer.
(function () {
  'use strict';
  function start() {
    const deck = document.querySelector('main.deck');
    if (!deck || !document.querySelector('#gosx-manifest')) return;
    const controllers = Array.from(deck.querySelectorAll('.slide-graphic[data-slide-steps]')).map(mount => {
      let frames = JSON.parse(mount.dataset.slideSteps).frames;
      const slide = mount.closest('[data-slide]'), plans = new Map();
      let index = 0, desired = {frame: 0, progress: 1}, binding = null, watching = null;
      const active = () => slide?.classList.contains('deck-active');
      const length = () => index === 0 ? 0 : frames[index].durationMs ?? 600;
      function render() {
        if (!active()) return;
        const motion = window.__gosx?.motion;
        // Navigation can address a cue before optional framework features load.
        if (!motion?.prepareSceneTransition) return;
        const target = desired.frame;
        let prepared = plans.get(target);
        if (!prepared) {
          prepared = {plan: motion.prepareSceneTransition(frames[Math.max(0,target-1)].commands,frames[target].commands), ease: motion.createEasing(frames[target].easing || 'ease-in-out')};
          plans.set(target,prepared);
        }
        if (!binding) binding = motion.bindSceneTransition(mount,prepared.plan);
        else binding.setTransition(prepared.plan);
        binding.sample(prepared.ease(desired.progress));
        if (!watching) watching = binding.settled().then(() => {
          mount.dataset.appliedStep = String(desired.frame);
          mount.setAttribute('aria-label',frames[desired.frame].label || 'Slide graphic');
          mount.removeAttribute('data-slide-step-error');
        }).catch(error => { mount.dataset.slideStepError = String(error.message || error); }).finally(() => { watching = null; });
      }
      return {mount, active, length,
        frames: () => frames,
        seek(ms) { const duration = length(); desired = {frame:index,progress:duration ? Math.max(0,Math.min(1,ms/duration)) : 1}; render(); },
        inspect(frame,progress) { desired = {frame:Math.max(0,Math.min(frames.length-1,frame)),progress}; render(); },
        replace(timeline) {
          if (timeline.version !== 1 || timeline.frames?.length !== frames.length) throw new Error('Cue count changed; reload the deck.');
          frames = timeline.frames; plans.clear(); this.sync();
        },
        sync() { index = Math.max(0,Math.min(frames.length-1,Number(slide?.dataset.activeStep)||0)); desired = {frame:index,progress:1}; render(); },
        render,
        async settled() {
          render();
          if (!binding) throw new Error('GoSX scene motion is unavailable');
          await binding.settled(); if (watching) await watching;
        },
        dispose() { binding?.dispose(); }
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
    deck.addEventListener('slides:change',sync);
    document.addEventListener('gosx:ready',() => controllers.forEach(c=>c.render()),{once:true});
    window.addEventListener('pagehide',event => { if (!event.persisted) controllers.forEach(c=>c.dispose()); });
    sync();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded',start); else start();
})();

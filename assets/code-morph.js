(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck || !window.SlidesNav) return;
  const activeBlocks = new WeakMap(), records = new WeakMap(), reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const groups = () => Array.from(deck.querySelectorAll('.deck-active .slides-code-morph'));
  const length = group => Math.max(1, Math.min(600000, Number(group.dataset.codeDuration) || 450));
  function sync() {
    const slide = deck.querySelector('.slide.deck-active');
    slide.querySelectorAll('.slides-code-morph').forEach(group => {
      const blocks = Array.from(group.children).filter(el => el.matches('pre'));
      blocks.forEach(block => { block.removeAttribute('data-emphasized'); block.removeAttribute('data-steps'); });
      const index = Math.min(blocks.length - 1, SlidesNav.step()), next = blocks[index];
      const previous = activeBlocks.get(group);
      if (previous === next) return;
      (records.get(group) || []).forEach(a => a.cancel());
      const animations = [];
      const old = new Map();
      const fromBlock = blocks[index - 1];
      // Measure the authored adjacent version, independent of navigation history.
      if (fromBlock && !reduced.matches) {
        blocks.forEach(block => { block.hidden = block !== fromBlock; });
        fromBlock.querySelectorAll('.ts-line').forEach(line => {
        const key = line.textContent.trim();
        if (key) { const list = old.get(key) || []; list.push(line.getBoundingClientRect()); old.set(key, list); }
        });
      }
      blocks.forEach(block => { block.hidden = block !== next; block.inert = block !== next; block.setAttribute('aria-hidden', String(block !== next)); });
      if (next && fromBlock && !reduced.matches) {
        next.querySelectorAll('.ts-line').forEach(line => {
          const matches = old.get(line.textContent.trim()), from = matches && matches.shift();
          const to = line.getBoundingClientRect();
          const frames = from ? [{transform: `translate(${from.x-to.x}px, ${from.y-to.y}px)`}, {transform: 'none'}] : [{opacity: 0}, {opacity: 1}];
          const animation = line.animate(frames, {duration: length(group), easing: group.dataset.codeEasing || 'cubic-bezier(.25,1,.5,1)', fill: 'both'});
          animation.pause(); animation.currentTime = 0; animations.push(animation);
        });
      }
      activeBlocks.set(group, next);
      records.set(group, animations);
    });
    // The shared timeline chooses this step's time before our change listener.
    // Sample new records now, including paused backward/direct navigation, so
    // they cannot paint the canonical source pose while the playhead is at end.
    if (window.SlidesMotion) window.SlidesCodeMotion.seek(SlidesMotion.state().time);
  }
  window.SlidesCodeMotion = {
    seek(ms) { groups().forEach(group => (records.get(group) || []).forEach(a => { a.pause(); a.currentTime = Math.max(0, Math.min(length(group), ms)); })); },
    duration() { return Math.max(0, ...groups().filter(group => (records.get(group) || []).length).map(length)); }
  };
  deck.addEventListener('slides:change', sync); sync();
})();

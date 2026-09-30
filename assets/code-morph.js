(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck || !window.SlidesNav) return;
  const activeBlocks = new WeakMap(), reduced = matchMedia('(prefers-reduced-motion: reduce)');
  function sync() {
    const slide = deck.querySelector('.slide.deck-active');
    slide.querySelectorAll('.slides-code-morph').forEach(group => {
      const blocks = Array.from(group.children).filter(el => el.matches('pre'));
      const index = Math.min(blocks.length - 1, SlidesNav.step()), next = blocks[index];
      const previous = activeBlocks.get(group);
      const old = new Map();
      if (previous && previous !== next && !reduced.matches) previous.querySelectorAll('.ts-line').forEach(line => {
        const key = line.textContent.trim();
        if (key) { const list = old.get(key) || []; list.push(line.getBoundingClientRect()); old.set(key, list); }
      });
      blocks.forEach(block => { block.hidden = block !== next; block.inert = block !== next; block.setAttribute('aria-hidden', String(block !== next)); });
      if (next && previous !== next && !reduced.matches) {
        next.querySelectorAll('.ts-line').forEach(line => {
          const matches = old.get(line.textContent.trim()), from = matches && matches.shift();
          const to = line.getBoundingClientRect();
          if (from) line.animate([{transform: `translate(${from.x-to.x}px, ${from.y-to.y}px)`}, {transform: 'none'}], {duration: 450, easing: 'cubic-bezier(.25,1,.5,1)'});
          else line.animate([{opacity: 0}, {opacity: 1}], {duration: 300});
        });
      }
      activeBlocks.set(group, next);
    });
  }
  deck.addEventListener('slides:change', sync); sync();
})();

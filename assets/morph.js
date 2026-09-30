(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck) return;
  let source = new Map();
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  function targets(slide) { return slide ? Array.from(slide.querySelectorAll('[data-morph-id]')) : []; }
  deck.addEventListener('slides:before-change', event => {
    source = new Map();
    const slide = deck.querySelector('.slide[data-slide="' + event.detail.from + '"]');
    targets(slide).forEach(el => {
      if (!el.getClientRects().length || el.closest('[data-gosx-island], [data-gosx-engine]')) return;
      const rect = el.getBoundingClientRect(), css = getComputedStyle(el);
      source.set(el.dataset.morphId, { rect, opacity: css.opacity });
    });
  });
  deck.addEventListener('slides:change', () => {
    if (reduced.matches || !source.size) return;
    const slide = deck.querySelector('.slide.deck-active');
    const duration = Math.max(0, Math.min(10000, Number(slide.dataset.morphDuration) || 450));
    targets(slide).forEach(el => {
      const old = source.get(el.dataset.morphId);
      if (!old || !el.getClientRects().length || el.closest('[data-gosx-island], [data-gosx-engine]')) return;
      const rect = el.getBoundingClientRect();
      if (!rect.width || !rect.height) return;
      // Animate destination transforms, preserving both DOM trees and widget
      // state. Authors opt in with stable IDs; no clone or layout tween loop.
      el.animate([{ transformOrigin: 'top left', transform: `translate(${old.rect.x - rect.x}px, ${old.rect.y - rect.y}px) scale(${old.rect.width / rect.width}, ${old.rect.height / rect.height})`, opacity: old.opacity },
        { transformOrigin: 'top left', transform: 'none', opacity: getComputedStyle(el).opacity }], { duration, easing: 'cubic-bezier(.25,1,.5,1)' });
    });
    source.clear();
  });
})();

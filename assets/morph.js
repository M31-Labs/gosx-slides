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
      const id = el.dataset.morphId; source.set(id, source.has(id) ? null : { rect, opacity: css.opacity });
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
      let dx = old.rect.x - rect.x, dy = old.rect.y - rect.y;
      if (el instanceof SVGGraphicsElement) { const ctm = el.getScreenCTM(); if (ctm) { const inverse = ctm.inverse(), origin = new DOMPoint(0,0).matrixTransform(inverse), delta = new DOMPoint(dx,dy).matrixTransform(inverse); dx = delta.x-origin.x; dy = delta.y-origin.y; } }
      else {
        // Slide fitting scales the ancestor tree. Convert screen deltas back to
        // the destination's CSS coordinates so morphs start at the source pose.
        let matrix = new DOMMatrix();
        for (let parent = el.parentElement; parent; parent = parent.parentElement) {
          const transform = getComputedStyle(parent).transform;
          if (transform !== 'none') matrix = new DOMMatrix(transform).multiply(matrix);
        }
        const inverse = matrix.inverse(), origin = new DOMPoint(0,0).matrixTransform(inverse), delta = new DOMPoint(dx,dy).matrixTransform(inverse);
        dx = delta.x - origin.x; dy = delta.y - origin.y;
      }
      const base = getComputedStyle(el).transform;
      const box = el instanceof SVGGraphicsElement ? 'fill-box' : 'border-box';
      el.animate([{ transformBox: box, transformOrigin: 'top left', transform: `translate(${dx}px, ${dy}px) scale(${old.rect.width / rect.width}, ${old.rect.height / rect.height}) ${base === 'none' ? '' : base}`, opacity: old.opacity },
        { transformBox: box, transformOrigin: 'top left', transform: base, opacity: getComputedStyle(el).opacity }], { duration, easing: 'cubic-bezier(.25,1,.5,1)' });
    });
    source.clear();
  });
})();

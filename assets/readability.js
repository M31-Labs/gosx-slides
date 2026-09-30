(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck) return;
  function rgb(value) { const m = /^rgba?\(([\d.]+)[ ,]+([\d.]+)[ ,]+([\d.]+)(?:[, /]+([\d.]+))?\)$/.exec(value); return m && (!m[4] || Number(m[4]) === 1) ? m.slice(1,4).map(Number) : null; }
  function luminance(c) { return c.map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }).reduce((n, v, i) => n + v * [.2126, .7152, .0722][i], 0); }
  function check() {
    const slide = deck.querySelector('.slide.deck-active'), warnings = [];
    if (!slide) return warnings;
    const scale = Math.min(1, slide.getBoundingClientRect().height / slide.offsetHeight);
    slide.querySelectorAll('h1,h2,h3,p,li,td,th,code').forEach(el => {
      if (!el.textContent.trim() || !el.getClientRects().length || el.closest('[aria-hidden="true"], [data-slides-cue-visible="false"], .slide-scene')) return;
      const css = getComputedStyle(el), font = parseFloat(css.fontSize) * scale;
      if (font < (el.matches('code') ? 14 : 18)) warnings.push({ kind: 'font-size', text: el.textContent.trim().slice(0,80), value: Math.round(font), advice: 'Increase type size or split this slide.' });
      let bg = null, node = el, complex = false;
      while (node && !bg) { const style = getComputedStyle(node); if (style.backgroundImage !== 'none' || Number(style.opacity) < 1) complex = true; bg = rgb(style.backgroundColor); node = node.parentElement; }
      const fg = rgb(css.color);
      if (fg && bg && !complex && css.backgroundImage === 'none') {
        const a = luminance(fg), b = luminance(bg), ratio = (Math.max(a,b)+.05)/(Math.min(a,b)+.05);
        if (ratio < 3) warnings.push({ kind: 'contrast', text: el.textContent.trim().slice(0,80), value: Number(ratio.toFixed(2)), advice: 'Choose colors with stronger contrast.' });
      }
    });
    return warnings;
  }
  function open() {
    let panel = document.querySelector('.slides-readability-panel');
    if (!panel) { panel = document.createElement('dialog'); panel.className = 'slides-author-panel slides-readability-panel'; panel.setAttribute('aria-label', 'Readability check'); deck.appendChild(panel); }
    panel.replaceChildren(); const header = document.createElement('header'), title = document.createElement('h2'), close = document.createElement('button'); title.textContent = 'Readability check'; close.textContent = '×'; close.type = 'button'; close.setAttribute('aria-label', 'Close readability check'); close.onclick = () => panel.close(); header.append(title,close); panel.append(header);
    const intro = document.createElement('p'); intro.textContent = 'Checks the current viewport. Gradients, transparent colors, and custom compositing need visual inspection.'; panel.append(intro);
    const warnings = check(); const list = document.createElement('ul');
    if (!warnings.length) { const item = document.createElement('li'); item.textContent = 'No small-type or measured contrast warnings on this slide.'; list.append(item); }
    warnings.forEach(warning => { const item = document.createElement('li'); item.textContent = warning.kind + ' (' + warning.value + '): ' + warning.text + ' — ' + warning.advice; list.append(item); }); panel.append(list); if (!panel.open) panel.showModal();
  }
  document.addEventListener('keydown', event => { if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest('input,textarea,select,[contenteditable],dialog,[role]')) return; if ((event.key === 'r' || event.key === 'R') && !SlidesNav.isOverview()) { event.preventDefault(); open(); } });
  window.SlidesReadability = { check, open };
})();

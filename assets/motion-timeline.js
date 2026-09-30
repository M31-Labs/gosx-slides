(function () {
  'use strict';
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const records = new WeakMap(), played = new WeakSet();
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let paused = false, panel = null, selected = null, refreshTimer = null;
  function active() { return deck.querySelector('.slide.deck-active'); }
  function items(slide) { return Array.from((slide || active()).querySelectorAll('[data-slides-motion-replay]')); }
  function frames(el) {
    const distance = Math.max(0, Number(el.dataset.gosxMotionDistance) || 18);
    const preset = el.dataset.gosxMotionPreset || 'fade';
    const transform = { 'slide-up': `translateY(${distance}px)`, 'slide-down': `translateY(${-distance}px)`,
      'slide-left': `translateX(${distance}px)`, 'slide-right': `translateX(${-distance}px)`, 'zoom-in': 'scale(.94)' }[preset];
    return [{ opacity: 0, ...(transform ? { transform } : {}) }, { opacity: 1, ...(transform ? { transform: 'none' } : {}) }];
  }
  function number(el, key, fallback) {
    const n = Number(el.getAttribute('data-gosx-motion-' + key));
    return Number.isFinite(n) && n >= 0 && n <= 600000 ? n : fallback;
  }
  function animations() { return active().getAnimations({ subtree: true }).filter(a => a.effect); }
  function pause() { paused = true; animations().forEach(a => a.pause()); updatePanel(); }
  function play() { paused = false; animations().forEach(a => a.play()); updatePanel(); }
  function seek(ms) { paused = true; animations().forEach(a => { a.pause(); a.currentTime = Math.max(0, Math.min(duration(), Number(ms) || 0)); }); updatePanel(); }
  function duration() { return animations().reduce((n, a) => Math.max(n, Number(a.effect.getComputedTiming().endTime) || 0), 0); }
  function reverse() { paused = false; animations().forEach(a => { if (a.currentTime === 0) a.currentTime = a.effect.getComputedTiming().endTime; a.reverse(); }); updatePanel(); }
  function run(el, delay) {
    const old = records.get(el); if (old) old.cancel();
    if (played.has(el) && el.dataset.slidesMotionReplay === "once") return;
    played.add(el);
    if (reduce.matches && el.dataset.gosxMotionRespectReduced !== 'false') return;
    const api = window.__gosx && window.__gosx.motion;
    if (api) api.dispose(el);
    const split = el.dataset.gosxMotionSplit;
    // Native GoSX splitting stays available for ordinary entrances. Cued groups
    // preserve their child markup and widgets rather than rebuilding text/DOM.
    const animation = el.animate(frames(el), { duration: number(el, 'duration', 220), delay,
      easing: el.dataset.gosxMotionEasing || 'ease-out', fill: 'both' });
    records.set(el, animation);
    el.dataset.gosxMotionState = 'running';
    animation.finished.then(() => { if (records.get(el) === animation) el.dataset.gosxMotionState = 'finished'; }, () => {});
    if (paused) animation.pause();
    if (split) el.dataset.slidesMotionNotice = 'Cue groups preserve markup; use staggered cue groups for rich content.';
  }
  function sync(replay) {
    const slide = active(), step = SlidesNav.step();
    const names = JSON.parse(slide.dataset.slideCues || '[]');
    const entrances = items(slide);
    const times = new Map(), visiting = new Set();
    function delay(el) {
      if (times.has(el)) return times.get(el);
      if (visiting.has(el)) { el.dataset.slidesMotionError = 'Circular after dependency'; return number(el, 'delay', 0); }
      visiting.add(el);
      let ms = number(el, 'delay', 0);
      const after = el.dataset.slidesMotionAfter;
      const predecessor = after && entrances.find(other => other !== el && other.dataset.slidesMotionCue === after);
      if (predecessor && predecessor.dataset.slidesMotionStep === el.dataset.slidesMotionStep) ms += delay(predecessor) + number(predecessor, 'duration', 220);
      const group = el.dataset.slidesMotionGroup;
      if (group) { const members = entrances.filter(other => other.dataset.slidesMotionGroup === group && other.dataset.slidesMotionStep === el.dataset.slidesMotionStep); ms += members.indexOf(el) * number(el, "stagger", 0); }
      visiting.delete(el); times.set(el, ms); return ms;
    }
    entrances.forEach(el => {
      const cue = el.dataset.slidesMotionCue;
      if (!el.hasAttribute('data-slides-motion-step') && cue) el.dataset.slidesMotionStep = String(Math.max(0, names.indexOf(cue)));
      if (!el.hasAttribute('data-slides-motion-step')) return;
      const start = Number(el.dataset.slidesMotionStep);
      const visible = step >= start;
      const was = el.dataset.slidesCueVisible === 'true';
      el.dataset.slidesCueVisible = String(visible);
      if (!visible) {
        if (!el.hasAttribute('data-slides-authored-inert')) el.dataset.slidesAuthoredInert = String(el.hasAttribute('inert'));
        if (!el.hasAttribute('data-slides-authored-aria')) el.dataset.slidesAuthoredAria = el.getAttribute('aria-hidden') || '';
        el.inert = true; el.setAttribute('aria-hidden', 'true');
        const animation = records.get(el); if (animation) animation.cancel();
        el.removeAttribute('data-gosx-motion');
      } else {
        if (el.dataset.slidesAuthoredInert !== 'true') el.inert = false;
        if (el.dataset.slidesAuthoredAria) el.setAttribute('aria-hidden', el.dataset.slidesAuthoredAria); else el.removeAttribute('aria-hidden');
        if (!was || replay) run(el, delay(el));
      }
    });
    updatePanel();
  }
  // Claim cued entrances before the deferred GoSX bootstrap mounts them.
  deck.querySelectorAll('[data-slides-motion-cue], [data-slides-motion-step]').forEach(el => el.removeAttribute('data-gosx-motion'));
  function replay() {
    items().forEach(el => {
      if (el.hasAttribute('data-slides-motion-step')) return;
      const api = window.__gosx && window.__gosx.motion;
      if (api) { api.dispose(el); el.removeAttribute('data-gosx-motion-revealed'); api.observe(el); }
    });
    sync(true);
  }
  deck.addEventListener('slides:change', () => { paused = false; sync(false); });
  deck.addEventListener('slides:before-change', event => {
    const previous = deck.querySelector('.slide[data-slide="' + event.detail.from + '"]');
    if (previous) items(previous).forEach(el => { const a = records.get(el); if (a) a.cancel(); el.dataset.slidesCueVisible = 'false'; });
  });
  function updatePanel() {
    if (!panel || !panel.open) return;
    panel.querySelector('[data-motion-pause]').textContent = paused ? 'Play' : 'Pause';
    const slider = panel.querySelector('[data-motion-seek]'); slider.max = String(Math.max(1, duration()));
    slider.value = String(Math.max(0, ...animations().map(a => Number(a.currentTime) || 0)));
    panel.querySelector('[data-motion-time]').textContent = Math.round(Number(slider.value)) + ' / ' + Math.round(duration()) + ' ms';
  }
  function open() {
    if (!panel) {
      panel = document.createElement('dialog'); panel.className = 'slides-author-panel';
      panel.setAttribute('aria-labelledby', 'slides-motion-title');
      panel.innerHTML = '<header><h2 id="slides-motion-title">Motion studio</h2><button type="button" data-motion-close aria-label="Close motion studio">×</button></header>' +
        '<p>Preview this slide’s timing. Edits are temporary; copy the directive into your deck.</p>' +
        '<label>Element<select data-motion-element></select></label>' +
        '<div class="slides-author-fields"><label>Preset<select data-motion-preset><option>fade</option><option>slide-up</option><option>slide-down</option><option>slide-left</option><option>slide-right</option><option>zoom-in</option></select></label>' +
        '<label>Duration (ms)<input data-motion-duration type="number" min="1" max="600000"></label><label>Delay (ms)<input data-motion-delay type="number" min="0" max="600000"></label>' +
        '<label>Easing<select data-motion-easing><option>ease-out</option><option>ease-in-out</option><option>linear</option><option>ease</option></select></label></div>' +
        '<div class="slides-author-actions"><button type="button" data-motion-pause>Pause</button><button type="button" data-motion-replay>Replay</button><button type="button" data-motion-reverse>Reverse</button><button type="button" data-motion-copy>Copy directive</button></div>' +
        '<label>Timeline<input data-motion-seek type="range" min="0" max="1" value="0" aria-label="Motion time"></label><output data-motion-time></output><output data-motion-status aria-live="polite"></output>';
      deck.appendChild(panel);
      panel.querySelector('[data-motion-close]').onclick = () => panel.close();
      panel.querySelector('[data-motion-pause]').onclick = () => paused ? play() : pause();
      panel.querySelector('[data-motion-replay]').onclick = replay;
      panel.querySelector('[data-motion-reverse]').onclick = reverse;
      panel.querySelector('[data-motion-seek]').oninput = event => seek(event.target.value);
      const selector = panel.querySelector('[data-motion-element]');
      selector.onchange = () => { selected = items()[Number(selector.value)]; fill(); };
      for (const key of ['preset', 'duration', 'delay', 'easing']) panel.querySelector('[data-motion-' + key + ']').onchange = event => {
        if (!selected) return;
        if (key === 'duration' || key === 'delay') { const n = Number(event.target.value); if (!Number.isFinite(n) || n < 0 || n > 600000) return; }
        selected.setAttribute('data-gosx-motion-' + key, event.target.value); replay();
      };
      panel.querySelector('[data-motion-copy]').onclick = async () => {
        if (!selected) return;
        const fields = ['preset', 'duration', 'delay', 'easing'].map(k => k + '=' + selected.getAttribute('data-gosx-motion-' + k));
        for (const key of ['cue', 'step', 'after', 'group']) if (selected.hasAttribute('data-slides-motion-' + key)) fields.push(key + '=' + selected.getAttribute('data-slides-motion-' + key));
        try { await navigator.clipboard.writeText(':::motion {' + fields.join(' ') + '}\nYour content\n:::'); panel.querySelector('[data-motion-status]').textContent = 'Copied'; }
        catch (_) { panel.querySelector('[data-motion-status]').textContent = 'Clipboard unavailable'; }
      };
      panel.addEventListener('close', () => { clearInterval(refreshTimer); refreshTimer = null; });
    }
    const selector = panel.querySelector('[data-motion-element]'); selector.replaceChildren();
    items().forEach((el, i) => { const option = document.createElement('option'); option.value = i; option.textContent = el.dataset.slidesMotionCue || el.id || el.textContent.trim().slice(0, 55) || 'Element ' + (i + 1); selector.appendChild(option); });
    selected = items()[0]; fill(); panel.showModal(); updatePanel(); refreshTimer = setInterval(updatePanel, 100);
  }
  function fill() {
    for (const key of ['preset', 'duration', 'delay', 'easing']) { const input = panel.querySelector('[data-motion-' + key + ']'); input.disabled = !selected; const value = selected ? selected.getAttribute('data-gosx-motion-' + key) || (key === "easing" ? "ease-out" : "") : "";
      if (input.tagName === "SELECT" && value && !Array.from(input.options).some(option => option.value === value)) { const option = document.createElement("option"); option.value = option.textContent = value; input.appendChild(option); }
      input.value = value; }
  }
  document.addEventListener('keydown', event => {
    if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest('input, textarea, select, [contenteditable], dialog, [role]')) return;
    if ((event.key === 'm' || event.key === 'M') && !SlidesNav.isOverview()) { event.preventDefault(); open(); }
  });
  window.SlidesMotion = { pause, play, seek, replay, reverse, open, duration, state: () => ({ paused, time: Math.max(0, ...animations().map(a => Number(a.currentTime) || 0)), duration: duration() }) };
  sync(false);
})();

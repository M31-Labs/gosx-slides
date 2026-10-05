(function () {
  'use strict';
  const deck = document.querySelector('main.deck'), manifest = document.querySelector('#slides-story');
  if (!deck || !manifest || !window.SlidesNav) return;
  const story = JSON.parse(manifest.textContent), baseline = new WeakMap(), reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const beats = new Map(story.beats.map(beat => [beat.slideIndex + ':' + beat.step, beat]));
  const active = () => deck.querySelector('.deck-active[data-slide]');
  const current = () => beats.get((SlidesNav.current() - 1) + ':' + SlidesNav.step());
  function original(el) {
    if (!baseline.has(el)) baseline.set(el, { opacity: el.style.opacity, hidden: el.hidden, inert: el.inert, aria: el.getAttribute('aria-hidden') });
    return baseline.get(el);
  }
  function visible(beat, id, actor, fallback) {
    if (actor) return beat?.reveal == null || beat.reveal.includes(id);
    if (beat?.hide?.includes(id)) return false;
    if (beat?.show?.includes(id)) return true;
    return fallback;
  }
  function opacity(beat, id, actor, fallback) {
    if (!visible(beat, id, actor, fallback)) return 0;
    return actor && beat?.focus?.length && !beat.focus.includes(id) ? .2 : 1;
  }
  const caption = document.createElement('p'); caption.className = 'slides-story-caption'; caption.setAttribute('aria-live', 'polite'); caption.setAttribute('aria-label', 'Story caption'); caption.tabIndex = 0; caption.hidden = true; deck.appendChild(caption);
  let layoutFrame = 0;
  function captionLayout() {
    layoutFrame = 0;
    const controls = deck.querySelector('.deck-controls'), bounds = controls?.getBoundingClientRect();
    const reserve = 2.7 * parseFloat(getComputedStyle(document.documentElement).fontSize);
    const bottom = bounds?.height ? Math.max(reserve, innerHeight - bounds.top + 12) : reserve;
    caption.style.bottom = bottom + 'px';
    deck.dispatchEvent(new Event('slides:caption-layout'));
  }
  function scheduleCaptionLayout() { if (!layoutFrame) layoutFrame = requestAnimationFrame(captionLayout); }
  if (window.ResizeObserver) {
    const observer = new ResizeObserver(scheduleCaptionLayout); observer.observe(caption);
    const controls = deck.querySelector('.deck-controls'); if (controls) observer.observe(controls);
  }
  window.addEventListener('resize', scheduleCaptionLayout); window.addEventListener('load', scheduleCaptionLayout);
  caption.addEventListener('keydown', event => {
    if (caption.scrollHeight > caption.clientHeight && ['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End', ' '].includes(event.key)) event.stopPropagation();
  });
  function seek(ms) {
    const slide = active(); if (!slide) return;
    const beat = current(), previous = beats.get((SlidesNav.current() - 1) + ':' + (SlidesNav.step() - 1));
    const t = reduced.matches || !beat?.durationMs ? 1 : Math.max(0, Math.min(1, ms / beat.durationMs));
    const actors = Array.from(slide.querySelectorAll('svg [data-sirena-id]'));
    const dom = Array.from(slide.querySelectorAll('[data-story-id],[id]')).filter(el => !el.closest('svg'));
    for (const el of [...actors, ...dom]) {
      const actor = el.hasAttribute('data-sirena-id'), id = actor ? el.dataset.sirenaId : el.dataset.storyId || el.id, base = original(el);
      const target = visible(beat, id, actor, !base.hidden), before = opacity(previous, id, actor, !base.hidden), after = opacity(beat, id, actor, !base.hidden);
      const amount = before + (after - before) * t;
      el.style.opacity = String(amount); el.hidden = !actor && !target && t >= 1; el.inert = !target; el.setAttribute('aria-hidden', String(!target));
      if (t >= 1 && !(actor ? beat?.focus?.length || beat?.reveal != null : beat?.show?.includes(id) || beat?.hide?.includes(id))) {
        el.style.opacity = base.opacity; el.hidden = base.hidden; el.inert = base.inert;
        if (base.aria === null) el.removeAttribute('aria-hidden'); else el.setAttribute('aria-hidden', base.aria);
      }
      el.dataset.storyVisible = String(target); el.dataset.storyFocus = String(!!beat?.focus?.includes(id));
    }
    const graph = story.graphs[beat?.graphKey], ids = new Map(graph?.actors.map(a => [a.name, a.id]) || []);
    slide.querySelectorAll('svg .edge[data-morph-id]').forEach(el => {
      original(el); const parts = el.dataset.morphId.split(':'), from = ids.get(parts[1]) || parts[1], to = ids.get(parts[2]) || parts[2];
      const traced = value => { const path = value?.trace || []; return path.some((id, i) => i && ((path[i - 1] === from && id === to) || (path[i - 1] === to && id === from))); };
      const strength = value => (!visible(value, from, true, true) || !visible(value, to, true, true)) ? 0 : !value?.trace?.length || traced(value) ? 1 : .15;
      el.style.opacity = String(strength(previous) + (strength(beat) - strength(previous)) * t); el.dataset.storyTrace = String(traced(beat));
    });
    slide.querySelectorAll('pre.code-block').forEach((block, index) => block.querySelectorAll('.ts-line').forEach((line, row) => {
      const base = original(line), selected = beat?.code?.block === index;
      if (selected) { line.style.opacity = beat.code.lines.includes(row + 1) ? '1' : '.25'; line.dataset.storyCode = String(beat.code.lines.includes(row + 1)); }
      else { line.style.opacity = base.opacity; delete line.dataset.storyCode; }
    }));
    const text = beat?.caption || '', changed = caption.textContent !== text || caption.hidden !== !text;
    if (caption.textContent !== text) caption.textContent = text;
    caption.hidden = !text;
    if (changed) scheduleCaptionLayout();
    if (beat) slide.dataset.storyCue = beat.cue; else delete slide.dataset.storyCue;
  }
  function target(id) {
    const slide = active();
    return Array.from(slide.querySelectorAll('[data-sirena-id],[data-story-id],[data-gosx-scene-label],[id]')).find(el => el.dataset.sirenaId === id || el.dataset.storyId === id || el.id === id || el.dataset.gosxSceneLabel === 'label:' + id);
  }
  function assertCurrent() {
    const beat = current(), errors = [];
    if (!beat?.expect) return { checks: 0, errors };
    let checks = 0;
    for (const [ids, expected] of [[beat.expect.visible || [], true], [beat.expect.hidden || [], false]]) {
      for (const id of ids) {
        checks++; const el = target(id), style = el && getComputedStyle(el);
        const actual = !!el && !el.hidden && style.display !== 'none' && style.visibility !== 'hidden' && Number(style.opacity) > 0;
        if (actual !== expected) errors.push(id + ': rendered visibility expected ' + expected);
      }
    }
    for (const [id, text] of Object.entries(beat.expect.labels || {})) { checks++; const el = target(id); if (!el || !el.textContent.includes(text)) errors.push(id + ': rendered label differs'); }
    return { checks, errors };
  }
  window.SlidesStory = { manifest: story, current, seek, duration: () => current()?.durationMs || 0, assertCurrent };
  deck.addEventListener('slides:change', () => seek(window.SlidesMotion?.state().time || 0));
  seek(window.SlidesMotion?.state().time || 0);
})();

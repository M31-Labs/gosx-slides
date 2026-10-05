(function () {
  'use strict';
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const records = new WeakMap(), played = new WeakSet(), segments = new WeakMap();
  const pendingUnits = new WeakMap(), unitRecords = new WeakMap();
  let segment = 0;
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let lastSlide = null, lastStep = -1;
  let paused = false, graphicsFrozen = false, panel = null, selected = null, refreshTimer = null;
  let time = 0, direction = 1, raf = 0, lastTick = null;
  const history = [], future = []; let sourceDraft = null, saving = false;
  const editable = !!document.querySelector('meta[name="slides-edit"]');
  const keys = ['preset','duration','delay','easing','replay'];
  const attribute = key => (key === 'replay' ? 'data-slides-motion-' : 'data-gosx-motion-') + key;
  const savedElements = new WeakMap();
  items(deck).forEach(el => savedElements.set(el, keys.map(key => el.getAttribute(attribute(key)))));
  let returnFocus = null, studioTab = null;
  const dirty = () => items(deck).some(el => keys.some((key,i) => el.getAttribute(attribute(key)) !== savedElements.get(el)?.[i]));
  function buttons() {
    if (!panel) return;
    panel.querySelector('[data-motion-undo]').disabled = !history.length;
    panel.querySelector('[data-motion-redo]').disabled = !future.length;
    panel.querySelector('[data-motion-save]').disabled = saving || !sourceDraft || !dirty() || !!panel.querySelector('[data-motion-elements] [aria-invalid="true"]');
    panel.querySelector('[data-motion-copy]').disabled = !selected;
  }
  function snapshot() { return items().map(el => keys.map(key => el.getAttribute(attribute(key)))); }
  function remember() { history.push(snapshot()); if (history.length > 100) history.shift(); future.length = 0; }
  function restore(state) { items().forEach((el,i) => keys.forEach((key,j) => { const value = state[i]?.[j]; if (value == null) el.removeAttribute(attribute(key)); else el.setAttribute(attribute(key), value); })); fill(); drawTracks(); replay(); status(dirty() ? 'Unsaved element edits.' : 'Element timings match deck.md.'); }
  function undo() { if (!history.length) return; future.push(snapshot()); restore(history.pop()); }
  function redo() { if (!future.length) return; history.push(snapshot()); restore(future.pop()); }
  function status(message) { panel.querySelector('[data-motion-status]').textContent = message; }
  async function loadDraft() { if (!editable) { status('Preview edits. Start with --edit to save them.'); return; } sourceDraft=null; buttons(); status('Loading deck source…'); try { const response = await fetch('/_slides/source?motion=1', {cache:'no-store'}); const data = await response.json(); if (!response.ok) throw new Error(data.error); if (data.revision !== deck.dataset.sourceRevision) throw new Error('Source changed; reload the deck before saving motion edits.'); sourceDraft = data; buttons(); status(dirty() ? 'Edits can be saved to deck.md. Unsaved element edits.' : 'Edits can be saved to deck.md.'); } catch (error) { sourceDraft = null; buttons(); status(error.message); } }
  async function saveDraft() {
    if (!sourceDraft || saving) return; saving = true; const button = panel.querySelector('[data-motion-save]'); button.disabled = true;
    try {
      const motions = items(deck).map(el => {
        const start = Number(el.dataset.slidesMotionSource);
        if (!sourceDraft.motions.some(row => row.start === start)) throw new Error('Could not locate this motion directive; reload the deck.');
        const attrs = {}; keys.forEach(key => { const value = el.getAttribute(attribute(key)); if (value != null && value !== '') attrs[key] = value; });
        return {start, attrs};
      });
      const response = await fetch('/_slides/source', {method:'PUT',headers:{'Content-Type':'application/json','X-Slides-Token':sourceDraft.token},body:JSON.stringify({motions,revision:sourceDraft.revision})}); const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Save failed'); status('Saved deck.md'); location.reload();
    } catch(error) { status(error.message); } finally { saving = false; buttons(); }
  }
  function change(key,value) { if (!selected || selected.getAttribute(attribute(key)) === value) return; if (key === 'duration' || key === 'delay') { const n=Number(value); if (value === '' || !Number.isFinite(n) || n < (key==='duration'?1:0) || n>600000) { status('Enter a valid ' + key + ' in milliseconds.'); return; } } remember(); selected.setAttribute(attribute(key),value); fill(); drawTracks(); replay(); status('Unsaved element edits.'); }
  function motionLabel(el, index) {
    const text = el.textContent.trim().replace(/\s+/g, ' ').slice(0, 55);
    return el.dataset.slidesMotionCue ? el.dataset.slidesMotionCue + ': ' + text : el.id || text || 'Element ' + (index + 1);
  }
  function drawTracks() {
    if (!panel) return;
    const tracklist = panel.querySelector('[data-motion-tracks]'); tracklist.replaceChildren();
    const elements = items(), starts = schedule(elements);
    const span = elements.reduce((end, el) => Math.max(end, starts.get(el) + motionLength(el)), 1000);
    elements.forEach((el, i) => {
      const row = document.createElement('div'); row.className = 'slides-motion-track';
      const select = document.createElement('button'); select.type = 'button';
      const step = el.dataset.slidesMotionStep || '0', label = motionLabel(el, i);
      select.textContent = 'Step ' + step + ': ' + label; select.title = select.textContent;
      const choose = () => { selected = el; panel.querySelector('[data-motion-element]').value = i; fill(); };
      select.onclick = choose; row.appendChild(select);
      const lane = document.createElement('div'); lane.className = 'slides-motion-lane';
      const bar = document.createElement('button'); bar.type = 'button'; bar.className = 'slides-motion-bar';
      const offset = starts.get(el) - number(el, 'delay', 0);
      const paint = () => {
        const start = offset + number(el, 'delay', 0), length = motionLength(el), end = start + length;
        bar.style.left = start / span * 100 + '%'; bar.style.width = Math.max(2, length / span * 100) + '%';
        bar.dataset.motionStart = String(start); bar.dataset.motionEnd = String(end);
        bar.textContent = number(el, 'duration', 220) + ' ms' + (length > number(el, 'duration', 220) ? ' + stagger' : '');
        bar.title = label + ', step ' + step + ', starts ' + start + ' ms, ends ' + end + ' ms';
        bar.setAttribute('aria-label', bar.title + '. Arrow keys adjust delay.');
      };
      paint();
      bar.onkeydown = event => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
        event.preventDefault(); event.stopPropagation(); choose();
        change('delay', String(Math.max(0, Math.min(600000, number(el, 'delay', 0) +
          (event.key === 'ArrowRight' ? 1 : -1) * (event.shiftKey ? 100 : 10)))));
      };
      bar.onpointerdown = event => {
        if (event.button !== 0) return;
        event.preventDefault(); choose();
        const start = event.clientX, resize = event.clientX >= bar.getBoundingClientRect().right - 10;
        const key = resize ? 'duration' : 'delay', initial = number(el, key, resize ? 220 : 0);
        const width = lane.getBoundingClientRect().width;
        remember(); bar.setPointerCapture(event.pointerId);
        bar.onpointermove = move => {
          if (!bar.hasPointerCapture(move.pointerId)) return;
          const value = Math.max(resize ? 1 : 0, Math.min(600000,
            Math.round((initial + (move.clientX - start) / width * span) / 10) * 10));
          el.setAttribute(attribute(key), String(value)); paint(); fill();
        };
        bar.onpointerup = up => { bar.releasePointerCapture(up.pointerId); drawTracks(); replay(); };
        bar.onpointercancel = () => drawTracks();
      };
      lane.appendChild(bar); row.appendChild(lane);
      if (el.dataset.slidesMotionError || el.dataset.slidesMotionNotice) {
        const warning = document.createElement('small'); warning.className = 'slides-motion-warning';
        warning.textContent = el.dataset.slidesMotionError || el.dataset.slidesMotionNotice; row.appendChild(warning);
      }
      tracklist.appendChild(row);
    });
  }
  function active() { return deck.querySelector('.slide.deck-active'); }
  function items(slide) { return Array.from((slide || active()).querySelectorAll('[data-slides-motion-replay]')); }
  function frames(el) {
    const distance = number(el, 'distance', 18);
    const preset = el.dataset.gosxMotionPreset || 'fade';
    const transform = { 'slide-up': `translateY(${distance}px)`, 'slide-down': `translateY(${-distance}px)`,
      'slide-left': `translateX(${distance}px)`, 'slide-right': `translateX(${-distance}px)`, 'zoom-in': 'scale(.94)' }[preset];
    return [{ opacity: 0, ...(transform ? { transform } : {}) }, { opacity: 1, ...(transform ? { transform: 'none' } : {}) }];
  }
  function number(el, key, fallback) {
    const raw = el.getAttribute('data-gosx-motion-' + key);
    if (raw == null || raw === '') return fallback;
    const n = Number(raw);
    if (!Number.isFinite(n) || n < 0 || n > 600000) return fallback;
    if (key === 'duration') return Math.max(1, Math.round(n));
    return key === 'delay' || key === 'stagger' ? Math.round(n) : n;
  }
  function controlled(el) {
    return ['step', 'cue', 'after', 'group'].some(key => el.hasAttribute('data-slides-motion-' + key));
  }
  function nativeSplit(el) {
    return !!el.dataset.gosxMotionSplit &&
      !['cue', 'after', 'group'].some(key => el.hasAttribute('data-slides-motion-' + key));
  }
  function disposeMotion(el) {
    pendingUnits.delete(el); unitRecords.delete(el);
    window.__gosx?.motion?.dispose(el);
    // Finished native units retain fill:both after their record drops them.
    if (nativeSplit(el)) el.querySelectorAll('.gosx-motion-unit').forEach(unit => unit.getAnimations().forEach(a => a.cancel()));
  }
  function motionLength(el) {
    let units = 0;
    if (nativeSplit(el)) {
      units = el.querySelectorAll('.gosx-motion-unit').length;
      if (!units) {
        const text = el.textContent, mode = el.dataset.gosxMotionSplit.trim().toLowerCase();
        units = mode === 'word' ? (text.match(/\S+/g) || []).length :
          mode === 'char' ? text.replace(/\s/g, '').length :
          mode === 'line' ? text.split(/\r?\n/).filter(line => line.trim()).length : 0;
      }
    }
    return number(el, 'duration', 220) + Math.max(0, units - 1) * number(el, 'stagger', 0);
  }
  // Resolve all steps before following dependencies, including forward refs.
  // Indexed cue/group lookups and an iterative walk keep long chains linear.
  function schedule(elements = items()) {
    const names = JSON.parse(active().dataset.slideCues || '[]');
    const cueSteps = new Map(); names.forEach((name, i) => { if (!cueSteps.has(name)) cueSteps.set(name, i); });
    const cues = new Map(), groups = new Map(), cueNames = new Set();
    const base = new Map(), predecessor = new Map(), starts = new Map();
    const step = el => el.dataset.slidesMotionStep || '0';
    elements.forEach(el => {
      el.removeAttribute('data-slides-motion-error');
      if (!el.hasAttribute('data-slides-motion-step') && controlled(el)) {
        el.dataset.slidesMotionStep = String(cueSteps.get(el.dataset.slidesMotionCue) || 0);
      }
    });
    elements.forEach(el => {
      const cue = el.dataset.slidesMotionCue, group = el.dataset.slidesMotionGroup;
      if (cue) {
        cueNames.add(cue);
        const key = step(el) + ':' + cue, entries = cues.get(key) || [];
        if (entries.length < 2) entries.push(el);
        cues.set(key, entries);
      }
      let start = number(el, 'delay', 0);
      if (group) {
        const key = step(el) + ':' + group, index = groups.get(key) || 0;
        start += index * number(el, 'stagger', 0); groups.set(key, index + 1);
      }
      base.set(el, start);
    });
    elements.forEach(el => {
      const after = el.dataset.slidesMotionAfter;
      if (!after) return;
      const entries = cues.get(step(el) + ':' + after), previous = entries?.find(other => other !== el);
      if (previous) predecessor.set(el, previous);
      else el.dataset.slidesMotionError = entries ? 'Circular after dependency; using local timing.' :
        cueNames.has(after) ? 'After cue "' + after + '" belongs to another step.' : 'Unknown after cue "' + after + '".';
    });
    for (const el of elements) {
      if (starts.has(el)) continue;
      const path = [], positions = new Map(); let cursor = el;
      while (cursor && !starts.has(cursor)) {
        if (positions.has(cursor)) {
          for (let i = positions.get(cursor); i < path.length; i++) {
            starts.set(path[i], base.get(path[i]));
            path[i].dataset.slidesMotionError = 'Circular after dependency; using local timing.';
          }
          break;
        }
        positions.set(cursor, path.length); path.push(cursor); cursor = predecessor.get(cursor);
      }
      for (let i = path.length - 1; i >= 0; i--) {
        const node = path[i], previous = predecessor.get(node);
        if (!starts.has(node)) starts.set(node, base.get(node) + (previous ? starts.get(previous) + motionLength(previous) : 0));
      }
    }
    return starts;
  }
  function animations() { return active().getAnimations({ subtree: true }).filter(a => {
    if (!a.effect) return false;
    const el = a.effect.target?.closest('[data-slides-motion-replay]');
    if (el?.dataset.slidesMotionStep && Number(el.dataset.slidesMotionStep) !== SlidesNav.step()) return false;
    return !(el?.dataset.slidesMotionReplay === 'once' && segments.has(a) && segments.get(a) !== segment);
  }); }
  function graphicsPause(value) {
    deck.querySelectorAll(".slide.deck-active .slide-graphic, .deck-graphics-background.deck-background-active").forEach(mount => { const scope = mount.closest("[data-gosx-scene3d-control-scope]"); const toggle = scope && scope.querySelector("[data-gosx-scene3d-animation-toggle]"); if (toggle && !toggle.disabled && (mount.dataset.gosxScene3dAnimationState === "paused") !== value) toggle.click(); });
  }
  deck.querySelectorAll(".slide-graphic, .deck-graphics-background").forEach(mount => {
    if (mount.closest("[data-gosx-scene3d-control-scope]")) return;
    // Scene mounting replaces its children; controls must live in an ancestor scope.
    const scope = document.createElement("div"); scope.style.display = "contents"; scope.setAttribute("data-gosx-scene3d-control-scope", ""); mount.before(scope); scope.appendChild(mount); const toggle = document.createElement("button"); toggle.type = "button"; toggle.hidden = true; toggle.setAttribute("data-gosx-scene3d-animation-toggle", ""); toggle.setAttribute("data-slides-motion-graphics-toggle", ""); toggle.setAttribute("aria-label", "Toggle scene animation"); scope.appendChild(toggle);
  });
  function pauseActive() {
    window.SlidesDiagramMotion?.pause();
    graphicsPause(true);
    animations().forEach(a => a.pause());
  }
  // Deferred engines and step replays inherit Pause or the native clock freeze
  // used during reverse. Disconnect whenever the transport resumes forward.
  const pauseObserver = new MutationObserver(() => {
    claimUnits();
    if (paused) pauseActive();
    else if (graphicsFrozen) graphicsPause(true);
    sample();
    if (!paused && !raf && ((direction > 0 && time < duration()) || (direction < 0 && time > 0))) startClock();
  });
  function setTransport(isPaused, freezeGraphics) {
    paused = isPaused; graphicsFrozen = freezeGraphics;
    pauseObserver.disconnect();
    pauseObserver.observe(deck, {subtree: true, attributes: true,
      attributeFilter: ['data-gosx-scene3d-ready', 'data-gosx-scene3d-animation-state', 'data-gosx-motion-state']});
  }
  function nativeMounts() { return Array.from(deck.querySelectorAll('.slide.deck-active .slide-graphic, .deck-graphics-background.deck-background-active')); }
  function sample() {
    window.SlidesCodeMotion?.seek(time);
    window.SlidesDiagramMotion?.seek(time);
    window.SlidesGraphicsMotion?.seek(time);
    window.SlidesStory?.seek(time);
    animations().forEach(a => {
      a.pause(); a.currentTime = time;
      const el = a.effect.target?.closest('[data-slides-motion-replay]');
      if (el && records.get(el) === a) { const state = time >= Number(a.effect.getComputedTiming().endTime) ? 'finished' : 'running'; if (el.dataset.gosxMotionState !== state) el.dataset.gosxMotionState = state; }
    });
    items().forEach(el => {
      const list = unitRecords.get(el); if (!list?.length) return;
      const state = time >= Number(list[list.length-1].effect.getComputedTiming().endTime) ? 'finished' : 'running';
      if (el.dataset.gosxMotionState !== state) el.dataset.gosxMotionState = state;
    });
    nativeMounts().forEach(mount => {
      const handle = mount.__gosxScene3DHandle;
      if (handle?.setAnimationClock) {
        const seconds = reduce.matches ? 0 : time / 1000, clock = handle.getAnimationClock?.();
        if (!clock || clock.timeSeconds !== seconds || !clock.paused) handle.setAnimationClock({timeSeconds: seconds});
      }
    });
  }
  function stopClock() { cancelAnimationFrame(raf); raf = 0; lastTick = null; }
  function tick(now) {
    raf = 0;
    if (paused || document.hidden) return;
    if (lastTick != null) time = Math.max(0, Math.min(duration(), time + Math.min(250, now - lastTick) * direction));
    lastTick = now; sample(); updatePanel();
    if ((direction > 0 && time < duration()) || (direction < 0 && time > 0)) raf = requestAnimationFrame(tick);
    else lastTick = null;
  }
  function startClock() { stopClock(); if (!paused) { if (reduce.matches) { time = duration(); sample(); updatePanel(); } else raf = requestAnimationFrame(tick); } }
  function pause() { stopClock(); setTransport(true, true); pauseActive(); sample(); updatePanel(); }
  function play() { setTransport(false, false); graphicsPause(false); direction = 1; startClock(); updatePanel(); }
  function seek(ms) { stopClock(); setTransport(true, true); pauseActive(); time = Math.max(0, Math.min(duration(), Number(ms) || 0)); sample(); updatePanel(); }
  function duration() {
    const story = window.SlidesStory?.current();
    const native = nativeMounts().length ? (story ? story.durationMs : Number(active().dataset.motionDuration) || 10000) : 0;
    return Math.max(native, window.SlidesStory?.duration() || 0, window.SlidesCodeMotion?.duration() || 0, window.SlidesGraphicsMotion?.duration() || 0, window.SlidesDiagramMotion?.duration() || 0,
      animations().reduce((n,a) => { const end = Number(a.effect.getComputedTiming().endTime); return Number.isFinite(end) ? Math.max(n,end) : n; },0));
  }
  function reverse() { setTransport(false, true); graphicsPause(true); if (time === 0) time = duration(); direction = -1; sample(); startClock(); updatePanel(); }
  // Capture waits for the latest command batch and two paint boundaries. Seek
  // never uses elapsed wall time, so video frame rate cannot change the pose.
  async function settled() {
    await window.SlidesGraphicsMotion?.settled();
    if (paused) {
      const clock = (reduce.matches ? 0 : time / 1000).toFixed(3), deadline = performance.now() + 3000;
      // Command acceptance can precede a paced native frame. Wait for its
      // published clock and render queue before counting paint boundaries.
      while (nativeMounts().some(mount => mount.__gosxScene3DHandle?.setAnimationClock &&
        (mount.dataset.gosxScene3dAnimationClock !== clock || window.__gosx_scene3d_debug?.inspect(mount.id)?.renderLoop.scheduled))) {
        if (performance.now() > deadline) throw new Error('Graphic frame did not settle');
        await new Promise(resolve => requestAnimationFrame(resolve));
      }
    }
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  }
  document.addEventListener('gosx:ready', () => {
    items().filter(el => pendingUnits.has(el)).forEach(el => window.__gosx?.motion?.observe(el));
    claimUnits();
    sample(); startClock();
  });
  document.addEventListener('visibilitychange', () => { if (document.hidden) stopClock(); else startClock(); });
  reduce.addEventListener('change', () => { if (reduce.matches) { stopClock(); time = duration(); sample(); updatePanel(); } else startClock(); });
  function claimUnits() {
    items().forEach(el => {
      if (!pendingUnits.has(el)) return;
      const units = Array.from(el.querySelectorAll('.gosx-motion-unit'));
      if (!units.length) return;
      const delay = pendingUnits.get(el); pendingUnits.delete(el);
      // GoSX builds the text units. The story transport owns their animations
      // so completed entrances remain seekable instead of committing styles.
      window.__gosx?.motion?.dispose(el); el.removeAttribute('data-gosx-motion');
      units.forEach(unit => unit.getAnimations().forEach(a => a.cancel()));
      const list = units.map((unit, i) => {
        const a = unit.animate(frames(el), {duration: number(el, 'duration', 220),
          delay: delay + i * number(el, 'stagger', 0),
          easing: CSS.supports('animation-timing-function', el.dataset.gosxMotionEasing || '') ? el.dataset.gosxMotionEasing : 'ease-out', fill: 'both'});
        a.pause(); a.currentTime = time; segments.set(a, segment); return a;
      });
      unitRecords.set(el, list); el.dataset.gosxMotionState = 'running';
    });
  }
  function run(el, delay, force) {
    if (!force && played.has(el) && el.dataset.slidesMotionReplay === "once") return;
    const old = records.get(el); if (old) old.cancel();
    played.add(el);
    if (reduce.matches && el.dataset.gosxMotionRespectReduced !== 'false') return;
    const api = window.__gosx && window.__gosx.motion;
    disposeMotion(el);
    const split = el.dataset.gosxMotionSplit;
    if (nativeSplit(el)) {
      // Explicit steps own visibility; native GoSX still owns the text units.
      // Keep its marker so later timing edits refresh rather than dispose them.
      el.setAttribute('data-gosx-motion', '');
      if (el.hasAttribute('data-slides-motion-step')) el.setAttribute('data-gosx-motion-trigger', 'load');
      el.removeAttribute('data-gosx-motion-revealed');
      pendingUnits.set(el, delay);
      if (el.querySelector('.gosx-motion-unit')) claimUnits();
      else if (api) api.observe(el);
      return;
    }
    // Native GoSX splitting stays available for ordinary entrances. Cued groups
    // preserve their child markup and widgets rather than rebuilding text/DOM.
    const animation = el.animate(frames(el), { duration: number(el, 'duration', 220), delay,
      easing: CSS.supports('animation-timing-function',el.dataset.gosxMotionEasing || '') ? el.dataset.gosxMotionEasing : 'ease-out', fill: 'both' });
    records.set(el, animation); segments.set(animation, segment);
    el.dataset.gosxMotionState = 'running';
    animation.finished.then(() => { if (records.get(el) === animation) el.dataset.gosxMotionState = 'finished'; }, () => {});
    animation.pause();
    if (split) el.dataset.slidesMotionNotice = 'Cue groups preserve markup; use staggered cue groups for rich content.';
  }
  function sync(replay) {
    const slide = active(), step = SlidesNav.step();
    const entrances = items(slide);
    const starts = schedule(entrances);
    entrances.forEach(el => {
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
        if (nativeSplit(el)) disposeMotion(el);
        el.removeAttribute('data-gosx-motion');
      } else {
        if (el.dataset.slidesAuthoredInert !== 'true') el.inert = false;
        if (el.dataset.slidesAuthoredAria) el.setAttribute('aria-hidden', el.dataset.slidesAuthoredAria); else el.removeAttribute('aria-hidden');
        if (!was || replay || (el.dataset.slidesMotionReplay === "step" && (lastSlide !== slide || lastStep !== step))) run(el, starts.get(el), replay);
      }
    });
    lastSlide = slide; lastStep = step;
    if (panel?.open) drawTracks();
    updatePanel();
  }
  // Claim cued entrances before the deferred GoSX bootstrap mounts them.
  items(deck).filter(controlled).forEach(el => el.removeAttribute('data-gosx-motion'));
  function replay() {
    window.SlidesDiagramMotion?.replay();
    items().forEach(el => {
      if (el.hasAttribute('data-slides-motion-step')) return;
      run(el, number(el, 'delay', 0), true);
    });
    segment++; sync(true); time = 0; direction = 1; sample(); startClock();
  }
  deck.addEventListener('slides:change', () => {
    const entered = lastSlide !== active(), previousStep = lastStep, changed = entered || lastStep !== SlidesNav.step();
    if (entered) {
      setTransport(false, false); history.length = future.length = 0;
      if (panel && panel.open) panel.close();
    }
    if (changed) segment++;
    graphicsPause(graphicsFrozen); sync(false);
    if (changed) {
      direction = 1; time = 0;
      // Previous steps and direct anchors land on their exact destination.
      if ((entered && SlidesNav.step() > 0) || SlidesNav.step() < previousStep || Math.abs(SlidesNav.step()-previousStep)>1) time = duration();
      startClock();
    }
    if (paused) { pauseActive(); sample(); }
  });
  deck.addEventListener('slides:before-change', event => {
    stopClock();
    const previous = deck.querySelector('.slide[data-slide="' + event.detail.from + '"]');
    if (previous) items(previous).forEach(el => { const a = records.get(el); if (a) a.cancel(); if (controlled(el) && nativeSplit(el)) { disposeMotion(el); el.removeAttribute('data-gosx-motion'); } el.dataset.slidesCueVisible = 'false'; });
  });
  function updatePanel() {
    if (!panel || !panel.open) return;
    panel.querySelector('[data-motion-pause]').textContent = paused ? 'Play' : 'Pause';
    const slider = panel.querySelector('[data-motion-seek]'); slider.max = String(Math.max(1, duration()));
    slider.value = String(time);
    panel.querySelector('[data-motion-time]').textContent = Math.round(Number(slider.value)) + ' / ' + Math.round(duration()) + ' ms';
  }
  function layoutStudio() {
    deck.toggleAttribute('data-studio-open', !!panel?.open);
    deck.dispatchEvent(new Event('slides:studio-layout'));
  }
  function selectTab(name) {
    studioTab = name;
    panel.querySelectorAll('[data-studio-tab]').forEach(button => {
      const active = button.dataset.studioTab === name;
      button.setAttribute('aria-selected', String(active)); button.tabIndex = active ? 0 : -1;
    });
    panel.querySelector('[data-motion-elements]').hidden = name !== 'elements';
    const scene = panel.querySelector('[data-scene-studio]'); if (scene) scene.hidden = name !== 'scene';
  }
  function closeStudio() {
    panel.close(); layoutStudio();
    if (panel.contains(document.activeElement)) document.activeElement.blur();
    if (returnFocus?.isConnected && returnFocus.getClientRects().length) returnFocus.focus({preventScroll:true});
  }
  function open() {
    if (panel?.open) return;
    if (!panel) {
      panel = document.createElement('dialog'); panel.className = 'slides-author-panel slides-motion-studio';
      panel.setAttribute('aria-labelledby', 'slides-motion-title');
      panel.setAttribute('aria-modal', 'false');
      panel.innerHTML = '<header><div><span class="slides-studio-eyebrow">Live preview</span><h2 id="slides-motion-title">Motion studio</h2><output data-studio-context></output></div><button type="button" data-motion-close aria-label="Close motion studio">×</button></header>' +
        '<div class="slides-studio-transport" aria-label="Playback"><div class="slides-author-actions"><button type="button" data-motion-pause>Pause</button><button type="button" data-motion-replay>Replay</button><button type="button" data-motion-reverse>Reverse</button><output data-motion-time></output></div><label class="slides-studio-seek">Story timeline<input data-motion-seek type="range" min="0" max="1" value="0" aria-label="Motion time"></label></div>' +
        '<div class="slides-studio-tabs" role="tablist" aria-label="Motion controls"><button type="button" role="tab" id="slides-studio-scene-tab" data-studio-tab="scene" aria-controls="slides-studio-scene">Scene</button><button type="button" role="tab" id="slides-studio-elements-tab" data-studio-tab="elements" aria-controls="slides-studio-elements">Elements</button></div>' +
        '<div data-studio-body><section id="slides-studio-elements" data-motion-elements role="tabpanel" aria-labelledby="slides-studio-elements-tab"><div class="slides-studio-scroll"><p data-motion-empty hidden>No motion elements on this slide. Add a motion block to give content an entrance.</p><fieldset data-motion-fields>' +
        '<label>Element<select data-motion-element></select></label>' +
        '<div class="slides-author-fields"><label>Preset<select data-motion-preset><option>fade</option><option>slide-up</option><option>slide-down</option><option>slide-left</option><option>slide-right</option><option>zoom-in</option></select></label>' +
        '<label>Duration (ms)<input data-motion-duration type="number" min="1" max="600000"></label><label>Delay (ms)<input data-motion-delay type="number" min="0" max="600000"></label>' +
        '<label>Replay<select data-motion-replay-mode><option value="slide">Every slide visit</option><option value="step">Every step</option><option value="once">Once</option></select></label><label>Easing<select data-motion-easing><option>ease-out</option><option>ease-in-out</option><option>linear</option><option>ease</option></select></label></div>' +
        '<h3>Timing tracks</h3><p class="slides-studio-hint">Drag to move; resize from the right edge. Arrow keys adjust delay; Shift makes larger changes.</p><div data-motion-tracks aria-label="Element timing tracks"></div></fieldset></div><footer><div class="slides-author-actions"><button type="button" data-motion-undo>Undo</button><button type="button" data-motion-redo>Redo</button><button type="button" data-motion-save>Save to deck.md</button><button type="button" data-motion-copy>Copy directive</button></div><output data-motion-status aria-live="polite"></output></footer></section></div>';
      deck.appendChild(panel);
      panel.querySelector('[data-motion-close]').onclick = closeStudio;
      panel.querySelector('[data-motion-pause]').onclick = () => paused ? play() : pause();
      panel.querySelector('[data-motion-replay]').onclick = replay;
      panel.querySelector('[data-motion-reverse]').onclick = reverse;
      panel.querySelector('[data-motion-seek]').oninput = event => seek(event.target.value);
      panel.querySelector('[data-motion-undo]').onclick = undo; panel.querySelector('[data-motion-redo]').onclick = redo; panel.querySelector('[data-motion-save]').onclick = saveDraft; panel.querySelector('[data-motion-save]').hidden = !editable; panel.querySelector('[data-motion-replay-mode]').onchange = event => change('replay', event.target.value);
      panel.addEventListener('keydown', event => {
        if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeStudio(); return; }
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z' && !event.target.closest('input,textarea,[contenteditable]')) {
          event.preventDefault();
          if (studioTab === 'scene') window.SlidesSceneStudio?.[event.shiftKey ? 'redo' : 'undo']();
          else event.shiftKey ? redo() : undo();
        }
      });
      const tabs = Array.from(panel.querySelectorAll('[data-studio-tab]'));
      tabs.forEach(button => {
        button.onclick = () => selectTab(button.dataset.studioTab);
        button.onkeydown = event => {
          if (!['ArrowLeft','ArrowRight','Home','End'].includes(event.key)) return;
          event.preventDefault(); const available = tabs.filter(tab => !tab.hidden);
          const index = available.indexOf(button), next = event.key === 'Home' ? 0 : event.key === 'End' ? available.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + available.length) % available.length;
          available[next].click(); available[next].focus();
        };
      });
      const selector = panel.querySelector('[data-motion-element]');
      selector.onchange = () => { selected = items()[Number(selector.value)]; fill(); };
      for (const key of ['preset', 'duration', 'delay', 'easing']) panel.querySelector('[data-motion-' + key + ']').onchange = event => {
        const valid = event.target.checkValidity() && event.target.value !== '';
        event.target.setAttribute('aria-invalid', String(!valid));
        buttons(); if (!valid) { status('Enter a valid ' + key + '.'); return; }
        change(key, event.target.value);
      };
      panel.querySelector('[data-motion-copy]').onclick = async () => {
        if (!selected) return;
        const quote = value => JSON.stringify(value);
        const fields = ['preset', 'duration', 'delay', 'easing'].filter(k => selected.hasAttribute('data-gosx-motion-' + k)).map(k => k + '=' + quote(selected.getAttribute('data-gosx-motion-' + k)));
        if (selected.hasAttribute('data-gosx-motion-respect-reduced')) fields.push('respect-reduced-motion=' + selected.getAttribute('data-gosx-motion-respect-reduced'));
        for (const key of ['distance', 'split', 'stagger']) if (selected.hasAttribute('data-gosx-motion-' + key)) fields.push(key + '=' + selected.getAttribute('data-gosx-motion-' + key));
        if (selected.dataset.slidesMotionReplay) fields.push('replay=' + selected.dataset.slidesMotionReplay);
        for (const key of ['cue', 'step', 'after', 'group']) if (selected.hasAttribute('data-slides-motion-' + key)) fields.push(key + '=' + selected.getAttribute('data-slides-motion-' + key));
        try { await navigator.clipboard.writeText(':::motion {' + fields.join(' ') + '}\nYour content\n:::'); panel.querySelector('[data-motion-status]').textContent = 'Copied'; }
        catch (_) { panel.querySelector('[data-motion-status]').textContent = 'Clipboard unavailable'; }
      };
      panel.addEventListener('close', () => { if (panel.open) return; clearInterval(refreshTimer); refreshTimer = null; layoutStudio(); });
    }
    const selector = panel.querySelector('[data-motion-element]'); selector.replaceChildren();
    items().forEach((el, i) => { const option = document.createElement('option'); option.value = i; option.textContent = motionLabel(el, i); selector.appendChild(option); });
    if (!items().includes(selected)) selected = items()[0];
    selector.value = String(Math.max(0,items().indexOf(selected)));
    const slide = active();
    panel.querySelector('[data-studio-context]').textContent = 'Slide ' + SlidesNav.current() + ' · ' + (slide.dataset.slideId || slide.querySelector('h1,h2')?.textContent || 'Untitled');
    panel.querySelector('[data-motion-empty]').hidden = !!selected;
    panel.querySelector('[data-motion-fields]').hidden = !selected;
    fill(); drawTracks(); loadDraft(); returnFocus = document.activeElement;
    clearInterval(refreshTimer); panel.show(); window.SlidesSceneStudio?.open(panel);
    const hasScene = !!window.SlidesGraphicsMotion?.current().length;
    panel.querySelector('[data-studio-tab="scene"]').hidden = !hasScene;
    selectTab(hasScene && (!selected || studioTab === 'scene' || !studioTab) ? 'scene' : 'elements');
    // Native entrances can have committed their final styles before authoring
    // begins. Reconstruct their seekable records at the existing playhead.
    items().filter(el=>!el.hasAttribute('data-slides-motion-step') && !records.has(el) && !unitRecords.has(el)).forEach(el=>run(el,number(el,'delay',0),true));
    layoutStudio(); pause(); updatePanel(); panel.querySelector('[data-motion-close]').focus({preventScroll:true});
    refreshTimer = setInterval(updatePanel, 100);
  }
  function fill() {
    panel.querySelector('[data-motion-replay-mode]').disabled = !selected; panel.querySelector('[data-motion-replay-mode]').value = selected?.dataset.slidesMotionReplay || 'slide';
    for (const key of ['preset', 'duration', 'delay', 'easing']) { const input = panel.querySelector('[data-motion-' + key + ']'); input.disabled = !selected; const value = selected ? selected.getAttribute('data-gosx-motion-' + key) || ({easing:'ease-out',preset:'fade',duration:'220',delay:'0'}[key]) : "";
      if (input.tagName === "SELECT" && value && !Array.from(input.options).some(option => option.value === value)) { const option = document.createElement("option"); option.value = option.textContent = value; input.appendChild(option); }
      input.value = value; input.setAttribute('aria-invalid','false'); }
    buttons();
  }
  document.addEventListener('keydown', event => {
    if (!event.defaultPrevented && event.key === 'Escape' && panel?.open && !deck.querySelector('dialog:modal')) { event.preventDefault(); closeStudio(); return; }
    if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest('input, textarea, select, [contenteditable], dialog, [role]')) return;
    if ((event.key === 'm' || event.key === 'M') && !SlidesNav.isOverview()) { event.preventDefault(); open(); }
  });
  function restoreTransport(state) {
    stopClock(); setTransport(state.paused, state.paused || state.direction < 0);
    direction = state.direction; time = Math.max(0,Math.min(duration(),state.time));
    graphicsPause(graphicsFrozen); if (paused) pauseActive(); sample(); startClock(); updatePanel();
  }
  window.SlidesMotion = { pause, play, seek, replay, reverse, open, duration, settled, restore: restoreTransport, state: () => ({paused, time, direction, duration: duration()}) };
  setTransport(false, false); sync(false); startClock();
})();

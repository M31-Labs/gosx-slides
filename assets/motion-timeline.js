(function () {
  'use strict';
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const records = new WeakMap(), played = new WeakSet();
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let lastSlide = null, lastStep = -1;
  let paused = false, graphicsFrozen = false, panel = null, selected = null, refreshTimer = null;
  const history = [], future = []; let sourceDraft = null, saving = false;
  const editable = !!document.querySelector('meta[name="slides-edit"]');
  const keys = ['preset','duration','delay','easing','replay'];
  const attribute = key => (key === 'replay' ? 'data-slides-motion-' : 'data-gosx-motion-') + key;
  function snapshot() { return items().map(el => keys.map(key => el.getAttribute(attribute(key)))); }
  function remember() { history.push(snapshot()); if (history.length > 100) history.shift(); future.length = 0; }
  function restore(state) { items().forEach((el,i) => keys.forEach((key,j) => { const value = state[i]?.[j]; if (value == null) el.removeAttribute(attribute(key)); else el.setAttribute(attribute(key), value); })); fill(); drawTracks(); replay(); }
  function undo() { if (!history.length) return; future.push(snapshot()); restore(history.pop()); }
  function redo() { if (!future.length) return; history.push(snapshot()); restore(future.pop()); }
  function status(message) { panel.querySelector('[data-motion-status]').textContent = message; }
  async function loadDraft() { if (!editable) return; sourceDraft=null; panel.querySelector('[data-motion-save]').disabled=true; status('Loading deck source…'); try { const response = await fetch('/_slides/source?motion=1', {cache:'no-store'}); const data = await response.json(); if (!response.ok) throw new Error(data.error); if (data.revision !== deck.dataset.sourceRevision) throw new Error('Source changed; reload the deck before saving motion edits.'); sourceDraft = data; panel.querySelector('[data-motion-save]').disabled=false; status('Edits can be saved to deck.md.'); } catch (error) { sourceDraft = null; status(error.message); } }
  async function saveDraft() {
    if (!sourceDraft || saving) return; saving = true; const button = panel.querySelector('[data-motion-save]'); button.disabled = true;
    try {
      const bytes = new TextEncoder().encode(sourceDraft.source), patches = [];
      items(deck).forEach(el => { const range = sourceDraft.motions.find(row => row.start === Number(el.dataset.slidesMotionSource)); if (!range) throw new Error('Could not locate this motion directive; reload the deck.'); const attrs = {...range.attrs}; keys.forEach(key => { const value = el.getAttribute(attribute(key)); if (value != null && value !== '') attrs[key] = value; }); const changes = {}; keys.forEach(key => { const value=el.getAttribute(attribute(key));if(value!=null&&value!=='')changes[key]=value; });const seen=new Set();let opening=range.opening.replace(/([A-Za-z][A-Za-z0-9_-]*)\s*=\s*("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s}]+)/g,(match,key)=>{if(!(key in changes))return match;seen.add(key);return key+'='+JSON.stringify(changes[key]);});const missing=Object.entries(changes).filter(([key])=>!seen.has(key)).map(([key,value])=>key+'='+JSON.stringify(value)).join(' ');if(missing){const end=opening.lastIndexOf('}');opening=end<0?opening.trimEnd()+' {'+missing+'}':opening.slice(0,end).trimEnd()+' '+missing+opening.slice(end);} patches.push({...range, opening}); });
      let source = '', offset = 0; patches.sort((a,b) => a.start-b.start).forEach(p => { source += new TextDecoder().decode(bytes.slice(offset,p.start)) + p.opening; offset=p.end; }); source += new TextDecoder().decode(bytes.slice(offset));
      const response = await fetch('/_slides/source', {method:'PUT',headers:{'Content-Type':'application/json','X-Slides-Token':sourceDraft.token},body:JSON.stringify({source,revision:sourceDraft.revision})}); const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Save failed'); status('Saved deck.md'); location.reload();
    } catch(error) { status(error.message); } finally { saving = false; button.disabled = false; }
  }
  function change(key,value) { if (!selected || selected.getAttribute(attribute(key)) === value) return; if (key === 'duration' || key === 'delay') { const n=Number(value); if (!Number.isFinite(n) || n < (key==='duration'?1:0) || n>600000) return; } remember(); selected.setAttribute(attribute(key),value); fill(); drawTracks(); replay(); }
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
  function animations() { return active().getAnimations({ subtree: true }).filter(a => a.effect); }
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
    if (paused) pauseActive();
    else if (graphicsFrozen) graphicsPause(true);
  });
  function setTransport(isPaused, freezeGraphics) {
    paused = isPaused; graphicsFrozen = freezeGraphics;
    pauseObserver.disconnect();
    if (isPaused || freezeGraphics) pauseObserver.observe(deck, {subtree: true, attributes: true,
      attributeFilter: ['data-gosx-scene3d-ready', 'data-gosx-scene3d-animation-state', 'data-gosx-motion-state']});
  }
  function pause() { setTransport(true, true); pauseActive(); updatePanel(); }
  function play() { setTransport(false, false); window.SlidesDiagramMotion?.play(); graphicsPause(false); animations().forEach(a => a.play()); updatePanel(); }
  function seek(ms) { setTransport(true, true); pauseActive(); window.SlidesDiagramMotion?.seek(Number(ms)||0); animations().forEach(a => { a.currentTime = Math.max(0, Math.min(duration(), Number(ms) || 0)); }); updatePanel(); }
  function duration() { return Math.max(window.SlidesDiagramMotion?.duration() || 0, animations().reduce((n, a) => { const end = Number(a.effect.getComputedTiming().endTime); return Number.isFinite(end) ? Math.max(n, end) : n; }, 0)); }
  function reverse() { setTransport(false, true); graphicsPause(true); window.SlidesDiagramMotion?.reverse(); animations().filter(a => Number.isFinite(Number(a.effect.getComputedTiming().endTime))).forEach(a => { if (a.currentTime === 0) a.currentTime = a.effect.getComputedTiming().endTime; a.reverse(); }); updatePanel(); }
  function run(el, delay, force) {
    const old = records.get(el); if (old) old.cancel();
    if (!force && played.has(el) && el.dataset.slidesMotionReplay === "once") return;
    played.add(el);
    if (reduce.matches && el.dataset.gosxMotionRespectReduced !== 'false') return;
    const api = window.__gosx && window.__gosx.motion;
    disposeMotion(el);
    const split = el.dataset.gosxMotionSplit;
    if (nativeSplit(el)) {
      // Explicit steps own visibility; native GoSX still owns the text units.
      // Keep its marker so later timing edits refresh rather than dispose them.
      el.setAttribute('data-gosx-motion', '');
      el.removeAttribute('data-gosx-motion-revealed');
      if (api) api.observe(el); return;
    }
    // Native GoSX splitting stays available for ordinary entrances. Cued groups
    // preserve their child markup and widgets rather than rebuilding text/DOM.
    const animation = el.animate(frames(el), { duration: number(el, 'duration', 220), delay,
      easing: CSS.supports('animation-timing-function',el.dataset.gosxMotionEasing || '') ? el.dataset.gosxMotionEasing : 'ease-out', fill: 'both' });
    records.set(el, animation);
    el.dataset.gosxMotionState = 'running';
    animation.finished.then(() => { if (records.get(el) === animation) el.dataset.gosxMotionState = 'finished'; }, () => {});
    if (paused) animation.pause();
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
    sync(true);
  }
  deck.addEventListener('slides:change', () => {
    const entered = lastSlide !== active();
    if (entered) {
      setTransport(false, false); history.length = future.length = 0;
      if (panel && panel.open) panel.close();
    }
    graphicsPause(graphicsFrozen); sync(false);
    if (paused) pauseActive();
  });
  deck.addEventListener('slides:before-change', event => {
    const previous = deck.querySelector('.slide[data-slide="' + event.detail.from + '"]');
    if (previous) items(previous).forEach(el => { const a = records.get(el); if (a) a.cancel(); if (controlled(el) && nativeSplit(el)) { disposeMotion(el); el.removeAttribute('data-gosx-motion'); } el.dataset.slidesCueVisible = 'false'; });
  });
  function updatePanel() {
    if (!panel || !panel.open) return;
    panel.querySelector('[data-motion-pause]').textContent = paused ? 'Play' : 'Pause';
    const slider = panel.querySelector('[data-motion-seek]'); slider.max = String(Math.max(1, duration()));
    slider.value = String(Math.max(window.SlidesDiagramMotion?.state().time || 0, 0, ...animations().map(a => Number(a.currentTime) || 0)));
    panel.querySelector('[data-motion-time]').textContent = Math.round(Number(slider.value)) + ' / ' + Math.round(duration()) + ' ms';
  }
  function open() {
    if (!panel) {
      panel = document.createElement('dialog'); panel.className = 'slides-author-panel';
      panel.setAttribute('aria-labelledby', 'slides-motion-title');
      panel.innerHTML = '<header><h2 id="slides-motion-title">Motion studio</h2><button type="button" data-motion-close aria-label="Close motion studio">×</button></header>' +
        '<p>Edit element timings, replay and easing. Drag a bar to adjust delay; drag its right edge to resize duration. Tracks include dependencies and staggering, with start times relative to each click step.</p>' +
        '<label>Element<select data-motion-element></select></label>' +
        '<div class="slides-author-fields"><label>Preset<select data-motion-preset><option>fade</option><option>slide-up</option><option>slide-down</option><option>slide-left</option><option>slide-right</option><option>zoom-in</option></select></label>' +
        '<label>Duration (ms)<input data-motion-duration type="number" min="1" max="600000"></label><label>Delay (ms)<input data-motion-delay type="number" min="0" max="600000"></label>' +
        '<label>Replay<select data-motion-replay-mode><option value="slide">Every slide visit</option><option value="step">Every step</option><option value="once">Once</option></select></label><label>Easing<select data-motion-easing><option>ease-out</option><option>ease-in-out</option><option>linear</option><option>ease</option></select></label></div>' +
        '<div data-motion-tracks aria-label="Element timing tracks"></div><div class="slides-author-actions"><button type="button" data-motion-undo>Undo</button><button type="button" data-motion-redo>Redo</button><button type="button" data-motion-save>Save to deck.md</button><button type="button" data-motion-pause>Pause</button><button type="button" data-motion-replay>Replay</button><button type="button" data-motion-reverse>Reverse</button><button type="button" data-motion-copy>Copy directive</button></div>' +
        '<label>Element timeline<input data-motion-seek type="range" min="0" max="1" value="0" aria-label="Motion time"></label><output data-motion-time></output><output data-motion-status aria-live="polite"></output>';
      deck.appendChild(panel);
      panel.querySelector('[data-motion-close]').onclick = () => panel.close();
      panel.querySelector('[data-motion-pause]').onclick = () => paused ? play() : pause();
      panel.querySelector('[data-motion-replay]').onclick = replay;
      panel.querySelector('[data-motion-reverse]').onclick = reverse;
      panel.querySelector('[data-motion-seek]').oninput = event => seek(event.target.value);
      panel.querySelector('[data-motion-undo]').onclick = undo; panel.querySelector('[data-motion-redo]').onclick = redo; panel.querySelector('[data-motion-save]').onclick = saveDraft; panel.querySelector('[data-motion-save]').hidden = !editable; panel.querySelector('[data-motion-replay-mode]').onchange = event => change('replay', event.target.value);
      panel.addEventListener('keydown', event => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z' && !event.target.matches('input')) { event.preventDefault(); event.shiftKey ? redo() : undo(); } });
      const selector = panel.querySelector('[data-motion-element]');
      selector.onchange = () => { selected = items()[Number(selector.value)]; fill(); };
      for (const key of ['preset', 'duration', 'delay', 'easing']) panel.querySelector('[data-motion-' + key + ']').onchange = event => {
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
      panel.addEventListener('close', () => { clearInterval(refreshTimer); refreshTimer = null; });
    }
    const selector = panel.querySelector('[data-motion-element]'); selector.replaceChildren();
    items().forEach((el, i) => { const option = document.createElement('option'); option.value = i; option.textContent = motionLabel(el, i); selector.appendChild(option); });
    selected = items()[0]; fill(); drawTracks(); loadDraft(); if (panel.open) return; panel.showModal(); replay(); updatePanel(); refreshTimer = setInterval(updatePanel, 100);
  }
  function fill() {
    panel.querySelector('[data-motion-replay-mode]').disabled = !selected; panel.querySelector('[data-motion-replay-mode]').value = selected?.dataset.slidesMotionReplay || 'slide';
    for (const key of ['preset', 'duration', 'delay', 'easing']) { const input = panel.querySelector('[data-motion-' + key + ']'); input.disabled = !selected; const value = selected ? selected.getAttribute('data-gosx-motion-' + key) || (key === "easing" ? "ease-out" : "") : "";
      if (input.tagName === "SELECT" && value && !Array.from(input.options).some(option => option.value === value)) { const option = document.createElement("option"); option.value = option.textContent = value; input.appendChild(option); }
      input.value = value; }
  }
  document.addEventListener('keydown', event => {
    if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest('input, textarea, select, [contenteditable], dialog, [role]')) return;
    if ((event.key === 'm' || event.key === 'M') && !SlidesNav.isOverview()) { event.preventDefault(); open(); }
  });
  window.SlidesMotion = { pause, play, seek, replay, reverse, open, duration, state: () => ({ paused, time: Math.max(window.SlidesDiagramMotion?.state().time || 0, 0, ...animations().map(a => Number(a.currentTime) || 0)), duration: duration() }) };
  sync(false);
})();

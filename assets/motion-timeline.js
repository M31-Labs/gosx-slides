(function () {
  'use strict';
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const records = new WeakMap(), played = new WeakSet();
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let lastSlide = null, lastStep = -1;
  let paused = false, panel = null, selected = null, refreshTimer = null;
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
  function drawTracks() {
    if (!panel) return; const tracklist=panel.querySelector('[data-motion-tracks]'); tracklist.replaceChildren(); const elements=items(); const span=Math.max(1000,...elements.map(el=>number(el,'delay',0)+number(el,'duration',220)));
    elements.forEach((el,i)=>{const row=document.createElement('div');row.className='slides-motion-track';const select=document.createElement('button');select.type='button';select.textContent=el.dataset.slidesMotionCue || el.textContent.trim().slice(0,30) || 'Element '+(i+1);select.onclick=()=>{selected=el;panel.querySelector('[data-motion-element]').value=i;fill();};row.appendChild(select);const lane=document.createElement('div');lane.className='slides-motion-lane';const bar=document.createElement('button');bar.type='button';bar.className='slides-motion-bar';bar.style.left=(number(el,'delay',0)/span*100)+'%';bar.style.width=Math.max(2,number(el,'duration',220)/span*100)+'%';bar.textContent=number(el,'duration',220)+' ms';bar.setAttribute('aria-label',select.textContent+' delay '+number(el,'delay',0)+' milliseconds; arrow keys adjust delay');bar.onkeydown=event=>{if(event.key!=='ArrowLeft'&&event.key!=='ArrowRight')return;event.preventDefault();event.stopPropagation();selected=el;change('delay',String(Math.max(0,Math.min(600000,number(el,'delay',0)+(event.key==='ArrowRight'?1:-1)*(event.shiftKey?100:10)))));};
      bar.onpointerdown=event=>{if(event.button!==0)return;event.preventDefault();selected=el;const start=event.clientX,resize=event.clientX>=bar.getBoundingClientRect().right-10,key=resize?'duration':'delay',initial=number(el,key,resize?220:0),width=lane.getBoundingClientRect().width;remember();bar.setPointerCapture(event.pointerId);bar.onpointermove=move=>{if(!bar.hasPointerCapture(move.pointerId))return;const value=Math.max(resize?1:0,Math.min(600000,Math.round((initial+(move.clientX-start)/width*span)/10)*10));el.setAttribute(attribute(key),String(value));if(resize){bar.style.width=Math.max(2,value/span*100)+'%';bar.textContent=value+' ms';}else{bar.style.left=value/span*100+'%';}fill();};bar.onpointerup=up=>{bar.releasePointerCapture(up.pointerId);drawTracks();replay();};bar.onpointercancel=()=>{drawTracks();};};lane.appendChild(bar);row.appendChild(lane);tracklist.appendChild(row);});
  }
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
    const raw = el.getAttribute('data-gosx-motion-' + key);
    if (raw == null || raw === '') return fallback;
    const n = Number(raw);
    return Number.isFinite(n) && n >= 0 && n <= 600000 ? n : fallback;
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
  // Deferred engines and step replays can mount after Pause. Apply the same
  // intent when they publish readiness; do no observation work while playing.
  const pauseObserver = new MutationObserver(() => { if (paused) pauseActive(); });
  function setPaused(value) {
    paused = value;
    pauseObserver.disconnect();
    if (value) pauseObserver.observe(deck, {subtree: true, attributes: true,
      attributeFilter: ['data-gosx-scene3d-ready', 'data-gosx-scene3d-animation-state', 'data-gosx-motion-state']});
  }
  function pause() { setPaused(true); pauseActive(); updatePanel(); }
  function play() { setPaused(false); window.SlidesDiagramMotion?.play(); graphicsPause(false); animations().forEach(a => a.play()); updatePanel(); }
  function seek(ms) { setPaused(true); pauseActive(); window.SlidesDiagramMotion?.seek(Number(ms)||0); animations().forEach(a => { a.currentTime = Math.max(0, Math.min(duration(), Number(ms) || 0)); }); updatePanel(); }
  function duration() { return Math.max(window.SlidesDiagramMotion?.duration() || 0, animations().reduce((n, a) => { const end = Number(a.effect.getComputedTiming().endTime); return Number.isFinite(end) ? Math.max(n, end) : n; }, 0)); }
  function reverse() { setPaused(false); graphicsPause(true); window.SlidesDiagramMotion?.reverse(); animations().filter(a => Number.isFinite(Number(a.effect.getComputedTiming().endTime))).forEach(a => { if (a.currentTime === 0) a.currentTime = a.effect.getComputedTiming().endTime; a.reverse(); }); updatePanel(); }
  function run(el, delay, force) {
    const old = records.get(el); if (old) old.cancel();
    if (!force && played.has(el) && el.dataset.slidesMotionReplay === "once") return;
    played.add(el);
    if (reduce.matches && el.dataset.gosxMotionRespectReduced !== 'false') return;
    const api = window.__gosx && window.__gosx.motion;
    if (api) api.dispose(el);
    const split = el.dataset.gosxMotionSplit;
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
        if (!was || replay || (el.dataset.slidesMotionReplay === "step" && (lastSlide !== slide || lastStep !== step))) run(el, delay(el), replay);
      }
    });
    lastSlide = slide; lastStep = step;
    updatePanel();
  }
  // Claim cued entrances before the deferred GoSX bootstrap mounts them.
  deck.querySelectorAll('[data-slides-motion-cue], [data-slides-motion-step]').forEach(el => el.removeAttribute('data-gosx-motion'));
  function replay() {
    window.SlidesDiagramMotion?.replay();
    items().forEach(el => {
      if (el.hasAttribute('data-slides-motion-step')) return;
      const api = window.__gosx && window.__gosx.motion;
      if (api) api.dispose(el); run(el, number(el, 'delay', 0), true);
    });
    sync(true);
  }
  deck.addEventListener('slides:change', () => {
    const entered = lastSlide !== active();
    if (entered) {
      setPaused(false); history.length = future.length = 0;
      if (panel && panel.open) panel.close();
    }
    graphicsPause(paused); sync(false);
    if (paused) pauseActive();
  });
  deck.addEventListener('slides:before-change', event => {
    const previous = deck.querySelector('.slide[data-slide="' + event.detail.from + '"]');
    if (previous) items(previous).forEach(el => { const a = records.get(el); if (a) a.cancel(); el.dataset.slidesCueVisible = 'false'; });
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
        '<p>Edit element timings, replay and easing. Drag a bar to adjust delay; drag its right edge to resize duration.</p>' +
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
    items().forEach((el, i) => { const option = document.createElement('option'); option.value = i; option.textContent = el.dataset.slidesMotionCue || el.id || el.textContent.trim().slice(0, 55) || 'Element ' + (i + 1); selector.appendChild(option); });
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

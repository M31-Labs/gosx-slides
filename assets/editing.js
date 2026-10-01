// Source editing is explicitly enabled by the server. Ink stays local to this tab.
(function () {
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const controls = deck.querySelector('.deck-controls');
  let editor, revision, token, saving = false;
  function button(label, action) { const b = document.createElement('button'); b.type = 'button'; b.classList.add('slides-extra-control'); b.textContent = label; b.onclick = action; return b; }
  async function openEditor() {
    if (!editor) {
      editor = document.createElement('dialog'); editor.className = 'slides-source-panel';
      editor.innerHTML = '<header><h2>Edit deck source</h2><button type="button" data-close aria-label="Close source editor">×</button></header><label for="slides-source">deck.md</label><textarea id="slides-source" spellcheck="false"></textarea><footer><button type="button" data-reload>Reload source</button><button type="button" data-save>Save and preview</button><span role="status" data-status></span></footer>';
      deck.appendChild(editor);
      editor.querySelector('[data-close]').onclick = () => editor.close();
      editor.querySelector('[data-reload]').onclick = loadSource;
      editor.querySelector('[data-save]').onclick = async () => {
        if (saving) return; saving = true; const save = editor.querySelector('[data-save]'); save.disabled = true;
        try {
          const response = await fetch('/_slides/source', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Slides-Token': token }, body: JSON.stringify({ source: editor.querySelector('textarea').value, revision }) });
          const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Save failed');
          status('Saved deck.md'); location.reload();
        } catch (error) { status(error.message); } finally { saving = false; save.disabled = false; }
      };
    }
    if (editor.open) return;
    editor.showModal(); await loadSource();
  }
  function status(message) { editor.querySelector('[data-status]').textContent = message; }
  async function loadSource() {
    try { const response = await fetch('/_slides/source', { cache: 'no-store' }); const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not load source'); editor.querySelector('textarea').value = data.source; revision = data.revision; token = data.token; status('Ready to edit'); }
    catch (error) { status(error.message); }
  }
  const canEdit = !!document.querySelector('meta[name="slides-edit"]');
  if (canEdit && controls) controls.appendChild(button('Edit', openEditor));
  const ns = 'http://www.w3.org/2000/svg';
  const overlay = document.createElementNS(ns, 'svg'); overlay.classList.add('slides-ink'); overlay.setAttribute('viewBox', '0 0 1000 1000'); overlay.setAttribute('preserveAspectRatio', 'none'); overlay.setAttribute('aria-hidden', 'true'); deck.appendChild(overlay);
  const toolbar = document.createElement('div'); toolbar.className = 'slides-ink-tools'; toolbar.setAttribute('role', 'toolbar'); toolbar.setAttribute('aria-label', 'Slide annotations'); toolbar.hidden = true;
  let mode = '', color = '#ff4b73', drawing, strokes = new Map(), laserTimer, frame, pointCount = 0;
  const key = () => String(SlidesNav.current());
  function rows() { if (!strokes.has(key())) strokes.set(key(), []); return strokes.get(key()); }
  const laser = document.createElementNS(ns, 'circle'); laser.setAttribute('r', '7'); laser.classList.add('slides-laser');
  function position() { const slide = deck.querySelector('.deck-active'); if (!slide) return; const r = slide.getBoundingClientRect(); Object.assign(overlay.style, { left: r.left+'px', top: r.top+'px', width: r.width+'px', height: r.height+'px' }); }
  function render() { overlay.replaceChildren(...rows().map(row => { const path = document.createElementNS(ns, 'polyline'); path.setAttribute('points', row.points.map(p => p.join(',')).join(' ')); path.setAttribute('stroke', row.color); path.setAttribute('stroke-width', '3'); path.setAttribute('vector-effect', 'non-scaling-stroke'); path.setAttribute('fill', 'none'); path.setAttribute('stroke-linecap', 'round'); path.setAttribute('stroke-linejoin', 'round'); row.path = path; return path; })); if (mode === 'laser') overlay.appendChild(laser); }
  function setMode(next) { if (next && SlidesNav.isOverview()) SlidesNav.closeOverview(); drawing = null; mode = next; overlay.style.pointerEvents = mode ? 'auto' : 'none'; toolbar.hidden = !mode; if (!mode && toolbar.contains(document.activeElement)) document.activeElement.blur(); deck.dataset.inkMode = mode; position(); render(); }
  toolbar.append(button('Pen', () => setMode('pen')), button('Laser', () => setMode('laser')), button('Undo', () => { const removed = rows().pop(); if (removed) pointCount -= removed.points.length; render(); }), button('Clear', () => { pointCount -= rows().reduce((n, row) => n + row.points.length, 0); rows().length = 0; render(); }));
  const picker = document.createElement('input'); picker.type = 'color'; picker.value = color; picker.setAttribute('aria-label', 'Ink color'); picker.oninput = () => { color = picker.value; }; toolbar.append(picker, button('Done', () => setMode(''))); deck.appendChild(toolbar);
  if (controls) controls.appendChild(button('Draw', () => setMode(mode ? '' : 'pen')));
  function point(event) { const r = overlay.getBoundingClientRect(); return [Math.round(Math.max(0, Math.min(1000, (event.clientX-r.left)/r.width*1000))), Math.round(Math.max(0, Math.min(1000, (event.clientY-r.top)/r.height*1000)))]; }
  overlay.addEventListener('pointerdown', event => { if (event.button !== 0) return; event.preventDefault(); event.stopPropagation(); overlay.setPointerCapture(event.pointerId); if (mode === 'pen') { if (rows().length >= 100 || pointCount >= 30000) return; drawing = { points: [point(event)], color }; rows().push(drawing); pointCount++; render(); } else move(event); });
  function move(event) { if (mode === 'laser') { const p = point(event); laser.setAttribute('cx', p[0]); laser.setAttribute('cy', p[1]); laser.style.opacity = '1'; clearTimeout(laserTimer); laserTimer = setTimeout(() => { laser.style.opacity = '0'; }, 700); } else if (drawing) { if (drawing.points.length < 2000 && pointCount < 30000) { drawing.points.push(point(event)); pointCount++; } if (!frame) { const row = drawing; frame = requestAnimationFrame(() => { frame = null; if (row.path) row.path.setAttribute('points', row.points.map(p => p.join(',')).join(' ')); }); } } }
  overlay.addEventListener('pointermove', move);
  for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) overlay.addEventListener(type, () => { drawing = null; });
  // Prevent swipe navigation while annotating, including pen/touch input.
  overlay.addEventListener('touchstart', event => event.stopPropagation(), { passive: true });
  deck.addEventListener('slides:change', () => { drawing = null; position(); render(); });
  window.addEventListener('resize', position);
  document.addEventListener('keydown', event => { let target = event.target; if (target.closest('dialog:not([open]), .slides-ink-tools[hidden]')) { target.blur(); target = deck; } if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || target.closest('input,textarea,select,[contenteditable],dialog,[role]')) return; const k = event.key.toLowerCase(); if (k === 'e' && canEdit) { event.preventDefault(); openEditor(); } else if (k === 'd' || k === 'l') { event.preventDefault(); const next = k === 'd' ? 'pen' : 'laser'; setMode(mode === next ? '' : next); } else if (k === 'escape' && mode) { event.preventDefault(); setMode(''); } });
  window.SlidesInk = { mode: setMode, clear: () => { pointCount -= rows().reduce((n,row)=>n+row.points.length,0); rows().length=0; render(); }, count: () => rows().length };
  position();
})();

// Source editing is explicitly enabled by the server. Ink stays local to this tab.
(function () {
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const controls = deck.querySelector('.deck-controls');
  let editor, revision, token, saving = false, analysis, timer, controller, selectedSymbol;
  let history = [], historyIndex = -1;
  function button(label, action) { const b = document.createElement('button'); b.type = 'button'; b.classList.add('slides-extra-control'); b.textContent = label; b.onclick = action; return b; }
  function sourceInput() { return editor.querySelector('textarea'); }
  function remember() {
    const input = sourceInput(), value = input.value;
    if (history[historyIndex]?.value === value) return;
    history.splice(historyIndex + 1);
    history.push({ value, start: input.selectionStart, end: input.selectionEnd });
    let bytes = history.reduce((sum, row) => sum + row.value.length * 2, 0);
    while (history.length > 1 && (history.length > 50 || bytes > 16 * 1024 * 1024)) bytes -= history.shift().value.length * 2;
    historyIndex = history.length - 1; historyButtons();
  }
  function historyButtons() { editor.querySelector('[data-undo]').disabled = historyIndex < 1; editor.querySelector('[data-redo]').disabled = historyIndex >= history.length - 1; }
  function restoreHistory(delta) {
    const next = historyIndex + delta; if (next < 0 || next >= history.length) return;
    historyIndex = next; const row = history[next], input = sourceInput(); input.value = row.value;
    input.focus(); input.setSelectionRange(row.start, row.end); historyButtons(); scheduleAnalysis(); status('Draft restored');
  }
  function byteOffset(character) { return new TextEncoder().encode(sourceInput().value.slice(0, character)).length; }
  function characterOffset(byte) {
    // TextDecoder keeps Unicode/emoji before a ranged symbol from shifting its selection.
    return new TextDecoder().decode(new TextEncoder().encode(sourceInput().value).slice(0, byte)).length;
  }
  function selectRange(range) {
    if (!analysis?.editable) { status('Convert to LF line endings before selecting structured source'); return; }
    const input = sourceInput(); input.focus(); input.setSelectionRange(characterOffset(range.StartByte), characterOffset(range.EndByte));
    const line = input.value.slice(0, input.selectionStart).split('\n').length;
    input.scrollTop = Math.max(0, (line - 4) * parseFloat(getComputedStyle(input).lineHeight)); updateSelection();
  }
  function listRow(label, action, description) {
    const b = button(label, action); b.className = 'slides-source-row'; if (description) b.title = description; return b;
  }
  function updateSelection() {
    if (!analysis) return;
    const offset = byteOffset(sourceInput().selectionStart);
    selectedSymbol = analysis.symbols.find(s => offset >= s.Range.StartByte && offset < s.Range.EndByte);
    const refs = selectedSymbol ? analysis.symbols.filter(s => s.Kind === selectedSymbol.Kind && s.Scope === selectedSymbol.Scope && s.Name === selectedSymbol.Name) : [];
    editor.querySelector('[data-symbol-label]').textContent = selectedSymbol ? selectedSymbol.Kind + ' ' + selectedSymbol.Name + ' · ' + refs.length + ' occurrences' : 'Select a slide ID, cue or Sirena actor';
    editor.querySelector('[data-rename]').disabled = !selectedSymbol || !analysis.renameable;
    const list = editor.querySelector('[data-references]'); list.replaceChildren(...refs.map(s => listRow((s.Declaration ? 'Definition' : 'Reference') + ' · line ' + s.Range.StartLine, () => selectRange(s.Range))));
  }
  function renderAnalysis(data) {
    analysis = data;
    const outline = editor.querySelector('[data-outline]'); outline.replaceChildren(...data.outline.map(row => {
      const item = document.createElement('div'); item.className = 'slides-source-slide';
      item.append(listRow((row.index + 1) + ' · ' + row.title, () => selectRange(row.range)));
      const preview = button('Preview', () => {
        if (!analysis) { status('Wait for source analysis'); return; }
        if (row.index >= deck.querySelectorAll('section.slide').length) { status('Save this new slide before previewing'); return; }
        editor.close(); window.SlidesNav.preview(row.index, 0);
      }); preview.setAttribute('aria-label', 'Preview slide ' + (row.index + 1)); item.append(preview); return item;
    }));
    const diagnostics = editor.querySelector('[data-diagnostics]'); diagnostics.replaceChildren(...data.diagnostics.map(d => {
      const b = listRow(d.severity + ' · line ' + d.range.StartLine + ' · ' + d.message, () => selectRange(d.range), d.code); b.dataset.severity = d.severity; return b;
    }));
    if (!data.diagnostics.length) { const p = document.createElement('p'); p.textContent = 'No parser or story findings'; diagnostics.append(p); }
    editor.querySelector('[data-diagnostic-count]').textContent = 'Findings (' + data.diagnostics.length + ')';
    editor.querySelector('[data-preview-caret]').disabled = false;
    renderSymbols(); updateSelection();
  }
  function renderSymbols() {
    if (!analysis) return;
    const query = editor.querySelector('[data-symbol-search]').value.trim().toLowerCase();
    editor.querySelector('[data-symbols]').replaceChildren(...analysis.symbols.filter(s => s.Declaration && (s.Kind + ' ' + s.Name).toLowerCase().includes(query)).slice(0, 200).map(s => listRow(s.Kind + ' · ' + s.Name, () => selectRange(s.Range), s.Scope)));
  }
  async function analyzeDraft(rename) {
    const source = sourceInput().value;
    if (controller) controller.abort(); controller = new AbortController();
    try {
      const response = await fetch('/_slides/analyze', { method: 'POST', signal: controller.signal, headers: { 'Content-Type': 'application/json', 'X-Slides-Token': token }, body: JSON.stringify({ source, ...(rename ? { rename } : {}) }) });
      const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Could not analyze source');
      if (sourceInput().value !== source) return;
      if (rename) { sourceInput().value = result.source; remember(); status('Renamed in draft; save to publish'); }
      renderAnalysis(result);
    } catch (error) { if (error.name !== 'AbortError') status(error.message); }
  }
  function scheduleAnalysis() {
    analysis = null; selectedSymbol = null;
    editor.querySelector('[data-rename]').disabled = true;
    editor.querySelector('[data-preview-caret]').disabled = true;
    for (const row of editor.querySelectorAll('.slides-source-structure button')) row.disabled = true;
    clearTimeout(timer); timer = setTimeout(() => analyzeDraft(), 300);
  }
  async function openEditor() {
    if (!editor) {
      editor = document.createElement('dialog'); editor.className = 'slides-source-panel';
      editor.innerHTML = '<header><h2>Edit deck source</h2><button type="button" data-close aria-label="Close source editor">×</button></header><div class="slides-source-workspace"><div class="slides-source-code"><label for="slides-source">deck.md</label><textarea id="slides-source" spellcheck="false" aria-describedby="slides-source-help"></textarea><p id="slides-source-help">Changes stay in this draft until saved. Preview uses the last saved deck.</p><div class="slides-source-history"><button type="button" data-undo>Undo source</button><button type="button" data-redo>Redo source</button><button type="button" data-preview-caret>Preview selected slide</button></div></div><div class="slides-source-structure" role="complementary" aria-label="Source structure"><details open><summary>Slides</summary><div data-outline></div></details><details open><summary data-diagnostic-count>Findings</summary><div data-diagnostics></div></details><details open><summary>Story symbols</summary><label for="slides-symbol-search">Find a slide, cue or actor</label><input id="slides-symbol-search" data-symbol-search type="search"><div data-symbols></div><p data-symbol-label></p><label for="slides-symbol-name">New name</label><input id="slides-symbol-name" data-symbol-name type="text" maxlength="64"><button type="button" data-rename disabled>Rename in draft</button><div data-references></div></details></div></div><footer><button type="button" data-reload>Reload source</button><button type="button" data-save>Save and preview</button><span role="status" data-status></span></footer>';
      deck.appendChild(editor);
      editor.querySelector('[data-close]').onclick = () => editor.close();
      editor.querySelector('[data-reload]').onclick = loadSource;
      editor.querySelector('[data-undo]').onclick = () => restoreHistory(-1);
      editor.querySelector('[data-redo]').onclick = () => restoreHistory(1);
      sourceInput().addEventListener('input', () => { remember(); scheduleAnalysis(); status('Unsaved draft'); });
      sourceInput().addEventListener('select', updateSelection);
      sourceInput().addEventListener('click', updateSelection);
      sourceInput().addEventListener('keyup', updateSelection);
      sourceInput().addEventListener('keydown', event => {
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') { event.preventDefault(); restoreHistory(event.shiftKey ? 1 : -1); }
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'y') { event.preventDefault(); restoreHistory(1); }
      });
      editor.querySelector('[data-symbol-search]').oninput = renderSymbols;
      editor.querySelector('[data-rename]').onclick = () => { if (selectedSymbol) analyzeDraft({ offset: selectedSymbol.Range.StartByte, name: editor.querySelector('[data-symbol-name]').value }); };
      editor.querySelector('[data-preview-caret]').onclick = () => {
        if (!analysis) return;
        const offset = byteOffset(sourceInput().selectionStart), row = analysis.outline.find(s => offset >= s.range.StartByte && offset <= s.range.EndByte);
        if (row && row.index < deck.querySelectorAll('section.slide').length) { editor.close(); window.SlidesNav.preview(row.index, 0); }
        else status('Save this new slide before previewing');
      };
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
    editor.showModal(); if (!revision) await loadSource();
  }
  function status(message) { editor.querySelector('[data-status]').textContent = message; }
  async function loadSource() {
    try { const response = await fetch('/_slides/source', { cache: 'no-store' }); const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not load source'); sourceInput().value = data.source; revision = data.revision; token = data.token; history = []; historyIndex = -1; remember(); await analyzeDraft(); const current = analysis?.outline[SlidesNav.current() - 1]; if (current) selectRange(current.range); status('Ready to edit'); }
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

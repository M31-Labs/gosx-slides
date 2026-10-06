// Source editing is explicitly enabled by the server. Ink stays local to this tab.
(function () {
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav) return;
  const controls = deck.querySelector('.deck-controls');
  let editor, revision, token, saving = false, analysis, timer, controller, selectedSymbol;
  let history = [], historyIndex = -1;
  let currentFile = 'deck.md', contextRevision, originalSource, projectFiles = [], projectController, projectTimer;
  const drafts = new Map();
  function stashDraft() {
    if (!revision) return;
    drafts.set(currentFile, { value: sourceInput().value, baseline: originalSource, revision, contextRevision, history, historyIndex, start: sourceInput().selectionStart, end: sourceInput().selectionEnd });
    renderProjectTabs();
  }
  function renderProjectTabs() {
    const tabs = editor?.querySelector('[data-project-tabs]'); if (!tabs) return;
    tabs.replaceChildren(...[...drafts.keys()].map(file => {
      const b = button(file + (drafts.get(file).value !== drafts.get(file).baseline ? ' *' : ''), () => switchFile(file)); b.setAttribute('aria-pressed', String(file === currentFile)); return b;
    }));
    editor.querySelector('[data-project-file]').value = currentFile;
    editor.querySelector('label[for="slides-source"]').textContent = currentFile;
  }
  async function switchFile(file, force = false) {
    if (saving || file === currentFile && !force) return;
    clearTimeout(timer); clearTimeout(projectTimer); controller?.abort(); projectController?.abort();
    stashDraft();
    try {
      let cached = !force && drafts.get(file);
      if (!cached) {
        // Retain dirty files; bound tab and undo storage without dropping drafts.
        if (drafts.size >= 8 && !drafts.has(file)) {
          const clean = [...drafts].find(([name, row]) => name !== currentFile && row.value === row.baseline);
          if (clean) drafts.delete(clean[0]); else throw new Error('Eight file drafts are open; save or reload a draft before opening another');
        }
        const response = await fetch('/_slides/project?file=' + encodeURIComponent(file), { cache: 'no-store' });
        const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not load project file');
        cached = { value: data.source, baseline: data.source, revision: data.revision, contextRevision: data.contextRevision, history: [], historyIndex: -1, start: 0, end: 0 };
      }
      currentFile = file; revision = cached.revision; contextRevision = cached.contextRevision; originalSource = cached.baseline;
      history = cached.history; historyIndex = cached.historyIndex; sourceInput().value = cached.value;
      sourceInput().setSelectionRange(cached.start, cached.end); remember(); stashDraft();
      await analyzeDraft(); scheduleProjectDiagnosis(); status('Ready to edit ' + file);
    } catch (error) { status(error.message); renderProjectTabs(); }
  }
  function scheduleProjectDiagnosis() {
    clearTimeout(projectTimer); projectController?.abort();
    projectTimer = setTimeout(diagnoseProject, 650);
  }
  async function diagnoseProject() {
    const file = currentFile, source = sourceInput().value;
    projectController?.abort(); projectController = new AbortController();
    try {
      const headers = { 'Content-Type': 'application/json', 'X-Slides-Token': token };
      const response = await fetch('/_slides/project', { method: 'POST', signal: projectController.signal, headers: window.SlidesSessionHeaders ? SlidesSessionHeaders(headers) : headers, body: JSON.stringify({ file, source }) });
      const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Project diagnosis failed');
      if (file !== currentFile || source !== sourceInput().value) return;
      const list = editor.querySelector('[data-project-diagnostics]');
      list.replaceChildren(...result.diagnostics.map(d => {
        const b = listRow((d.file || 'deck.md') + ':' + d.range.StartLine + ' · ' + d.message, async () => {
          const target = d.file || 'deck.md';
          if (!projectFiles.some(row => row.path === target)) { status('This finding has no editable source location'); return; }
          await switchFile(target); if (currentFile === target) selectRange(d.range);
        }, d.code); b.dataset.severity = d.severity; return b;
      }));
      if (!result.diagnostics.length) { const p = document.createElement('p'); p.textContent = 'Project validates'; list.append(p); }
      const outline = editor.querySelector('[data-project-outline]');
      outline.replaceChildren(...result.outline.map(row => {
        const item = document.createElement('div'); item.className = 'slides-source-slide';
        item.append(listRow((row.index + 1) + ' · ' + row.title + ' · ' + row.file, async () => {
          if (!projectFiles.some(file => file.path === row.file)) return;
          await switchFile(row.file); if (currentFile === row.file) selectRange(row.range);
        }));
        const preview = button('Preview', () => { if (row.index < deck.querySelectorAll('section.slide').length) { editor.close(); SlidesNav.preview(row.index, 0); } else status('Save this new slide before previewing'); });
        preview.setAttribute('aria-label', 'Preview project slide ' + (row.index + 1)); item.append(preview); return item;
      }));
    } catch (error) { if (error.name !== 'AbortError') status(error.message); }
  }
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
    // Histories across all eight tabs share one 16 MiB memory budget.
    for (const row of drafts.values()) {
      if (row.history === history) continue;
      while (row.history.length > 1 && [...drafts.values()].reduce((sum, r) => sum + r.history.reduce((n, item) => n + item.value.length * 2, 0), bytes) > 16 * 1024 * 1024) { row.history.shift(); row.historyIndex = Math.max(0, row.historyIndex - 1); }
    }
    const memory = () => history.reduce((n, item) => n + item.value.length * 2, 0) + [...drafts.values()].filter(row => row.history !== history).reduce((n, row) => n + row.history.reduce((sum, item) => sum + item.value.length * 2, 0), 0);
    while (history.length > 1 && memory() > 16 * 1024 * 1024) { history.shift(); historyIndex--; }
    historyButtons();
  }
  function historyButtons() { editor.querySelector('[data-undo]').disabled = historyIndex < 1; editor.querySelector('[data-redo]').disabled = historyIndex >= history.length - 1; }
  function restoreHistory(delta) {
    const next = historyIndex + delta; if (next < 0 || next >= history.length) return;
    historyIndex = next; const row = history[next], input = sourceInput(); input.value = row.value;
    input.focus(); input.setSelectionRange(row.start, row.end); historyButtons(); scheduleAnalysis(); scheduleProjectDiagnosis(); stashDraft(); status('Draft restored');
  }
  function byteOffset(character) { return new TextEncoder().encode(sourceInput().value.slice(0, character)).length; }
  function characterOffset(byte) {
    // TextDecoder keeps Unicode/emoji before a ranged symbol from shifting its selection.
    return new TextDecoder().decode(new TextEncoder().encode(sourceInput().value).slice(0, byte)).length;
  }
  function selectRange(range) {
    if (sourceInput().value.includes('\r')) { status('Convert to LF line endings before selecting structured source'); return; }
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
    editor.querySelector('[data-rename]').disabled = !selectedSymbol || !analysis.renameable || currentFile !== 'deck.md';
    const list = editor.querySelector('[data-references]'); list.replaceChildren(...refs.map(s => listRow((s.Declaration ? 'Definition' : 'Reference') + ' · line ' + s.Range.StartLine, () => selectRange(s.Range))));
  }
  function renderAnalysis(data) {
    analysis = data;
    const outline = editor.querySelector('[data-outline]'); outline.replaceChildren(...(currentFile === 'deck.md' ? data.outline : []).map(row => {
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
    editor.querySelector('[data-preview-caret]').disabled = currentFile !== 'deck.md';
    renderSymbols(); updateSelection();
  }
  function renderSymbols() {
    if (!analysis) return;
    const query = editor.querySelector('[data-symbol-search]').value.trim().toLowerCase();
    editor.querySelector('[data-symbols]').replaceChildren(...analysis.symbols.filter(s => s.Declaration && (s.Kind + ' ' + s.Name).toLowerCase().includes(query)).slice(0, 200).map(s => listRow(s.Kind + ' · ' + s.Name, () => selectRange(s.Range), s.Scope)));
  }
  async function analyzeDraft(rename) {
    const source = sourceInput().value, file = currentFile;
    if (controller) controller.abort(); controller = new AbortController();
    try {
      if (!file.toLowerCase().endsWith('.md')) {
        renderAnalysis({ symbols: [], outline: [], diagnostics: [], editable: !source.includes('\r'), renameable: false }); return;
      }
      const headers = { 'Content-Type': 'application/json', 'X-Slides-Token': token };
      const response = await fetch('/_slides/analyze', { method: 'POST', signal: controller.signal, headers: window.SlidesSessionHeaders ? SlidesSessionHeaders(headers) : headers, body: JSON.stringify({ source, ...(rename ? { rename } : {}) }) });
      const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Could not analyze source');
      if (currentFile !== file || sourceInput().value !== source) return;
      if (rename) { sourceInput().value = result.source; remember(); stashDraft(); scheduleProjectDiagnosis(); status('Renamed in draft; save to publish'); }
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
      editor.querySelector('h2').textContent = 'Edit project source';
      const files = document.createElement('div'); files.className = 'slides-project-files';
      files.innerHTML = '<label for="slides-project-file">Project file</label><select id="slides-project-file" data-project-file aria-label="Project file"></select><nav data-project-tabs aria-label="Open project files"></nav>';
      editor.querySelector('header').after(files);
      editor.querySelector('[data-project-file]').onchange = event => switchFile(event.target.value);
      const findings = document.createElement('details'); findings.open = true;
      findings.innerHTML = '<summary>Project findings</summary><p>Checks this draft with the other saved files.</p><div data-project-diagnostics></div><p>Project slides</p><div data-project-outline></div>';
      editor.querySelector('.slides-source-structure').prepend(findings);
      editor.querySelector('[data-close]').onclick = () => editor.close();
      editor.querySelector('[data-reload]').onclick = loadSource;
      editor.querySelector('[data-undo]').onclick = () => restoreHistory(-1);
      editor.querySelector('[data-redo]').onclick = () => restoreHistory(1);
      sourceInput().addEventListener('input', () => { remember(); stashDraft(); scheduleAnalysis(); scheduleProjectDiagnosis(); status('Unsaved draft'); });
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
          const headers = { 'Content-Type': 'application/json', 'X-Slides-Token': token };
          const otherDraft = [...drafts].some(([file, row]) => file !== currentFile && row.value !== row.baseline);
          const previousContext = contextRevision;
          const response = await fetch('/_slides/project', { method: 'PUT', headers: window.SlidesSessionHeaders ? SlidesSessionHeaders(headers) : headers, body: JSON.stringify({ file: currentFile, source: sourceInput().value, revision, contextRevision }) });
          const result = await response.json(); if (!response.ok) throw new Error(result.error || 'Save failed');
          revision = result.revision; contextRevision = result.contextRevision; originalSource = sourceInput().value;
          // This successful save proves the previous context was current. Rebase
          // only drafts from that exact context across this known local write.
          for (const row of drafts.values()) if (row.contextRevision === previousContext) row.contextRevision = contextRevision;
          history = []; historyIndex = -1; remember(); stashDraft();
          status('Saved ' + currentFile);
          if (!otherDraft) location.reload(); else status('Saved ' + currentFile + '; other drafts remain open.');
        } catch (error) { status(error.message); } finally { saving = false; save.disabled = false; }
      };
    }
    if (editor.open) return;
    editor.showModal(); if (!revision) await loadSource();
  }
  function status(message) { editor.querySelector('[data-status]').textContent = message; }
  async function loadSource() {
    try {
      const response = await fetch('/_slides/project', { cache: 'no-store' }); const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not load project');
      token = data.token; projectFiles = data.files;
      editor.querySelector('[data-project-file]').replaceChildren(...data.files.map(file => { const option = document.createElement('option'); option.value = file.path; option.textContent = file.path; return option; }));
      await switchFile(currentFile, true);
      if (currentFile === 'deck.md') { const current = analysis?.outline[SlidesNav.current() - 1]; if (current) selectRange(current.range); }
      status('Ready to edit');
    }
    catch (error) { status(error.message); }
  }
  const canEdit = !!document.querySelector('meta[name="slides-edit"]');
  if (canEdit) window.SlidesEditor = { hasDrafts: () => [...drafts.values()].some(row => row.value !== row.baseline) };
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

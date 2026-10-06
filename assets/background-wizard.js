// A local authoring wizard. Only Apply writes, through the project's guarded save.
(function () {
  const deck = document.querySelector('main.deck');
  if (!deck || !window.SlidesNav || !document.querySelector('meta[name="slides-edit"]')) return;
  const trigger = document.createElement('button');
  trigger.type = 'button'; trigger.className = 'slides-extra-control'; trigger.textContent = 'Backgrounds';
  trigger.setAttribute('aria-label', 'Background wizard');
  deck.querySelector('.deck-controls')?.append(trigger);
  let panel, state, options, presets = [], step = 0, slide = 0, scope = 'slide', loading = false, saving = false, request = 0, previewTimer, controller;
  const $ = selector => panel.querySelector(selector);
  function status(message) { $('[data-background-status]').textContent = message; }
  function controls() {
    $('[data-background-back]').disabled = step === 0 || saving;
    $('[data-background-next]').disabled = saving || !options;
    $('[data-background-next]').hidden = step === 2;
    $('[data-background-apply]').hidden = step !== 2;
    $('[data-background-apply]').disabled = loading || saving || !state;
    $('[data-background-close]').disabled = saving;
  }
  function go(next) {
    step = next;
    panel.querySelectorAll('[data-background-step]').forEach(node => { node.hidden = Number(node.dataset.backgroundStep) !== step; });
    panel.querySelectorAll('[data-background-progress]').forEach(node => { if (Number(node.dataset.backgroundProgress) === step) node.setAttribute('aria-current', 'step'); else node.removeAttribute('aria-current'); });
    $('[data-background-next]').textContent = step === 0 ? 'Tune background →' : 'Choose where →';
    controls();
  }
  function renderOptions() {
    for (const key of ['ink', 'glow', 'speed', 'strength', 'scale']) {
      const input = $('[data-background-option="' + key + '"]'); input.value = options[key];
      const output = $('[data-background-value="' + key + '"]'); if (output) output.textContent = Number(options[key]).toFixed(2);
    }
    panel.querySelectorAll('[data-background-preset]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.backgroundPreset === options.preset)));
    const preset = presets.find(row => row.name === options.preset);
    $('[data-background-description]').textContent = preset.description;
    $('[data-background-preview-title]').textContent = preset.title + ' · sample slide';
    snippet(); preview();
  }
  function snippet() {
    if (!options) return;
    const rows = ['scene: "shader:' + options.preset + '"', 'shader-ink: "' + options.ink + '"', 'shader-glow: "' + options.glow + '"', 'shader-speed: ' + options.speed, 'shader-strength: ' + options.strength, 'shader-scale: ' + options.scale];
    $('[data-background-snippet]').textContent = (scope === 'deck' ? '---\n' : '```yaml\n') + rows.join('\n') + (scope === 'deck' ? '\n---' : '\n```');
  }
  function preview() {
    clearTimeout(previewTimer);
    previewTimer = setTimeout(() => {
      if (!panel.open) return;
      const query = new URLSearchParams(options);
      $('[data-background-preview]').src = '/_slides/background/preview?' + query;
    }, 180);
  }
  async function load(retain = false) {
    const serial = ++request; controller?.abort(); controller = new AbortController(); loading = true; state = null; controls();
    status('Loading background settings…');
    try {
      const response = await fetch('/_slides/background?scope=' + scope + '&slide=' + slide, { cache: 'no-store', signal: controller.signal });
      const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not load settings');
      if (serial !== request || !panel.open) return;
      state = data; presets = data.presets;
      if (!retain || !options) options = { ...data.options };
      $('[data-background-presets]').replaceChildren(...presets.map(row => {
        const button = document.createElement('button'); button.type = 'button'; button.dataset.backgroundPreset = row.name;
        button.setAttribute('aria-label', row.title + ' background');
        const sample = document.createElement('span'); sample.className = 'background-swatch background-swatch-' + row.name; sample.setAttribute('aria-hidden', 'true');
        const title = document.createElement('strong'); title.textContent = row.title;
        button.append(sample, title); button.onclick = () => { options = { ...row.defaults }; renderOptions(); status(row.title + ' selected'); };
        return button;
      }));
      $('[data-background-target]').textContent = data.title + ' · ' + data.file;
      renderOptions(); status('Ready. Preview changes stay in this wizard.');
    } catch (error) { if (serial === request && error.name !== 'AbortError') status(error.message); }
    finally { if (serial === request) { loading = false; controls(); } }
  }
  function download() {
    const source = presets.find(row => row.name === options.preset).source.replace(/^(\s*param (ink|glow|speed|strength|scale)\s*:\s*\w+\s*=).*/gm, (line, declaration, name) => {
      let value = options[name];
      if (name === 'ink' || name === 'glow') value = 'rgb(' + [1, 3, 5].map(i => (parseInt(value.slice(i, i + 2), 16) / 255).toFixed(6)).join(', ') + ')';
      else { value = String(value); if (!value.includes('.')) value += '.0'; }
      return declaration + ' ' + value;
    });
    const url = URL.createObjectURL(new Blob([source], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a'); link.href = url; link.download = options.preset + '.sel'; link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    status('Downloaded ' + options.preset + '.sel with your settings.');
  }
  async function apply() {
    if (loading || saving || !state) return;
    if (window.SlidesEditor?.hasDrafts()) { status('Save or reload your source drafts in Edit before applying a background. Your drafts are retained.'); return; }
    saving = true; controls(); status('Validating and saving ' + state.file + '…');
    try {
      let headers = { 'Content-Type': 'application/json', 'X-Slides-Token': state.token };
      if (window.SlidesSessionHeaders) headers = SlidesSessionHeaders(headers);
      const response = await fetch('/_slides/background', { method: 'PUT', headers, body: JSON.stringify({ scope, slide, file: state.file, revision: state.revision, contextRevision: state.contextRevision, options }) });
      const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Could not save background');
      status('Saved ' + state.file); SlidesNav.reloadPreview();
    } catch (error) { status(error.message + '. Use Reload settings to keep your chosen colors and retry.'); }
    finally { saving = false; controls(); }
  }
  function create() {
    panel = document.createElement('dialog'); panel.className = 'slides-background-wizard'; panel.setAttribute('aria-labelledby', 'slides-background-title');
    panel.innerHTML = '<header><div><p class="background-eyebrow">SELENA BACKGROUNDS</p><h2 id="slides-background-title">A little atmosphere.</h2></div><button type="button" data-background-close aria-label="Close background wizard">Close</button></header>' +
      '<ol class="background-progress" aria-label="Wizard progress"><li data-background-progress="0">1 · Choose</li><li data-background-progress="1">2 · Tune</li><li data-background-progress="2">3 · Apply</li></ol>' +
      '<div class="background-workspace"><div class="background-settings">' +
      '<section data-background-step="0"><h3>Start with a texture</h3><div data-background-presets class="background-presets"></div><p data-background-description></p></section>' +
      '<section data-background-step="1" hidden><h3>Make it yours</h3><div class="background-colors"><label>Base color<input type="color" data-background-option="ink"></label><label>Highlight color<input type="color" data-background-option="glow"></label></div>' +
      '<label class="background-range">Motion speed <output data-background-value="speed"></output><input type="range" min="0" max="2" step="0.01" data-background-option="speed"></label><p>Set speed to zero for a still background.</p>' +
      '<label class="background-range">Texture strength <output data-background-value="strength"></output><input type="range" min="0" max="1" step="0.01" data-background-option="strength"></label>' +
      '<label class="background-range">Pattern scale <output data-background-value="scale"></output><input type="range" min="0.5" max="4" step="0.05" data-background-option="scale"></label><p>Lower strength keeps text easier to read. Reduced motion uses your deck’s plain background.</p>' +
      '<button type="button" data-background-reset>Reset preset</button></section>' +
      '<section data-background-step="2" hidden><h3>Choose where it belongs</h3><fieldset><legend>Apply background to</legend><label><input type="radio" name="background-scope" value="slide" checked> Current slide</label><label><input type="radio" name="background-scope" value="deck"> Deck default</label></fieldset><p data-background-target></p><p>Deck defaults keep existing per-slide overrides. Saves retain a recovery copy.</p>' +
      '<details><summary>Copy Markdown or download the shader</summary><pre data-background-snippet></pre><div class="background-code-actions"><button type="button" data-background-copy>Copy Markdown</button><button type="button" data-background-download>Download .sel</button></div><p>For a custom shader, save the .sel under shaders/ and point scene at that file.</p></details></section>' +
      '</div><section class="background-preview"><div class="background-preview-heading"><span data-background-preview-title>Sample slide</span><span>LIVE PREVIEW</span></div><iframe title="Live Selena background preview" data-background-preview></iframe><p>This sample uses the same native shader as your deck. Previewing leaves source files untouched.</p></section></div>' +
      '<footer><p role="status" aria-live="polite" data-background-status></p><div class="background-actions"><button type="button" data-background-reload>Reload settings</button><button type="button" data-background-back>Back</button><button type="button" class="background-primary" data-background-next>Tune background →</button><button type="button" class="background-primary" data-background-apply hidden>Apply background</button></div></footer>';
    deck.append(panel);
    $('[data-background-close]').onclick = () => panel.close();
    $('[data-background-back]').onclick = () => go(step - 1);
    $('[data-background-next]').onclick = () => go(step + 1);
    $('[data-background-reset]').onclick = () => { options = { ...presets.find(row => row.name === options.preset).defaults }; renderOptions(); };
    $('[data-background-reload]').onclick = () => load(true);
    $('[data-background-apply]').onclick = apply;
    $('[data-background-download]').onclick = download;
    $('[data-background-copy]').onclick = async () => { try { await navigator.clipboard.writeText($('[data-background-snippet]').textContent); status('Copied Markdown'); } catch (_) { status('Select the Markdown above and copy it.'); } };
    panel.querySelectorAll('[data-background-option]').forEach(input => input.oninput = () => { const key = input.dataset.backgroundOption; options[key] = input.type === 'color' ? input.value : Number(input.value); renderOptions(); });
    panel.querySelectorAll('[name="background-scope"]').forEach(input => input.onchange = () => { scope = input.value; snippet(); load(true); });
    panel.addEventListener('cancel', event => { if (saving) event.preventDefault(); });
    panel.addEventListener('close', () => { clearTimeout(previewTimer); controller?.abort(); request++; $('[data-background-preview]').src = 'about:blank'; trigger.focus(); });
  }
  trigger.onclick = () => {
    if (!panel) create(); if (panel.open) return;
    slide = SlidesNav.current() - 1; scope = 'slide'; $('[name="background-scope"][value="slide"]').checked = true;
    options = null; state = null; panel.showModal(); go(0); load();
  };
})();

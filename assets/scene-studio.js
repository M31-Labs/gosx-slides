(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck) return;
  const editable = !!document.querySelector('meta[name="slides-edit"]');
  let panel, section, mount, preferredMount, draft, steps, savedSteps, generation = 0, applied = 0, epoch = 0, busy = false;
  const history = [], future = [];
  const sessions = new WeakMap();
  const clone = value => JSON.parse(JSON.stringify(value));
  const control = name => section.querySelector('[data-scene-' + name + ']');
  const cue = () => Math.max(0, Math.min(steps.length - 1, SlidesNav.step()));
  const actors = () => (draft.scene.objects || []).filter(object => draft.scene.labels?.some(label => label.id === 'label:' + object.id));
  const message = text => { control('status').textContent = text; };
  function remember() {
    if (mount && draft && steps) {
      preferredMount = mount;
      sessions.set(mount,{draft,steps:clone(steps),savedSteps,generation,applied,actor:control('actor').value,history:clone(history),future:clone(future)});
    }
  }
  function buttons() {
    control('undo').disabled = busy || !history.length;
    control('redo').disabled = busy || !future.length;
    control('save').disabled = busy || !draft || !steps || JSON.stringify(steps) === savedSteps || applied !== generation;
  }
  function fill() {
    if (!steps) return;
    const index = cue(), step = steps[index], camera = {...draft.camera, ...step.camera};
    control('cue').value = String(index);
    control('duration').value = step.durationMs ?? 600; control('duration').disabled = index === 0;
    control('easing').value = step.easing || 'ease-in-out'; control('easing').disabled = index === 0;
    for (const key of ['x','y','z','fov']) {
      control('camera-' + key).value = camera[key] ?? (key === 'fov' ? 50 : 0);
      control('camera-' + key).disabled = key === 'fov' && camera.kind === 'orthographic';
    }
    const actor = actors().find(actor => actor.id === control('actor').value), patch = step.patches?.find(patch => patch.target === actor?.id) || {};
    for (const key of ['x','y','z','scale','color']) {
      const input = control('actor-' + key); input.disabled = !actor;
      input.value = patch[key] ?? ''; input.placeholder = key === 'scale' ? '1' : String(actor?.[key] ?? (key === 'color' ? '#88cab9' : 0));
    }
    buttons();
  }
  async function request(method, source) {
    const response = await fetch('/_slides/scene?graphic=' + encodeURIComponent(mount.dataset.slideSceneSource), {
      method, cache:'no-store', headers: method === 'GET' ? {} : {'Content-Type':'application/json','X-Slides-Token':draft.token},
      ...(method === 'GET' ? {} : {body:JSON.stringify({source:JSON.stringify(source,null,2)+'\n', revision:draft.revision, contextRevision:draft.contextRevision})})
    });
    const data = await response.json(); if (!response.ok) throw new Error(data.error || 'Scene edit failed'); return data;
  }
  async function preview() {
    if (busy || !steps) return;
    busy = true; const version = generation, source = clone(steps), selectedMount = mount, selectedEpoch = epoch;
    buttons(); message('Updating scene…');
    try {
      const result = await request('POST', source);
      if (selectedMount !== mount || selectedEpoch !== epoch || !panel.open) return;
      if (version === generation) {
        SlidesMotion.pause(); SlidesGraphicsMotion.replace(mount,result.timeline); SlidesMotion.seek(SlidesMotion.state().time);
        await SlidesMotion.settled(); applied = version; message('Preview ready. ' + draft.file + ' has unsaved cue edits.');
      }
    } catch (error) { if (selectedMount === mount && selectedEpoch === epoch) message(error.message); }
    finally { busy = false; buttons(); if (panel.open && steps && (selectedEpoch !== epoch || version !== generation)) preview(); }
  }
  function change(edit) {
    if (!steps) return;
    const before = clone(steps); edit(steps[cue()]);
    if (JSON.stringify(before) === JSON.stringify(steps)) return;
    history.push(before); if (history.length > 100) history.shift(); future.length = 0; generation++; fill(); preview();
  }
  function restore(from, to) {
    if (!from.length || busy) return; to.push(clone(steps)); steps = from.pop(); generation++; fill(); preview();
  }
  async function load() {
    const sourceMount = mount, loadEpoch = ++epoch; draft = steps = null; history.length = future.length = 0; savedSteps = ''; generation = applied = 0; buttons();
    section.querySelector('fieldset').disabled = true; control('save').disabled = true;
    if (!editable || !mount.dataset.slideSceneSource) { message('Camera and actors share the timeline below. To edit cues, declare a .sir source with Steps and serve with --edit.'); return; }
    message('Loading scene cues…');
    try {
      const data = await request('GET'); if (mount !== sourceMount || loadEpoch !== epoch || !panel.open) return;
      if (data.deckRevision !== deck.dataset.sourceRevision || data.revision !== mount.dataset.slideSceneRevision || data.inputRevision !== mount.dataset.slideSceneInputs) throw new Error('Scene source changed; reload the deck before editing cues.');
      draft = data; steps = JSON.parse(data.source); savedSteps = JSON.stringify(steps);
      const previous = sessions.get(mount);
      if (previous && previous.draft.revision === data.revision && previous.draft.contextRevision === data.contextRevision) {
        steps = clone(previous.steps); generation = previous.generation; applied = previous.applied;
        history.push(...clone(previous.history)); future.push(...clone(previous.future));
      }
      control('cue').replaceChildren(...steps.map((step,index)=>new Option((index + 1) + ' · ' + (step.label || 'Cue'),String(index))));
      control('actor').replaceChildren(...actors().map(actor=>new Option(draft.scene.labels.find(label=>label.id==='label:'+actor.id)?.text || actor.id,actor.id)));
      if (previous?.actor && actors().some(actor=>actor.id===previous.actor)) control('actor').value=previous.actor;
      section.querySelector('fieldset').disabled = false; fill(); message('Edit the current cue. Blank actor values restore the original layout.');
      if (generation !== applied) preview();
    } catch (error) { if (mount === sourceMount && loadEpoch === epoch) message(error.message); }
  }
  function open(target) {
    remember();
    panel = target;
    if (!section) {
      section = document.createElement('section'); section.dataset.sceneStudio = ''; section.setAttribute('aria-label','Scene motion');
      section.innerHTML = '<h3>Scene motion</h3><label>Scene<select data-scene-surface></select></label><fieldset disabled><label>Cue<select data-scene-cue></select></label>' +
        '<div class="slides-author-fields"><label>Cue duration (ms)<input data-scene-duration type="number" min="0" max="600000" step="1"></label><label>Cue easing<select data-scene-easing><option>ease-in-out</option><option>ease-in</option><option>ease-out</option><option>linear</option><option>ease</option></select></label></div>' +
        '<h4>Camera</h4><div class="slides-scene-fields">' + ['x','y','z','fov'].map(key=>'<label>Camera '+key.toUpperCase()+'<input data-scene-camera-'+key+' type="number" step="any"'+(key==='fov'?' min="1" max="179"':'')+'></label>').join('') + '</div>' +
        '<h4>Actor</h4><label>Actor<select data-scene-actor></select></label><div class="slides-scene-fields">' + ['x','y','z','scale','color'].map(key=>'<label>Actor '+key+'<input data-scene-actor-'+key+' '+(key==='color'?'type="text" pattern="#[0-9a-fA-F]{6}"':'type="number" step="any"'+(key==='scale'?' min="0.001"':''))+'></label>').join('') + '</div></fieldset>' +
        '<div class="slides-author-actions"><button type="button" data-scene-undo>Undo scene</button><button type="button" data-scene-redo>Redo scene</button><button type="button" data-scene-save>Save scene cues</button></div><output data-scene-status aria-live="polite"></output>';
      panel.querySelector('[data-motion-element]').parentElement.before(section);
      control('surface').onchange = () => { remember(); mount = SlidesGraphicsMotion.current()[Number(control('surface').value)]?.mount; if (mount) load(); };
      control('cue').onchange = () => { SlidesMotion.pause(); SlidesNav.show(SlidesNav.current()-1,Number(control('cue').value),true); SlidesMotion.seek(SlidesMotion.duration()); fill(); };
      control('actor').onchange = fill;
      for (const key of ['duration','easing']) control(key).onchange = event => {
        if (!event.target.checkValidity() || event.target.value === '') { message('Enter a valid cue timing.'); return; }
        change(step=>{step[key === 'duration' ? 'durationMs' : 'easing'] = key === 'duration' ? Number(event.target.value) : event.target.value;});
      };
      for (const key of ['x','y','z','fov']) control('camera-' + key).onchange = event => {
        if (!event.target.checkValidity() || event.target.value === '') { message('Enter a valid camera value.'); return; }
        change(step=>{step.camera = {...draft.camera,...step.camera,[key]:Number(event.target.value)};});
      };
      for (const key of ['x','y','z','scale','color']) control('actor-' + key).onchange = event => {
        if (!event.target.checkValidity()) { message('Enter a valid actor value.'); return; }
        change(step=>{
          step.patches ||= []; let patch = step.patches.find(patch=>patch.target===control('actor').value);
          if (!patch) { patch = {target:control('actor').value}; step.patches.push(patch); }
          if (event.target.value === '') delete patch[key]; else patch[key] = key==='color' ? event.target.value : Number(event.target.value);
          if (Object.keys(patch).length===1) step.patches = step.patches.filter(value=>value!==patch);
        });
      };
      control('undo').onclick = () => restore(history,future); control('redo').onclick = () => restore(future,history);
      control('save').hidden = !editable;
      control('save').onclick = async () => {
        if (busy || applied !== generation) return; busy = true; buttons(); message('Saving scene cues…');
        try { await request('PUT',steps); location.reload(); } catch(error) { message(error.message); } finally { busy=false; buttons(); }
      };
      // A dialog's queued close event may arrive after it has reopened.
      panel.addEventListener('close',()=>{if(panel.open)return;remember();epoch++;mount=null;draft=steps=null;});
      deck.addEventListener('slides:change',()=>{if (panel.open && steps && mount?.closest('.deck-active')) fill();});
    }
    const scenes = window.SlidesGraphicsMotion?.current() || []; section.hidden = !scenes.length;
    if (!scenes.length) return;
    control('surface').replaceChildren(...scenes.map((scene,index)=>new Option(scene.mount.getAttribute('aria-label') || 'Scene '+(index+1),String(index))));
    control('surface').parentElement.hidden = scenes.length===1;
    const preferred = Math.max(0,scenes.findIndex(scene=>scene.mount===preferredMount));
    control('surface').value = String(preferred); mount = scenes[preferred].mount; load();
  }
  window.SlidesSceneStudio = {open};
})();

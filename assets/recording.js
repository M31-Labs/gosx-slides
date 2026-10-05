(function () {
  'use strict';
  const deck = document.querySelector('main.deck'), nav = window.SlidesNav;
  if (!deck || !nav || deck.classList.contains('deck-presenter')) return;
  const MAX_DURATION = 30 * 60 * 1000, MAX_BYTES = 256 * 1024 * 1024, MAX_EVENTS = 10000;
  const types = ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm', 'video/mp4'];
  const mimeType = window.MediaRecorder && types.find(type => MediaRecorder.isTypeSupported(type));
  const supported = Boolean(navigator.mediaDevices?.getDisplayMedia && mimeType && HTMLCanvasElement.prototype.captureStream);
  const devicesSupported = Boolean(navigator.mediaDevices?.getUserMedia);
  let authored = [];
  try { authored = JSON.parse(document.getElementById('slides-recording-data')?.textContent || '[]'); } catch (_) {}
  const panel = document.createElement('dialog');
  panel.className = 'slides-recording-panel'; panel.setAttribute('aria-labelledby', 'slides-recording-title');
  panel.innerHTML = '<h2 id="slides-recording-title">Record this presentation</h2><p>Choose this presentation tab in the browser share picker. Your recording and sidecars stay on this device.</p><label><input type="checkbox" data-recording-mic> Include microphone</label><label><input type="checkbox" data-recording-camera> Include a camera inset</label><p data-recording-limits>Recording stops after 30 minutes or 256 MiB. Captions use authored caption text when available; navigation labels describe slide changes.</p><p class="slides-recording-status" role="status" aria-live="polite"></p><div class="slides-recording-downloads" hidden></div><footer><button type="button" data-recording-start>Start recording</button><button type="button" data-recording-close>Close</button></footer>';
  deck.append(panel);
  const status = panel.querySelector('.slides-recording-status'), downloads = panel.querySelector('.slides-recording-downloads');
  const startButton = panel.querySelector('[data-recording-start]');
  const button = document.createElement('button'); button.type = 'button'; button.textContent = '⏺'; button.title = 'Record presentation'; button.setAttribute('aria-label', 'Record presentation');
  deck.querySelector('.deck-controls')?.append(button);
  const live = document.createElement('div'); live.className = 'slides-recording-live'; live.hidden = true;
  const clock = document.createElement('span'); clock.textContent = 'Recording 00:00';
  const stopButton = document.createElement('button'); stopButton.type = 'button'; stopButton.textContent = 'Stop recording';
  live.append(clock, stopButton); deck.append(live);
  let phase = 'idle', session = 0, resources = [], videos = [], recorder, chunks = [], bytes = 0;
  let timer = 0, raf = 0, started = 0, ended = 0, events = [], result = null, objectURLs = [];
  let resolveStopped = null, stopped = Promise.resolve(null), reason = '', error = '', selection = { microphone:false, camera:false };
  let returnFocus = null;
  function open() {
    returnFocus = document.activeElement;
    if (!panel.open) panel.showModal();
    if (!supported) status.textContent = 'This browser cannot record a shared tab here. Use a browser with screen capture and MediaRecorder support on HTTPS or localhost.';
  }
  function close() { panel.close(); (returnFocus?.offsetParent ? returnFocus : button).focus?.(); }
  function update() {
    const busy = ['requesting','recording','finishing'].includes(phase);
    startButton.disabled = !supported || busy; button.textContent = phase === 'recording' ? '■' : '⏺';
    button.setAttribute('aria-label', phase === 'recording' ? 'Stop recording' : 'Record presentation');
    panel.querySelectorAll('input').forEach(input => { input.disabled = busy || !devicesSupported; });
    live.hidden = phase !== 'recording' && phase !== 'finishing'; stopButton.disabled = phase !== 'recording';
    clock.textContent = 'Recording ' + elapsedLabel(duration());
  }
  function elapsedLabel(ms) { const seconds = Math.floor(ms/1000); return String(Math.floor(seconds/60)).padStart(2,'0')+':'+String(seconds%60).padStart(2,'0'); }
  function duration() { return started ? Math.max(0, (ended || performance.now()) - started) : 0; }
  function stopResources() {
    cancelAnimationFrame(raf); raf = 0; clearInterval(timer); timer = 0;
    const tracks = new Set(resources.flatMap(stream => stream.getTracks()));
    tracks.forEach(track => { try { track.stop(); } catch (_) {} }); resources = [];
    videos.forEach(video => { video.pause(); video.srcObject = null; }); videos = [];
  }
  function revokeDownloads() { objectURLs.forEach(URL.revokeObjectURL); objectURLs = []; downloads.replaceChildren(); downloads.hidden = true; }
  function addDownload(blob, filename, label) {
    const link = document.createElement('a'), url = URL.createObjectURL(blob); objectURLs.push(url);
    link.href = url; link.download = filename; link.textContent = label; downloads.append(link); downloads.hidden = false;
  }
  function timestamp(ms) {
    ms = Math.max(0,Math.round(ms)); const seconds = Math.floor(ms/1000), minutes = Math.floor(seconds/60), hours = Math.floor(minutes/60);
    return String(hours).padStart(2,'0')+':'+String(minutes%60).padStart(2,'0')+':'+String(seconds%60).padStart(2,'0')+'.'+String(ms%1000).padStart(3,'0');
  }
  function vttText(text) { return text.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\n\s*\n/g,'\n'); }
  function sidecar(end) {
    const hasCaptions = authored.some(slide => slide.caption), kind = hasCaptions ? 'authored' : 'navigation';
    let vtt = 'WEBVTT\n\nNOTE '+(hasCaptions ? 'Authored captions timed to slide navigation.' : 'Slide navigation labels; this file does not transcribe speech.')+'\n\n';
    let count = 0;
    events.forEach((event,index) => {
      const until = index+1 < events.length ? events[index+1].timeMs : end;
      const text = hasCaptions ? authored[event.slide-1]?.caption : 'Slide '+event.slide+' · '+event.title+' · step '+event.step;
      if (text && until > event.timeMs) vtt += (++count)+'\n'+timestamp(event.timeMs)+' --> '+timestamp(until)+'\n'+vttText(text)+'\n\n';
    });
    return { kind, vtt };
  }
  function navigation() {
    if (phase !== 'recording') return;
    const slide = nav.current(), step = nav.step();
    const previous = events[events.length-1]; if (previous && previous.slide === slide && previous.step === step) return;
    if (events.length >= MAX_EVENTS) { stop('Navigation event limit reached.'); return; }
    const node = deck.querySelector(':scope > .slide[data-slide="'+(slide-1)+'"]');
    events.push({ timeMs: Math.round(duration()), slide, step, id:node?.dataset.slideId || '', title:(node?.querySelector('h1,h2,h3,h4,h5,h6')?.textContent || 'Slide '+slide).trim().slice(0,512) });
  }
  async function inputVideo(stream) {
    const video = document.createElement('video'); video.muted = true; video.playsInline = true; video.srcObject = stream; videos.push(video);
    await video.play(); return video;
  }
  function finished() {
    if (!['recording','finishing'].includes(phase)) return;
    ended ||= performance.now(); stopResources();
    const blob = new Blob(chunks, { type:recorder?.mimeType || mimeType }); chunks = [];
    phase = error ? 'error' : 'ready';
    if (blob.size) {
      const end = Math.round(duration()), captions = sidecar(end);
      const timeline = { version:1, durationMs:end, mimeType:blob.type, microphone:selection.microphone, camera:selection.camera, captionKind:captions.kind, navigation:events };
      result = { video:blob, timeline, vtt:captions.vtt, captionKind:captions.kind };
      revokeDownloads();
      const name = (document.title || 'presentation').replace(/[^a-zA-Z0-9_-]+/g,'-').slice(0,80) || 'presentation';
      addDownload(blob, name+(blob.type.startsWith('video/mp4') ? '.mp4' : '.webm'), 'Download recording');
      addDownload(new Blob([JSON.stringify(timeline,null,2)],{type:'application/json'}), name+'.navigation.json', 'Download navigation');
      addDownload(new Blob([captions.vtt],{type:'text/vtt'}), name+'.'+(captions.kind === 'authored' ? 'captions' : 'navigation')+'.vtt', captions.kind === 'authored' ? 'Download captions' : 'Download navigation labels');
    }
    status.textContent = error || reason || (blob.size ? 'Recording ready. Download your files below.' : 'No recording was produced.');
    recorder = null; update(); resolveStopped?.(result); resolveStopped = null; open();
  }
  async function start(options = {}) {
    if (!supported) throw Error('Screen recording is unavailable in this browser or context.');
    if ((options.microphone || options.camera) && !devicesSupported) throw Error('Microphone and camera capture are unavailable in this browser or context.');
    if (['requesting','recording','finishing'].includes(phase)) throw Error('A recording is already active.');
    const current = ++session; phase = 'requesting'; error = ''; reason = ''; selection = { microphone:Boolean(options.microphone), camera:Boolean(options.camera) };
    let maximum = Number(options.maxDurationMs ?? MAX_DURATION);
    if (!Number.isFinite(maximum) || maximum < 1000 || maximum > MAX_DURATION) maximum = MAX_DURATION;
    status.textContent = 'Choose this presentation tab in the browser share picker.'; update(); if (panel.open) close();
    const own = stream => { if (current !== session) { stream.getTracks().forEach(track=>track.stop()); throw new DOMException('Recording canceled','AbortError'); } resources.push(stream); return stream; };
    try {
      // Invoke screen sharing before awaiting other permissions: browsers require
      // getDisplayMedia to run directly within the Start button's activation.
      const display = own(await navigator.mediaDevices.getDisplayMedia({ video:{frameRate:30}, audio:false, preferCurrentTab:true, selfBrowserSurface:'include' }));
      const displayTrack = display.getVideoTracks()[0];
      function liveDisplay() {
        if (current !== session) throw new DOMException('Recording canceled','AbortError');
        if (!displayTrack || displayTrack.readyState !== 'live') throw new DOMException('Screen sharing ended before recording started.','AbortError');
      }
      // Sharing can end while a separate device permission is still pending.
      // Cancel this generation immediately; own() stops any late granted tracks.
      displayTrack?.addEventListener('ended',()=>{
        if (current !== session) return;
        if (phase === 'requesting') { stop('Screen sharing ended.'); open(); }
        else if (phase === 'recording') stop('Screen sharing ended.');
      },{once:true});
      liveDisplay();
      const microphone = selection.microphone ? own(await navigator.mediaDevices.getUserMedia({audio:true,video:false})) : null;
      const camera = selection.camera ? own(await navigator.mediaDevices.getUserMedia({video:{width:{ideal:640},height:{ideal:360}},audio:false})) : null;
      liveDisplay();
      const screen = await inputVideo(display), face = camera ? await inputVideo(camera) : null;
      liveDisplay();
      const canvas = document.createElement('canvas'), scale = Math.min(1,1920/(screen.videoWidth || 1280),1080/(screen.videoHeight || 720));
      canvas.width = Math.max(2, Math.round((screen.videoWidth || 1280)*scale)); canvas.height = Math.max(2,Math.round((screen.videoHeight || 720)*scale));
      const context = canvas.getContext('2d');
      function draw() {
        context.drawImage(screen,0,0,canvas.width,canvas.height);
        if (face) {
          const width = Math.min(320,canvas.width*.28), height = width*9/16, gap = Math.max(8,canvas.width*.016);
          const ratio = Math.max(width/(face.videoWidth || width),height/(face.videoHeight || height)), sw = width/ratio, sh = height/ratio;
          context.fillStyle = '#151923'; context.fillRect(canvas.width-width-gap-3,canvas.height-height-gap-3,width+6,height+6);
          context.drawImage(face,((face.videoWidth || sw)-sw)/2,((face.videoHeight || sh)-sh)/2,sw,sh,canvas.width-width-gap,canvas.height-height-gap,width,height);
        }
        raf = requestAnimationFrame(draw);
      }
      draw(); const output = own(canvas.captureStream(30)); microphone?.getAudioTracks().forEach(track=>output.addTrack(track));
      recorder = new MediaRecorder(output,{mimeType,videoBitsPerSecond:3000000,audioBitsPerSecond:128000});
      chunks = []; bytes = 0; events = []; started = 0; ended = 0;
      stopped = new Promise(resolve => { resolveStopped = resolve; });
      recorder.ondataavailable = event => { if (event.data.size) { chunks.push(event.data); bytes += event.data.size; if (bytes >= MAX_BYTES && phase === 'recording') stop('The 256 MiB recording limit was reached.'); } };
      recorder.onerror = event => { error = event.error?.message || 'The browser could not finish recording.'; if (recorder.state !== 'inactive') stop(); else finished(); };
      recorder.onstop = finished;
      liveDisplay();
      if (!output.getVideoTracks().some(track=>track.readyState==='live')) throw new DOMException('Recording video is unavailable.','AbortError');
      recorder.start(1000); started = performance.now(); phase = 'recording'; navigation(); update();
      timer = setInterval(()=>{ update(); if (duration() >= maximum) stop('The recording duration limit was reached.'); },250);
      return state();
    } catch (failure) {
      if (current === session) {
        phase = 'error'; stopResources();
        try { if (recorder?.state !== 'inactive') recorder?.stop(); } catch (_) {}
        recorder = null; resolveStopped?.(null); resolveStopped = null;
        error = failure.name === 'NotAllowedError' ? 'Recording permission was denied. Your devices are off.' : failure.message || 'Recording could not start.';
        status.textContent = error; update(); open();
      }
      throw failure;
    }
  }
  function stop(message = '') {
    reason = message;
    if (phase === 'requesting') { session++; stopResources(); phase = 'idle'; status.textContent = (message ? message+' ' : '')+'Recording canceled. Cancel any remaining browser permission prompt.'; update(); return Promise.resolve(null); }
    if (phase !== 'recording') return stopped;
    ended = performance.now(); phase = 'finishing'; clearInterval(timer); timer = 0; cancelAnimationFrame(raf); raf = 0; update();
    if (recorder.state !== 'inactive') recorder.stop(); else finished();
    return stopped;
  }
  function state() { return { phase, durationMs:Math.round(duration()), bytes, microphone:selection.microphone, camera:selection.camera, error }; }
  startButton.addEventListener('click',()=>start({microphone:panel.querySelector('[data-recording-mic]').checked,camera:panel.querySelector('[data-recording-camera]').checked}).catch(()=>{}));
  button.addEventListener('click',()=>phase === 'recording' ? stop() : open());
  stopButton.addEventListener('click',()=>stop()); panel.querySelector('[data-recording-close]').addEventListener('click',close);
  panel.addEventListener('cancel',()=>{ returnFocus?.focus?.(); });
  deck.addEventListener('slides:change',navigation);
  window.addEventListener('pagehide',()=>{ session++; try { if (recorder?.state !== 'inactive') recorder?.stop(); } catch (_) {} phase = 'idle'; stopResources(); revokeDownloads(); });
  window.SlidesRecording = { open, start, stop, state, result:()=>result, supported };
  update();
})();

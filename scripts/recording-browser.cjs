const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

async function fakeCapture(page) {
  await page.addInitScript(() => {
    window.recordingTest = { streams:[], calls:[], denial:'', delay:false, resolve:null, contexts:[] };
    function screen(color, camera = false) {
      const canvas = document.createElement('canvas'); canvas.width = camera ? 160 : 640; canvas.height = camera ? 90 : 360;
      const context = canvas.getContext('2d'); context.fillStyle = color; context.fillRect(0,0,canvas.width,canvas.height);
      const stream = canvas.captureStream(20); recordingTest.streams.push(stream); return stream;
    }
    navigator.mediaDevices.getDisplayMedia = async options => {
      recordingTest.calls.push({method:'display',options});
      if (recordingTest.denial === 'display') throw new DOMException('Permission denied','NotAllowedError');
      const stream = screen('#314e93');
      if (recordingTest.delay) return new Promise(resolve => { recordingTest.resolve = () => resolve(stream); });
      return stream;
    };
    navigator.mediaDevices.getUserMedia = async options => {
      const kind = options.audio ? 'microphone' : 'camera'; recordingTest.calls.push({method:kind,options});
      if (recordingTest.denial === kind) throw new DOMException('Permission denied','NotAllowedError');
      if (options.video) return screen('#2fa478', true);
      const audio = new AudioContext(), oscillator = audio.createOscillator(), destination = audio.createMediaStreamDestination();
      oscillator.frequency.value = 440; oscillator.connect(destination); oscillator.start();
      recordingTest.contexts.push(audio); recordingTest.streams.push(destination.stream); return destination.stream;
    };
  });
}

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const fixture = fs.mkdtempSync(path.resolve('testdata/recording-browser-'));
  fs.writeFileSync(path.join(fixture,'deck.md'), '---\ntitle: Local recording\ntheme: swiss\noffline-required: true\ntransition: none\n---\n\n```yaml\nid: opening\ncaption: Explicit first caption\n```\n\n# First slide\n\nLive content.\n\n<!-- Private speaker note -->\n\n---\n\n```yaml\nid: second\ncaption: Explicit second caption\n```\n\n# Second slide\n\nAnother explanation.\n');
  const reserve = http.createServer(); await new Promise(resolve=>reserve.listen(0,'127.0.0.1',resolve));
  const port = reserve.address().port; await new Promise(resolve=>reserve.close(resolve));
  const server = spawn(binary,['serve',fixture,'--port',String(port)],{stdio:['ignore','ignore','inherit']});
  let browser;
  try {
    const url = 'http://127.0.0.1:'+port+'/';
    for(let i=0;;i++) {
      try {if ((await fetch(url)).ok) break;} catch (_) {}
      if (i>200 || server.exitCode!==null) throw Error('Recording server did not start');
      await new Promise(resolve=>setTimeout(resolve,100));
    }
    browser = await chromium.launch({args:['--no-sandbox'],...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{})});
    const page = await browser.newPage({viewport:{width:1280,height:720}}); await fakeCapture(page);
    const errors=[];page.on('pageerror',error=>errors.push(error.message));
    await page.goto(url);await page.waitForFunction(()=>window.SlidesRecording?.supported);
    await page.getByRole('button',{name:'Record presentation',exact:true}).click();
    await page.getByLabel('Include microphone').check();await page.getByLabel('Include a camera inset').check();
    await page.screenshot({path:'/tmp/slides-recording-desktop.png'});
    await page.getByRole('button',{name:'Start recording',exact:true}).click();
    await page.waitForFunction(()=>SlidesRecording.state().phase==='recording');
    assert.equal(await page.locator('.slides-recording-panel:visible').count(),0);
    await page.waitForTimeout(650);await page.evaluate(()=>SlidesNav.show(1,0,true));await page.waitForTimeout(650);
    await page.getByRole('button',{name:'Stop recording',exact:true}).last().click();
    await page.waitForFunction(()=>SlidesRecording.state().phase==='ready');
    const result=await page.evaluate(async()=>{
      const r=SlidesRecording.result();return {size:r.video.size,header:[...new Uint8Array(await r.video.slice(0,4).arrayBuffer())],timeline:r.timeline,vtt:r.vtt,tracks:recordingTest.streams.flatMap(s=>s.getTracks()).map(t=>t.readyState),calls:recordingTest.calls.map(c=>c.method)};
    });
    assert.ok(result.size>1000);assert.deepEqual(result.header,[26,69,223,163]);
    const decoded=await page.evaluate(async()=>{
      const video=document.createElement('video');video.muted=true;const url=URL.createObjectURL(SlidesRecording.result().video);video.src=url;
      try {
        await video.play();await new Promise(resolve=>video.requestVideoFrameCallback(resolve));
        const canvas=document.createElement('canvas');canvas.width=video.videoWidth;canvas.height=video.videoHeight;
        const context=canvas.getContext('2d');context.drawImage(video,0,0);
        return {width:canvas.width,height:canvas.height,screen:[...context.getImageData(100,100,1,1).data],camera:[...context.getImageData(550,300,1,1).data]};
      } finally {video.pause();video.removeAttribute('src');video.load();URL.revokeObjectURL(url);}
    });
    assert.deepEqual([decoded.width,decoded.height],[640,360]);
    assert.ok(decoded.screen[2]>decoded.screen[0]+60,'Shared-screen pixels did not survive recording');
    assert.ok(decoded.camera[1]>decoded.camera[0]+60,'Camera inset pixels did not survive recording');
    assert.deepEqual(result.calls,['display','microphone','camera']);assert.ok(result.tracks.every(state=>state==='ended'));
    assert.equal(result.timeline.microphone,true);assert.equal(result.timeline.camera,true);assert.equal(result.timeline.navigation.length,2);
    assert.ok(result.timeline.durationMs>=1200);assert.equal(result.timeline.captionKind,'authored');
    assert.match(result.vtt,/Explicit first caption/);assert.match(result.vtt,/Explicit second caption/);
    assert.ok(!result.vtt.includes('First slide')&&!JSON.stringify(result.timeline).includes('Private speaker note'));
    const downloadEvent=page.waitForEvent('download');await page.getByRole('link',{name:'Download recording',exact:true}).click();
    const download=await downloadEvent;assert.match(download.suggestedFilename(),/\.webm$/);assert.equal(await download.failure(),null);
    const firstURL=await page.getByRole('link',{name:'Download recording',exact:true}).getAttribute('href');
    await page.getByRole('button',{name:'Close',exact:true}).click();

    // Device denial tears down the already-granted screen stream and preserves
    // the previous completed download until a later recording replaces it.
    const denied=await page.evaluate(async()=>{
      recordingTest.denial='microphone';try {await SlidesRecording.start({microphone:true});}catch(_) {}
      return {state:SlidesRecording.state(),live:recordingTest.streams.flatMap(s=>s.getTracks()).filter(t=>t.readyState==='live').length};
    });
    assert.equal(denied.state.phase,'error');assert.equal(denied.live,0);assert.match(denied.state.error,/permission was denied/i);
    assert.equal(await page.getByRole('link',{name:'Download recording',exact:true}).getAttribute('href'),firstURL);
    await page.getByRole('button',{name:'Close',exact:true}).click();

    // A delayed browser prompt cannot retain a stream after cancellation.
    await page.evaluate(()=>{recordingTest.denial='';recordingTest.delay=true;window.pendingRecording=SlidesRecording.start().catch(()=>null);});
    await page.waitForFunction(()=>recordingTest.resolve!==null);await page.evaluate(()=>SlidesRecording.stop());
    await page.evaluate(async()=>{recordingTest.resolve();await pendingRecording;recordingTest.delay=false;recordingTest.resolve=null;});
    assert.equal(await page.evaluate(()=>recordingTest.streams.flatMap(s=>s.getTracks()).filter(t=>t.readyState==='live').length),0);
    assert.equal(await page.evaluate(()=>SlidesRecording.state().phase),'idle');

    // Exercise a real automatic MediaRecorder stop and object-URL replacement.
    await page.evaluate(()=>SlidesRecording.start({maxDurationMs:1000}));
    await page.waitForFunction(()=>SlidesRecording.state().phase==='ready');
    assert.ok(await page.evaluate(()=>SlidesRecording.result().video.size)>1000);
    assert.match(await page.locator('.slides-recording-status').textContent(),/duration limit/);
    assert.equal(await page.evaluate(async url=>{try {await fetch(url);return true;}catch(_){return false;}},firstURL),false);
    assert.deepEqual(errors,[]);
    await page.setViewportSize({width:390,height:844});
    await page.screenshot({path:'/tmp/slides-recording-mobile.png'});
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
    await page.evaluate(()=>Promise.all(recordingTest.contexts.map(context=>context.close())));

    // Without authored text, the VTT explicitly identifies navigation labels.
    const fallback=await browser.newPage();await fakeCapture(fallback);
    await fallback.route('**/*',async route=>{
      if(route.request().resourceType()!=='document')return route.continue();
      const response=await route.fetch();const body=(await response.text()).replace(/(<script type="application\/json" id="slides-recording-data">)[\s\S]*?(<\/script>)/,'$1[]$2');
      await route.fulfill({response,body});
    });
    await fallback.goto(url);await fallback.evaluate(()=>SlidesRecording.start({maxDurationMs:1000}));
    await fallback.waitForFunction(()=>SlidesRecording.state().phase==='ready');
    const vtt=await fallback.evaluate(()=>SlidesRecording.result().vtt);assert.match(vtt,/does not transcribe speech/);assert.match(vtt,/Slide [12]/);
    await fallback.close();

    const unsupported=await browser.newPage();await unsupported.addInitScript(()=>{window.MediaRecorder=undefined;});await unsupported.goto(url);
    await unsupported.getByRole('button',{name:'Record presentation',exact:true}).click();
    assert.equal(await unsupported.getByRole('button',{name:'Start recording',exact:true}).isDisabled(),true);
    assert.match(await unsupported.locator('.slides-recording-status').textContent(),/cannot record/);
    const noDevices=await browser.newPage();await fakeCapture(noDevices);await noDevices.addInitScript(()=>{navigator.mediaDevices.getUserMedia=undefined;});await noDevices.goto(url);
    await noDevices.getByRole('button',{name:'Record presentation',exact:true}).click();
    assert.equal(await noDevices.getByLabel('Include microphone').isDisabled(),true);assert.equal(await noDevices.getByLabel('Include a camera inset').isDisabled(),true);
    console.log('Recording browser checks passed: decoded WebM screen/camera pixels with microphone tracks, navigation/authored VTT, downloads, denial/cancellation/automatic stop cleanup, support gating and responsive dialog.');
  } finally {
    if(browser)await browser.close();server.kill('SIGTERM');fs.rmSync(fixture,{recursive:true,force:true});
  }
})().catch(error=>{console.error(error);process.exitCode=1});

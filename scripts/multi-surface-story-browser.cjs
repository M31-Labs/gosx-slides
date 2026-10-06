const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const binary = path.resolve(process.argv[2] || './slides');

async function withServer(deck, port, run) {
  const server = spawn(binary, ['serve',deck,'--port',String(port)], {stdio:['ignore','ignore','inherit']});
  let exited=false; server.once('exit',()=>{exited=true});
  const url='http://127.0.0.1:'+port+'/';
  try {
    for(let attempt=0;;attempt++) {
      if(exited)throw Error('Multi-surface server exited');
      try {if((await fetch(url)).ok)break;}catch(_){}
      if(attempt>=600)throw Error('Multi-surface startup timed out');
      await new Promise(resolve=>setTimeout(resolve,500));
    }
    await run(url);
  }finally{server.kill('SIGTERM')}
}

async function ready(page) {
  await page.waitForFunction(()=>window.SlidesStory && Array.from(document.querySelectorAll('.deck-active .slide-graphic')).every(el=>el.__gosxScene3DHandle?.__gosxScene3DCommandReady),undefined,{timeout:30000});
  await page.evaluate(async()=>{await document.fonts.ready;SlidesMotion.pause();await SlidesMotion.settled()});
}

async function sample(page,ms) {
  return page.evaluate(async ms=>{
    SlidesMotion.seek(ms);await SlidesMotion.settled();
    await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
    const active=document.querySelector('.deck-active'), round=value=>Math.round(value*100)/100;
    const surfaces=Array.from(active.querySelectorAll('[data-mdpp-container="story-surface"]')).map(container=>({
      id:container.id,
      actors:Array.from(container.querySelectorAll('svg [data-sirena-id]')).map(el=>{const rect=el.getBBox();return [el.dataset.sirenaId,el.style.opacity,el.getAttribute('aria-hidden'),round(rect.width),round(rect.height),el.getAttribute('transform')]}),
      geometry:Array.from(container.querySelectorAll('svg rect,svg path,svg circle,svg line')).map(el=>[el.getAttribute('d'),el.getAttribute('width'),el.getAttribute('height'),el.getAttribute('x'),el.getAttribute('y')]),
      scene:Array.from(container.querySelectorAll('.slide-graphic')).map(el=>({camera:__gosx_scene3d_debug.inspect(el.id).camera,labels:Array.from(el.querySelectorAll('.gosx-scene-label')).map(label=>[label.dataset.gosxSceneLabel,label.style.cssText]).sort((a,b)=>a[0].localeCompare(b[0]))}))
    }));
    return {surfaces,caption:document.querySelector('.slides-story-caption').textContent,assertions:SlidesStory.assertCurrent()};
  },ms);
}

(async()=>{
  const browser=await chromium.launch({args:['--enable-unsafe-swiftshader'],...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{})});
  const errors=[],output=process.env.SLIDES_TEST_OUTPUT || 'browser-test-output';fs.mkdirSync(output,{recursive:true});
  const fixture=fs.mkdtempSync(path.resolve('testdata/multi-surface-browser-'));
  try {
    await withServer('examples/multi-surface-story',8173,async url=>{
      const page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',error=>errors.push(error.message));
      await page.goto(url+'#pressure/burst',{waitUntil:'domcontentloaded'});await ready(page);
      let published=0;page.on('request',request=>{if(request.url().endsWith('/presenter/state'))published++});
      assert.equal(await page.evaluate(()=>SlidesMotion.duration()),1200);
      const start=await sample(page,0),middle=await sample(page,600),end=await sample(page,1200);
      assert.deepEqual(end.assertions.errors,[]);
      const surface=(state,name)=>state.surfaces.find(surface=>surface.id.endsWith('-'+name));
      assert.equal(surface(end,'map').actors.find(actor=>actor[0]==='worker')[1],'0');
      assert.equal(surface(end,'load').actors.find(actor=>actor[0]==='worker')[1],'0.2','same actor ID on another SVG surface keeps its own focus');
      assert.notDeepEqual(surface(start,'load').geometry,surface(end,'load').geometry,'authored chart poses change actual geometry');
      assert.equal(surface(end,'runtime').scene[0].camera.z,14);
      assert.ok(Math.abs(surface(middle,'runtime').scene[0].camera.z-(surface(start,'runtime').scene[0].camera.z+14)/2)<.001,'native camera uses shared midpoint');
      assert.deepEqual(await sample(page,600),middle,'reverse seek restores all SVG/native geometry');
      assert.deepEqual(await sample(page,1200),end);
      await page.screenshot({path:path.join(output,'multi-surface-story-burst.png')});
      await page.evaluate(()=>SlidesNav.preview(0,2));await sample(page,1200);
      await page.evaluate(()=>SlidesNav.preview(0,0));const baseline=await sample(page,1200);assert.deepEqual(baseline.assertions.errors,[]);
      await page.evaluate(()=>SlidesNav.preview(0,1));assert.deepEqual(await sample(page,1200),end,'backward navigation resets omitted effects');
      assert.deepEqual(await sample(page,600),middle,'navigated midpoint equals direct-link midpoint');await sample(page,1200);
      assert.equal(published,0,'scrubbing and preview remain local');
      await page.reload({waitUntil:'domcontentloaded'});await ready(page);assert.deepEqual(await sample(page,1200),end,'direct-link reload reconstructs the pose');
      await page.emulateMedia({reducedMotion:'reduce'});assert.deepEqual(await sample(page,0),end,'reduced motion chooses the exact addressed pose');
      await page.close();
    });

    // Two native scenes share one source and actor IDs but have independent poses.
    fs.copyFileSync('examples/multi-surface-story/runtime.sir',path.join(fixture,'runtime.sir'));
    fs.writeFileSync(path.join(fixture,'deck.md'),'---\nstory: story.yaml\noffline-required: true\n---\n\n```yaml\nid: twins\ncues: baseline, split\n```\n\n# Two views, one clock\n\n:::columns\n:::story-surface {name=left}\n<Scene3D Src="runtime.sir" />\n:::\n:::story-surface {name=right}\n<Scene3D Src="runtime.sir" />\n:::\n:::\n');
    fs.writeFileSync(path.join(fixture,'deck.css'),'[data-mdpp-container="columns"]{display:grid;grid-template-columns:1fr 1fr;gap:2rem}.slide-graphic{height:28rem}');
    fs.writeFileSync(path.join(fixture,'story.yaml'),'version: 1\nbeats:\n  - slide: twins\n    cue: split\n    caption: Different views keep independent actors and cameras.\n    durationMs: 600\n    surfaces:\n      left: {camera: {x: 0, y: 0, z: 8, fov: 45}, reveal: [api]}\n      right: {camera: {x: 0, y: 0, z: 16, fov: 50}, reveal: [worker]}\n    expect:\n      visible: [left/api, right/worker]\n      hidden: [left/worker, right/api]\n');
    if(fs.existsSync('examples/multi-surface-story/build'))fs.cpSync('examples/multi-surface-story/build',path.join(fixture,'build'),{recursive:true});
    await withServer(fixture,8174,async url=>{
      const page=await browser.newPage({viewport:{width:1440,height:1000}});page.on('pageerror',error=>errors.push(error.message));
      await page.goto(url+'#twins/split',{waitUntil:'domcontentloaded'});await ready(page);
      const middle=await sample(page,300),end=await sample(page,600);
      assert.deepEqual(end.surfaces.map(surface=>surface.scene[0].camera.z),[8,16]);assert.deepEqual(end.assertions.errors,[]);
      await sample(page,0);assert.deepEqual(await sample(page,300),middle);assert.deepEqual(await sample(page,600),end);
      await page.close();
    });
    assert.deepEqual(errors,[]);console.log('Multi-surface browser passed: qualified SVG/native actors, chart geometry, same-source scenes, shared midpoint/reverse, direct links and reduced motion.');
  }finally{await browser.close();fs.rmSync(fixture,{recursive:true,force:true})}
})().catch(error=>{console.error(error);process.exitCode=1});

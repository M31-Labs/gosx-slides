const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const {PNG} = require('pngjs');

(async () => {
  const browser = await chromium.launch({args:['--enable-unsafe-swiftshader'],
    ...(process.env.SLIDES_BROWSER ? {executablePath:process.env.SLIDES_BROWSER} : {})});
  try {
    const page = await browser.newPage({viewport:{width:1440,height:900}}), errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.goto(process.argv[2] + '#request/accepted', {waitUntil:'domcontentloaded'});
    await page.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady === 'true');
    if(process.env.SLIDES_SCENE_RENDERER) {
      assert.equal(await page.evaluate(()=>__gosx_scene3d_debug.inspect(document.querySelector('.deck-active .slide-graphic').id).renderer),process.env.SLIDES_SCENE_RENDERER,'the requested GPU backend must actually render');
    }
    await page.evaluate(async () => {
      SlidesMotion.pause(); await SlidesMotion.settled();
      const handle = document.querySelector('.deck-active .slide-graphic').__gosxScene3DHandle;
      const apply = handle.applyCommands.bind(handle);
      window.storyCommands = new Map(); window.storyBatches = 0;
      handle.applyCommands = async commands => {
        await apply(commands); window.storyBatches++;
        commands.forEach(c => storyCommands.set(c.kind + ':' + (c.objectId || ''), JSON.parse(JSON.stringify(c))));
      };
    });
    const sample = ms => page.evaluate(async ms => {
      SlidesMotion.seek(ms); await SlidesMotion.settled();
      const mount = document.querySelector('.deck-active .slide-graphic');
      const debug = __gosx_scene3d_debug.inspect(mount.id);
      return {clock:mount.__gosxScene3DHandle.getAnimationClock(), camera:debug.camera,
        counts:debug.counts, commands:Array.from(storyCommands.entries()).sort((a,b)=>a[0].localeCompare(b[0])),
        labels:Array.from(mount.querySelectorAll('.gosx-scene-label')).map(e=>[e.dataset.gosxSceneLabel,e.style.cssText]),
        code:Array.from(document.querySelectorAll('.deck-active .slides-code-morph pre:not([hidden]) .ts-line')).map(e=>[getComputedStyle(e).opacity,getComputedStyle(e).transform])};
    }, ms);
    assert.equal(await page.evaluate(()=>SlidesMotion.duration()),1200);
    await sample(0); const middle = await sample(600), final = await sample(1200);
    assert.equal(middle.clock.timeSeconds,.6); assert.equal(middle.clock.paused,true);
    assert.equal(middle.camera.z,10); assert.equal(final.camera.z,9);
    const command = (pose,key) => pose.commands.find(([k])=>k===key)[1];
    assert.equal(command(middle,'2:api').data.z,.6);
    assert.equal(command(final,'2:api').data.z,1.2);
    // Object counts and successful draw calls do not prove visible geometry.
    // Pearl's authored violet/mint range is distinct from the other actors.
    const pixels = PNG.sync.read(await page.locator('.deck-active .slide-graphic canvas').first().screenshot());
    let pearlPixels = 0;
    for(let i=0;i<pixels.data.length;i+=4) {
      const [r,g,b,a] = pixels.data.subarray(i,i+4);
      if(a>0&&r>=70&&r<=130&&g>=70&&g<=240&&b>=170&&b<=255) pearlPixels++;
    }
    assert.ok(pearlPixels>=200,`the API shader box must paint visible pixels (${pearlPixels} found)`);
    assert.notDeepEqual(command(middle,'0:edge:0').data.props.points,command(final,'0:edge:0').data.props.points,'routes must follow actors during interpolation');
    assert.ok(middle.code.some(([opacity])=>Number(opacity)>0 && Number(opacity)<1),'code lines share the playhead');
    assert.deepEqual(await sample(600),middle,'repeated seeks must reconstruct the rendered pose');
    const batches = await page.evaluate(()=>storyBatches); await sample(600);
    assert.equal(await page.evaluate(()=>storyBatches),batches,'unchanged samples must not dispatch GPU commands');
    await page.evaluate(()=>SlidesNav.show(1,2,true)); await sample(1200);
    assert.equal(await page.evaluate(()=>location.hash),'#request/failure');
    await page.evaluate(()=>SlidesNav.show(1,0,true)); const reset = await sample(1200);
    assert.equal(reset.camera.z,11); assert.equal(command(reset,'2:worker').data.z,0);
    assert.deepEqual(reset.counts,middle.counts,'seeking must retain scene resource counts');
    // Keep only the authored code group: native entrances must not incidentally
    // resample new code records and hide a paused-navigation ordering bug.
    const codePage = await browser.newPage({viewport:{width:1440,height:900}});
    codePage.on('pageerror',e=>errors.push(e.message));
    await codePage.goto(process.argv[2]+'#request/accepted',{waitUntil:'domcontentloaded'});
    await codePage.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady==='true');
    await codePage.evaluate(async()=>{
      SlidesMotion.pause(); await SlidesMotion.settled();
      const slide=document.querySelector('.deck-active'),group=slide.querySelector('.slides-code-morph');
      slide.replaceChildren(group);
      window.codePose=()=>Array.from(group.querySelectorAll('pre:not([hidden]) .ts-line')).map(e=>[e.textContent,getComputedStyle(e).opacity,getComputedStyle(e).transform]);
      SlidesNav.show(1,0,true); SlidesNav.show(1,1,true);
    });
    const codeSample=ms=>codePage.evaluate(ms=>{SlidesMotion.seek(ms);return codePose()},ms);
    const codeStart=await codeSample(0),codeMiddle=await codeSample(600),codeEnd=await codeSample(1200);
    assert.notDeepEqual(codeStart,codeMiddle);
    assert.notDeepEqual(codeMiddle,codeEnd);
    for(const from of [2,3]) {
      const backward=await codePage.evaluate(async from=>{
        SlidesNav.show(1,from,true); SlidesMotion.seek(1200);
        SlidesNav.show(1,1,true);
        const immediate=codePose(),state=SlidesMotion.state();
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
        return {immediate,painted:codePose(),state};
      },from);
      assert.equal(backward.state.time,1200);
      assert.deepEqual(backward.immediate,codeEnd,'paused backward navigation must select the final code pose before incidental sampling');
      assert.deepEqual(backward.painted,codeEnd,'the first painted backward destination must remain settled');
      assert.deepEqual(await codeSample(600),codeMiddle,'step1 midpoint must not depend on the previous authored block');
      assert.deepEqual(await codeSample(0),codeStart,'step1 source must remain the canonical authored block0');
    }
    await codeSample(1200);
    await codePage.evaluate(()=>SlidesMotion.reverse());
    await codePage.waitForFunction(()=>SlidesMotion.state().time<1000&&SlidesMotion.state().time>0);
    const reversePose=await codePage.evaluate(()=>{SlidesMotion.pause();return {state:SlidesMotion.state(),pose:codePose()}});
    assert.equal(reversePose.state.direction,-1);
    assert.deepEqual(await codeSample(reversePose.state.time),reversePose.pose,'reverse transport must sample the same authored segment as seek');
    assert.deepEqual(await codeSample(600),codeMiddle);
    await codePage.close();
    const alignment = async target => {
      await target.evaluate(async()=>{
        await SlidesMotion.settled();
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
      });
      const bounds = await target.evaluate(()=>{
        const mount=document.querySelector('.deck-active .slide-graphic');
        const canvas=mount.querySelector('canvas'),layer=mount.querySelector('.gosx-scene-label').parentElement;
        const box=element=>{const r=element.getBoundingClientRect();return {left:r.left,top:r.top,right:r.right,bottom:r.bottom,width:r.width,height:r.height}};
        return {mount:box(mount),canvas:box(canvas),layer:box(layer)};
      });
      for(const key of ['left','top','width','height']) {
        assert.ok(Math.abs(bounds.canvas[key]-bounds.layer[key])<=1,`scene label plane must match canvas ${key}: ${JSON.stringify(bounds)}`);
      }
      for(const key of ['left','top']) assert.ok(bounds.canvas[key]>=bounds.mount[key]-1,`canvas ${key} must stay inside its mount`);
      for(const key of ['right','bottom']) assert.ok(bounds.canvas[key]<=bounds.mount[key]+1,`canvas ${key} must stay inside its mount`);
    };
    for(const viewport of [{width:1440,height:900},{width:1280,height:720},{width:1440,height:900}]) {
      await page.setViewportSize(viewport);
      for(const step of [0,1,2,3]) {
        await page.evaluate(async step=>{SlidesNav.show(1,step,true);SlidesMotion.seek(1200);await SlidesMotion.settled()},step);
        await alignment(page);
      }
    }
    await page.evaluate(()=>SlidesNav.show(1,1,true));
    assert.deepEqual((await sample(600)).code,middle.code,'resize and cue changes must preserve the canonical code midpoint');
    const mobile = await browser.newPage({viewport:{width:390,height:844},reducedMotion:'reduce'});
    mobile.on('pageerror',e=>errors.push(e.message));
    await mobile.goto(process.argv[2]+'#request/accepted',{waitUntil:'domcontentloaded'});
    await mobile.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady==='true');
    await mobile.evaluate(async()=>{SlidesMotion.seek(600);await SlidesMotion.settled()});
    assert.equal(await mobile.evaluate(()=>__gosx_scene3d_debug.inspect(document.querySelector('.deck-active .slide-graphic').id).camera.z),9,'reduced motion settles the destination immediately');
    assert.equal(await mobile.evaluate(()=>document.querySelector('.deck-active .slide-graphic').__gosxScene3DHandle.getAnimationClock().timeSeconds),0);
    assert.equal(await mobile.evaluate(()=>document.querySelector('.deck-active').scrollHeight>document.querySelector('.deck-active').clientHeight),false,'mobile slide must fit');
    assert.equal(await mobile.locator('.deck-active .path').textContent(),'Browser → API → Queue → Worker → Database');
    await alignment(mobile);
    assert.deepEqual(errors,[]);
    if(process.env.SLIDES_SCREENSHOT_DIR) {
      fs.mkdirSync(process.env.SLIDES_SCREENSHOT_DIR,{recursive:true});
      await page.evaluate(()=>SlidesNav.show(1,1,true)); await sample(600);
      await page.screenshot({path:process.env.SLIDES_SCREENSHOT_DIR+'/request-desktop.png'});
      await mobile.screenshot({path:process.env.SLIDES_SCREENSHOT_DIR+'/request-mobile.png'});
    }
    console.log('PASS shared camera/geometry/labels/code/shader clock, canonical code pairs and paused backward first paint, repeatable seeks/reverse, retained resources, named cues, fitted canvas/label alignment across resize and reduced-motion mobile');
  } finally {await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});

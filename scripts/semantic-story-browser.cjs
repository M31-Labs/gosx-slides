const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const binary = path.resolve(process.argv[2] || './slides');

async function captionFits(page) {
  await page.waitForFunction(() => {
    const caption=document.querySelector('.slides-story-caption'),controls=document.querySelector('.deck-controls');
    if(!caption || caption.hidden)return false;
    const box=caption.getBoundingClientRect(),toolbar=controls.getBoundingClientRect();
    const content=Array.from(document.querySelectorAll('.deck-active h1,.deck-active h2,.deck-active h3,.deck-active p,.deck-active pre,.deck-active figure,.deck-active .slide-graphic,.deck-active .mdpp-diagram')).filter(el=>!el.closest('.slide-notes')&&el.getBoundingClientRect().height>0);
    const bottom=Math.max(0,...content.map(el=>el.getBoundingClientRect().bottom));
    return box.left>=0 && box.right<=innerWidth && Math.abs((box.left+box.right)/2-innerWidth/2)<1 && box.bottom+8<=toolbar.top && bottom+8<=box.top;
  },undefined,{timeout:5000});
}

async function withServer(deck, port, run) {
  const server = spawn(binary, ['serve', deck, '--port', String(port)], { stdio: ['ignore', 'ignore', 'inherit'] });
  let exited = false; server.once('exit', () => { exited = true; });
  const url = 'http://127.0.0.1:' + port + '/';
  try {
    for (let attempt = 0; ; attempt++) {
      if (exited) throw Error('Story server exited');
      try { if ((await fetch(url)).ok) break; } catch (_) {}
      if (attempt >= 600) throw Error('Story startup timed out');
      await new Promise(resolve => setTimeout(resolve, 500));
    }
    await run(url);
  } finally { server.kill('SIGTERM'); }
}

(async () => {
  const browser = await chromium.launch({ args: ['--enable-unsafe-swiftshader'], ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}) });
  const fixture = fs.mkdtempSync(path.resolve('testdata/semantic-story-browser-'));
  const output = process.env.SLIDES_TEST_OUTPUT || 'browser-test-output'; fs.mkdirSync(output, { recursive: true });
  try {
    const original = fs.readFileSync('examples/semantic-story/deck.md', 'utf8');
    const graph = fs.readFileSync('examples/semantic-story/request.sir', 'utf8');
    const manifest = fs.readFileSync('examples/semantic-story/story.yaml', 'utf8');
    fs.writeFileSync(path.join(fixture, 'deck.md'), original.replace('<Scene3D Src="request.sir" />', '```sirena\n' + graph + '```'));
    fs.writeFileSync(path.join(fixture, 'story.yaml'), manifest.replace(/^    camera:.*\n/gm, ''));
    // Reuse staged test runtime when present; production does not need Node.
    if (fs.existsSync('examples/semantic-story/build')) fs.cpSync('examples/semantic-story/build', path.join(fixture, 'build'), { recursive: true });
    const errors = [];
    await withServer('examples/semantic-story', 8153, async url => {
      const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(url + '#request/accepted', { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady === 'true');
      await page.evaluate(async () => { SlidesMotion.pause(); await SlidesMotion.settled(); });
      const sample = ms => page.evaluate(async ms => {
        SlidesMotion.seek(ms); await SlidesMotion.settled();
        const mount = document.querySelector('.deck-active .slide-graphic');
        return { camera: __gosx_scene3d_debug.inspect(mount.id).camera,
          labels: Array.from(mount.querySelectorAll('.gosx-scene-label')).map(el => [el.dataset.gosxSceneLabel, el.style.cssText]).sort((a, b) => a[0].localeCompare(b[0])),
          code: Array.from(document.querySelectorAll('.deck-active pre.code-block .ts-line')).map(el => [el.dataset.storyCode, el.style.opacity]),
          caption: document.querySelector('.slides-story-caption').textContent,
          assertion: SlidesStory.assertCurrent() };
      }, ms);
      assert.equal(await page.evaluate(() => SlidesMotion.duration()), 1200);
      const start = await sample(0), middle = await sample(600), final = await sample(1200);
      assert.notEqual(start.camera.z, final.camera.z, 'semantic camera must lower to scene commands');
      assert.ok(Math.abs(middle.camera.z - (start.camera.z + final.camera.z) / 2) < .001, 'shared playhead interpolates native camera');
      assert.equal(final.camera.z, 10);
      assert.deepEqual(final.assertion.errors, []);
      assert.deepEqual(final.code.map(row => row[0]), ['true', 'true', 'false']);
      assert.match(final.caption, /API accepts/);
      assert.deepEqual(await sample(600), middle, 'native backward seek reconstructs absolute state');
      await page.evaluate(() => SlidesNav.show(1, 2, true));
      const completed = await sample(900);
      assert.equal(completed.camera.z, 12);
      assert.deepEqual(completed.assertion.errors, []);
      assert.equal(await page.locator('[data-story-id="completion"]').isVisible(), true);
      await captionFits(page);
      await page.screenshot({ path: path.join(output, 'semantic-story-native.png') });
      await page.evaluate(() => SlidesNav.show(1, 0, true));
      const overview = await sample(0);
      assert.deepEqual(overview.assertion.errors, []);
      assert.equal(await page.locator('[data-story-id="completion"]').isVisible(), false);
      await page.evaluate(() => SlidesNav.show(1, 1, true));
      assert.deepEqual(await sample(1200), final, 'backward navigation clears previous narrative effects');
      await page.close();
    });
    await withServer(fixture, 8154, async url => {
      const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(url + '#request/accepted', { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.SlidesStory);
      await page.evaluate(() => SlidesMotion.pause());
      let published = 0; page.on('request', request => { if (request.url().endsWith('/presenter/state')) published++; });
      const sample = ms => page.evaluate(ms => {
        SlidesMotion.seek(ms);
        return { actors: Array.from(document.querySelectorAll('.deck-active svg [data-sirena-id]')).map(el => [el.dataset.sirenaId, el.style.opacity, el.getAttribute('aria-hidden')]),
          edges: Array.from(document.querySelectorAll('.deck-active svg .edge')).map(el => [el.dataset.storyTrace, el.style.opacity]),
          assertion: SlidesStory.assertCurrent() };
      }, ms);
      const start = await sample(0), middle = await sample(600), final = await sample(1200);
      const opacity = (pose, id) => Number(pose.actors.find(row => row[0] === id)[1]);
      assert.equal(opacity(start, 'db'), 1);
      assert.ok(opacity(middle, 'db') > 0 && opacity(middle, 'db') < 1);
      assert.equal(opacity(final, 'db'), 0);
      assert.equal(opacity(final, 'api'), 1);
      assert.equal(opacity(final, 'worker'), .2);
      assert.ok(final.edges.some(row => row[0] === 'true' && Number(row[1]) === 1), 'semantic trace highlights relationship paths');
      assert.deepEqual(final.assertion.errors, []);
      assert.deepEqual(await sample(600), middle, 'SVG seek is deterministic');
      assert.equal(published, 0, 'asserting and seeking do not publish audience navigation');
      await page.evaluate(() => SlidesNav.show(1, 2, true)); await sample(900);
      assert.equal(await page.locator('[data-story-id="completion"]').isVisible(), true);
      await page.evaluate(() => SlidesNav.show(1, 1, true));
      assert.deepEqual(await sample(1200), final, 'SVG backward cue reconstructs canonical pose');
      await page.emulateMedia({ reducedMotion: 'reduce' });
      assert.deepEqual(await sample(0), final, 'reduced motion settles the absolute semantic pose');
      await page.setViewportSize({ width: 390, height: 844 });
      await page.keyboard.press('ArrowRight');
      assert.equal(await page.evaluate(() => SlidesStory.current().cue), 'completed', 'keyboard uses named story cues');
      const bounds = await page.locator('.slides-story-caption').boundingBox();
      assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= 390 && bounds.y + bounds.height < 844, 'mobile captions fit the viewport');
      assert.deepEqual(await page.evaluate(() => SlidesStory.assertCurrent().errors), []);
      await captionFits(page);
      await page.screenshot({ path: path.join(output, 'semantic-story-mobile.png') });
      await page.setViewportSize({width:320,height:720});
      await captionFits(page);
      await page.screenshot({path:path.join(output,'semantic-story-mobile-320.png')});
      await page.evaluate(()=>{SlidesStory.current().caption+=' More detail about this narrated beat.'.repeat(80);SlidesStory.seek(900);});
      await captionFits(page);
      const cueBeforeScroll=await page.evaluate(()=>SlidesStory.current().cue);
      await page.locator('.slides-story-caption').focus();
      await page.keyboard.press('ArrowDown');
      await page.waitForFunction(()=>document.querySelector('.slides-story-caption').scrollTop>0);
      assert.equal(await page.evaluate(()=>SlidesStory.current().cue),cueBeforeScroll,'scrolling a long caption does not advance cues');
      await page.emulateMedia({media:'print'});
      assert.equal(await page.locator('.slides-story-caption').isVisible(),false,'print hides captions');
      await page.emulateMedia({media:'screen'});
      await page.evaluate(()=>document.querySelector('main.deck').classList.add('deck-reading'));
      assert.equal(await page.locator('.slides-story-caption').isVisible(),false,'reading hides captions');
      await page.close();
    });
    assert.deepEqual(errors, []);
    console.log('Semantic story browser passed: native/SVG absolute poses, trace/reveal, camera/code/DOM/captions, assertions, reverse seeks, reduced motion, mobile.');
  } finally { await browser.close(); fs.rmSync(fixture, { recursive: true, force: true }); }
})().catch(error => { console.error(error); process.exitCode = 1; });

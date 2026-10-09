const {spawn, spawnSync} = require('node:child_process');
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict');
const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const binary = path.resolve(process.argv[2] || './slides');
const dir = fs.mkdtempSync(path.resolve('testdata/presenter-cues-'));
const out = process.env.SLIDES_TEST_OUTPUT || 'browser-test-output';
fs.mkdirSync(out, {recursive: true});
async function assertOrderedBurst(driver, receivers, advance, expected) {
  for (const page of receivers) await page.evaluate(() => {
    if (!window.cueTrace) document.querySelector('main.deck').addEventListener('slides:change', event => cueTrace.push([event.detail.index,event.detail.step]));
    window.cueTrace = [];
  });
  let release, held = false;
  const gate = new Promise(resolve => { release = resolve; });
  const route = async route => {
    if (!held && route.request().method() === 'POST') { held = true; await gate; }
    await route.continue();
  };
  await driver.route('**/presenter/state',route);
  try {
    const firstPost = driver.waitForRequest(request => request.url().endsWith('/presenter/state') && request.method()==='POST');
    // Hold the first POST while the remaining clicks queue, reproducing network latency.
    await advance();
    await firstPost;
    for (const page of receivers) assert.deepEqual(await page.evaluate(()=>cueTrace),[]);
    release();
    for (const page of receivers) {
      await page.waitForFunction(count=>cueTrace.length>=count,expected.length);
      assert.deepEqual(await page.evaluate(()=>cueTrace),expected,'every cue must arrive in order');
    }
  } finally { release(); await driver.unroute('**/presenter/state',route); }
}
fs.copyFileSync('examples/storytelling-lab/plot.scene.json', path.join(dir, 'plot.scene.json'));
fs.writeFileSync(path.join(dir, 'deck.md'), `---
title: Every beat, from the presenter
theme: aurora
offline-required: true
transition: fade
transition-duration: 800
---

\`\`\`yaml
id: opening
cues: start, explain, finish
\`\`\`

# One click, one beat

:::motion {preset=slide-up replay=step duration=800}
The presenter and audience stay together.
:::

:::motion {cue=explain duration=2000}
## Explain the idea
:::

:::motion {cue=finish duration=800}
## Then show the result
:::

<!-- These notes stay with the speaker. -->

---

\`\`\`yaml
id: list
reveal: true
\`\`\`

# Build the explanation

- Start with a question.
- Add the evidence.
- Reach a conclusion.

:::code-morph

\`\`\`go {1|2|3|4|5}
first()
second()
third()
fourth()
fifth()
\`\`\`

\`\`\`go
finished()
\`\`\`

:::

---

\`\`\`yaml
id: native
\`\`\`

# Step through the scene

<Scene3D Src="plot.scene.json" Label="A three-dimensional data story" />
`);
const server = spawn(binary, ['serve', dir, '--port', '8139'], {stdio: ['ignore', 'ignore', 'inherit']});
let browser;
(async () => {
  const url = 'http://127.0.0.1:8139/';
  for (let i=0;;i++) {
    try {if ((await fetch(url)).ok) break;} catch (_) {}
    if (server.exitCode !== null || i>240) throw Error('Presenter fixture did not start');
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  browser = await chromium.launch({args:['--enable-unsafe-swiftshader'], ...(process.env.SLIDES_BROWSER ? {executablePath:process.env.SLIDES_BROWSER} : {})});
  const errors = [];
  // Different contexts cannot communicate over BroadcastChannel: this verifies SSE too.
  const audience = await browser.newPage({viewport:{width:1280,height:720}});
  const presenter = await browser.newPage({viewport:{width:1440,height:900}});
  for (const page of [audience,presenter]) page.on('pageerror', error => errors.push(error.message));
  await audience.goto(url+'#opening/start');
  await presenter.goto(url+'?present#opening/start');
  for (const page of [audience,presenter]) await page.waitForFunction(() => window.SlidesMotion && window.__gosx?.ready);
  const prev = presenter.getByRole('button',{name:'Previous slide',exact:true});
  const next = presenter.getByRole('button',{name:'Next slide',exact:true});
  assert.equal(await prev.isDisabled(), true);
  assert.equal(await next.isEnabled(), true);
  await presenter.evaluate(() => {
    window.previewMoves = 0;
    new MutationObserver(records => { previewMoves += records.filter(r=>r.type==='childList').length; }).observe(document.querySelector('.pv-current .pv-screen'),{childList:true});
  });
  await next.click();
  for (const page of [audience,presenter]) await page.waitForFunction(() => SlidesNav.current()===1 && SlidesNav.step()===1 && SlidesMotion.state().time>100);
  assert.equal(await prev.isEnabled(), true, 'first-slide cues can go backward');
  assert.equal(await presenter.evaluate(()=>previewMoves), 0, 'cue change must not detach the current preview');
  const before = await presenter.locator('[data-slides-motion-cue="explain"]').evaluate(el=>Number(getComputedStyle(el).opacity));
  await presenter.waitForFunction(before => Number(getComputedStyle(document.querySelector('[data-slides-motion-cue="explain"]')).opacity)>before+.05, before);
  await presenter.waitForFunction(()=>Number(getComputedStyle(document.querySelector('[data-slides-motion-cue="explain"]')).opacity)>.99);
  await presenter.screenshot({path:path.join(out,'presenter-cues.png')});
  await next.click();
  await audience.waitForFunction(()=>SlidesNav.step()===2);
  await prev.click();
  await audience.waitForFunction(()=>SlidesNav.step()===1);
  await prev.click();
  await audience.waitForFunction(()=>SlidesNav.step()===0);
  assert.equal(await prev.isDisabled(),true);
  await assertOrderedBurst(presenter,[audience],()=>next.evaluate(button=>{button.click();button.click();button.click();}),[[0,1],[0,2],[1,0]]);
  await presenter.evaluate(()=>SlidesNav.show(2,0,true));
  for (const page of [audience,presenter]) await page.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady==='true');
  assert.equal(await next.isEnabled(),true,'last-slide native cues remain reachable');
  await presenter.evaluate(()=>{window.nativeHandle=document.querySelector('.deck-active .slide-graphic').__gosxScene3DHandle;window.nativeSlide=document.querySelector('.deck-active');});
  await next.click();
  for (const page of [audience,presenter]) await page.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.appliedStep==='1');
  assert.equal(await presenter.evaluate(()=>nativeSlide===document.querySelector('.deck-active') && nativeHandle===document.querySelector('.deck-active .slide-graphic').__gosxScene3DHandle),true);
  const budgets = await presenter.evaluate(() => Array.from(document.querySelectorAll('section.slide')).sort((a,b)=>a.dataset.slide-b.dataset.slide).map((_,i)=>{SlidesNav.show(i,0,true);return SlidesNav.stepCount();}));
  const remote = await browser.newPage({viewport:{width:390,height:844}});
  remote.on('pageerror', error => errors.push(error.message));
  await remote.goto(url+'remote');
  assert.deepEqual(await remote.evaluate(()=>window.budgets),budgets,'remote budgets match the live deck');
  await remote.locator('#n').fill('1');
  await remote.locator('form button').click();
  await audience.waitForFunction(()=>SlidesNav.current()===1 && SlidesNav.step()===0);
  await remote.locator('#next').click();
  for (const page of [audience,presenter]) await page.waitForFunction(()=>SlidesNav.current()===1 && SlidesNav.step()===1);
  await assertOrderedBurst(remote,[audience,presenter],()=>remote.evaluate(()=>{move(1);move(1);move(1);}),[[0,2],[1,0],[1,1]]);
  assert.deepEqual(await remote.evaluate(()=>[cur,step]),[1,1],'delayed self-echoes must not roll the remote back');
  await remote.locator('#prev').click();
  await audience.waitForFunction(()=>SlidesNav.current()===2 && SlidesNav.step()===0);
  await remote.locator('#prev').click();
  await audience.waitForFunction(()=>SlidesNav.current()===1 && SlidesNav.step()===2);
  await remote.locator('#n').fill('3');
  await remote.locator('form button').click();
  await audience.waitForFunction(()=>SlidesNav.current()===3 && SlidesNav.step()===0);
  for (let step=1;step<=budgets[2];step++) {
    await remote.locator('#next').click();
    await audience.waitForFunction(step=>SlidesNav.step()===step,step);
  }
  assert.equal(await remote.locator('#next').isDisabled(),true);
  assert.equal(await next.isDisabled(),true);
  await presenter.evaluate(async()=>{SlidesMotion.seek(SlidesMotion.duration());await SlidesMotion.settled();});
  await presenter.screenshot({path:path.join(out,'presenter-live-scene.png')});
  await remote.screenshot({path:path.join(out,'presenter-remote.png')});
  const reduced = await browser.newPage({reducedMotion:'reduce'});
  await reduced.goto(url+'?present#opening/start');
  await reduced.waitForFunction(()=>window.SlidesMotion);
  await reduced.getByRole('button',{name:'Next slide',exact:true}).click();
  assert.equal(await reduced.evaluate(()=>getComputedStyle(document.querySelector('.pv-current .slide')).animationName),'none');
  assert.deepEqual(errors,[]);
  const result=spawnSync(binary,['export',dir,'--format','pdf','--steps','--pdf-navigation','--out',path.join(out,'presenter-states.pdf')],{stdio:'inherit',env:process.env});
  assert.equal(result.status,0,'clickable state PDF export failed');
  const pdf=fs.readFileSync(path.join(out,'presenter-states.pdf')).toString('latin1');
  const states=budgets.reduce((sum,n)=>sum+n+1,0);
  assert.equal((pdf.match(/\/Type\s*\/Page\b/g)||[]).length,states);
  assert.equal((pdf.match(/\/Subtype\s*\/Link\b/g)||[]).length,2*(states-1));
  assert.ok(pdf.includes('capture-1'),'PDF retains named internal destinations');
  console.log('PASS presenter/remote ordered cue delivery, boundaries, live motion, persistent Scene3D, SSE, reduced motion and '+states+' clickable PDF states');
})().catch(error=>{console.error(error);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();server.kill('SIGTERM');fs.rmSync(dir,{recursive:true,force:true});});

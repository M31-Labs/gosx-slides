const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({ ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}), args: ['--enable-unsafe-swiftshader'] });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    await page.addInitScript(() => {
      window.slidesKeyTrace = [];
      document.addEventListener('keydown', event => {
        const target = event.target;
        queueMicrotask(() => { slidesKeyTrace.push({ key: event.key, tag: target.tagName, type: target.type, connected: target.isConnected,
          visible: !!target.getClientRects().length, dialog: !!target.closest('dialog[open]'), prevented: event.defaultPrevented,
          hash: location.hash }); if (slidesKeyTrace.length > 12) slidesKeyTrace.shift(); });
      }, true);
    });
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    const url = process.argv[2] || 'http://127.0.0.1:8110/';
    await page.goto(url+'#opening', { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.__gosx && __gosx.ready && window.SlidesRuntime);
    assert.deepEqual(await page.evaluate(() => SlidesRuntime.stats()), { total: 3, deferred: 3, hydrated: 0 });
    await page.evaluate(() => SlidesNav.show(2, 0, true));
    await page.waitForFunction(() => SlidesRuntime.stats().hydrated === 2);
    assert.equal(await page.evaluate(() => SlidesRuntime.stats().deferred), 1);
    await page.locator('.deck-active .counter-btn').last().click();
    await page.waitForFunction(() => document.querySelector('.deck-active .counter-label').textContent.includes('1'));
    await page.evaluate(() => { window.counterIdentity = document.querySelector('.deck-active [data-gosx-island]'); SlidesNav.show(1,2,true); });
    assert.ok(page.url().endsWith('#pipeline/worker'));
    const visible = await page.locator('.deck-active [data-slides-cue-visible="true"]').count(); assert.equal(visible, 3);
    await page.evaluate(() => { SlidesMotion.replay(); SlidesMotion.pause(); });
    await page.evaluate(() => SlidesMotion.seek(150));
    assert.equal(await page.evaluate(() => SlidesMotion.state().paused), true);
    assert.equal(await page.evaluate(() => Math.round(SlidesMotion.state().time)), 150);
    await page.evaluate(() => SlidesNav.prev());
    assert.equal(await page.locator('.deck-active [data-slides-motion-cue="worker"]').first().getAttribute('aria-hidden'), 'true');
    await page.evaluate(() => SlidesNav.show(2,0,true));
    assert.equal(await page.evaluate(() => counterIdentity === document.querySelector('.deck-active [data-gosx-island]')), true);
    assert.ok((await page.locator('.deck-active .counter-label').textContent()).includes('1'));
    await page.goto(url + '#pipeline/worker', { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.step() === 2);
    await page.keyboard.press('m');
    assert.equal(await page.locator('dialog:visible').count(), 1);
    await page.locator('[data-motion-element]').selectOption('1');
    const chosen=await page.locator('[data-motion-element]').inputValue();
    await page.keyboard.press('Escape');await page.evaluate(()=>SlidesMotion.open());
    assert.equal(await page.locator('[data-motion-element]').inputValue(),chosen,'reopening keeps the element selection');
    await page.locator('[data-motion-element]').selectOption('0');
    await page.locator('[data-motion-duration]').fill('900');
    await page.locator('[data-motion-duration]').dispatchEvent('change');
    assert.equal(await page.locator('.deck-active [data-slides-motion-cue="request"]').getAttribute('data-gosx-motion-duration'), '900');
    await page.keyboard.press('Escape');
    await page.evaluate(()=>{SlidesNav.show(0,0,true);SlidesMotion.open()});
    assert.equal(await page.locator('[data-motion-empty]').isVisible(),true,'slides without entrances explain the empty state');
    assert.equal(await page.locator('[data-motion-undo]').isDisabled(),true);
    assert.equal(await page.locator('[data-motion-copy]').isDisabled(),true);
    await page.keyboard.press('Escape');
    await page.evaluate(() => SlidesNav.show(5,0,true));
    assert.equal(await page.evaluate(() => SlidesNav.stepCount()), 2);
    assert.equal(await page.locator('.deck-active .slides-code-morph pre:visible').count(), 1);
    // Chromium can deliver a key to the closed editor before restoring focus.
    await page.evaluate(() => document.querySelector('[data-motion-duration]').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',bubbles:true,cancelable:true})));
    assert.equal(await page.evaluate(() => SlidesNav.step()),1);
    await page.evaluate(() => SlidesNav.show(5,0,true));
    await page.keyboard.press('ArrowRight');
    assert.ok((await page.locator('.deck-active .slides-code-morph pre:visible').textContent()).includes('name string'), JSON.stringify(await page.evaluate(()=>({step:SlidesNav.step(),slide:SlidesNav.current(),hash:location.hash,focus:document.activeElement.tagName,focusClass:document.activeElement.className,dialog:!!document.querySelector('dialog[open]'),keys:window.slidesKeyTrace}))));
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(200); // Allow SSE echoes to arrive; they must not revert rapid steps.
    assert.ok((await page.locator('.deck-active .slides-code-morph pre:visible').textContent()).includes('message :='));
    await page.keyboard.press('ArrowLeft');
    assert.ok(!(await page.locator('.deck-active .slides-code-morph pre:visible').textContent()).includes('message :='));
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, hasTouch: true });
    mobile.on('pageerror', error => errors.push(error.message));
    await mobile.goto(url+'#pipeline/worker', { waitUntil: 'domcontentloaded' });
    await mobile.waitForFunction(() => window.SlidesMotion);
    await mobile.evaluate(() => SlidesMotion.open());
    const bounds = await mobile.locator('dialog').boundingBox(); assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= 390);
    const reduced = await browser.newPage({ reducedMotion: 'reduce' });
    reduced.on('pageerror', error => errors.push(error.message));
    await reduced.goto(url+'#pipeline/worker', { waitUntil: 'domcontentloaded' });
    await reduced.waitForFunction(() => window.SlidesMotion);
    assert.equal(await reduced.evaluate(() => document.querySelector('.deck-active').getAnimations({subtree:true}).length), 0);
    assert.deepEqual(errors, []);
    console.log('Authoring browser checks passed: named cues, deferred islands, studio, reverse code steps, mobile, reduced motion.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });

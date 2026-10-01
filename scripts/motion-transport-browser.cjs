const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');

(async () => {
  const browser = await chromium.launch({args: ['--enable-unsafe-swiftshader'],
    ...(process.env.SLIDES_BROWSER ? {executablePath: process.env.SLIDES_BROWSER} : {})});
  try {
    const page = await browser.newPage(), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(process.argv[2], {waitUntil: 'domcontentloaded'});
    await page.waitForFunction(() => window.SlidesMotion && window.SlidesDiagramMotion);

    // Pause before the deferred Scene3D engine mounts, then retain that intent
    // while applying absolute click poses. Native clocks must stop their loop.
    await page.evaluate(() => { SlidesNav.show(6, 0, true); SlidesMotion.pause(); });
    const native = page.locator('.deck-active .slide-graphic');
    await page.waitForFunction(() => {
      const mount = document.querySelector('.deck-active .slide-graphic');
      return mount?.dataset.gosxScene3dReady === 'true' &&
        mount.dataset.gosxScene3dAnimationState === 'paused' && mount.dataset.appliedStep === '0';
    });
    await page.evaluate(() => SlidesNav.next());
    await page.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.appliedStep === '1');
    assert.equal(await page.evaluate(() => SlidesMotion.state().paused), true);
    assert.equal(await native.getAttribute('data-gosx-scene3d-animation-state'), 'paused');
    await page.evaluate(() => SlidesMotion.play());
    await page.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dAnimationState === 'playing');
    await page.evaluate(() => SlidesMotion.seek(400));
    await page.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dRenderLoop === 'stopped');
    const clock = await native.getAttribute('data-gosx-scene3d-animation-clock');
    await page.waitForTimeout(180);
    assert.equal(await native.getAttribute('data-gosx-scene3d-animation-clock'), clock, 'scrubbing must freeze native time');
    await page.evaluate(() => { SlidesMotion.play(); SlidesMotion.reverse(); });
    assert.equal(await native.getAttribute('data-gosx-scene3d-animation-state'), 'paused', 'reverse must freeze unsupported native clocks');

    await page.evaluate(() => { SlidesNav.show(1, 0, true); SlidesMotion.pause(); SlidesNav.next(); });
    assert.equal(await page.evaluate(() => SlidesDiagramMotion.state().paused), true);
    await page.waitForTimeout(180);
    assert.equal(await page.evaluate(() => SlidesDiagramMotion.state().time), 0, 'paused step must not run a new transition');
    await page.evaluate(() => { SlidesMotion.seek(450); SlidesMotion.replay(); });
    await page.waitForTimeout(180);
    assert.equal(await page.evaluate(() => SlidesDiagramMotion.state().time), 0, 'paused replay must remain at its start');
    await page.evaluate(() => SlidesMotion.play());
    await page.waitForFunction(() => SlidesDiagramMotion.state().time > 50);
    await page.evaluate(() => { SlidesMotion.seek(600); SlidesMotion.reverse(); });
    await page.waitForFunction(() => SlidesDiagramMotion.state().time < 550);
    await page.evaluate(() => { SlidesMotion.pause(); SlidesMotion.replay(); SlidesMotion.play(); });
    await page.waitForFunction(() => SlidesDiagramMotion.state().time > 50);
    await page.evaluate(() => SlidesMotion.pause());

    // A same-slide navigation notification must preserve the open editor,
    // draft undo history and pause state rather than behave like a slide entry.
    await page.evaluate(() => SlidesNav.show(0, 0, true));
    await page.keyboard.press('m');
    const duration = page.locator('[data-motion-duration]'), original = await duration.inputValue();
    await duration.fill('1200');
    await duration.dispatchEvent('change');
    await page.evaluate(() => { SlidesMotion.pause(); SlidesNav.show(0, 0, true); });
    assert.equal(await page.locator('dialog[open]').count(), 1);
    assert.equal(await page.evaluate(() => SlidesMotion.state().paused), true);
    await page.locator('[data-motion-undo]').click();
    assert.equal(await duration.inputValue(), original);
    const time = await page.evaluate(() => SlidesMotion.state().time);
    await page.waitForTimeout(180);
    assert.equal(await page.evaluate(() => SlidesMotion.state().time), time, 'paused DOM replay must stay frozen');
    await page.evaluate(() => SlidesNav.show(1, 0, true));
    assert.equal(await page.locator('dialog[open]').count(), 0);
    assert.equal(await page.evaluate(() => SlidesMotion.state().paused), false);
    assert.deepEqual(errors, []);

    const reduced = await browser.newPage({reducedMotion: 'reduce'});
    await reduced.goto(process.argv[2], {waitUntil: 'domcontentloaded'});
    await reduced.waitForFunction(() => window.SlidesMotion && window.SlidesDiagramMotion);
    await reduced.evaluate(() => { SlidesNav.show(1, 0, true); SlidesMotion.pause(); SlidesNav.next(); });
    assert.equal(await reduced.evaluate(() => SlidesDiagramMotion.state().time === SlidesDiagramMotion.duration()), true);
    await reduced.evaluate(() => { SlidesNav.show(6, 0, true); SlidesMotion.pause(); SlidesMotion.play(); });
    await reduced.waitForFunction(() => document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dAnimationState === 'reduced-motion');
    console.log('PASS coordinated pause, deferred GPU mounts, step navigation, SVG/DOM replay, reverse, undo and reduced motion');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });

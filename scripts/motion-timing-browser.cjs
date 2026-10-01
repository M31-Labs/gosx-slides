const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');

(async () => {
  const browser = await chromium.launch({args: ['--enable-unsafe-swiftshader'],
    ...(process.env.SLIDES_BROWSER ? {executablePath: process.env.SLIDES_BROWSER} : {})});
  try {
    const page = await browser.newPage({viewport: {width: 1440, height: 900}}), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(process.argv[2] + '#1/1', {waitUntil: 'domcontentloaded'});
    await page.waitForFunction(() => window.__gosx?.ready && window.SlidesMotion);
    const actual = selector => (typeof selector === 'string' ? page.locator(selector) : selector).evaluate(el => el.getAnimations()[0].effect.getTiming().delay);
    const follower = page.locator('[data-slides-motion-cue="beat"]').first();
    assert.equal(await actual(follower), 150, 'implicit-step forward reference must resolve on first entry');
    assert.equal(await actual(page.locator('[data-slides-motion-group="pair"]').nth(1)), 50);
    assert.equal(await actual('[data-slides-motion-cue="cycle_a"]'), 5);
    assert.equal(await actual('[data-slides-motion-cue="cycle_b"]'), 15);
    assert.equal(await actual('[data-slides-motion-after="cycle_a"]:not([data-slides-motion-cue])'), 65);
    assert.deepEqual(await page.locator('[data-gosx-motion-distance="0"]').evaluate(el => {
      const effect = el.getAnimations()[0].effect, timing = effect.getTiming();
      return {delay: timing.delay, duration: timing.duration, y: new DOMMatrix(effect.getKeyframes()[0].transform).m42};
    }), {delay: 3, duration: 11, y: 0}, 'preview must preserve zero distance and native time rounding');
    assert.ok((await page.locator('[data-slides-motion-cue="cycle_a"]').getAttribute('data-slides-motion-error')).includes('Circular'));
    assert.ok((await page.locator('[data-slides-motion-after="absent"]').getAttribute('data-slides-motion-error')).includes('Unknown'));
    assert.ok((await page.locator('[data-slides-motion-after="other"]').getAttribute('data-slides-motion-error')).includes('another step'));
    await page.keyboard.press('m');
    await page.evaluate(() => SlidesMotion.pause());
    const bars = page.locator('.slides-motion-bar');
    assert.equal(await bars.nth(0).getAttribute('data-motion-start'), '150');
    assert.equal(await bars.nth(0).getAttribute('data-motion-end'), '350');
    assert.equal(await bars.nth(3).getAttribute('data-motion-start'), '50');
    assert.equal(await bars.last().getAttribute('data-motion-end'), '210', 'split text includes every staggered word');
    const split = page.locator('[data-gosx-motion-split="word"]');
    await page.waitForFunction(() => document.querySelector('[data-gosx-motion-split="word"]').getAnimations({subtree: true}).length === 3);
    await page.evaluate(() => SlidesMotion.replay());
    await page.waitForFunction(() => document.querySelector('[data-gosx-motion-split="word"]').getAnimations({subtree: true}).length === 3);
    assert.equal(await split.locator('.gosx-motion-unit').count(), 3, 'Replay preserves native text units');
    await bars.first().press('ArrowRight');
    assert.equal(await page.locator('[data-motion-delay]').inputValue(), '30');
    assert.equal(await bars.first().getAttribute('data-motion-start'), '160');
    assert.equal(await actual(follower), 160, 'preview and track must share timing');
    await page.evaluate(() => { document.querySelector('[data-slides-motion-cue="cycle_a"]').removeAttribute('data-slides-motion-after'); SlidesMotion.replay(); });
    assert.equal(await page.locator('[data-slides-motion-cue="cycle_a"]').getAttribute('data-slides-motion-error'), null);
    assert.equal(await page.locator('[data-slides-motion-cue="cycle_b"]').getAttribute('data-slides-motion-error'), null);
    assert.equal(await page.locator('.slides-motion-warning').count(), 2, 'repaired dependencies clear stale warnings');
    await page.keyboard.press('Escape');
    // A long forward chain used to depend on recursive traversal and repeated
    // linear cue scans. Verify its actual WAAPI delay without a stack overflow.
    const measured = await page.evaluate(() => {
      const count = 6000, slide = document.querySelector('.deck-active'), nodes = [];
      for (let i = 0; i < count; i++) {
        const el = document.createElement('div');
        el.dataset.slidesMotionReplay = 'slide'; el.dataset.slidesMotionStep = '0';
        el.dataset.slidesMotionCue = 'chain_' + i; el.dataset.gosxMotionDuration = '1';
        if (i + 1 < count) el.dataset.slidesMotionAfter = 'chain_' + (i + 1);
        nodes.push(el);
      }
      slide.append(...nodes);
      const start = performance.now(); SlidesMotion.replay();
      return {count, millis: performance.now() - start, delay: nodes[0].getAnimations()[0].effect.getTiming().delay};
    });
    assert.equal(measured.delay, measured.count - 1);
    assert.deepEqual(errors, []);
    const mobile = await browser.newPage({viewport: {width: 390, height: 844}, reducedMotion: 'reduce'});
    await mobile.goto(process.argv[2], {waitUntil: 'domcontentloaded'});
    await mobile.waitForFunction(() => window.SlidesMotion);
    await mobile.evaluate(() => SlidesMotion.open());
    assert.equal(await mobile.evaluate(() => document.querySelector('dialog[open]').scrollWidth <= document.querySelector('dialog[open]').clientWidth), true);
    assert.equal(await mobile.evaluate(() => document.querySelector('.deck-active').getAnimations({subtree:true}).length), 0);
    assert.equal(await mobile.locator('.slides-motion-warning').count(), 4);
    console.log('PASS first-entry forward refs, dependency/stagger tracks, cycles, diagnostics, native split replay, mobile and reduced motion; chain:', JSON.stringify(measured));
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });

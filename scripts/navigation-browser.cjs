// Run against examples/navigation-lab. Playwright is an optional test dependency.
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');

async function check() {
  const browser = await chromium.launch({
    ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}),
    args: ['--enable-unsafe-swiftshader'],
  });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const url = new URL(process.argv[2] || 'http://127.0.0.1:8080/');
    url.hash = '2';
    await page.goto(url.href, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.current() === 2);
    await page.locator('.counter-btn').last().click();
    await page.waitForFunction(() => document.querySelector('.counter-label').textContent.includes('1'));
    await page.locator('.counter-btn').last().focus();
    await page.keyboard.press('Space');
    await page.waitForFunction(() => document.querySelector('.counter-label').textContent.includes('2'));
    assert.equal(await page.evaluate(() => SlidesNav.current()), 2);
    await page.keyboard.press('o');
    assert.equal(await page.locator('.slide:visible').count(), 0);
    assert.equal(await page.locator('.deck-overview-dialog [data-gosx-island], .deck-overview-dialog canvas').count(), 0);
    await page.locator('.deck-overview-search').fill('PRIVATE PRESENTER NOTE');
    assert.equal(await page.locator('.deck-overview-card:visible').count(), 0);
    await page.locator('.deck-overview-search').fill('creme brings');
    assert.equal(await page.locator('.deck-overview-card:visible').count(), 1);
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('.counter-label').textContent(), 'count is 2');

    url.hash = '3/1';
    await page.goto(url.href, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.step() === 1);
    await page.locator('a[href="#3/4"]').click();
    await page.waitForFunction(() => SlidesNav.step() === 4);
    assert.ok((await page.locator('.deck-active .slides-step-active').textContent()).includes('release'));
    await page.goBack();
    await page.waitForFunction(() => SlidesNav.step() === 1);
    await page.keyboard.press('ArrowRight');
    assert.ok(page.url().endsWith('#3/2'));
    await page.reload();
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.step() === 2);

    // Reload leaves focus on the body. Closing search must restore a real focus
    // target, otherwise subsequent Home/arrow keys stay trapped in a hidden input.
    await page.keyboard.press('/');
    await page.locator('.deck-overview-search').fill('5');
    await page.keyboard.press('Enter');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 5);
    await page.keyboard.press('Home');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 1);

    await page.mouse.click(120, 120);
    await page.mouse.move(130, 120);
    await page.waitForTimeout(2800);
    assert.equal(await page.locator('.deck-controls').evaluate(node => getComputedStyle(node).opacity), '0');
    await page.mouse.move(140, 120);
    await page.waitForFunction(() => getComputedStyle(document.querySelector('.deck-controls')).opacity === '1');
    await page.keyboard.press('Tab');
    await page.locator('.deck-controls button').first().focus();
    await page.waitForTimeout(2800);
    assert.equal(await page.locator('.deck-controls').evaluate(node => getComputedStyle(node).opacity), '1');
    assert.deepEqual(errors, []);
    console.log('Navigation browser checks passed: widget state, search, step links, focus, idle controls.');
  } finally {
    await browser.close();
  }
}

check().catch(error => { console.error(error); process.exitCode = 1; });

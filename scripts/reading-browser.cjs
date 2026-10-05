const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const dir = fs.mkdtempSync(path.join(path.resolve('testdata'), 'browser-reading-'));
  const out = path.join(dir, 'output');
  fs.mkdirSync(path.join(dir, 'public'));
  fs.writeFileSync(path.join(dir, 'public', 'pixel.png'), Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=', 'base64'));
  fs.writeFileSync(path.join(dir, 'deck.md'), `---\ntitle: Reading fixture\noffline: true\n---\n\n# First\n\n![Pixel](/public/pixel.png)\n\n<!-- Private speaker note -->\n\n---\n\n\x60\x60\x60yaml\nreveal: true\nid: second\n\x60\x60\x60\n\n# Second\n\n- First item\n- [Second item](https://example.test/)\n\n\x60\x60\x60sirena\nservice api { label: "API" }\n\x60\x60\x60\n`);
  const server = spawn(binary, ['serve', dir, '--port', '8137'], { stdio: ['ignore', 'ignore', 'inherit'] });
  let browser, staticServer;
  try {
    for (let attempt = 0; ; attempt++) {
      if (server.exitCode !== null) throw Error('Reading fixture server exited');
      try { if ((await fetch('http://127.0.0.1:8137/')).ok) break; } catch (_) {}
      if (attempt >= 120) throw Error('Reading fixture startup timed out');
      await new Promise(resolve => setTimeout(resolve, 250));
    }
    browser = await chromium.launch({ args: ['--no-sandbox'], ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}) });
    const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    await page.goto('http://127.0.0.1:8137/?read');
    await page.waitForFunction(() => window.SlidesReading?.enabled());
    assert.equal(await page.locator('.slide:visible').count(), 2);
    assert.equal(await page.locator('.reading-toc a').count(), 2);
    assert.equal(await page.locator('[data-fragment][inert], [data-fragment][aria-hidden="true"]').count(), 0);
    assert.equal(await page.locator('.diagram-description:visible').count(), 1);
    await page.locator('.reading-toc a').last().click();
    assert.ok(page.url().endsWith('#second'));
    assert.equal(await page.locator('#second').count(), 0); // slide identity is a data attribute
    await page.keyboard.press('v');
    await page.waitForFunction(() => !SlidesReading.enabled());
    assert.equal(await page.evaluate(() => SlidesNav.current()), 2, 'reading table of contents retains selected slide');
    assert.equal(await page.locator('.slide:visible').count(), 1);
    await page.keyboard.press('v');
    assert.equal(await page.locator('.slide:visible').count(), 2);
    await page.setViewportSize({ width: 390, height: 844 });
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
    fs.mkdirSync('browser-test-output', { recursive: true });
    await page.screenshot({ path: 'browser-test-output/reading-mobile.png', fullPage: true });
    assert.deepEqual(errors, []);

    for (const format of ['single', 'handout', 'spa']) {
      const result = spawnSync(binary, ['export', dir, '--format', format, '--out', path.join(out, format)], { encoding: 'utf8', env: process.env });
      assert.equal(result.status, 0, result.stderr);
    }
    const handout = fs.readFileSync(path.join(out, 'handout', 'handout.html'), 'utf8');
    assert.ok(handout.includes('data:image/png;base64,'));
    assert.ok(!handout.includes('Private speaker note'));
    const single = fs.readFileSync(path.join(out, 'single', 'deck.html'), 'utf8');
    assert.ok(single.includes('data:image/png;base64,'));
    staticServer = http.createServer((request, response) => {
      const resource = path.join(out, request.url.replace(/^\/talk\//, '').split(/[?#]/)[0]);
      try { response.end(fs.readFileSync(resource)); } catch (_) { response.writeHead(404); response.end(); }
    });
    await new Promise(resolve => staticServer.listen(0, '127.0.0.1', resolve));
    const address = 'http://127.0.0.1:' + staticServer.address().port + '/talk/';
    await page.route('https://**/*', route => route.abort());
    await page.goto(address + 'handout/handout.html');
    await page.waitForFunction(() => SlidesReading.enabled());
    assert.equal(await page.locator('.slide:visible').count(), 2);
    assert.equal(await page.locator('img').first().evaluate(image => image.complete && image.naturalWidth > 0), true);
    await page.goto(address + 'single/deck.html');
    assert.equal(await page.locator('img').first().evaluate(image => image.complete && image.naturalWidth > 0), true);
    await page.goto(address + 'spa/index.html?read');
    await page.waitForFunction(() => SlidesReading.enabled());
    assert.equal(await page.locator('img').first().evaluate(image => image.complete && image.naturalWidth > 0), true);
    assert.deepEqual(errors, []);
    console.log('Reading browser passed: desktop/mobile, fragment accessibility, notes privacy, offline local images and subpath SPA.');
  } finally {
    if (browser) await browser.close();
    if (staticServer) await new Promise(resolve => staticServer.close(resolve));
    server.kill('SIGTERM');
    fs.rmSync(dir, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { spawn, spawnSync } = require('node:child_process');
const { PNG } = require('pngjs');
const { launchTestBrowser } = require('./test-browser.cjs');

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const dir = fs.mkdtempSync(path.resolve('testdata/browser-backgrounds-'));
  const root = fs.readFileSync('examples/background-gallery/deck.md', 'utf8') + '\n---\n\n<!-- slides:include parts/body.md -->\n';
  const part = '```yaml\nid: included\nlayout: center\nscene: false # keep this comment\n```\n\n# Included café 🦊\n\n```yaml\nscene: example-in-code\n```\n\n<!-- private included note -->\n';
  fs.writeFileSync(path.join(dir, 'deck.md'), root);
  fs.mkdirSync(path.join(dir, 'parts')); fs.writeFileSync(path.join(dir, 'parts/body.md'), part);
  const reserve = http.createServer(); await new Promise(r => reserve.listen(0, '127.0.0.1', r));
  const port = reserve.address().port; await new Promise(r => reserve.close(r));
  const server = spawn(binary, ['serve', dir, '--edit', '--port', String(port)], { stdio: ['ignore', 'ignore', 'pipe'] });
  let logs = ''; server.stderr.on('data', chunk => logs = (logs + chunk).slice(-16000));
  let browser;
  try {
    const url = 'http://127.0.0.1:' + port + '/';
    for (let n = 0; ; n++) {
      try { if ((await fetch(url)).ok) break; } catch (_) {}
      if (n > 300 || server.exitCode !== null) throw Error('Background server failed: ' + logs);
      await new Promise(r => setTimeout(r, 100));
    }
    browser = await launchTestBrowser({ args: ['--no-sandbox', '--enable-unsafe-swiftshader'] });
    const page = await browser.newPage({ viewport: { width: 1280, height: 850 }, acceptDownloads: true });
    const errors = []; page.on('pageerror', e => errors.push(e.message));
    let published = 0; page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/presenter/state')) published++; });
    await page.route('**/*', route => new URL(route.request().url()).origin === new URL(url).origin ? route.continue() : route.abort());
    await page.goto(url); await page.waitForFunction(() => window.SlidesNav);
    const native = (process.env.SLIDES_TEST_ENGINE || 'chromium') === 'chromium';
    fs.mkdirSync('browser-test-output', { recursive: true });
    if (native) {
      for (let i = 0; i < 6; i++) {
        await page.evaluate(i => SlidesNav.preview(i, 0), i);
        await page.waitForFunction(() => document.querySelector('.deck-background-active')?.dataset.gosxScene3dReady === 'true');
        assert.equal(await page.locator('.deck-background-active:visible').count(), 1);
        const png = PNG.sync.read(await page.locator('.deck-background-active canvas').screenshot());
        const colors = new Set();
        for (let y = 0; y < png.height; y += 20) for (let x = 0; x < png.width; x += 20) {
          const offset = (y * png.width + x) * 4; colors.add(png.data.subarray(offset, offset + 3).toString('hex'));
        }
        assert.ok(colors.size > 12, 'Preset ' + i + ' must render real varied shader pixels: ' + colors.size);
      }
      await page.evaluate(() => SlidesNav.preview(0, 0));
      const canvas = page.locator('.deck-background-active canvas');
      await page.waitForTimeout(400); const before = await canvas.screenshot();
      await page.waitForTimeout(900); assert.notDeepEqual(await canvas.screenshot(), before, 'Selena context time must animate actual pixels');
      await page.screenshot({ path: 'browser-test-output/background-aurora.png' });
      await page.emulateMedia({ reducedMotion: 'reduce' });
      assert.equal(await page.locator('.deck-background-active').isVisible(), false, 'reduced motion hides decoration');
      await page.emulateMedia({ reducedMotion: 'no-preference' });
    }
    await page.evaluate(() => SlidesNav.preview(6, 0));
    await page.getByRole('button', { name: 'Background wizard', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'A little atmosphere.' });
    const status = dialog.locator('[data-background-status]');
    await status.filter({ hasText: 'Ready.' }).waitFor();
    assert.equal(await dialog.locator('[data-background-preset]').count(), 6);
    await dialog.getByRole('button', { name: 'Silk background', exact: true }).focus(); await page.keyboard.press('ArrowRight');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 7, 'wizard keys cannot advance the deck');
    await dialog.getByRole('button', { name: 'Silk background', exact: true }).click();
    await dialog.getByRole('button', { name: 'Tune background →', exact: true }).click();
    const set = async (key, value) => dialog.locator('[data-background-option="' + key + '"]').evaluate((input, value) => { input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); }, value);
    await set('ink', '#20172a'); await set('glow', '#f0c68a'); await set('speed', 0); await set('strength', .25);
    if (native) {
      const frame = page.frameLocator('iframe[title="Live Selena background preview"]');
      await frame.locator('[data-gosx-scene3d-ready="true"]').waitFor();
      const still = frame.locator('canvas'); await page.waitForTimeout(400);
      const before = await still.screenshot(); await page.waitForTimeout(500);
      assert.deepEqual(await still.screenshot(), before, 'speed zero must produce a stable still');
    }
    const desktop = await dialog.boundingBox(), nextButton = await dialog.getByRole('button', { name: 'Choose where →', exact: true }).boundingBox();
    assert.ok(desktop.y >= 0 && desktop.y + desktop.height <= 850 && nextButton.y >= 0 && nextButton.y + nextButton.height <= 850, 'desktop wizard and next action fit: ' + JSON.stringify({desktop,nextButton}));
    await page.screenshot({ path: 'browser-test-output/background-wizard-desktop.png' });
    assert.equal(fs.readFileSync(path.join(dir, 'parts/body.md'), 'utf8'), part, 'tuning leaves source untouched');
    await page.setViewportSize({ width: 320, height: 640 });
    const bounds = await dialog.boundingBox();
    assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= 320 && bounds.y >= 0 && bounds.y + bounds.height <= 640, 'wizard fits mobile');
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'mobile page has no horizontal overflow');
    await page.screenshot({ path: 'browser-test-output/background-wizard-mobile.png' });
    await dialog.getByRole('button', { name: 'Choose where →', exact: true }).click();
    assert.ok((await dialog.locator('[data-background-target]').textContent()).includes('parts/body.md'), 'save targets original included Markdown');
    assert.ok((await dialog.locator('[data-background-snippet]').textContent()).includes('shader-speed: 0'));
    await dialog.getByText('Copy Markdown or download the shader', { exact: true }).click();
    const download = page.waitForEvent('download'); await dialog.getByRole('button', { name: 'Download .sel', exact: true }).click();
    const asset = await download; const savedShader = path.join(dir, 'downloaded.sel'); await asset.saveAs(savedShader);
    assert.ok(fs.readFileSync(savedShader, 'utf8').includes('param speed : float = 0.0'));
    const shaderDeck = path.join(dir, 'shader-download'); fs.mkdirSync(shaderDeck);
    fs.copyFileSync(savedShader, path.join(shaderDeck, 'silk.sel')); fs.writeFileSync(path.join(shaderDeck, 'deck.md'), '---\nscene: silk.sel\n---\n\n# Downloaded shader\n');
    const inspected = spawnSync(binary, ['inspect', shaderDeck, '--json'], { encoding: 'utf8' });
    assert.equal(inspected.status, 0, inspected.stderr); assert.equal(JSON.parse(inspected.stdout).graphics[0].compiles, true);
    // Supporting source changes invalidate context, even when the target is unchanged.
    const changed = part + '\nExternal edit preserved.\n'; fs.writeFileSync(path.join(dir, 'parts/body.md'), changed);
    await dialog.getByRole('button', { name: 'Apply background', exact: true }).click();
    await status.filter({ hasText: 'project changed' }).waitFor();
    assert.equal(fs.readFileSync(path.join(dir, 'parts/body.md'), 'utf8'), changed);
    await dialog.getByRole('button', { name: 'Reload settings', exact: true }).click(); await status.filter({ hasText: 'Ready.' }).waitFor();
    assert.equal(await dialog.locator('[data-background-option="ink"]').inputValue(), '#20172a', 'conflict reload keeps chosen colors');
    await Promise.all([page.waitForEvent('domcontentloaded'), dialog.getByRole('button', { name: 'Apply background', exact: true }).click()]);
    await page.waitForFunction(() => window.SlidesNav);
    assert.equal(await page.evaluate(() => SlidesNav.current()), 7, 'save retains current slide');
    const saved = fs.readFileSync(path.join(dir, 'parts/body.md'), 'utf8');
    assert.ok(saved.includes('scene: "shader:silk" # keep this comment') && saved.includes('scene: example-in-code') && saved.includes('External edit preserved.') && saved.includes('<!-- private included note -->'), 'focused save preserves comments, code and notes');
    assert.equal(fs.readFileSync(path.join(dir, 'deck.md'), 'utf8'), root);
    assert.ok(fs.readdirSync(path.join(dir, 'parts')).some(name => name.startsWith('.slides-history-')), 'save retains recovery source');
    // A source draft closed in Edit must survive opening and using this wizard.
    await page.setViewportSize({ width: 1280, height: 850 }); await page.keyboard.press('e');
    await page.locator('[data-status]').filter({ hasText: 'Ready to edit' }).waitFor();
    await page.locator('#slides-source').fill(root + '\nUnpublished source draft.\n'); await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Background wizard', exact: true }).click(); await status.filter({ hasText: 'Ready.' }).waitFor();
    await dialog.getByRole('button', { name: 'Tune background →', exact: true }).click(); await dialog.getByRole('button', { name: 'Choose where →', exact: true }).click();
    await dialog.getByRole('button', { name: 'Apply background', exact: true }).click(); await status.filter({ hasText: 'Your drafts are retained' }).waitFor();
    assert.equal(fs.readFileSync(path.join(dir, 'deck.md'), 'utf8'), root);
    await page.keyboard.press('Escape'); assert.equal(await dialog.isVisible(), false);
    await page.keyboard.press('e'); assert.ok((await page.locator('#slides-source').inputValue()).includes('Unpublished source draft.'));
    assert.equal(published, 0, 'wizard preview and apply stay local to the author tab'); assert.deepEqual(errors, []);
    console.log('Background wizard passed: six native presets, real motion/still pixels, reduced motion, tuned shader download, included source save, conflicts/recovery, draft retention, mobile and local preview.');
  } finally {
    if (browser) await browser.close(); server.kill('SIGTERM');
    await new Promise(resolve => server.exitCode !== null ? resolve() : server.once('exit', resolve)); fs.rmSync(dir, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });

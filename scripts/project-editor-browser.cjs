const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const binary = path.resolve(process.argv[2] || './slides');
const dir = fs.mkdtempSync(path.join(path.resolve('testdata'), 'browser-project-'));
const files = {
  'deck.md': '# Root\n\n<!-- notes -->\n\n---\n\n<!-- slides:include sections/body.md -->\n',
  'sections/body.md': '```yaml\nid: detail\ncues: initial, next\n```\n\n# Detail\n\nCafé 🦊\n',
  'graph.sir': 'service api\n',
  'Counter.gsx': 'package main\n\n//gosx:island\nfunc Counter(props any) Node {\n    return <p>Hello</p>\n}\n',
  'story.yaml': 'version: 1\nbeats: []\n',
  '.slides-team.json': '{"draft":"PRIVATE-DRAFT"}',
  'private/notes.md': 'PRIVATE-NOTES',
};
for (const [name, text] of Object.entries(files)) { fs.mkdirSync(path.dirname(path.join(dir, name)), { recursive: true }); fs.writeFileSync(path.join(dir, name), text); }
const server = spawn(binary, ['serve', dir, '--edit', '--port', '8173'], { stdio: ['ignore', 'ignore', 'inherit'] });
let browser;
(async () => {
  let exited = false; server.once('exit', () => { exited = true; });
  const url = 'http://127.0.0.1:8173/';
  for (let n = 0; ; n++) { if (exited) throw Error('project server exited'); try { if ((await fetch(url)).ok) break; } catch (_) {} if (n > 180) throw Error('project startup timed out'); await new Promise(r => setTimeout(r, 250)); }
  browser = await chromium.launch({ args: ['--enable-unsafe-swiftshader'], ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}) });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } }), errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await page.goto(url, { waitUntil: 'domcontentloaded' }); await page.waitForFunction(() => window.SlidesNav);
  await page.keyboard.press('e');
  await page.locator('[data-status]').filter({ hasText: 'Ready to edit' }).waitFor();
  const picker = page.getByRole('combobox', { name: 'Project file' }), input = page.locator('#slides-source');
  const options = await picker.locator('option').allTextContents();
  assert.deepEqual(options, ['Counter.gsx', 'deck.md', 'graph.sir', 'sections/body.md', 'story.yaml']);
  assert.equal(await input.inputValue(), files['deck.md']);
  const rootDraft = files['deck.md'] + '\nRoot draft\n';
  await input.fill(rootDraft); await picker.selectOption('sections/body.md');
  await page.locator('[data-status]').filter({ hasText: 'Ready to edit sections/body.md' }).waitFor();
  const bodyDraft = files['sections/body.md'] + '\n[Jump](#detail/missing)\n';
  await input.fill(bodyDraft);
  const finding = page.locator('[data-project-diagnostics]').getByRole('button', { name: /sections\/body.md:.*unknown cue "missing"/ });
  await finding.waitFor(); await finding.click();
  assert.equal(await input.evaluate(el => el.value.slice(el.selectionStart, el.selectionEnd)), 'missing', 'original file UTF-8 diagnostic selection');
  let published = 0; page.on('request', req => { if (req.url().endsWith('/presenter/state')) published++; });
  await page.getByRole('button', { name: 'Preview project slide 2', exact: true }).click();
  assert.equal(await page.evaluate(() => SlidesNav.current()), 2);
  await page.keyboard.press('e'); await picker.selectOption('deck.md');
  await page.locator('[data-status]').filter({ hasText: 'Ready to edit deck.md' }).waitFor();
  assert.equal(await input.inputValue(), rootDraft, 'tabs retain root draft');
  await picker.selectOption('sections/body.md'); await page.locator('[data-status]').filter({ hasText: 'Ready to edit sections/body.md' }).waitFor();
  assert.equal(await input.inputValue(), bodyDraft, 'tabs retain included draft');
  await page.getByRole('button', { name: 'Save and preview', exact: true }).click();
  await page.locator('[data-status]').filter({ hasText: 'other drafts remain open' }).waitFor().catch(async error => { console.error('Save status:', await page.locator('[data-status]').textContent()); throw error; });
  assert.equal(fs.readFileSync(path.join(dir, 'sections/body.md'), 'utf8'), bodyDraft);
  assert.equal(fs.readFileSync(path.join(dir, 'deck.md'), 'utf8'), files['deck.md'], 'fragment save never writes expanded deck');
  await picker.selectOption('Counter.gsx'); await page.locator('[data-status]').filter({ hasText: 'Ready to edit Counter.gsx' }).waitFor();
  await input.fill('package main\nfunc Counter( {');
  await page.locator('[data-project-diagnostics]').getByRole('button', { name: /Counter.gsx:.*GoSX syntax error/ }).waitFor();
  await page.getByRole('button', { name: 'Save and preview', exact: true }).click();
  await page.locator('[data-status]').filter({ hasText: 'PROJECT-GOSX' }).waitFor();
  assert.equal(fs.readFileSync(path.join(dir, 'Counter.gsx'), 'utf8'), files['Counter.gsx'], 'invalid component remains a draft');
  await page.getByRole('button', { name: 'Undo source', exact: true }).click();
  assert.equal(await input.inputValue(), files['Counter.gsx']);
  await picker.selectOption('deck.md'); await page.locator('[data-status]').filter({ hasText: 'Ready to edit deck.md' }).waitFor();
  fs.writeFileSync(path.join(dir, 'graph.sir'), 'service api\nservice worker\n');
  await page.getByRole('button', { name: 'Save and preview', exact: true }).click();
  await page.locator('[data-status]').filter({ hasText: 'project changed' }).waitFor();
  assert.equal(await input.inputValue(), rootDraft, 'dependency conflict keeps draft');
  assert.equal(fs.readFileSync(path.join(dir, 'deck.md'), 'utf8'), files['deck.md']);
  await page.setViewportSize({ width: 320, height: 640 });
  const bounds = await page.locator('.slides-source-panel').boundingBox(), save = await page.getByRole('button', { name: 'Save and preview', exact: true }).boundingBox();
  assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= 320 && bounds.y >= 0 && bounds.y + bounds.height <= 640, 'mobile panel fits');
  assert.ok(save.x >= 0 && save.x + save.width <= 320 && save.y + save.height <= 640, 'mobile save remains accessible');
  await page.screenshot({ path: '/tmp/slides-project-editor-mobile.png' });
  await picker.focus(); await page.keyboard.press('Home'); await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Escape'); assert.equal(await page.locator('.slides-source-panel').isVisible(), false);
  assert.equal(published, 0, 'source navigation and preview remain local'); assert.deepEqual(errors, []);
  console.log('Project editor passed: discovery/privacy, included ranges, independent drafts, validated component edits, dependency conflicts, silent preview, keyboard/mobile.');
})().catch(error => { console.error(error); process.exitCode = 1; }).finally(async () => { if (browser) await browser.close(); server.kill('SIGTERM'); fs.rmSync(dir, { recursive: true, force: true }); });

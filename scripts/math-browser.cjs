// Offline equation coverage for live decks, bundle subpaths, snapshots and
// browser-captured exports. Production rendering uses no Node dependencies.
const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { pathToFileURL } = require('node:url');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

const binary = path.resolve(process.argv[2] || './slides');
const fixture = fs.mkdtempSync(path.resolve('testdata/math-browser-'));
const source = String.raw`---
title: Offline equations
theme: swiss
transition: none
offline-required: true
---

# Equations

Energy $E=mc^2$ and the fraction $\frac{1}{\sqrt{2}}$ are inline.

$$
\int_0^1 x^2\,dx=\frac{1}{3} \qquad \sum_{n=1}^\infty \frac{1}{n^2}=\frac{\pi^2}{6}
$$

<!-- First slide -->

---

# Structured math

$$
\begin{bmatrix}1 & 2 \\ 3 & 4\end{bmatrix}
$$

$$
f(x)=\begin{cases}x^2 & x \geq 0 \\ -x & x < 0\end{cases}
$$

Unsupported $\unknowncommand{x}$ keeps the source and a diagnostic.
`;
fs.writeFileSync(path.join(fixture, 'deck.md'), source);

function run(args, env = process.env) {
  const result = spawnSync(binary, args, { encoding: 'utf8', env, timeout: 120000 });
  assert.equal(result.status, 0, result.stderr || result.error?.message || args.join(' '));
}

async function assertMath(page, url, offline) {
  const errors = [], remoteRequests = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.route('**/*', async route => {
    const target = route.request().url();
    if (/^https?:/.test(target) && (offline || !target.startsWith('http://127.0.0.1:'))) {
      remoteRequests.push(target); await route.abort();
    } else await route.continue();
  });
  await page.goto(url, { waitUntil: 'load' });
  await page.evaluate(() => document.fonts.ready);
  assert.equal(await page.locator('.katex-mathml math').count(), 5);
  assert.equal(await page.locator('.math-error').count(), 1);
  assert.equal(await page.locator('.math-inline').count(), 3);
  assert.equal(await page.locator('.math-block').count(), 3);
  assert.match(await page.locator('.math-error').textContent(), /unknowncommand/);
  assert.ok(await page.locator('.katex-mathml mfrac').count() >= 4);
  assert.ok(await page.locator('.katex-mathml mtable').count() >= 2);
  assert.ok(await page.locator('.katex-html[aria-hidden="true"]').count() === 5);
  const fonts = await page.evaluate(() => [...document.fonts].filter(font => font.family.startsWith('KaTeX') && font.status === 'loaded').map(font => font.family));
  assert.ok(fonts.includes('KaTeX_Main') && fonts.includes('KaTeX_Math'), 'local math fonts must actually load');
  await page.evaluate(() => SlidesNav.show(1, 0, true));
  await page.evaluate(() => document.fonts.ready);
  const bounds = await page.locator('.deck-active .math-block').first().boundingBox();
  assert.ok(bounds.width > 50 && bounds.height > 50, 'matrix must have visible typeset geometry');
  assert.deepEqual(remoteRequests, [], 'equations must not request a CDN or font server');
  assert.deepEqual(errors, []);
}

(async () => {
  const browser = await chromium.launch(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {});
  let server, staticServer;
  try {
    // Initial semantics survive disabled JavaScript as well.
    const output = path.join(fixture, 'output');
    run(['export', fixture, '--format', 'single', '--out', output]);
    const context = await browser.newContext({ javaScriptEnabled: false });
    const noScript = await context.newPage();
    await noScript.goto(pathToFileURL(path.join(output, 'deck.html')).href);
    assert.equal(await noScript.locator('.katex-mathml math').count(), 5);
    await context.close();
    const snapshot = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    await assertMath(snapshot, pathToFileURL(path.join(output, 'deck.html')).href, true);
    await snapshot.close();

    // Use an OS-selected port and reserve it only while obtaining the address.
    const reserve = http.createServer();
    await new Promise(resolve => reserve.listen(0, '127.0.0.1', resolve));
    const port = reserve.address().port;
    await new Promise(resolve => reserve.close(resolve));
    server = spawn(binary, ['serve', fixture, '--port', String(port)], { stdio: ['ignore', 'ignore', 'inherit'] });
    for (let attempt = 0; ; attempt++) {
      try { if ((await fetch(`http://127.0.0.1:${port}/`)).ok) break; } catch (_) {}
      if (attempt >= 200 || server.exitCode !== null) throw Error('Math deck server did not start');
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    const live = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    await assertMath(live, `http://127.0.0.1:${port}/`, false);
    await live.close();

    const spa = path.join(output, 'spa');
    run(['build', fixture, '--out', spa]);
    staticServer = http.createServer((request, response) => {
      if (request.url !== '/talk/index.html') { response.writeHead(404); response.end(); return; }
      response.setHeader('content-type', 'text/html');
      response.end(fs.readFileSync(path.join(spa, 'index.html')));
    });
    await new Promise(resolve => staticServer.listen(0, '127.0.0.1', resolve));
    const bundle = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    await assertMath(bundle, `http://127.0.0.1:${staticServer.address().port}/talk/index.html`, false);
    await bundle.close();

    const env = { ...process.env, SLIDES_CHROME: process.env.SLIDES_CHROME || chromium.executablePath() };
    const pdf = path.join(output, 'equations.pdf');
    run(['export', fixture, '--format', 'pdf', '--out', pdf], env);
    assert.ok(fs.readFileSync(pdf).subarray(0, 4).equals(Buffer.from('%PDF')));
    const frames = path.join(output, 'frames');
    run(['export', fixture, '--format', 'frames', '--out', frames], env);
    const png = fs.readFileSync(path.join(frames, 'slide-001-step-000.png'));
    assert.ok(png.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])));
    console.log('Math browser checks passed: server-rendered MathML, local fonts, live deck, offline snapshot, SPA subpath, PDF and captured frames.');
  } finally {
    server?.kill('SIGTERM');
    if (staticServer) await new Promise(resolve => staticServer.close(resolve));
    await browser.close();
    fs.rmSync(fixture, { recursive: true, force: true });
  }
})().catch(error => {
  fs.rmSync(fixture, { recursive: true, force: true });
  console.error(error); process.exitCode = 1;
});

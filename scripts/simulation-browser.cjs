const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { spawn } = require('node:child_process');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

async function served(binary, run) {
  const server = spawn(path.resolve(binary), ['serve', 'examples/simulation-lab', '--port', '8156'], { stdio: ['ignore', 'ignore', 'inherit'] });
  let exited = false; server.once('exit', () => { exited = true; });
  const url = 'http://127.0.0.1:8156/';
  try {
    for (let attempt = 0; ; attempt++) {
      if (exited) throw Error('Simulation server exited');
      try { if ((await fetch(url)).ok) break; } catch (_) {}
      if (attempt >= 600) throw Error('Simulation startup timed out');
      await new Promise(resolve => setTimeout(resolve, 500));
    }
    await run(url);
  } finally { server.kill('SIGTERM'); }
}

(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'], ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}) });
  try {
    const run = async url => {
      const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
      const errors = []; page.on('pageerror', error => errors.push(error.message));
      await page.goto(url + '#particles/impulse', { waitUntil: 'load' });
      await page.waitForFunction(() => window.SlidesSimulation && window.SlidesMotion);
      await page.evaluate(async () => { await document.fonts.ready; SlidesMotion.seek(0); await SlidesMotion.settled(); });
      const mount = page.locator('.deck-active [data-simulation="demo"]'), svg = mount.locator('svg');
      const sample = ms => page.evaluate(async ms => { SlidesMotion.seek(ms); await SlidesMotion.settled(); return SlidesSimulation.state('demo'); }, ms);
      assert.equal(await page.evaluate(() => SlidesMotion.duration()), 2000, 'duration comes from the explicitly authored tick range');
      const start = await sample(0), middle = await sample(1000), pixels = await svg.screenshot(), final = await sample(2000);
      assert.equal(start.tick, 60); assert.equal(middle.tick, 90); assert.equal(final.tick, 120);
      assert.notEqual(start.hash, middle.hash); assert.notEqual(middle.hash, final.hash);
      assert.deepEqual(await sample(1000), middle, 'reverse seek restores the exact authoritative state');
      assert.ok((await svg.screenshot()).equals(pixels), 'forward/reverse capture pixels must repeat');
      await page.evaluate(() => SlidesSimulation.branch('demo', 'wind'));
      const wind = await sample(1000); assert.notEqual(wind.hash, middle.hash, 'authored future inputs change the branch');
      await page.evaluate(() => SlidesSimulation.branch('demo', 'baseline'));
      assert.deepEqual(await sample(1000), middle, 'baseline survives branching');
      const branchPixels = await svg.screenshot();
      // Chromium's retained SVG raster can change a handful of antialiased edge
      // channels after a different pose. State and geometry remain exact; bound
      // that raster allowance tightly rather than comparing PNG compression.
      if (!branchPixels.equals(pixels)) {
        const difference=await page.evaluate(async({a,b})=>{
          const decode=async data=>{const raw=atob(data),bytes=Uint8Array.from(raw,c=>c.charCodeAt(0)),image=await createImageBitmap(new Blob([bytes],{type:'image/png'})),canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;const ctx=canvas.getContext('2d');ctx.drawImage(image,0,0);return ctx.getImageData(0,0,canvas.width,canvas.height);};
          const left=await decode(a),right=await decode(b);let changes=0,max=0;for(let i=0;i<left.data.length;i++) { const d=Math.abs(left.data[i]-right.data[i]);if(d)changes++;max=Math.max(max,d); }return{changes,max,total:left.data.length,sameSize:left.width===right.width && left.height===right.height};
        },{a:pixels.toString('base64'),b:branchPixels.toString('base64')});
        assert.ok(difference.sameSize && difference.max <= 4 && difference.changes / difference.total <= .0001, 'branch return has identical geometry with only bounded SVG antialiasing');
      }
      await page.evaluate(() => { SlidesSimulation.seek('demo', 60); SlidesSimulation.branch('demo', 'wind'); });
      const branchPrefix = await page.evaluate(() => SlidesSimulation.state('demo'));
      assert.equal(branchPrefix.hash, start.hash, 'branch preserves the exact prefix through fork tick');
      await page.evaluate(() => { SlidesSimulation.branch('demo', 'baseline'); SlidesSimulation.follow('demo'); });
      assert.equal((await page.evaluate(() => SlidesSimulation.state('demo'))).manual, false);
      await mount.locator('.simulation-tick').focus(); await page.keyboard.press('ArrowRight');
      assert.equal(await page.evaluate(() => SlidesNav.step()), 1, 'slider keys retain native input behavior');
      assert.equal((await page.evaluate(() => SlidesSimulation.state('demo'))).manual, true);
      await page.evaluate(() => { SlidesNav.show(0, 2, false); SlidesMotion.seek(0); });
      assert.equal((await page.evaluate(() => SlidesSimulation.state('demo'))).tick, 120, 'navigation clears exploratory seek');
      await page.evaluate(() => { SlidesNav.show(0, 1, false); SlidesMotion.seek(1000); });
      assert.deepEqual(await sample(1000), middle, 'backward navigation restores absolute state');
      await page.reload({ waitUntil: 'load' }); await page.waitForFunction(() => window.SlidesSimulation);
      assert.deepEqual(await sample(1000), middle, 'fresh load reproduces the seed and logged inputs');
      await page.setViewportSize({ width: 390, height: 844 });
      const bounds = await mount.boundingBox(); assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= 391, 'mobile controls remain inside the deck');
      await page.emulateMedia({ reducedMotion: 'reduce' });
      assert.equal((await sample(0)).tick, 120, 'reduced motion settles the explicitly authored end tick');
      await page.evaluate(() => SlidesSimulation.seek('demo', 70));
      assert.equal((await page.evaluate(() => SlidesSimulation.state('demo'))).tick, 70, 'reduced motion still permits explicit static exploration');
      assert.deepEqual(errors, [], 'simulation must not emit runtime errors');
      await page.close();
    };
    if (process.env.SLIDES_SIMULATION_FIXTURE) await run(pathToFileURL(path.resolve(process.env.SLIDES_SIMULATION_FIXTURE)).href);
    else await served(process.argv[2] || './slides', run);
    console.log('Simulation browser checks passed: seeded replay, checkpoints, authored branch prefix, forward/reverse pixel capture, reload, keyboard and mobile bounds.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });

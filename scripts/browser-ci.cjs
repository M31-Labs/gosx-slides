const { spawn, spawnSync } = require('node:child_process');
const path = require('node:path');
const fs = require('node:fs');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const binary = path.resolve(process.argv[2] || './slides');
async function withServer(deck, port, run) {
  const server = spawn(binary, ['serve', deck, '--port', String(port)], { stdio: ['ignore', 'ignore', 'inherit'] });
  let exited = false; server.once('exit', () => { exited = true; });
  const url = 'http://127.0.0.1:'+port+'/';
  try {
    for (let attempt = 0; ; attempt++) {
      if (exited) throw Error('Deck server exited: '+deck);
      try { if ((await fetch(url)).ok) break; } catch (_) {}
      if (attempt >= 600) throw Error('Deck startup timed out: '+deck);
      await new Promise(resolve => setTimeout(resolve, 500));
    }
    await run(url);
  } finally { server.kill('SIGTERM'); }
}
function script(name, url) { const result=spawnSync(process.execPath,[path.join(__dirname,name),url],{stdio:'inherit',env:process.env}); assert.equal(result.status,0,name+' failed'); }
(async()=>{
  await withServer('examples/navigation-lab',8111,url=>script('navigation-browser.cjs',url));
  await withServer('examples/authoring-lab',8112,url=>script('authoring-browser.cjs',url));
  await withServer('examples/shader-lab',8113,async url=>{
    const browser=await chromium.launch({args:['--enable-unsafe-swiftshader'],...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{})});
    try {const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(url,{waitUntil:'domcontentloaded'});
      await page.waitForFunction(()=>document.querySelector('.deck-background-active[data-gosx-scene3d-ready="true"]'),undefined,{timeout:30000});
      await page.evaluate(()=>SlidesMotion.pause());
      await page.waitForFunction(()=>document.querySelector('.deck-background-active').dataset.gosxScene3dAnimationState==='paused');
      const before=await page.locator('.deck-background-active').getAttribute('data-gosx-scene3d-animation-clock');await page.waitForTimeout(300);
      assert.equal(await page.locator('.deck-background-active').getAttribute('data-gosx-scene3d-animation-clock'),before);
      await page.evaluate(()=>SlidesMotion.play());await page.waitForFunction(()=>document.querySelector('.deck-background-active').dataset.gosxScene3dAnimationState==='playing');assert.deepEqual(errors,[]);
    } finally {await browser.close()}
  });
  const out=process.env.SLIDES_TEST_OUTPUT || 'browser-test-output';fs.mkdirSync(out,{recursive:true});
  for(const args of [['export','examples/authoring-lab','--format','pdf','--steps','--out',path.join(out,'steps.pdf')],['export','examples/shader-lab','--format','frames','--out',path.join(out,'graphics')]]){
    const result=spawnSync(binary,args,{stdio:'inherit',env:process.env});assert.equal(result.status,0,'capture export failed');
  }
  const pdf=fs.readFileSync(path.join(out,'steps.pdf'));
  assert.ok(pdf.subarray(0,4).equals(Buffer.from('%PDF')));
  assert.equal((pdf.toString('latin1').match(/\/Type\s*\/Page\b/g)||[]).length,11,'every authored code/cue state must be a PDF page');
  assert.ok(fs.readFileSync(path.join(out,'graphics','slide-001-step-000.png')).subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])));
  console.log('Browser CI passed: desktop, mobile, presenter, motion, graphics pause, PDF steps, captured graphics.');
})().catch(error=>{console.error(error);process.exitCode=1});

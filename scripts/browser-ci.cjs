const { spawn, spawnSync } = require('node:child_process');
const path = require('node:path');
const fs = require('node:fs');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
const binary = path.resolve(process.argv[2] || './slides');
async function withServer(deck, port, run, flags=[]) {
  const server = spawn(binary, ['serve', deck, '--port', String(port), ...flags], { stdio: ['ignore', 'ignore', 'inherit'] });
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
  const editDir=fs.mkdtempSync(path.join(path.resolve('testdata'),'browser-edit-'));
  const sourcePath=path.join(editDir,'deck.md');fs.writeFileSync(sourcePath,'# Source\n\nOriginal text\n\n<!-- notes -->\n\n---\n\n# Second\n\nAnother slide\n');
  try {await withServer(editDir,8120,async url=>{const result=spawnSync(process.execPath,[path.join(__dirname,'editing-browser.cjs'),url,sourcePath],{stdio:'inherit',env:process.env});assert.equal(result.status,0,'editing browser failed');},['--edit']);}finally{fs.rmSync(editDir,{recursive:true,force:true})}

  const storyDir=fs.mkdtempSync(path.join(path.resolve('testdata'),'browser-story-'));
  try{fs.cpSync('examples/storytelling-lab',storyDir,{recursive:true});await withServer(storyDir,8126,async url=>{const result=spawnSync(process.execPath,[path.join(__dirname,'storytelling-browser.cjs'),url,path.join(storyDir,'deck.md')],{stdio:'inherit',env:process.env});assert.equal(result.status,0,'storytelling browser failed');script('pptx-editable-browser.cjs',url);},['--edit']);}finally{fs.rmSync(storyDir,{recursive:true,force:true})}

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
  await withServer('examples/diagram-lab',8118,async url=>{
    const browser=await chromium.launch({args:['--enable-unsafe-swiftshader'],...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{})});
    try {
      const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
      await page.goto(url+'#state',{waitUntil:'domcontentloaded'});
      await page.waitForFunction(()=>window.SlidesNav);
      assert.equal(await page.locator('.mdpp-diagram svg').count(),9);
      assert.equal(await page.locator('.diagram-error').count(),0);
      assert.ok((await page.locator('[data-slide-id="class"] svg').textContent()).includes('submit()'));
      await page.evaluate(()=>SlidesNav.show(5,0,true));
      await page.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.appliedStep==='0');
      assert.equal(await page.evaluate(()=>SlidesNav.stepCount()),4);
      for(const step of [1,2,0,3,4,1,0]) {
        await page.evaluate(step=>SlidesNav.show(5,step,true),step);
        await page.waitForFunction(step=>Number(document.querySelector('.deck-active .slide-graphic').dataset.appliedStep)===step,step);
      }
      assert.deepEqual(errors,[]);
    } finally {await browser.close()}
  });
  const out=process.env.SLIDES_TEST_OUTPUT || 'browser-test-output';fs.mkdirSync(out,{recursive:true});
  for(const args of [['export','examples/authoring-lab','--format','pptx','--steps','--out',path.join(out,'steps.pptx')],['export','examples/authoring-lab','--format','pdf','--steps','--out',path.join(out,'steps.pdf')],['export','examples/shader-lab','--format','frames','--out',path.join(out,'graphics')]]){
    const result=spawnSync(binary,args,{stdio:'inherit',env:process.env});assert.equal(result.status,0,'capture export failed');
  }
  assert.ok(fs.readFileSync(path.join(out,'steps.pptx')).subarray(0,2).equals(Buffer.from('PK')));
  const editable=path.join(out,'editable.pptx');
  const editableExport=spawnSync(binary,['export','examples/storytelling-lab','--format','pptx','--editable','--out',editable],{stdio:'inherit',env:process.env});assert.equal(editableExport.status,0,'editable PPTX export failed');
  const editableXML=spawnSync('unzip',['-p',editable,'ppt/slides/slide5.xml'],{encoding:'utf8'});assert.equal(editableXML.status,0,'editable PPTX XML missing');assert.ok(editableXML.stdout.includes('<p:txBody>')&&editableXML.stdout.includes('<a:custGeom>')&&editableXML.stdout.includes('<a:alpha val="15000"/>'),'radar must retain editable labels, geometry and translucency');
  const bench=spawnSync(binary,['bench','examples/authoring-lab','--runs','1','--budget','scripts/performance-budget.json'],{encoding:'utf8',env:process.env});assert.equal(bench.status,0,bench.stderr);const report=JSON.parse(bench.stdout);assert.equal(report.runs.length,1);assert.ok(report.runs[0].readyMillis>0 && report.runs[0].heapBytes>0 && report.runs[0].domNodes>0 && report.runs[0].frameP95Millis>0);
  const pdf=fs.readFileSync(path.join(out,'steps.pdf'));
  assert.ok(pdf.subarray(0,4).equals(Buffer.from('%PDF')));
  assert.equal((pdf.toString('latin1').match(/\/Type\s*\/Page\b/g)||[]).length,11,'every authored code/cue state must be a PDF page');
  assert.ok(fs.readFileSync(path.join(out,'graphics','slide-001-step-000.png')).subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])));
  console.log('Browser CI passed: desktop, mobile, presenter, motion, graphics pause, PDF steps, captured graphics.');
})().catch(error=>{console.error(error);process.exitCode=1});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const {spawn} = require('node:child_process');
const {launchTestBrowser} = require('./test-browser.cjs');

(async()=>{
  const binary=path.resolve(process.argv[2] || './slides');
  assert.ok(process.env.SLIDES_RUNTIME_DIR,'set SLIDES_RUNTIME_DIR to a real packaged runtime');
  const fixture=fs.mkdtempSync(path.resolve('testdata/portable-runtime-'));
  const reserve=http.createServer();await new Promise(resolve=>reserve.listen(0,'127.0.0.1',resolve));
  const port=reserve.address().port;await new Promise(resolve=>reserve.close(resolve));
  fs.writeFileSync(path.join(fixture,'deck.md'),'---\ntitle: Portable explanation\ntheme: swiss\noffline-required: true\n---\n\n# A portable live deck\n\nMath remains offline: $E=mc^2$.\n\n<!-- Private portable note -->\n\n---\n\n# A real island\n\n<Counter Initial={7}/>\n\n<!-- island -->\n\n---\n\n# A native scene\n\n<Scene3D Src="request.sir" />\n');
  fs.copyFileSync('examples/showcase/Counter.gsx',path.join(fixture,'Counter.gsx'));
  fs.copyFileSync('examples/semantic-story/request.sir',path.join(fixture,'request.sir'));
  const env={...process.env,PATH:''};
  for(const key of Object.keys(env))if(key.toLowerCase()==='path')env[key]='';
  const server=spawn(binary,['serve',fixture,'--port',String(port)],{env,stdio:['ignore','ignore','pipe']});
  let logs='';server.stderr.on('data',data=>logs=(logs+data).slice(-12000));
  let browser;
  try {
    const url=`http://127.0.0.1:${port}/`;
    for(let attempt=0;;attempt++) {try {if((await fetch(url)).ok)break;}catch(_){}if(attempt>200 || server.exitCode!==null)throw Error('Portable server did not start without Go: '+logs);await new Promise(resolve=>setTimeout(resolve,100));}
    browser=await launchTestBrowser({args:['--no-sandbox','--enable-unsafe-swiftshader']});
    const page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[];
    page.on('pageerror',error=>errors.push(error.message));
    await page.route('**/*',route=>new URL(route.request().url()).origin===new URL(url).origin?route.continue():route.abort());
    await page.goto(url);await page.waitForFunction(()=>window.SlidesNav);
    assert.equal(await page.locator('.katex math').count(),1);
    await page.evaluate(()=>SlidesNav.show(1,0,false));
    await page.waitForFunction(()=>document.querySelector('.deck-active .counter-label')?.textContent.includes('7'));
    await page.locator('.deck-active .counter-btn').last().click();
    await page.waitForFunction(()=>document.querySelector('.deck-active .counter-label')?.textContent.includes('8'));
    await page.evaluate(()=>SlidesNav.show(2,0,false));
    await page.waitForFunction(()=>document.querySelector('.deck-active .slide-graphic')?.dataset.gosxScene3dReady==='true');
    assert.ok(await page.locator('.deck-active canvas').count()>0);
    assert.deepEqual(errors,[],'portable WASM and native graphics must load without errors');
    assert.equal(fs.existsSync(path.join(fixture,'go.mod')),false,'this deck has no module');
    assert.equal(fs.existsSync(path.join(fixture,'go.sum')),false,'no Go resolution happened');
    console.log('Portable runtime browser passed: no Go/PATH/module or external network; actual reactive island, offline math and native Scene3D.');
  }finally {if(browser)await browser.close();server.kill('SIGTERM');await new Promise(resolve=>server.exitCode!==null?resolve():server.once('exit',resolve));fs.rmSync(fixture,{recursive:true,force:true});}
})().catch(error=>{console.error(error);process.exitCode=1;});

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const binary = path.resolve(process.argv[2] || './slides');

async function withServer(deck,port,run) {
  const server=spawn(binary,['serve',deck,'--port',String(port)],{stdio:['ignore','ignore','inherit']});
  let exited=false;server.once('exit',()=>{exited=true});
  const url='http://127.0.0.1:'+port+'/';
  try {
    for(let attempt=0;;attempt++) {
      if(exited)throw Error('Starter server exited');
      try{if((await fetch(url)).ok)break}catch{}
      if(attempt>120)throw Error('Starter startup timed out');
      await new Promise(resolve=>setTimeout(resolve,250));
    }
    await run(url);
  } finally {server.kill('SIGTERM')}
}

(async()=>{
  const output=process.env.SLIDES_TEST_OUTPUT || 'browser-test-output';fs.mkdirSync(output,{recursive:true});
  const browser=await chromium.launch({...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{}),args:['--enable-unsafe-swiftshader']});
  const errors=[];
  try {
    const templates=[['architecture-review',5],['technical-talk',6],['teaching',5]];
    for(let index=0;index<templates.length;index++) {
      const [name,count]=templates[index];
      await withServer('examples/starters/'+name,8193+index,async url=>{
        const page=await browser.newPage({viewport:{width:1440,height:900}});
        page.on('pageerror',error=>errors.push(name+': '+error.message));
        await page.goto(url,{waitUntil:'domcontentloaded'});
        await page.waitForFunction(()=>window.SlidesNav && document.fonts.status==='loaded');
        assert.equal(await page.locator('main.deck > section.slide').count(),count);
        assert.equal(await page.locator('[data-gosx-unresolved]').count(),0);
        const typography=await page.locator('.studio-cover .studio-kicker').evaluate(element=>{const s=getComputedStyle(element);return {fontSize:parseFloat(s.fontSize),fontStyle:s.fontStyle,maxWidth:s.maxWidth}});
        assert.ok(typography.fontSize<=14,name+' cover eyebrow should stay small');
        assert.equal(typography.fontStyle,'normal');
        assert.equal(typography.maxWidth,'none');
        for(let slide=0;slide<count;slide++) {
          await page.evaluate(slide=>SlidesNav.show(slide,0,true),slide);
          await page.waitForTimeout(80);
          const bounds=await page.evaluate(()=>{
            const slide=document.querySelector('.deck-active');
            return Array.from(slide.querySelectorAll('h1,h2,table,pre,.mdpp-container-card,.slides-simulation')).map(element=>{const r=element.getBoundingClientRect();return {name:element.tagName,left:r.left,right:r.right,top:r.top,bottom:r.bottom,visible:r.width>0&&r.height>0}});
          });
          for(const box of bounds.filter(box=>box.visible))assert.ok(box.left>=-1&&box.right<=1441&&box.top>=-1&&box.bottom<=901,name+' slide '+slide+' content escapes viewport: '+JSON.stringify(box));
          const footerOverlap=await page.evaluate(()=>{
            const slide=document.querySelector('.deck-active'),footer=slide.querySelector('.slide-footer');
            if(!footer)return false;const f=footer.getBoundingClientRect();
            return Array.from(slide.children).filter(element=>!element.matches('.slide-footer,.slide-header,.slide-scene')).some(element=>{const r=element.getBoundingClientRect();return r.width>0&&r.height>0&&r.top<f.bottom&&r.bottom>f.top&&r.left<f.right&&r.right>f.left});
          });
          assert.equal(footerOverlap,false,name+' slide '+slide+' footer overlaps content');
          if(slide===0 || slide===1 || name==='teaching'&&slide===2)await page.screenshot({path:path.join(output,'starter-'+name+'-'+slide+'.png')});
        }
        if(name==='architecture-review') {
          await page.goto(url+'#system/committed');
          await page.waitForFunction(()=>window.SlidesStory?.current()?.cue==='committed');
          assert.deepEqual(await page.evaluate(()=>SlidesStory.assertCurrent().errors),[]);
        }
        if(name==='teaching') {
          await page.goto(url+'#particles/impulse');
          await page.waitForFunction(()=>window.SlidesSimulation && document.querySelector('[data-simulation="demo"]'));
          assert.equal(await page.evaluate(()=>JSON.parse(document.getElementById('slides-simulations').textContent).simulations[0].playhead[1].cue),'impulse');
        }
        await page.setViewportSize({width:390,height:844});
        await page.evaluate(()=>SlidesNav.show(0,0,true));await page.waitForTimeout(120);
        await page.screenshot({path:path.join(output,'starter-'+name+'-mobile.png')});
        await page.close();
      });
    }
    assert.deepEqual(errors,[],'Starter browser errors');
    console.log('Curated starter desktop/mobile, story and simulation browser checks passed');
  } finally {await browser.close()}
})().catch(error=>{console.error(error);process.exit(1)});

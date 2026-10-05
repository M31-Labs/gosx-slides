const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || "playwright");
const assert = require("node:assert/strict"), fs = require("node:fs"), path = require("node:path");
const extraction = fs.readFileSync(path.join(__dirname,"../assets/pptx-editable.js"),"utf8");
(async () => {
  const browser = await chromium.launch({ ...(process.env.SLIDES_BROWSER ? { executablePath:process.env.SLIDES_BROWSER } : {}), args:["--enable-unsafe-swiftshader"] });
  try {
    const page = await browser.newPage({viewport:{width:960,height:720}});
    await page.goto(process.argv[2],{waitUntil:"domcontentloaded"});
    await page.waitForFunction(() => window.SlidesNav && document.readyState === "complete");
    for (let index=0;index<3;index++) {
      await page.evaluate(i => SlidesNav.show(i,0,true),index);
      await page.waitForTimeout(300);
      const objects = await page.evaluate(extraction), native = objects.find(o => o.kind === (index ? "chart" : "table"));
      assert.ok(native,`slide ${index+1} has native ${index ? "chart" : "table"}`);
      assert.ok(native.x >= 0 && native.y >= 0 && native.x+native.width <= 961 && native.y+native.height <= 721);
      if (index) {
        assert.equal(native.chart.type,index===1 ? "bar" : "pie");
        assert.deepEqual(native.chart.categories,["Authoring","Presenting","Sharing"]);
        assert.deepEqual(native.chart.values,[45,35,20]);
        assert.ok(native.chart.textColor && native.chart.textColor !== "000000","dark chart has readable native labels");
        assert.equal(await page.locator(".deck-active svg g.chart-item").first().evaluate(el => el.style.visibility),"hidden");
      } else {
        assert.equal(native.table.rows[1][0].text,"Native tables");
        assert.equal(native.table.columnWidths.length,2);
        assert.equal(await page.locator(".deck-active table").evaluate(el => el.style.visibility),"hidden");
      }
      await page.evaluate(() => __slidesPPTXRestore());
      assert.equal(await page.locator(index ? ".deck-active svg g.chart-item" : ".deck-active table").first().evaluate(el => el.style.visibility),"");
    }
    await page.evaluate(() => {
      SlidesNav.show(0,0,true);
      const slide=document.querySelector(".deck-active"), wrapper=document.createElement("div");
      wrapper.style.cssText="position:absolute;left:30px;top:550px;font:18px Arial;color:black";
      wrapper.innerHTML='<table id="merged"><tr><td colspan="2">MERGED_PIXEL_FALLBACK</td></tr><tr><td>A</td><td>B</td></tr></table><table id="rich"><tr><td>RICH <b>PIXEL_FALLBACK</b></td></tr></table>';
      slide.append(wrapper);
      const independent = document.createElement('p');
      independent.id='independent-scale-text';independent.textContent='INDEPENDENT_SCALE_PIXEL_FALLBACK';
      independent.style.cssText='position:absolute;left:40px;top:260px;font:20px Arial;color:rgb(0,0,0);scale:.5 .8;transform-origin:top left';
      slide.append(independent);
    });
    const fallback = await page.evaluate(extraction);
    assert.ok(!fallback.some(o => o.text?.includes("PIXEL_FALLBACK") || o.table?.rows.flat().some(c => c.text.includes("PIXEL_FALLBACK"))),"unsupported tables stay entirely captured");
    assert.equal(await page.locator("#merged").evaluate(el => el.style.visibility),"");
    assert.equal(await page.locator("#rich").evaluate(el => el.style.visibility),"");
    await page.evaluate(() => __slidesPPTXRestore());
    await page.evaluate(() => {
      const slide=document.querySelector(".deck-active"), wrapper=document.createElement("div");
      wrapper.innerHTML='<div style="position:absolute;left:40px;top:40px;transform:scale(.5);transform-origin:top left"><p id="uniform-text" style="font:40px Arial;color:rgb(0,0,0)">UNIFORM_SCALED_TEXT</p></div><div style="position:absolute;left:40px;top:100px;transform:scale(.5,.8);transform-origin:top left"><p id="nonuniform-text" style="font:40px Arial;color:rgb(0,0,0)">NON_UNIFORM_PIXEL_FALLBACK</p></div><div style="position:absolute;left:40px;top:200px;transform:rotateX(30deg)"><p id="three-d-text" style="font:20px Arial;color:rgb(0,0,0)">THREE_D_PIXEL_FALLBACK</p></div><table id="cjk-table" style="position:absolute;left:40px;top:300px;width:400px;border-collapse:collapse;font:1px/1px Arial;color:rgb(0,0,0)"><tr><td style="padding:0;font:1px/1px Arial">'+"界".repeat(7000)+'</td></tr></table>';
      slide.append(wrapper);
    });
    const scaled=await page.evaluate(extraction),uniform=scaled.find(o=>o.text==='UNIFORM_SCALED_TEXT');
    assert.ok(uniform,"uniformly scaled text remains editable");assert.equal(uniform.fontSize,20,"native font follows the rendered ancestor scale");
    assert.ok(!scaled.some(o=>o.text?.includes('NON_UNIFORM_PIXEL_FALLBACK') || o.text?.includes('THREE_D_PIXEL_FALLBACK') || o.text?.includes('INDEPENDENT_SCALE_PIXEL_FALLBACK')),"unsupported transforms retain pixels");
    assert.equal(await page.locator('#nonuniform-text').evaluate(el=>el.childElementCount),0);
    assert.equal(await page.locator('#three-d-text').evaluate(el=>el.childElementCount),0);
    assert.equal(await page.locator('#independent-scale-text').evaluate(el=>el.childElementCount),0);
    assert.ok(!scaled.some(o=>o.table?.rows.flat().some(c=>c.text.includes('界'))),"UTF-8 cell byte overflow retains the whole table");
    assert.equal(await page.locator('#cjk-table').evaluate(el=>el.style.visibility),'');
    await page.evaluate(()=>__slidesPPTXRestore());
    console.log("Office browser checks passed: native tables/bar/pie data, dark contrast, fallback/restoration, uniform font scaling, nonuniform/3D fallback, and CJK byte-bound table preservation.");
  } finally { await browser.close(); }
})().catch(e => {console.error(e);process.exit(1)});

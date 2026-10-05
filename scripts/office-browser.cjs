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
    });
    const fallback = await page.evaluate(extraction);
    assert.ok(!fallback.some(o => o.text?.includes("PIXEL_FALLBACK") || o.table?.rows.flat().some(c => c.text.includes("PIXEL_FALLBACK"))),"unsupported tables stay entirely captured");
    assert.equal(await page.locator("#merged").evaluate(el => el.style.visibility),"");
    assert.equal(await page.locator("#rich").evaluate(el => el.style.visibility),"");
    await page.evaluate(() => __slidesPPTXRestore());
    console.log("Office browser checks passed: native tables, bar/pie data, dark contrast, fallback and restoration.");
  } finally { await browser.close(); }
})().catch(e => {console.error(e);process.exit(1)});

const { chromium } = require(
  process.env.SLIDES_PLAYWRIGHT_MODULE || "playwright",
);
const assert = require("node:assert/strict"),
  fs = require("node:fs"),
  path = require("node:path");
(async () => {
  const browser = await chromium.launch({
    ...(process.env.SLIDES_BROWSER
      ? { executablePath: process.env.SLIDES_BROWSER }
      : {}),
    args: ["--enable-unsafe-swiftshader"],
  });
  try {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 720 },
    });
    await page.goto(process.argv[2] + "#opening");
    await page.waitForFunction(() => window.SlidesNav);
    await page.evaluate(() => SlidesNav.show(0, 0, true));
    await page.evaluate(() => {
      const slide = document.querySelector(".slide.deck-active");
      const clip = document.createElement("div");
      clip.id = "export-clip";
      clip.style.cssText =
        "position:absolute;left:100px;top:140px;width:60px;height:30px;overflow:hidden;white-space:nowrap;color:rgb(0,0,0);font:20px Arial";
      clip.textContent = "CLIPPED_TEXT_MUST_STAY_CAPTURED";
      slide.append(clip);
      const plain = document.createElement("div");
      plain.style.cssText =
        "position:absolute;left:100px;top:190px;width:400px;height:30px;color:rgb(0,0,0);font:20px Arial";
      plain.textContent = "NATIVE_TEXT_CONTROL";
      slide.append(plain);
      const shapes = document.createElement("div");
      shapes.style.cssText =
        "position:absolute;left:100px;top:240px;width:60px;height:40px;overflow:hidden";
      shapes.innerHTML =
        '<svg width="200" height="40" style="overflow:visible"><rect id="export-clipped-shape" x="0" y="0" width="100" height="15" fill="#ff0000"/><rect id="export-contained-shape" x="5" y="20" width="10" height="10" fill="#00ff00"/></svg>';
      slide.append(shapes);
      const rounded = document.createElement("div");
      rounded.style.cssText =
        "position:absolute;left:100px;top:290px;width:80px;height:50px;overflow:hidden;border-radius:12px";
      rounded.innerHTML =
        '<svg width="80" height="50" style="overflow:visible"><rect x="0" y="0" width="15" height="15" fill="#ff00ff"/><rect x="20" y="15" width="15" height="15" fill="#0000ff"/></svg>';
      slide.append(rounded);
    });
    const objects = await page.evaluate(
      fs.readFileSync(
        path.join(__dirname, "../assets/pptx-editable.js"),
        "utf8",
      ),
    );
    assert.ok(
      objects.some((o) => o.text === "NATIVE_TEXT_CONTROL"),
      "unclipped text remains editable",
    );
    assert.ok(
      !objects.some((o) => o.text?.includes("CLIPPED_TEXT")),
      "overflow-clipped text stays captured",
    );
    assert.ok(
      !objects.some((o) => o.fill === "ff0000"),
      "overflow-clipped shape stays captured",
    );
    assert.ok(
      objects.some((o) => o.fill === "00ff00"),
      "contained shape remains editable",
    );
    assert.equal(
      await page
        .locator("#export-clipped-shape")
        .evaluate((el) => el.style.visibility),
      "",
    );
    assert.equal(
      await page
        .locator("#export-clip")
        .evaluate((el) => el.firstChild.nodeType === Node.TEXT_NODE),
      true,
    );
    assert.ok(
      !objects.some((o) => o.fill === "ff00ff"),
      "rounded corner content stays captured",
    );
    assert.ok(
      objects.some((o) => o.fill === "0000ff"),
      "inset rounded-panel content stays editable",
    );
    await page.evaluate(() => window.__slidesPPTXRestore());
    assert.equal(
      await page
        .locator("#export-contained-shape")
        .evaluate((el) => el.style.visibility),
      "",
    );
    console.log(
      "PASS editable PPTX overflow fallback, contained geometry and restoration",
    );
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e);
  process.exit(1);
});

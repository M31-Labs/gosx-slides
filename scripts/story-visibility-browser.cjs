const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
const script = fs.readFileSync(path.join(__dirname, '../assets/semantic-story.js'), 'utf8');

(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'], ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}) });
  const page = await browser.newPage();
  try {
    for (const test of [
      { name: 'hidden ancestor', content: '<div data-story-id="outer"><span data-story-id="target">Hidden</span></div>', hide: ['outer'], visible: false },
      { name: 'opacity-zero ancestor', content: '<div style="opacity:0"><span data-story-id="target">Hidden</span></div>', visible: false },
      { name: 'display-none ancestor', content: '<div style="display:none"><span data-story-id="target">Hidden</span></div>', visible: false },
      { name: 'zero geometry', content: '<span data-story-id="target" style="display:block;width:0;height:0;font-size:0">Hidden</span>', visible: false },
      { name: 'offscreen geometry', content: '<span data-story-id="target" style="position:absolute;left:-9999px">Hidden</span>', visible: false },
      { name: 'visible control', content: '<span data-story-id="target">Visible</span>', visible: true },
      // Morph snapshots intentionally repeat stable actor IDs. Assertions must
      // inspect the rendered candidate, rather than the first hidden snapshot.
      { name: 'hidden earlier SVG snapshot', content: '<div hidden><svg width="80" height="50"><g data-sirena-id="target"><rect width="40" height="40"/></g></svg></div><svg width="80" height="50"><g data-sirena-id="target"><rect width="40" height="40"/></g></svg>', visible: true },
    ]) {
      for (const expected of [true, false]) {
        const story = { beats: [{ slideIndex: 0, step: 0, hide: test.hide, expect: { [expected ? 'visible' : 'hidden']: ['target'] } }], graphs: {} };
        await page.setContent('<main class="deck"><section class="slide deck-active" data-slide="0">' + test.content + '</section></main><script type="application/json" id="slides-story">' + JSON.stringify(story) + '</script><script>window.SlidesNav={current:()=>1,step:()=>0}</script>');
        await page.addScriptTag({ content: script });
        const result = await page.evaluate(() => SlidesStory.assertCurrent());
        assert.equal(result.checks, 1);
        assert.equal(result.errors.length, expected === test.visible ? 0 : 1, test.name + ': actual rendered visibility differs');
      }
    }
    console.log('Story visibility browser checks passed: ancestors, zero/offscreen geometry and visible SVG morph candidates.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });

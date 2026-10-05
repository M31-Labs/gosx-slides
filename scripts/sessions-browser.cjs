const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const fixture = fs.mkdtempSync(path.resolve('testdata/sessions-browser-'));
  const sourcePath = path.join(fixture, 'deck.md');
  const editorToken = 'editor-' + 'e'.repeat(40), audienceToken = 'audience-' + 'a'.repeat(40);
  fs.writeFileSync(sourcePath, '# Shared room\n\nFirst slide.\n\n<!-- Private notes for presenter -->\n\n---\n\n# Second slide\n\nSecond slide.\n');
  fs.writeFileSync(path.join(fixture, 'editor-token'), editorToken, {mode:0o600});
  fs.writeFileSync(path.join(fixture, 'audience-token'), audienceToken, {mode:0o600});
  const reserve = http.createServer(); await new Promise(resolve => reserve.listen(0, '127.0.0.1', resolve));
  const port = reserve.address().port; await new Promise(resolve => reserve.close(resolve));
  const server = spawn(binary, ['serve', fixture, '--edit', '--host', '0.0.0.0', '--port', String(port), '--session-http', '--editor-token-file', path.join(fixture,'editor-token'), '--audience-token-file', path.join(fixture,'audience-token')], {stdio:['ignore','ignore','inherit']});
  const url = 'http://127.0.0.1:'+port+'/';
  let browser;
  try {
    for(let i=0;;i++) {
      try {if ((await fetch(url+'_slides/session')).ok) break;} catch (_) {}
      if(i>300 || server.exitCode !== null) throw Error('Session server failed to start');
      await new Promise(resolve => setTimeout(resolve,100));
    }
    browser = await chromium.launch({args:['--no-sandbox'], ...(process.env.SLIDES_BROWSER ? {executablePath:process.env.SLIDES_BROWSER} : {})});
    const editorContext = await browser.newContext(), audienceContext = await browser.newContext();
    const editor = await editorContext.newPage(), audience = await audienceContext.newPage(), errors = [];
    for (const page of [editor, audience]) page.on('pageerror', error => errors.push(error.message));
    async function login(page, token) {
      await page.goto(url); assert.ok(page.url().endsWith('/_slides/session'));
      await page.getByLabel('Room token').fill(token);
      await page.getByRole('button',{name:'Join',exact:true}).click();
      await page.waitForFunction(() => !!window.SlidesNav);
    }
    await login(editor, editorToken); await login(audience, audienceToken);
    assert.equal(await editor.locator('meta[name="slides-edit"]').count(),1);
    assert.equal(await audience.locator('meta[name="slides-edit"]').count(),0);
    assert.equal(await audience.locator('.slide-notes').count(),0);
    assert.ok((await editor.locator('.slide-notes').textContent()).includes('Private notes'));
    const cookie = (await editorContext.cookies()).find(cookie => cookie.name === 'slides_session');
    assert.ok(cookie.httpOnly && cookie.sameSite === 'Strict');
    const forbidden = await audience.evaluate(async () => {
      const statuses = [];
      for (const route of ['/_slides/source','/_slides/team','/remote','/?present','/deck.md','/editor-token']) statuses.push((await fetch(route)).status);
      statuses.push((await fetch('/presenter/state', {method:'POST',headers:SlidesSessionHeaders({'Content-Type':'application/json'}),body:'{"index":1}'})).status);
      return statuses;
    });
    assert.deepEqual(forbidden,[403,403,403,403,404,404,403]);
    await editor.evaluate(() => SlidesNav.show(1,0,true));
    await audience.waitForFunction(() => SlidesNav.current() === 2);
    await audience.evaluate(() => SlidesNav.show(0,0,true));
    await editor.waitForTimeout(150);
    assert.equal(await editor.evaluate(() => SlidesNav.current()),2,'audience navigation changed presenter');
    const csrfDenied = await editor.evaluate(async () => (await fetch('/presenter/state', {method:'POST',headers:{'Content-Type':'application/json'},body:'{"index":0}'})).status);
    assert.equal(csrfDenied,403);
    await editor.keyboard.press('e');
    await editor.waitForFunction(() => document.querySelector('.slides-source-panel [data-status]')?.textContent === 'Ready to edit');
    await editor.locator('.slides-source-panel textarea').fill('# Shared room\n\nAuthenticated browser save.\n');
    await editor.getByRole('button',{name:'Save and preview',exact:true}).click();
    await editor.getByText('Authenticated browser save.',{exact:true}).waitFor();
    assert.ok(fs.readFileSync(sourcePath,'utf8').includes('Authenticated browser save.'));
    await editor.setViewportSize({width:320,height:720});
    await editor.mouse.move(160,680);
    await editor.locator('.deck-controls').evaluate(node => node.classList.add('deck-controls-visible'));
    const bounds = await editor.locator('.deck-controls').boundingBox();
    assert.ok(bounds.x >= 0 && bounds.x+bounds.width <= 321,'mobile toolbar exceeds viewport');
    const buttons = await editor.locator('.deck-controls button').evaluateAll(nodes => nodes.map(node => {const b=node.getBoundingClientRect();return {left:b.left,right:b.right,width:b.width,height:b.height}}));
    assert.ok(buttons.every(b => b.left>=0 && b.right<=321 && b.width>=40 && b.height>=40),'mobile toolbar shrinks controls');
    fs.mkdirSync(process.env.SLIDES_TEST_OUTPUT || 'browser-test-output',{recursive:true});
    await editor.screenshot({path:path.join(process.env.SLIDES_TEST_OUTPUT || 'browser-test-output','shared-room-mobile.png')});
    const loggedOut = await editor.evaluate(async () => (await fetch('/_slides/logout',{method:'POST',headers:SlidesSessionHeaders({})})).status);
    assert.equal(loggedOut,200); // fetch follows the redirect to the join page.
    assert.equal((await editorContext.request.get(url+'_slides/source')).status(),401);
    assert.deepEqual(errors,[]);
    console.log('Session browser checks passed: isolated audience/editor roles, private notes, source saves, CSRF, SSE, logout and mobile controls.');
  } finally {
    await browser?.close(); server.kill('SIGTERM'); fs.rmSync(fixture,{recursive:true,force:true});
  }
})().catch(error => {console.error(error);process.exitCode=1;});

const {spawn} = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');
const {launchTestBrowser} = require('./test-browser.cjs');

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const fixture = fs.mkdtempSync(path.resolve('testdata/team-session-revocation-'));
  const editorToken = 'editor-' + 'e'.repeat(40), audienceToken = 'audience-' + 'a'.repeat(40);
  fs.writeFileSync(path.join(fixture, 'deck.md'), '# Private room\n\nShared initial wording.\n\n<!-- Private speaker notes -->\n');
  for (const [name, content] of [['editor.token',editorToken], ['audience.token',audienceToken], ['session.secret','s'.repeat(64)]]) {
    fs.writeFileSync(path.join(fixture, name), content, {mode:0o600});
  }
  const reserve = http.createServer(); await new Promise(resolve => reserve.listen(0,'127.0.0.1',resolve));
  const port = reserve.address().port; await new Promise(resolve => reserve.close(resolve));
  const url = 'http://127.0.0.1:'+port+'/', args = ['serve',fixture,'--edit','--collab','--port',String(port),'--session-http','--editor-token-file',path.join(fixture,'editor.token'),'--audience-token-file',path.join(fixture,'audience.token'),'--session-secret-file',path.join(fixture,'session.secret')];
  let server, browser;
  async function start() {
    server = spawn(binary,args,{stdio:['ignore','ignore','inherit']});
    for(let i=0;;i++) {
      try { if((await fetch(url+'_slides/session')).ok) break; } catch (_) {}
      if(i>300 || server.exitCode!==null) throw Error('Session collaboration server failed to start');
      await new Promise(resolve => setTimeout(resolve,100));
    }
  }
  async function stop() { if(server?.exitCode===null) { server.kill('SIGTERM'); await new Promise(resolve => server.once('exit',resolve)); } }
  async function login(page,token) {
    await page.goto(url+'_slides/session'); await page.getByLabel('Room token').fill(token);
    await page.getByRole('button',{name:'Join',exact:true}).click(); await page.waitForURL(url); await page.waitForFunction(()=>!!window.SlidesNav);
  }
  async function openSocket(page) {
    await page.evaluate(() => {
      window.socketEvents = [];
      window.oldSocket = new WebSocket('ws://'+location.host+'/_slides/team');
      oldSocket.addEventListener('message',event=>socketEvents.push(JSON.parse(event.data)));
    });
    await page.waitForFunction(()=>oldSocket.readyState===WebSocket.OPEN && socketEvents.some(event=>event.event==='team:state'));
    await page.evaluate(()=>oldSocket.send(JSON.stringify({event:'team:presence',data:{label:'Revocable editor',slide:0,step:0}})));
  }
  async function waitClosed(page) { await page.waitForFunction(()=>oldSocket.readyState===WebSocket.CLOSED,undefined,{timeout:3000}); }
  async function replay(cookie) {
    return (await fetch(url+'_slides/source',{headers:{Cookie:cookie.name+'='+cookie.value}})).status;
  }
  try {
    await start(); browser = await launchTestBrowser({args:['--no-sandbox']});
    const context = await browser.newContext(), observerContext = await browser.newContext();
    const editor = await context.newPage(), session = await context.newPage(), observer = await observerContext.newPage();
    const errors=[]; [editor,session,observer].forEach(page=>page.on('pageerror',error=>errors.push(error.message)));
    await login(editor,editorToken); await login(observer,editorToken);
    await observer.getByRole('button',{name:'Shared draft and review',exact:true}).click();
    await observer.waitForFunction(()=>SlidesTeam.state().connected&&SlidesTeam.state().text);
    await editor.getByRole('button',{name:'Shared draft and review',exact:true}).click();
    await editor.waitForFunction(()=>SlidesTeam.state().connected&&SlidesTeam.state().text);
    await openSocket(editor);
    await observer.waitForFunction(()=>document.querySelector('.slides-team-presence').textContent.includes('Revocable editor'));
    const editorCookie = (await context.cookies()).find(cookie=>cookie.name==='slides_session');
    const originalDraft = await observer.evaluate(()=>SlidesTeam.state().text);
    const heldDraft=originalDraft+'\nPreserved unsent local change.';
    await editor.evaluate(text=>{
      const original=WebSocket.prototype.send;
      WebSocket.prototype.send=function(data){if(JSON.parse(data).event==='team:edit')return;return original.call(this,data);};
      const input=document.querySelector('.slides-team-draft');input.value=text;input.dispatchEvent(new Event('input',{bubbles:true}));SlidesTeam.flush();
    },heldDraft);

    // Keep the socket and page alive while a second tab signs out. The room
    // must revoke the first tab before it can request private drafts again.
    await session.goto(url); await session.getByRole('button',{name:'Leave presentation room',exact:true}).click();
    await session.waitForURL(url+'_slides/session'); await waitClosed(editor);
    await editor.waitForFunction(()=>!SlidesTeam.state().connected&&SlidesTeam.state().paused);
    assert.equal(await editor.evaluate(()=>SlidesTeam.state().text),heldDraft,'logout erased buffered local typing');
    assert.equal(await replay(editorCookie),401,'old cookie replayed after logout');
    assert.equal((await context.request.get(url+'_slides/source')).status(),401);
    await observer.waitForFunction(()=>!document.querySelector('.slides-team-presence').textContent.includes('Revocable editor'));
    const countAfterLogout = await editor.evaluate(()=>socketEvents.length);
    await observer.evaluate(()=>{const input=document.querySelector('.slides-team-draft');input.value+='\nOther editor changed private draft.';input.dispatchEvent(new Event('input',{bubbles:true}));SlidesTeam.flush();});
    await observer.waitForFunction(()=>!SlidesTeam.state().pending && SlidesTeam.state().text.includes('Other editor changed'));
    await editor.waitForTimeout(150);
    assert.equal(await editor.evaluate(()=>socketEvents.length),countAfterLogout,'revoked socket received private state');
    assert.ok(!await editor.evaluate(()=>socketEvents.some(event=>JSON.stringify(event).includes('Other editor changed'))));

    // Rejoining as an audience member in the same browser cannot restore the
    // previously opened editor socket or grant source/review privileges.
    await login(session,audienceToken);
    assert.equal(await session.locator('.slide-notes').count(),0);
    assert.equal(await session.evaluate(()=>typeof window.SlidesTeam),'undefined');
    assert.equal(await editor.evaluate(()=>oldSocket.readyState),3);
    assert.equal((await context.request.get(url+'_slides/source')).status(),403);
    assert.equal(await replay(editorCookie),401);

    // Direct editor-to-audience re-login also revokes an already open socket.
    await login(session,editorToken); await editor.goto(url); await openSocket(editor);
    const beforeDowngradeCookie=(await context.cookies()).find(cookie=>cookie.name==='slides_session');
    await login(session,audienceToken); await waitClosed(editor);
    assert.equal(await replay(beforeDowngradeCookie),401,'downgraded editor cookie replayed');
    assert.equal((await context.request.get(url+'_slides/team')).status(),403);
    await editor.evaluate(()=>oldSocket.send(JSON.stringify({event:'team:edit',data:{id:'late-edit',base:socketEvents.find(event=>event.event==='team:state').data.revision,index:0,insert:'Unauthorized'}})));
    await observer.waitForTimeout(100);
    assert.ok(!(await observer.evaluate(()=>SlidesTeam.state().text)).includes('Unauthorized'));
    assert.ok((await observer.evaluate(()=>SlidesTeam.state().text)).startsWith(originalDraft));

    // A stable secret preserves the other editor while revoked IDs remain
    // rejected after server restart. No grant ledger appears on public routes.
    const liveCookie=(await observerContext.cookies()).find(cookie=>cookie.name==='slides_session');
    await stop(); await start();
    assert.equal(await replay(liveCookie),200,'stable restart lost active editor');
    assert.equal(await replay(editorCookie),401); assert.equal(await replay(beforeDowngradeCookie),401);
    for(const route of ['.slides-sessions.json','public/.slides-sessions.json','.slides-team.json']) {
      const response=await observerContext.request.get(url+route); assert.equal(response.status(),404,'private state published at '+route);
    }
    assert.deepEqual(errors,[]);
    console.log('Session socket browser checks passed: second-tab logout, closed sockets, buffered local typing preserved, no private broadcasts, audience re-login, direct downgrade, old-cookie replay rejection, stable-secret restart and private state routes.');
  } finally { await browser?.close(); await stop(); fs.rmSync(fixture,{recursive:true,force:true}); }
})().catch(error=>{console.error(error);process.exitCode=1;});

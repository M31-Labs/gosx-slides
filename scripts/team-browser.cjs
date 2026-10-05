const { spawn }=require('node:child_process');
const fs=require('node:fs');
const path=require('node:path');
const http=require('node:http');
const assert=require('node:assert/strict');
const { chromium }=require(process.env.SLIDES_PLAYWRIGHT_MODULE||'playwright');

(async()=>{
  const binary=path.resolve(process.argv[2]||'./slides'),fixture=fs.mkdtempSync(path.resolve('testdata/team-browser-'));
  const initial='---\ntitle: Shared authoring\ntheme: swiss\noffline-required: true\n---\n\n```yaml\nid: opening\n```\n\n# Unicode 🙂 café\n\nShared body for collaboration.\n\n<!-- Private author note -->\n';
  fs.writeFileSync(path.join(fixture,'deck.md'),initial);
  const reserve=http.createServer();await new Promise(resolve=>reserve.listen(0,'127.0.0.1',resolve));const port=reserve.address().port;await new Promise(resolve=>reserve.close(resolve));
  const url='http://127.0.0.1:'+port+'/';let server,browser,sessions=false;
  const editorToken='editor-'+require('node:crypto').randomBytes(32).toString('hex'),audienceToken='audience-'+require('node:crypto').randomBytes(32).toString('hex');
  fs.writeFileSync(path.join(fixture,'editor.token'),editorToken,{mode:0o600});fs.writeFileSync(path.join(fixture,'audience.token'),audienceToken,{mode:0o600});
  async function start(){
    const args=['serve',fixture,'--port',String(port),'--edit','--collab'];
    if(sessions)args.push('--editor-token-file',path.join(fixture,'editor.token'),'--audience-token-file',path.join(fixture,'audience.token'),'--session-http');
    server=spawn(binary,args,{stdio:['ignore','ignore','inherit']});
    for(let i=0;;i++){try{if((await fetch(url)).ok)break;}catch(_){}if(i>200||server.exitCode!==null)throw Error('Team server did not start');await new Promise(resolve=>setTimeout(resolve,100));}
  }
  async function stop(){if(server?.exitCode===null){server.kill('SIGTERM');await new Promise(resolve=>server.once('exit',resolve));}}
  async function ready(page){await page.waitForFunction(()=>window.SlidesTeam?.state().connected&&SlidesTeam.state().text&& !SlidesTeam.state().pending);}
  async function setDraft(page,text,flush=false){await page.evaluate(({text,flush})=>{const input=document.querySelector('.slides-team-draft');input.value=text;input.dispatchEvent(new Event('input',{bubbles:true}));if(flush)SlidesTeam.flush();},{text,flush});}
  async function converged(a,b,expected){await Promise.all([a,b].map(page=>page.waitForFunction(expected=>{const state=SlidesTeam.state();return !state.pending&&!state.paused&&expected.every(text=>state.text.includes(text))&&state.text===state.latest;},expected)));assert.equal(await a.evaluate(()=>SlidesTeam.state().text),await b.evaluate(()=>SlidesTeam.state().text));}
  try{
    await start();browser=await chromium.launch({args:['--no-sandbox'],...(process.env.SLIDES_BROWSER?{executablePath:process.env.SLIDES_BROWSER}:{})});
    const a=await browser.newPage({viewport:{width:1280,height:720}}),b=await browser.newPage();const errors=[];
    [a,b].forEach(page=>page.on('pageerror',error=>errors.push(error.message)));
    await Promise.all([a,b].map(page=>page.goto(url)));
    // No authoring websocket connects or source/notes are fetched before the
    // editor explicitly opens the shared authoring surface.
    assert.equal(await a.evaluate(()=>SlidesTeam.state().connected),false);
    await Promise.all([a,b].map(page=>page.getByRole('button',{name:'Shared draft and review',exact:true}).click()));await Promise.all([ready(a),ready(b)]);
    await a.getByLabel('Display name').fill('<b>Ada</b>');await a.getByLabel('Display name').dispatchEvent('change');
    await b.getByLabel('Display name').fill('Lin');await b.getByLabel('Display name').dispatchEvent('change');
    await a.waitForFunction(()=>document.querySelector('.slides-team-presence').textContent.includes('Lin'));
    assert.equal(await a.locator('.slides-team-presence b').count(),0);
    const base=await a.evaluate(()=>SlidesTeam.state().text);
    await Promise.all([setDraft(a,base.replace('Shared body','First editor 🙂 Shared body')),setDraft(b,base.replace('collaboration.','collaboration. Second editor é'))]);
    await converged(a,b,['First editor 🙂','Second editor é']);

    // Type again during an in-flight splice. A branch base allows buffered
    // typing to merge without erasing the other editor's changes.
    await a.evaluate(()=>{
      const input=document.querySelector('.slides-team-draft');input.value+='\nBuffered first';input.dispatchEvent(new Event('input',{bubbles:true}));SlidesTeam.flush();
      input.value+=' and 🙂 second';input.dispatchEvent(new Event('input',{bubbles:true}));
    });
    await converged(a,b,['Buffered first and 🙂 second','Second editor é']);

    await a.locator('summary').filter({hasText:'Review comments'}).click();
    const literal='Review <img src=x onerror="window.injected=true"> literally.';
    await a.getByLabel('Comment',{exact:true}).fill(literal);await a.getByRole('button',{name:'Add review comment',exact:true}).click();
    await b.waitForFunction(()=>document.querySelector('.slides-team-comments')?.textContent.includes('Review <img'));
    assert.equal(await a.locator('.slides-team-comments img').count(),0);assert.equal(await a.evaluate(()=>window.injected),undefined);
    await a.getByRole('button',{name:'Resolve',exact:true}).click();await a.getByRole('button',{name:'Reopen',exact:true}).waitFor();
    await a.getByRole('button',{name:'Reopen',exact:true}).click();await a.getByRole('button',{name:'Resolve',exact:true}).waitFor();
    const commentID=await a.locator('.slides-team-comments li').getAttribute('data-comment-id');
    const beforeRestart=await a.evaluate(()=>SlidesTeam.state().text);
    await stop();await a.waitForFunction(()=>!SlidesTeam.state().connected);await start();await Promise.all([ready(a),ready(b)]);
    assert.equal(await a.evaluate(()=>SlidesTeam.state().text),beforeRestart);
    assert.equal(await a.locator('.slides-team-comments li').getAttribute('data-comment-id'),commentID);
    await a.getByRole('button',{name:'Publish deck.md',exact:true}).click();await a.waitForFunction(()=>document.querySelector('.slides-team-status').textContent.includes('Saved deck.md'));
    for(let i=0;i<50 && fs.readFileSync(path.join(fixture,'deck.md'),'utf8')!==beforeRestart;i++)await a.waitForTimeout(100);
    assert.equal(fs.readFileSync(path.join(fixture,'deck.md'),'utf8'),beforeRestart);

    // Hold an unsent local edit while the other editor advances enough states
    // to evict its base. The rejected splice keeps every local character.
    const held=await b.evaluate(()=>SlidesTeam.state().text)+'\nPreserved local tail';
    await b.evaluate(text=>{
      const original=WebSocket.prototype.send;window.releaseTeamEdit=null;
      WebSocket.prototype.send=function(data){if(JSON.parse(data).event==='team:edit'){window.releaseTeamEdit=()=>{WebSocket.prototype.send=original;original.call(this,data);};return;}return original.call(this,data);};
      const input=document.querySelector('.slides-team-draft');input.value=text;input.dispatchEvent(new Event('input',{bubbles:true}));SlidesTeam.flush();
    },held);
    for(let i=0;i<10;i++){const text=await a.evaluate(()=>SlidesTeam.state().text);await setDraft(a,'Prefix '+i+'\n'+text,true);await a.waitForFunction(()=>!SlidesTeam.state().pending);}
    await b.evaluate(()=>releaseTeamEdit());await b.waitForFunction(()=>SlidesTeam.state().paused);
    assert.equal(await b.evaluate(()=>SlidesTeam.state().text),held);
    await b.getByRole('button',{name:'Rebase local edits',exact:true}).click();await converged(a,b,['Prefix 9','Preserved local tail']);

    // A disk writer outside collaboration produces a visible revision conflict,
    // preserving both the external source and the shared draft.
    const draftBeforeConflict=await a.evaluate(()=>SlidesTeam.state().text);
    fs.writeFileSync(path.join(fixture,'deck.md'),'# External editor\n');
    await a.getByRole('button',{name:'Publish deck.md',exact:true}).click();await a.waitForFunction(()=>document.querySelector('.slides-team-status').textContent.includes('changed outside'));
    assert.equal(fs.readFileSync(path.join(fixture,'deck.md'),'utf8'),'# External editor\n');
    assert.equal(await a.evaluate(()=>SlidesTeam.state().text),draftBeforeConflict);
    await a.screenshot({path:'/tmp/slides-team-desktop.png'});await a.setViewportSize({width:390,height:844});await a.screenshot({path:'/tmp/slides-team-mobile.png'});
    assert.ok(await a.evaluate(()=>{const box=document.querySelector('.slides-team-panel').getBoundingClientRect();return box.left>=0&&box.right<=innerWidth&&box.height<=innerHeight;}));
    assert.ok(await a.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);

    // Session editors retain the same collaboration and source-save lane;
    // audience and anonymous sessions never see draft source or review state.
    await stop();fs.unlinkSync(path.join(fixture,'.slides-team.json'));fs.writeFileSync(path.join(fixture,'deck.md'),initial);sessions=true;await start();
    const editor=await browser.newPage(),audience=await browser.newPage();
    for(const [page,token]of [[editor,editorToken],[audience,audienceToken]]){
      await page.goto(url);assert.match(page.url(),/_slides\/session/);await page.getByLabel('Room token').fill(token);await page.getByRole('button',{name:'Join',exact:true}).click();await page.waitForURL(url);
    }
    assert.equal(await audience.evaluate(()=>typeof window.SlidesTeam),'undefined');
    const blocked=await audience.request.get(url+'_slides/team');assert.equal(blocked.status(),403);assert.ok(!(await blocked.text()).includes('Private author note'));
    assert.equal(await audience.evaluate(()=>new Promise(resolve=>{const socket=new WebSocket('ws://'+location.host+'/_slides/team');socket.onopen=()=>{socket.close();resolve(true);};socket.onerror=()=>resolve(false);})),false);
    await editor.getByRole('button',{name:'Shared draft and review',exact:true}).click();await ready(editor);
    await setDraft(editor,initial+'\nAuthenticated editor saved 🙂\n',true);await editor.waitForFunction(()=>!SlidesTeam.state().pending);
    await editor.getByRole('button',{name:'Publish deck.md',exact:true}).click();await editor.waitForFunction(()=>document.querySelector('.slides-team-status').textContent.includes('Saved deck.md'));
    assert.match(fs.readFileSync(path.join(fixture,'deck.md'),'utf8'),/Authenticated editor saved 🙂/);
    console.log('Team browser checks passed: concurrent Unicode edits, buffered typing, plain reviews/presence, restart, publication, expired-base preservation/rebase, external conflict, responsive dialog, editor cookie+CSRF save and audience source exclusion.');
  }finally{if(browser)await browser.close();await stop();fs.rmSync(fixture,{recursive:true,force:true});}
})().catch(error=>{console.error(error);process.exitCode=1;});

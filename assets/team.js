(function () {
  'use strict';
  const deck=document.querySelector('main.deck'), nav=window.SlidesNav;
  if (!deck || !nav || deck.classList.contains('deck-presenter')) return;
  const panel=document.createElement('dialog'); panel.className='slides-team-panel';panel.setAttribute('aria-labelledby','slides-team-title');
  panel.innerHTML='<h2 id="slides-team-title">Shared draft and review</h2><p>Changes sync with connected editors and stay in this deck folder. Publish validates and saves deck.md. Display names are chosen labels, not verified identities.</p><label>Display name<input data-team-name maxlength="64" value="Editor" autocomplete="off"></label><ul class="slides-team-presence" aria-label="Connected editors"></ul><p class="slides-team-status" role="status" aria-live="polite">Open this panel to connect.</p><label>Shared Markdown draft<textarea class="slides-team-draft" spellcheck="false" aria-label="Shared Markdown draft" disabled></textarea></label><div class="slides-team-actions"><button data-team-publish disabled>Publish deck.md</button><button data-team-copy>Copy local draft</button><button data-team-rebase disabled>Rebase local edits</button><button data-team-load disabled>Load shared draft</button><button data-team-close>Close</button></div><details><summary>Review comments</summary><p>Comments use the current slide ID and cue, or a selected source quote. Source quotes stay on this authoring surface.</p><label>Comment<textarea data-team-comment maxlength="4096" rows="2"></textarea></label><button data-team-add disabled>Add review comment</button><ul class="slides-team-comments"></ul></details>';
  deck.append(panel);
  const openButton=document.createElement('button');openButton.type='button';openButton.textContent='♧';openButton.title='Shared draft and review';openButton.setAttribute('aria-label','Shared draft and review');deck.querySelector('.deck-controls')?.append(openButton);
  const draft=panel.querySelector('.slides-team-draft'), status=panel.querySelector('.slides-team-status'), label=panel.querySelector('[data-team-name]');
  const publishButton=panel.querySelector('[data-team-publish]'), loadButton=panel.querySelector('[data-team-load]'), rebaseButton=panel.querySelector('[data-team-rebase]');
  let socket=null, clientID='', latest=null, baseText='', baseRevision='', pending=null, paused=false, connected=false, reconnect=0, debounce=0, acknowledgment=0, serial=0, returnedFocus=null, publishing=false, notice='';
  const headers=value=>window.SlidesSessionHeaders?window.SlidesSessionHeaders(value):value;
  function clipBytes(value,limit){while(new TextEncoder().encode(value).length>limit)value=Array.from(value).slice(0,-1).join('');return value;}
  function send(event,data) { if(socket?.readyState===WebSocket.OPEN)socket.send(JSON.stringify({event,data}));else throw Error('Connection is offline; your local draft is preserved.'); }
  function changed() { return draft.value!==baseText; }
  function controls() {
    draft.disabled=!latest || publishing;publishButton.disabled=!connected || !latest || paused || publishing || Boolean(pending) || changed();
    loadButton.disabled=!latest || publishing || Boolean(pending);rebaseButton.disabled=!latest || !paused || publishing;
    panel.querySelector('[data-team-add]').disabled=!connected || !latest;
  }
  function patch(before,after) {
    const a=Array.from(before),b=Array.from(after);let first=0,lastA=a.length,lastB=b.length;
    while(first<lastA && first<lastB && a[first]===b[first])first++;
    while(lastA>first && lastB>first && a[lastA-1]===b[lastB-1]){lastA--;lastB--;}
    return {index:first,delete:lastA-first,insert:b.slice(first,lastB).join('')};
  }
  function flush() {
    clearTimeout(debounce);if(!connected || !latest || pending || paused || publishing || !changed())return;
    const edit=patch(baseText,draft.value), deleted=Array.from(baseText).slice(edit.index,edit.index+edit.delete).join('');
    if(new TextEncoder().encode(edit.insert).length>16384 || new TextEncoder().encode(deleted).length>16384){paused=true;status.textContent='This edit exceeds 16 KiB. Your local draft is preserved; split the edit or rebase it in smaller changes.';controls();return;}
    pending={id:clientID+'-'+(++serial),text:draft.value};
    send('team:edit',{id:pending.id,base:baseRevision,...edit});controls();
    acknowledgment=setTimeout(()=>{if(!pending)return;pending=null;paused=true;notice='Edit acknowledgment timed out. Your local draft is preserved; verify the shared draft and rebase.';status.textContent=notice;controls();if(connected)send('team:sync',{});},10000);
  }
  function renderComments(comments) {
    const list=panel.querySelector('.slides-team-comments');list.replaceChildren();
    (comments||[]).forEach(comment=>{
      const item=document.createElement('li');item.dataset.commentId=comment.id;item.dataset.resolved=String(comment.resolved);
      const anchor=document.createElement('small');anchor.textContent=comment.label+' · '+(comment.slide ? '#'+comment.slide+(comment.cue?'/'+comment.cue:'') : 'Source quote: '+comment.quote)+(comment.resolved?' · resolved':'');
      const body=document.createElement('p');body.textContent=comment.text;
      const action=document.createElement('button');action.type='button';action.textContent=comment.resolved?'Reopen':'Resolve';action.addEventListener('click',()=>send('team:resolve',{id:comment.id,resolved:!comment.resolved}));
      item.append(anchor,body,action);list.append(item);
      const locate=document.createElement('button');locate.type='button';locate.textContent=comment.slide?'Go to slide':'Locate quote';
      locate.addEventListener('click',()=>{if(comment.slide){close();location.hash=encodeURIComponent(comment.slide)+(comment.cue?'/'+encodeURIComponent(comment.cue):'');}else{const at=draft.value.indexOf(comment.quote);if(at<0){status.textContent='This source quote is no longer in the draft.';return;}draft.focus();draft.setSelectionRange(at,at+comment.quote.length);}});
      item.append(' ',locate);
    });
  }
  function received(data) {
    const first=!latest;latest=data;publishing=data.publishing;renderComments(data.comments);
    if(pending && data.ack?.client===clientID && data.ack.id===pending.id) {
      clearTimeout(acknowledgment);
      notice='';
      if(draft.value===pending.text){draft.value=data.text;baseText=data.text;baseRevision=data.revision;}
      else {baseText=pending.text;baseRevision=data.ack.branch;}
      pending=null;
    } else if(!pending && draft.value===data.text){baseText=data.text;baseRevision=data.revision;if(paused){paused=false;notice='';}}
    else if(first || (!pending && !changed())) {draft.value=data.text;baseText=data.text;baseRevision=data.revision;}
    status.textContent=notice || (paused?'Sync paused. Your local draft is preserved; rebase or copy it before loading the shared draft.':publishing?'An editor is publishing; local pending edits will resume afterward.':pending || changed()?'Syncing your local edits; remote changes will appear after acknowledgment.':'Shared draft is up to date. Publishing checks the saved disk revision.');
    controls();if(!publishing)flush();
  }
  function presence() { if(connected)send('team:presence',{label:clipBytes(label.value,64),slide:nav.current(),step:nav.step()}); }
  function connect() {
    clearTimeout(reconnect);if(socket && [WebSocket.OPEN,WebSocket.CONNECTING].includes(socket.readyState))return;
    socket=new WebSocket((location.protocol==='https:'?'wss:':'ws:')+'//'+location.host+'/_slides/team');
    socket.addEventListener('open',()=>{connected=true;status.textContent='Connected. Loading the shared draft…';controls();presence();});
    socket.addEventListener('message',event=>{
      try {
        const message=JSON.parse(event.data),data=message.data;
        if(message.event==='__welcome'){clientID=data.clientId;presence();}
        else if(message.event==='team:state')received(data);
        else if(message.event==='team:presence'){
          const list=panel.querySelector('.slides-team-presence');list.replaceChildren();
          data.sort((a,b)=>a.id.localeCompare(b.id)).forEach(member=>{const node=document.createElement('li');node.textContent=member.label+(member.id===clientID?' (you)':'')+' · slide '+member.slide;list.append(node);});
        } else if(message.event==='team:error'){
          if(pending && data.id===pending.id){clearTimeout(acknowledgment);pending=null;paused=!data.retry;send('team:sync',{});}
          notice=data.message;status.textContent=notice;controls();
          if(data.retry)setTimeout(()=>{if(connected)send('team:sync',{});},1000);
        } else if(message.event==='team:comment-saved'){
          const comment=panel.querySelector('[data-team-comment]');if(comment.value.trim()===data.text)comment.value='';
        } else if(message.event==='team:publish-ready')publish(data);
      } catch(error) {status.textContent=error.message;}
    });
    socket.addEventListener('close',()=>{
      connected=false;publishing=false;
      clearTimeout(acknowledgment);
      if(pending || changed()){paused=true;pending=null;}
      status.textContent='Connection lost. Your local draft is preserved. Reconnect, then rebase any unsent changes.';controls();
      if(panel.open)reconnect=setTimeout(connect,1500);
    });
    socket.addEventListener('error',()=>{status.textContent='Editor connection unavailable. Sign in as an editor or check collaboration serving.';});
  }
  function open(){returnedFocus=document.activeElement;if(!panel.open)panel.showModal();connect();}
  function close(){panel.close();(returnedFocus?.offsetParent?returnedFocus:openButton).focus?.();}
  async function publish(data) {
    try {
      const response=await fetch('/_slides/source',{cache:'no-store',headers:headers({})}),source=await response.json();
      if(!response.ok)throw Error(source.error||'Could not load authoring token.');
      if(source.revision!==data.diskRevision){
        if(source.source===data.text){send('team:publish',{phase:'finish'});notice='Saved deck.md already matches the shared draft. Publication verified.';status.textContent=notice;return;}
        throw Error('deck.md changed outside the shared draft. Your draft is preserved; reconcile the disk before publishing.');
      }
      const saved=await fetch('/_slides/source',{method:'PUT',headers:headers({'Content-Type':'application/json','X-Slides-Token':source.token}),body:JSON.stringify({source:data.text,revision:data.diskRevision})});
      const result=await saved.json();if(!saved.ok)throw Error(result.error||'Publish validation failed.');
      send('team:publish',{phase:'finish'});notice='Saved deck.md. Reload the presentation to view the published content.';status.textContent=notice;
    } catch(error){try{send('team:publish',{phase:'cancel'});}catch(_){}notice=error.message;status.textContent=notice;}
  }
  function rebase() {
    if(!latest)return;
    const edit=patch(baseText,draft.value),old=Array.from(baseText),shared=Array.from(latest.text);
    const prefix=old.slice(Math.max(0,edit.index-24),edit.index).join(''),removed=old.slice(edit.index,edit.index+edit.delete).join(''),suffix=old.slice(edit.index+edit.delete,edit.index+edit.delete+24).join('');
    const anchor=prefix+removed+suffix,at=latest.text.indexOf(anchor);
    if(!anchor || at<0 || latest.text.indexOf(anchor,at+1)>=0){status.textContent='Automatic rebase cannot identify this edit uniquely. Copy your local draft, load the shared draft, and apply the intended change manually.';return;}
    const offset=Array.from(latest.text.slice(0,at)).length+Array.from(prefix).length;
    shared.splice(offset,edit.delete,...Array.from(edit.insert));draft.value=shared.join('');baseText=latest.text;baseRevision=latest.revision;paused=false;notice='';status.textContent='Local edits rebased onto the current shared draft.';controls();flush();
  }
  draft.addEventListener('input',()=>{if(!paused)notice='';controls();clearTimeout(debounce);debounce=setTimeout(flush,250);});
  label.addEventListener('change',presence);deck.addEventListener('slides:change',presence);
  openButton.addEventListener('click',open);panel.querySelector('[data-team-close]').addEventListener('click',close);
  panel.addEventListener('cancel',()=>{returnedFocus?.focus?.();});
  publishButton.addEventListener('click',()=>{notice='';if(latest)send('team:publish',{phase:'begin',revision:latest.revision});});
  panel.querySelector('[data-team-copy]').addEventListener('click',async()=>{try{await navigator.clipboard.writeText(draft.value);status.textContent='Local draft copied.';}catch(_){draft.focus();draft.select();status.textContent='Select and copy the local draft with your keyboard.';}});
  rebaseButton.addEventListener('click',rebase);
  loadButton.addEventListener('click',()=>{if(changed() && !window.confirm('Replace this local draft with the current shared draft? Copy your local draft first to keep it.'))return;draft.value=latest.text;baseText=latest.text;baseRevision=latest.revision;pending=null;paused=false;notice='';controls();status.textContent='Current shared draft loaded.';});
  panel.querySelector('[data-team-add]').addEventListener('click',()=>{
    const slide=deck.querySelector(':scope > .slide[data-slide="'+(nav.current()-1)+'"]'),text=panel.querySelector('[data-team-comment]');
    const quote=clipBytes(draft.value.slice(draft.selectionStart,draft.selectionEnd),512),id=slide?.dataset.slideId||'';
    if(!id && !quote){status.textContent='Give this slide a stable id, or select a source quote in the shared draft.';return;}
    let hash=location.hash.slice(1).split('/');try{hash=hash.map(decodeURIComponent);}catch(_){}
    send('team:comment',{slide:id,cue:hash[1]&&!/^\d+$/.test(hash[1])?hash[1]:'',quote,text:text.value});
  });
  window.addEventListener('pagehide',()=>{clearTimeout(reconnect);clearTimeout(debounce);clearTimeout(acknowledgment);socket?.close();});
  window.SlidesTeam={open,close,state:()=>({connected,paused,pending:Boolean(pending),text:draft.value,revision:baseRevision,latest:latest?.text}),flush,rebase};
})();

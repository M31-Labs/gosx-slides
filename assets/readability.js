(function () {
  'use strict';
  const deck = document.querySelector('main.deck'); if (!deck) return;
  function rgb(value) { const m = /^rgba?\(([\d.]+)[ ,]+([\d.]+)[ ,]+([\d.]+)(?:[, /]+([\d.]+))?\)$/.exec(value); return m && (!m[4] || Number(m[4]) === 1) ? m.slice(1,4).map(Number) : null; }
  function luminance(c) { return c.map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }).reduce((n, v, i) => n + v * [.2126, .7152, .0722][i], 0); }
  function check(sceneOnly = false) {
    const slide = deck.querySelector('.slide.deck-active'), warnings = [];
    if (!slide) return warnings;
    const scale = Math.min(1, slide.getBoundingClientRect().height / slide.offsetHeight);
    if (!sceneOnly) slide.querySelectorAll('h1,h2,h3,p,li,td,th,code,svg text').forEach(el => {
      if (!el.textContent.trim() || !el.getClientRects().length || el.closest('[aria-hidden="true"], [data-slides-cue-visible="false"], .slide-scene')) return;
      const css = getComputedStyle(el), matrix = el.getScreenCTM?.(), renderedScale = matrix ? Math.hypot(matrix.c,matrix.d) : scale, font = parseFloat(css.fontSize) * renderedScale;
      if (font < (el.matches('code,text') ? 14 : 18)) warnings.push({ kind: 'font-size', text: el.textContent.trim().slice(0,80), value: Math.round(font), advice: 'Increase type size or split this slide.' });
      let bg = null, node = el, complex = false;
      while (node && !bg) { const style = getComputedStyle(node); if (style.backgroundImage !== 'none' || Number(style.opacity) < 1) complex = true; bg = rgb(style.backgroundColor); node = node.parentElement; }
      const fg = rgb(css.color);
      if (fg && bg && !complex && css.backgroundImage === 'none') {
        const a = luminance(fg), b = luminance(bg), ratio = (Math.max(a,b)+.05)/(Math.min(a,b)+.05);
        if (ratio < 3) warnings.push({ kind: 'contrast', text: el.textContent.trim().slice(0,80), value: Number(ratio.toFixed(2)), advice: 'Choose colors with stronger contrast.' });
      }
    });
    slide.querySelectorAll('.slide-graphic .gosx-scene-label').forEach(el => {
      const text = el.textContent.trim().slice(0,80); if (!text) return;
      if (el.dataset.gosxSceneLabelVisibility === 'hidden') { warnings.push({kind:'hidden-label',text,value:'hidden',advice:'Check camera framing, occlusion and label collisions.'}); return; }
      const bounds = el.getBoundingClientRect(), surface = el.closest('.slide-graphic').querySelector('canvas')?.getBoundingClientRect();
      const css = getComputedStyle(el), font = parseFloat(css.fontSize) * scale;
      if (font < 14) warnings.push({kind:'font-size',text,value:Math.round(font),advice:'Increase the scene label font or give the scene more space.'});
      if (surface && (bounds.left < surface.left-1 || bounds.top < surface.top-1 || bounds.right > surface.right+1 || bounds.bottom > surface.bottom+1)) warnings.push({kind:'clipped-label',text,value:'clipped',advice:'Widen the camera field of view, move the camera back or reposition the actor.'});
      if (el.dataset.gosxSceneLabelTruncated === 'true') warnings.push({kind:'truncated-label',text,value:'truncated',advice:'Increase label width or shorten its text.'});
    });
    return warnings;
  }
  let scanning = false;
  async function scan(signal, progress) {
    if (scanning) throw new Error('A scene scan is already running.');
    const scenes = window.SlidesGraphicsMotion?.current() || [], state = SlidesMotion.state(), warnings = [];
    const count = Math.max(0,...scenes.map(scene=>scene.frames.length)); let poses = 0;
    if (!count) return {warnings,poses:0,limited:false};
    scanning = true; SlidesMotion.pause();
    try {
      for (let cue = 0; cue < count; cue++) {
        for (const fraction of cue ? [.5,1] : [1]) {
          if (signal?.aborted || poses >= 64) return {warnings,poses,limited:poses>=64};
          SlidesGraphicsMotion.inspect(cue,fraction); await SlidesGraphicsMotion.settled();
          await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
          poses++; warnings.push(...check(true).map(warning=>({...warning,cue,pose:fraction===1?'end':'midpoint'})));
          progress?.(poses);
        }
      }
      return {warnings,poses,limited:false};
    } finally {
      try { SlidesMotion.restore({...state,paused:true}); await SlidesMotion.settled(); }
      finally { SlidesMotion.restore(state); scanning = false; }
    }
  }
  function renderWarnings(list,warnings) {
    list.replaceChildren();
    if (!warnings.length) { const item = document.createElement('li'); item.textContent = 'No small-type, measured contrast or scene label warnings in the checked poses.'; list.append(item); }
    warnings.slice(0,200).forEach(warning => { const item = document.createElement('li'); item.textContent = (warning.cue==null?'':'Cue '+(warning.cue+1)+' '+warning.pose+' · ') + warning.kind + ' (' + warning.value + '): ' + warning.text + ' — ' + warning.advice; list.append(item); });
    if (warnings.length>200) { const item=document.createElement('li'); item.textContent='Showing the first 200 of '+warnings.length+' warnings.'; list.append(item); }
  }
  function open() {
    let panel = document.querySelector('.slides-readability-panel');
    if (!panel) { panel = document.createElement('dialog'); panel.className = 'slides-author-panel slides-readability-panel'; panel.setAttribute('aria-label', 'Readability check'); deck.appendChild(panel); }
    panel.replaceChildren(); const header = document.createElement('header'), title = document.createElement('h2'), close = document.createElement('button'); title.textContent = 'Readability check'; close.textContent = '×'; close.type = 'button'; close.setAttribute('aria-label', 'Close readability check'); close.onclick = () => panel.close(); header.append(title,close); panel.append(header);
    const intro = document.createElement('p'); intro.textContent = 'Checks visible type, SVG text and Scene3D labels at this viewport. Gradients, transparent colors, and custom compositing need visual inspection.'; panel.append(intro);
    const warnings = check(); const list = document.createElement('ul');
    renderWarnings(list,warnings); panel.append(list);
    if (window.SlidesGraphicsMotion?.current().length) {
      const button=document.createElement('button'), status=document.createElement('output'), controller=new AbortController();
      button.type='button'; button.textContent='Scan scene cues'; status.setAttribute('aria-live','polite'); panel.insertBefore(button,list); panel.insertBefore(status,list);
      panel.addEventListener('close',()=>controller.abort(),{once:true});
      button.onclick=async()=>{button.disabled=true;try{const result=await scan(controller.signal,poses=>{status.textContent='Checking scene pose '+poses+'…';});if(panel.open){renderWarnings(list,result.warnings);status.textContent='Checked '+result.poses+' scene poses'+(result.limited?' (64-pose limit).':'.')+' Timeline restored.';}}catch(error){status.textContent=error.message;}finally{button.disabled=false;}};
    }
    if (!panel.open) panel.showModal();
  }
  document.addEventListener('keydown', event => { if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest('input,textarea,select,[contenteditable],dialog,[role]')) return; if ((event.key === 'r' || event.key === 'R') && !SlidesNav.isOverview()) { event.preventDefault(); open(); } });
  window.SlidesReadability = { check, scan, open };
})();

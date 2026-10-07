package slides

import "m31labs.dev/gosx"

// WebPage is a managed native island, like the graphics components. Its shell
// is server-rendered; this controller owns iframe lifetime without a WASM VM.
func webPageAssets(d *IslandDeck) gosx.Node {
	if d.web == nil || len(d.web.pages) == 0 || d.web.static {
		return gosx.Text("")
	}
	return gosx.RawHTML(`<script data-slides-webpage>` + webPageScript + `</script>`)
}

const webPageStyle = `
main.deck .webpage{display:flex;flex-direction:column;width:100%;max-width:1280px;margin:0;min-height:0;gap:8px;font:14px/1.4 var(--font-body,system-ui)}
main.deck .webpage-viewport{position:relative;aspect-ratio:var(--web-aspect,16/9);max-height:52vh;min-height:160px;background:#eef1f5;border:1px solid #8d96a6;border-radius:10px;overflow:hidden}
main.deck .webpage-snapshot{position:absolute;inset:0;width:100%;height:100%;max-height:none;object-fit:contain;margin:0}
main.deck .webpage-placeholder{display:grid;place-content:center;height:100%;color:#394353}
main.deck .webpage-live{position:absolute;inset:0;overflow:auto}
main.deck .webpage-live iframe{display:block;border:0;background:white;transform-origin:top left;opacity:0}
main.deck .webpage[data-web-loaded="true"] iframe{opacity:1}
main.deck .webpage[data-web-loaded="true"] .webpage-snapshot,main.deck .webpage[data-web-loaded="true"] .webpage-placeholder{visibility:hidden}
main.deck .webpage[data-web-loaded="true"] .webpage-live{background:white}
main.deck .webpage-lock{position:absolute;inset:0;z-index:1;touch-action:none}
main.deck .webpage[data-web-unlocked="true"] .webpage-lock{display:none}
main.deck .webpage figcaption{display:flex;flex-wrap:wrap;justify-content:space-between;gap:4px 16px;font:12px/1.4 var(--font-body,system-ui);opacity:.8}
main.deck .webpage-url{overflow-wrap:anywhere}
main.deck .webpage-controls{display:flex;flex-wrap:wrap;gap:6px}
main.deck .webpage-controls button,main.deck .webpage-controls a{font:inherit;color:inherit;background:transparent;border:1px solid currentColor;border-radius:5px;padding:4px 10px;cursor:pointer;text-decoration:none}
main.deck .webpage-controls button:hover,main.deck .webpage-controls a:hover{background:#8793a422}
main.deck .webpage-controls :focus-visible{outline:3px solid var(--accent,#245bb0);outline-offset:3px}
main.deck.deck-presenter .pv-current>.pv-label{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:8px}
main.deck.deck-presenter .webpage-presenter-controls{display:flex;flex-wrap:wrap;gap:6px;font:14px/1.4 var(--font-body,system-ui);letter-spacing:normal;text-transform:none}
main.deck.deck-presenter .pv-screen .webpage-controls{display:none}
@media print{main.deck .webpage-live,main.deck .webpage-lock,main.deck .webpage-controls{display:none!important}}
`

const webPageScript = `(function(){
  'use strict';
  const nav=window.SlidesNav, deck=document.querySelector('main.deck');
  if(!nav || !deck || deck.dataset.offline==='1')return;
  const pages=Array.from(deck.querySelectorAll('[data-web-page]'));
  const channel=typeof BroadcastChannel==='function'?new BroadcastChannel('gosx-webpage:'+location.pathname):null;
  const source='web-'+Math.random().toString(36).slice(2),useServer=deck.dataset.liveSync==='1',seen=new Map();let sequence=0,events;
  // A deployment must never turn a remote component into deck-origin code,
  // including after a redirect. Narrow the delivered frame policy before mount.
  const policy=document.querySelector('meta[http-equiv="Content-Security-Policy"]');
  if(policy){const origins=policy.content.replace(/^frame-src\s+|;$/g,'').split(/\s+/).filter(s=>s!==location.origin);policy.content='frame-src '+(origins.join(' ')||"'none'")+';';}
  function slideOf(page){return page.closest('.slide');}
  function stop(page){const frame=page.querySelector('iframe');if(frame)frame.remove();delete page.dataset.webLoaded;}
  function speakerControls(){
    if(!nav.isPresenter())return;const label=deck.querySelector('.pv-current>.pv-label');if(!label)return;
    let controls=label.querySelector('.webpage-presenter-controls');if(!controls){controls=document.createElement('div');controls.className='webpage-controls webpage-presenter-controls';label.appendChild(controls);}
    const page=pages.find(p=>Number(slideOf(p).dataset.slide)+1===nav.current());
    controls.replaceChildren();if(page)page.querySelectorAll('.webpage-controls>*').forEach(button=>controls.appendChild(button.cloneNode(true)));
  }
  function lock(page){const frame=page.querySelector('iframe');if(!frame)return;const locked=page.dataset.webUnlocked!=='true';if(locked && document.activeElement===frame)frame.blur();frame.inert=locked;frame.tabIndex=locked?-1:0;}
  function size(page){
    const frame=page.querySelector('iframe');if(!frame)return;
    const width=Number(page.dataset.webWidth),height=Number(page.dataset.webHeight),port=page.querySelector('.webpage-live');
    const fit=page.dataset.webZoom!=='100',scale=fit?Math.min(port.clientWidth/width,port.clientHeight/height):1;
    frame.style.width=width+'px';frame.style.height=height+'px';frame.style.transform='scale('+scale+')';
    frame.style.position='absolute';frame.style.left=(fit?(port.clientWidth-width*scale)/2:0)+'px';frame.style.top=(fit?(port.clientHeight-height*scale)/2:0)+'px';
    port.style.overflow=fit?'hidden':'auto';
  }
  function sync(){
    let mounted=false;
    pages.forEach(page=>{
      const slide=slideOf(page),src=page.dataset.webSrc;
      const active=slide && Number(slide.dataset.slide)+1===nav.current();
      const permitted=src && new URL(src).origin!==location.origin;
      const show=active && permitted && !mounted && !nav.isPresenter() && navigator.onLine && !nav.isOverview() && deck.dataset.reading!=='1' && document.visibilityState!=='hidden';
      if(!show){stop(page);return;}mounted=true;
      if(!page.querySelector('iframe')){
        const frame=document.createElement('iframe');
        frame.title=page.dataset.webTitle;frame.setAttribute('sandbox',page.dataset.webSandbox||'');
        frame.referrerPolicy='no-referrer';frame.loading='lazy';
        frame.allow="autoplay 'none'; camera 'none'; microphone 'none'; geolocation 'none'; fullscreen 'none'; payment 'none'; display-capture 'none'; usb 'none'";
        frame.addEventListener('load',()=>{if(frame.isConnected)page.dataset.webLoaded='true';});
        frame.addEventListener('error',()=>stop(page));
        frame.src=src;page.querySelector('.webpage-live').appendChild(frame);
        lock(page);
      }
      size(page);
    });
    speakerControls();
  }
  function apply(page,action,value){
    if(action==='reload'){stop(page);sync();}
    if(action==='zoom'){page.dataset.webZoom=value||(page.dataset.webZoom==='100'?'fit':'100');const b=page.querySelector('[data-web-action="zoom"]');b.textContent=page.dataset.webZoom==='100'?'Fit':'100%';b.setAttribute('aria-pressed',String(page.dataset.webZoom==='100'));size(page);}
    if(action==='lock'){page.dataset.webUnlocked=value||(page.dataset.webUnlocked==='true'?'false':'true');const b=page.querySelector('[data-web-action="lock"]');b.textContent=page.dataset.webUnlocked==='true'?'Lock scroll':'Unlock scroll';b.setAttribute('aria-pressed',String(page.dataset.webUnlocked!=='true'));lock(page);}
    speakerControls();
  }
  function receive(message){const m=message.web||message;if(!m || !['reload','zoom','lock'].includes(m.action) || message.source===source || (seen.get(message.source)||0)>=message.sequence)return;seen.set(message.source,message.sequence);if(seen.size>64)seen.delete(seen.keys().next().value);const page=pages[m.page];if(page && Number(slideOf(page).dataset.slide)===message.index && message.index+1===nav.current() && page.querySelector('[data-web-action="'+m.action+'"]'))apply(page,m.action,m.value);}
  deck.addEventListener('click',event=>{
    const button=event.target.closest('[data-web-action]');if(!button)return;
    event.preventDefault();event.stopPropagation();const page=button.closest('[data-web-page]')||pages.find(p=>Number(slideOf(p).dataset.slide)+1===nav.current()),action=button.dataset.webAction;if(!page)return;
    apply(page,action);const message={index:Number(slideOf(page).dataset.slide),step:nav.step(),source,sequence:++sequence,web:{page:pages.indexOf(page),action,value:action==='zoom'?page.dataset.webZoom:action==='lock'?page.dataset.webUnlocked:''}};
    if(deck.dataset.sessionRole==='audience')return;
    if(useServer){const headers={'Content-Type':'application/json'};fetch('presenter/state',{method:'POST',headers:window.SlidesSessionHeaders?SlidesSessionHeaders(headers):headers,body:JSON.stringify(message)}).catch(()=>{if(channel)channel.postMessage(message);});}else if(channel)channel.postMessage(message);
  });
  if(channel)channel.onmessage=event=>receive(event.data);
  if(useServer && typeof EventSource==='function'){events=new EventSource('presenter/events');events.addEventListener('state',event=>{try{const message=JSON.parse(event.data);if(message.web)receive(message);}catch(e){}});}
  nav.onChange(sync);window.addEventListener('resize',()=>pages.forEach(size));
  window.addEventListener('online',sync);window.addEventListener('offline',sync);
  document.addEventListener('visibilitychange',sync);
  new MutationObserver(sync).observe(deck,{attributes:true,attributeFilter:['class','data-reading']});
  window.addEventListener('beforeprint',()=>pages.forEach(stop));window.addEventListener('afterprint',sync);
  window.addEventListener('pagehide',()=>{pages.forEach(stop);if(channel)channel.close();if(events)events.close();},{once:true});
  sync();
})();`

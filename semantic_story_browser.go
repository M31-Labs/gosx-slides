package slides

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"
)

// AssertStoryBrowser tests actual rendered states with the same local Chrome
// harness used by capture exports. It has a two-minute browser deadline and
// visits at most 100 authored beats. Navigation remains local to this server.
func AssertStoryBrowser(deck *IslandDeck) (StoryAssertionReport, error) {
	report, err := AssertStory(deck)
	if err != nil || deck.Story == nil {
		return report, err
	}
	if len(deck.Story.Beats) > 100 {
		return report, fmt.Errorf("rendered story assertions support at most 100 authored beats")
	}
	app, err := deck.NewServer(ServeOptions{StageRuntime: true, Static: true})
	if err != nil {
		return report, err
	}
	server := httptest.NewServer(app.Build())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		return report, err
	}
	defer browser.close()
	if err = browser.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 800, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		return report, err
	}
	if err = browser.call("Page.navigate", map[string]any{"url": server.URL}, nil); err != nil {
		return report, err
	}
	if err = browser.wait(`document.readyState==='complete' && !!window.SlidesStory && !!window.SlidesMotion`); err != nil {
		return report, err
	}
	var rendered StoryAssertionReport
	if err = browser.eval(storyBrowserAssertions, &rendered); err != nil {
		return report, err
	}
	report.Checks += rendered.Checks
	report.RenderedStates = rendered.RenderedStates
	report.Errors = append(report.Errors, rendered.Errors...)
	return report, nil
}

const storyBrowserAssertions = `(async()=>{
const report={checks:0,renderedStates:0,errors:[]}, remembered=new Map(), beats=SlidesStory.manifest.beats;
const paint=()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
const key=beat=>beat.slide+'/'+beat.cue;
async function ready(){
  for(let retry=0;retry<240;retry++){
    const slide=document.querySelector('.deck-active');
    if(!['pending','error'].includes(slide.dataset.slideHydration) && Array.from(slide.querySelectorAll('.slide-graphic')).every(el=>el.__gosxScene3DHandle?.__gosxScene3DCommandReady)) return;
    await new Promise(resolve=>setTimeout(resolve,50));
  }
  throw Error('story surface did not become ready');
}
async function pose(time){
  SlidesMotion.seek(time);await SlidesMotion.settled();await paint();
  const slide=document.querySelector('.deck-active'), mounts=Array.from(slide.querySelectorAll('.slide-graphic'));
  report.renderedStates++;
  return JSON.stringify({
    scenes:mounts.map(el=>{const debug=window.__gosx_scene3d_debug?.inspect(el.id);return {camera:debug?.camera,counts:debug?.counts,
      labels:Array.from(el.querySelectorAll('.gosx-scene-label')).map(label=>[label.dataset.gosxSceneLabel,label.textContent,label.style.cssText]).sort((a,b)=>a[0].localeCompare(b[0]))}}),
    svg:Array.from(slide.querySelectorAll('svg [data-sirena-id],svg .edge')).map(el=>[el.getAttribute('data-sirena-id')||el.getAttribute('data-morph-id'),el.style.opacity,el.getAttribute('transform'),el.querySelector('path')?.getAttribute('d')]),
    code:Array.from(slide.querySelectorAll('pre.code-block .ts-line')).map(el=>[el.textContent,el.style.opacity,el.dataset.storyCode]),
    dom:Array.from(slide.querySelectorAll('[data-story-id]')).map(el=>[el.dataset.storyId,el.hidden,el.inert,el.style.opacity,el.getAttribute('aria-hidden')]),
    caption:document.querySelector('.slides-story-caption')?.textContent
  });
}
await document.fonts.ready;SlidesMotion.pause();
for(const beat of beats){
  SlidesNav.preview(beat.slideIndex,beat.step);await ready();
  // A zero-duration story still has the deck's own entrance transition. Settle
  // the full presentation pose before making claims about rendered visibility.
  const duration=Math.max(beat.durationMs,SlidesMotion.duration());
  const end=await pose(duration), assertions=SlidesStory.assertCurrent();
  report.checks+=assertions.checks;assertions.errors.forEach(error=>report.errors.push(key(beat)+': '+error));
  remembered.set(key(beat),end);
  const middle=await pose(duration/2);await pose(0);
  report.checks++;if(await pose(duration/2)!==middle)report.errors.push(key(beat)+': repeated midpoint seek differs');
  report.checks++;if(await pose(duration)!==end)report.errors.push(key(beat)+': repeated final seek differs');
}
for(const beat of [...beats].reverse()){
  SlidesNav.preview(beat.slideIndex,beat.step);await ready();
  report.checks++;if(await pose(Math.max(beat.durationMs,SlidesMotion.duration()))!==remembered.get(key(beat)))report.errors.push(key(beat)+': backward navigation differs');
}
return report;
})()`

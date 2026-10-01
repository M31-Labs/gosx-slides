package slides

// Replay through GoSX's managed-motion lifecycle, preserving child engines and
// islands. Slides own replay policy; GoSX still owns keyframes and animation.
func motionReplayScript() string {
	return `(function(){
  var currentIndex = null, currentStep = null, attempts = 0;
  function api() { return window.__gosx && window.__gosx.motion; }
  function state() {
    var slide = document.querySelector('main.deck > .slide.deck-active');
    if (!slide) return null;
    return {slide: slide, index: Number(slide.getAttribute('data-slide')), step: Number(slide.getAttribute('data-active-step') || 0)};
  }
  function restart(element) {
    var motion = api();
    if (!motion || typeof motion.dispose !== 'function' || typeof motion.observe !== 'function') return;
    motion.dispose(element);
    // GoSX keeps this per-page reveal token to prevent accidental remount
    // flashes. An explicit slide replay intentionally starts a new entrance.
    element.removeAttribute('data-gosx-motion-revealed');
    element.setAttribute('data-gosx-motion-state', 'idle');
    motion.observe(element);
  }
  function changed(event) {
    var next = state();
    if (!next) return;
    var detail = event && event.detail;
    var step = detail && Number.isFinite(detail.step) ? detail.step : next.step;
    var entered = currentIndex !== null && currentIndex !== next.index;
    var stepped = currentStep !== null && currentStep !== step;
    if (entered) {
      var previous = document.querySelector('main.deck > .slide[data-slide="' + currentIndex + '"]');
      if (previous && api()) previous.querySelectorAll('[data-slides-motion-replay]').forEach(function(element) {
        if (element.getAttribute('data-slides-motion-replay') !== 'once') api().dispose(element);
      });
    }
    currentIndex = next.index; currentStep = step;
    if (!entered && !stepped) return;
    next.slide.querySelectorAll('[data-slides-motion-replay]').forEach(function(element) {
      if (element.hasAttribute('data-slides-motion-step') || element.hasAttribute('data-slides-motion-cue')) return;
      var mode = element.getAttribute('data-slides-motion-replay');
      if ((entered && mode !== 'once') || (stepped && mode === 'step')) restart(element);
    });
  }
  function ready() {
    if (!api()) { if (++attempts < 100) setTimeout(ready, 100); return; }
    changed();
    var deck = document.querySelector('main.deck');
    if (deck) deck.addEventListener('slides:change', changed);
  }
  if (document.querySelector('[data-slides-motion-replay]')) ready();
})();`
}

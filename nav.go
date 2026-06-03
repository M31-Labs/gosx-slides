package slides

// nav.go is the real lane's slide-navigation layer (Phase 1, Slice 6). The real
// lane (serve.go's renderPage) lowers every slide to a
// `<section class="slide" data-slide="N">…</section>` and stacks them in
// `<main class="deck">`; without this layer they render as one flat vertical
// scroll. navStyle hides every slide but the active one, and navScript is a
// small self-contained vanilla-JS controller that shows ONE slide at a time and
// wires keyboard + URL-hash navigation over the SAME data-slide sections.
//
// It is the real-lane counterpart to runtime_script.go's fallback controller and
// deliberately shares NONE of its code: the fallback controller drives canvases,
// polls, presenter SSE state and fallback-component step-reveals; this one keeps
// only the deck mechanics (show-one-slide, ←/→/Space, f-fullscreen, hash sync).
// renderPage injects navStyle into the document head and navScript at the END of
// the body (so the sections exist when it runs); see serve.go.
//
// Class-name note: the fallback lane (style.go) uses `.slide.is-active` and its
// own `.slide{display:none}` rule. The real lane never loads style.go, but to
// keep the two lanes unambiguous this controller uses a DISTINCT active class,
// `deck-active`, and scopes its display rule under `main.deck` so it cannot be
// confused with — or accidentally collide with — the fallback styling.

// navActiveClass is the CSS class navScript toggles onto the one visible slide.
// It is intentionally different from the fallback lane's `is-active` (style.go)
// so the two lanes' navigation never share a selector. Kept as a const so the
// style and the script are guaranteed to agree.
const navActiveClass = "deck-active"

// navStyle is the slide-visibility stylesheet for the real lane: inside
// `main.deck`, every `.slide` is hidden and only the one carrying navActiveClass
// is shown. Scoping under `main.deck` (the wrapper renderPage emits) keeps this
// rule from touching anything else and from colliding with the fallback lane's
// global `.slide{display:none}` rule. Returned as the inner CSS text (no <style>
// wrapper) so renderPage can place it via gosx.RawHTML.
func navStyle() string {
	// Hide every slide EXCEPT the active one, with !important so it beats a
	// theme's higher-specificity layout rule (e.g. `.slide.layout-title { display:
	// flex }`) — otherwise an inactive layout slide stays visible and stacks on
	// top of the active one. The active slide gets no display override here, so it
	// falls back to its theme layout display (flex for layout-title/center) or the
	// <section> default (block).
	//
	// The active slide also runs a short ENTER transition (fade + slight upward
	// settle) so advancing feels intentional, not a hard cut. It is purely an
	// opacity/transform animation on `.deck-active` and never touches `display`, so
	// it cannot disturb the visibility rule above (the only thing that controls
	// which slide is shown). It is gated behind `prefers-reduced-motion:
	// no-preference` so motion-sensitive viewers get an instant cut, and is theme-
	// agnostic (it lives here, beside the visibility rule both lanes depend on).
	// The easing matches the themes' settle curve (ease-out-quart).
	return `main.deck > .slide:not(.` + navActiveClass + `) { display: none !important; }
@media (prefers-reduced-motion: no-preference) {
  @keyframes slidesDeckEnter {
    from { opacity: 0; transform: translateY(14px); }
    to   { opacity: 1; transform: translateY(0); }
  }
  main.deck > .slide.` + navActiveClass + ` {
    animation: slidesDeckEnter 220ms cubic-bezier(0.25,1,0.5,1) both;
  }
}`
}

// navScript is the real lane's self-contained navigation controller, returned as
// the inner JS (no <script> wrapper) so renderPage can place it via
// gosx.RawHTML at the end of the body.
//
// Behavior:
//   - Collects every `[data-slide]` section under `main.deck` and orders them by
//     their numeric data-slide value (the generator emits 0-based indices).
//   - URL hash is 1-BASED (`#1` == first slide), matching the fallback lane's
//     convention and human expectation; it maps to array position (n-1).
//   - On load, reads `location.hash` (`#N`); a missing/invalid/out-of-range hash
//     defaults to slide 1. The chosen section gets navActiveClass; all others
//     have it removed.
//   - keydown: ArrowRight or Space -> next, ArrowLeft -> prev, `f` -> toggle
//     fullscreen. Typing in an input/textarea/select is ignored. Arrow/Space
//     default scrolling is prevented.
//   - On every change, `history.replaceState(null, '', '#'+n)` keeps the URL in
//     sync without polluting history; `#N` deep-links on reload.
//   - Zero slides is a no-op (every guard short-circuits), so an empty deck
//     never throws.
//
// It exposes `window.SlidesNav = { show, next, prev, current }` for manual
// driving/debugging and is wrapped in an IIFE so it leaks nothing else. It has
// NO dependency on the island runtime: hidden (display:none) slides still hydrate
// their islands on load — CSS visibility does not block JS — so toggling the
// active class only changes which slide is visible; island state persists across
// navigation.
func navScript() string {
	return `(function () {
  var deck = document.querySelector('main.deck');
  if (!deck) return;
  var slides = Array.prototype.slice.call(deck.querySelectorAll('[data-slide]'));
  slides.sort(function (a, b) {
    return (parseInt(a.getAttribute('data-slide'), 10) || 0) - (parseInt(b.getAttribute('data-slide'), 10) || 0);
  });
  if (!slides.length) return;

  var ACTIVE = '` + navActiveClass + `';
  var index = initialIndex();

  function initialIndex() {
    var fromHash = parseInt((location.hash || '').replace('#', ''), 10);
    if (!isNaN(fromHash) && fromHash > 0) return Math.min(fromHash - 1, slides.length - 1);
    return 0;
  }

  function show(next, push) {
    index = Math.max(0, Math.min(slides.length - 1, next));
    for (var i = 0; i < slides.length; i++) {
      slides[i].classList.toggle(ACTIVE, i === index);
    }
    if (push) history.replaceState(null, '', '#' + (index + 1));
  }

  function next() { show(index + 1, true); }
  function prev() { show(index - 1, true); }

  function toggleFullscreen() {
    if (!document.fullscreenElement && document.documentElement.requestFullscreen) {
      document.documentElement.requestFullscreen();
    } else if (document.exitFullscreen) {
      document.exitFullscreen();
    }
  }

  document.addEventListener('keydown', function (event) {
    var tag = event.target && event.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    if (event.key === 'ArrowRight' || event.key === ' ') { event.preventDefault(); next(); }
    else if (event.key === 'ArrowLeft') { event.preventDefault(); prev(); }
    else if (event.key === 'f' || event.key === 'F') { toggleFullscreen(); }
  });

  window.addEventListener('hashchange', function () { show(initialIndex(), false); });

  window.SlidesNav = { show: show, next: next, prev: prev, current: function () { return index + 1; } };
  show(index, false);
})();`
}

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

// navOverviewClass is the CSS class navScript toggles onto `main.deck` when the
// OVERVIEW grid (the `o` key) is open. While set, navStyle's overview rules
// override the one-slide-at-a-time visibility so EVERY slide shows as a scaled
// thumbnail card. Kept as a const so the style and the script agree.
const navOverviewClass = "deck-overview"

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
}

/* ── Overview grid (the 'o' key) ────────────────────────────────────────────
   navScript toggles main.deck.` + navOverviewClass + ` on. While set, the
   one-slide-at-a-time visibility above is overridden so EVERY slide shows as a
   scaled thumbnail card laid out in a responsive grid. The selectors below all
   include .` + navOverviewClass + ` so they have higher specificity than the
   single-class visibility/layout rules (and use !important where they must beat
   the !important display:none above) — overview only takes effect when the class
   is present, and the deck snaps back to the single-slide view when it is removed.

   Scaling: each card uses CSS zoom to shrink a full slide (min-height:100vh +
   room-sized padding and type) proportionally into a readable thumbnail without
   restructuring the DOM. The card clips overflow so a long slide degrades to a
   cropped preview. Theme-agnostic on purpose: it reads the slide's own themed
   colors, so a thumbnail looks like a miniature of the real slide in every theme. */
main.deck.` + navOverviewClass + ` {
  display: grid !important;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr));
  gap: clamp(1rem, 2.4vw, 2rem);
  align-content: start;
  align-items: start;
  padding: clamp(1.25rem, 4vw, 3rem) !important;
  max-width: 1600px;
  margin: 0 auto;
  min-height: 100vh;
  box-sizing: border-box;
}
/* Every slide becomes a visible card — beat the :not(.deck-active) display:none
   !important above and any layout-* flex/centering with an equally-specific
   !important. The card itself is the clip frame; CSS zoom scales its content. */
main.deck.` + navOverviewClass + ` > .slide {
  display: block !important;
  zoom: 0.26;
  min-height: 132vh;
  max-height: 132vh;
  overflow: hidden;
  cursor: pointer;
  border: 2px solid rgba(128,128,128,0.28);
  border-radius: 14px;
  margin: 0;
  position: relative;
  -webkit-user-select: none; user-select: none;
  /* No slide-enter animation in overview (it would flash every card on open). */
  animation: none !important;
}
/* The active slide's card is highlighted so the current position reads at a glance.
   currentColor picks up the slide's themed foreground, so the ring matches the
   theme without this rule knowing the palette. */
main.deck.` + navOverviewClass + ` > .slide.` + navActiveClass + ` {
  border-color: currentColor;
  box-shadow: 0 0 0 4px rgba(128,128,128,0.18);
}
/* Hover / keyboard-focus affordance on a card. focus-visible covers Tab + the
   roving focus navScript sets when arrowing through the grid. */
main.deck.` + navOverviewClass + ` > .slide:hover,
main.deck.` + navOverviewClass + ` > .slide:focus-visible {
  border-color: currentColor;
  outline: none;
}
@media (prefers-reduced-motion: no-preference) {
  main.deck.` + navOverviewClass + ` > .slide {
    transition: border-color 160ms cubic-bezier(0.25,1,0.5,1),
                transform 160ms cubic-bezier(0.25,1,0.5,1);
  }
  main.deck.` + navOverviewClass + ` > .slide:hover {
    transform: translateY(-4px);
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
//   - keydown (single-slide view): ArrowRight or Space -> next, ArrowLeft ->
//     prev, `f` -> toggle fullscreen, `o` -> open the overview grid. Typing in an
//     input/textarea/select is ignored. Arrow/Space default scrolling is prevented.
//   - OVERVIEW GRID (`o`): toggles navOverviewClass on `main.deck`, so navStyle's
//     overview rules lay every slide out as a scaled thumbnail card. While open,
//     cards become keyboard-operable (role=button + tabindex); ArrowLeft/Right (and
//     Up/Down by row) move a roving focus, Enter/Space (or a CLICK) jumps to that
//     slide and closes the grid, and `o`/Esc close it. Closing restores the cards'
//     attributes and the single-slide view. Island state is untouched throughout.
//   - On every change, `history.replaceState(null, ”, '#'+n)` keeps the URL in
//     sync without polluting history; `#N` deep-links on reload.
//   - Zero slides is a no-op (every guard short-circuits), so an empty deck
//     never throws.
//
// It exposes `window.SlidesNav = { show, next, prev, current, openOverview,
// closeOverview, toggleOverview, isOverview }` for manual driving/debugging and is
// wrapped in an IIFE so it leaks nothing else. It has NO dependency on the island
// runtime: hidden (display:none) slides still hydrate their islands on load — CSS
// visibility does not block JS — so toggling the active class (or the overview
// grid) only changes what is shown; island state persists across navigation.
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
  var OVERVIEW = '` + navOverviewClass + `';
  var index = initialIndex();
  var overview = false;

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

  // --- Overview grid (the o key) ------------------------------------------
  // Toggling the OVERVIEW class on the deck flips navStyle's overview rules on,
  // laying every slide out as a scaled thumbnail card. Cards are made keyboard-
  // operable (role=button + tabindex) only while overview is open, and a single
  // delegated click handler (installed once below) jumps to the clicked card and
  // closes overview — so a click on a thumbnail selects that slide.
  function openOverview() {
    if (overview) return;
    overview = true;
    deck.classList.add(OVERVIEW);
    for (var i = 0; i < slides.length; i++) {
      var s = slides[i];
      s.setAttribute('tabindex', '0');
      s.setAttribute('role', 'button');
      s.setAttribute('aria-label', 'Go to slide ' + (i + 1));
    }
    // Focus the current slide's card so arrow keys + Enter work immediately.
    if (slides[index]) slides[index].focus();
  }

  function closeOverview() {
    if (!overview) return;
    overview = false;
    deck.classList.remove(OVERVIEW);
    for (var i = 0; i < slides.length; i++) {
      slides[i].removeAttribute('tabindex');
      slides[i].removeAttribute('role');
      slides[i].removeAttribute('aria-label');
    }
  }

  function toggleOverview() { overview ? closeOverview() : openOverview(); }

  // Jump to a slide from overview: select it, close the grid, land on it.
  function jumpTo(i) {
    closeOverview();
    show(i, true);
  }

  // One delegated click handler: find the [data-slide] card the click landed in.
  deck.addEventListener('click', function (event) {
    if (!overview) return;
    var node = event.target;
    while (node && node !== deck && !node.hasAttribute('data-slide')) node = node.parentNode;
    if (!node || node === deck) return;
    var i = slides.indexOf(node);
    if (i >= 0) { event.preventDefault(); jumpTo(i); }
  });

  // Roving focus within the grid: track which card has focus so arrows move it.
  function focusedIndex() {
    var el = document.activeElement;
    var i = slides.indexOf(el);
    return i >= 0 ? i : index;
  }
  function focusCard(i) {
    i = Math.max(0, Math.min(slides.length - 1, i));
    if (slides[i]) slides[i].focus();
  }

  document.addEventListener('keydown', function (event) {
    var tag = event.target && event.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

    // o toggles the overview grid from either state.
    if (event.key === 'o' || event.key === 'O') { event.preventDefault(); toggleOverview(); return; }

    if (overview) {
      // While the grid is open, the keyboard drives card selection, not slide nav.
      if (event.key === 'Escape') { event.preventDefault(); closeOverview(); return; }
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault(); jumpTo(focusedIndex()); return;
      }
      if (event.key === 'ArrowRight') { event.preventDefault(); focusCard(focusedIndex() + 1); return; }
      if (event.key === 'ArrowLeft') { event.preventDefault(); focusCard(focusedIndex() - 1); return; }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        // Approximate row movement from the on-screen column count.
        event.preventDefault();
        var cols = columnCount();
        focusCard(focusedIndex() + (event.key === 'ArrowDown' ? cols : -cols));
        return;
      }
      return; // swallow other keys while overview is open
    }

    // Single-slide navigation (overview closed).
    if (event.key === 'ArrowRight' || event.key === ' ') { event.preventDefault(); next(); }
    else if (event.key === 'ArrowLeft') { event.preventDefault(); prev(); }
    else if (event.key === 'f' || event.key === 'F') { toggleFullscreen(); }
  });

  // columnCount estimates how many cards sit per row from the grid's computed
  // template, so ArrowUp/Down can move by a row. Falls back to 1 if unknown.
  function columnCount() {
    try {
      var tmpl = getComputedStyle(deck).gridTemplateColumns;
      var n = tmpl ? tmpl.split(' ').filter(function (x) { return x && x !== 'none'; }).length : 0;
      return n > 0 ? n : 1;
    } catch (e) { return 1; }
  }

  window.addEventListener('hashchange', function () { show(initialIndex(), false); });

  window.SlidesNav = {
    show: show, next: next, prev: prev,
    current: function () { return index + 1; },
    openOverview: openOverview, closeOverview: closeOverview,
    toggleOverview: toggleOverview,
    isOverview: function () { return overview; }
  };
  show(index, false);
})();`
}

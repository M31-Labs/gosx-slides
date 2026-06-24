package slides

import (
	"strconv"
	"strings"
)

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
// only the deck mechanics (show-one-slide, ←/→/Space, f-fullscreen, o-overview,
// hash sync). renderPage injects navStyle into the document head and navScript at
// the END of the body (so the sections exist when it runs); see serve.go.
//
// navScript is also the single owner of slide STATE for the presenter view layer
// (presenter.go): pressing `p` opens a presenter window (the same page + ?present),
// a ?present/#present load is detected here and handed to the presenter chrome
// controller, and BOTH windows are kept in lockstep peer-to-peer over a
// BroadcastChannel keyed to the deck path (no server/Hub). Any navigation in either
// window posts the new index; the other applies it behind a self-echo guard so the
// two can't ping-pong. The presenter chrome subscribes via onChange so its previews
// and counter re-render on every change, including ones arriving from the peer.
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

// navActiveStepAttr is the data-attribute navScript sets on the ACTIVE slide to
// record which click STEP is currently lit (1-based; "0" / absent = no step yet,
// every emphasized line shown). The theme CSS keys its code-block spotlight off
// `.slide[data-active-step="K"] pre[data-steps] .ts-line[data-step~="K"]`. Kept as
// a const so the style and the script can never drift on the attribute name.
const navActiveStepAttr = "data-active-step"

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
	return `/* Base reset + viewport lock: each .slide is min-height:100vh. The browser's
   default 8px body margin pushed it 16px past the viewport (a permanent scrollbar,
   even fullscreen), and the enter transition's translateY briefly pushes the slide
   below the fold (a scrollbar that FLASHES on every slide change). A deck owns the
   viewport — lock html/body to it so neither can scroll. The overview grid and the
   presenter notes panel get their own internal scroll where they need it. */
html, body { margin: 0; padding: 0; height: 100%; overflow: hidden; }
main.deck > .slide:not(.` + navActiveClass + `) { display: none !important; }
main.deck > .slide.` + navActiveClass + ` { transform-origin: center top; }
/* Slide ENTER transition is OPACITY-ONLY so the fit-to-viewport transform that
   navScript applies to the active slide (auto-scale) is never fought by an
   animated transform. Pick it via ` + "`transition:`" + ` headmatter (fade | none);
   fade is the default. */
@media (prefers-reduced-motion: no-preference) {
  @keyframes slidesDeckEnter { from { opacity: 0; } to { opacity: 1; } }
  main.deck:not([data-transition="none"]) > .slide.` + navActiveClass + ` {
    animation: slidesDeckEnter 220ms ease both;
  }
}

/* Audience chrome: a thin themed progress bar + a slide counter. navScript builds
   these and updates them in show(). Hidden in overview and print. */
main.deck .deck-progress { position: fixed; left: 0; right: 0; bottom: 0; height: 3px; z-index: 40; pointer-events: none; }
main.deck .deck-progress-fill { height: 100%; width: 0; background: var(--accent, #888); }
@media (prefers-reduced-motion: no-preference) { main.deck .deck-progress-fill { transition: width 260ms cubic-bezier(0.25,1,0.5,1); } }
main.deck .deck-counter { position: fixed; right: 1rem; bottom: 0.85rem; z-index: 40; font: 600 0.8rem/1 var(--font-mono, ui-monospace, monospace); color: var(--fg-muted, #888); opacity: 0.7; pointer-events: none; }
main.deck.` + navOverviewClass + ` .deck-progress, main.deck.` + navOverviewClass + ` .deck-counter { display: none; }
/* Dev-only overflow cue: navScript shows this when the active slide's content
   exceeds the viewport (it is auto-scaled to fit, but the badge says "split me"). */
main.deck .deck-overflow-badge { position: fixed; left: 1rem; bottom: 0.8rem; z-index: 41; display: none; font: 700 0.78rem/1 var(--font-mono, ui-monospace, monospace); color: #ff6b6b; background: rgba(255,107,107,0.12); border: 1px solid #ff6b6b; border-radius: 999px; padding: 0.35rem 0.7rem; }
/* Print / PDF: lay every slide out one-per-page (overriding the viewport lock and
   the one-slide visibility), drop the on-screen chrome, and undo any fit-scale so
   pages print at full size. Use the browser's "Save as PDF" for a handout. */
@media print {
  html, body { height: auto !important; overflow: visible !important; }
  main.deck { display: block !important; }
  main.deck > .slide { display: block !important; min-height: 100vh; transform: none !important; box-shadow: none !important; break-after: page; page-break-after: always; animation: none !important; }
  main.deck > .slide.layout-center, main.deck > .slide.layout-title, main.deck > .slide.layout-section, main.deck > .slide.layout-quote { display: flex !important; }
  main.deck .deck-progress, main.deck .deck-counter, main.deck .deck-overflow-badge, main.deck .code-copy, main.deck .slide-notes { display: none !important; }
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
  /* The deck body is overflow:hidden, so a big deck's grid scrolls HERE, not the
     page — height-bound to the viewport with its own scroll. */
  height: 100vh;
  overflow-y: auto;
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
}
` + stepSpotlightCSS()
}

// navStepMax is the largest click-step index the generated spotlight CSS
// enumerates. CSS cannot compare a slide's data-active-step value against a line's
// data-step value (no attribute-to-attribute matching), so stepSpotlightCSS emits
// one rule per step index 1..navStepMax. A real walkthrough rarely exceeds a
// handful of `|`-groups; 16 is a generous ceiling. A fence with MORE steps than
// this still steps correctly (navScript has no cap) — only the later steps fall
// back to the no-spotlight look (every emphasized line lit) for those rare slides.
const navStepMax = 16

// stepSpotlightCSS returns the theme-agnostic click-through spotlight stylesheet
// for code blocks: when the active slide carries data-active-step="K" (navScript
// sets it once stepping begins), the lines tagged data-step~="K" stay fully lit
// while the OTHER emphasized lines drop back to the dim level — so a `{2-3|6}`
// fence lights 2-3 on the first ArrowRight, then 6 on the next, the rest dimmed.
//
// It is intentionally written ONCE here rather than per theme: it only refines the
// per-theme [data-emphasized] .ts-line rules (themes_css.go) using the SAME
// inherited tokens (--accent / --accent-soft / --fg), so it inherits each theme's
// palette without knowing it. Specificity is higher than the per-theme
// .ts-line.emphasis lit rule (it adds .slide, [data-active-step], [data-steps]), so
// the dim-the-rest rule wins; the per-K re-light rule adds [data-step~="K"] on top
// so the active step's lines win again. When NO step is active (data-active-step
// absent — step 0, a reload, or a slide with no stepped block) none of these match,
// so every emphasized line shows lit exactly as the static-emphasis feature did.
func stepSpotlightCSS() string {
	var b strings.Builder
	b.WriteString(`/* ── Click-through code stepping (the marquee nicey) ─────────────────────────
   navScript advances a STEP within a slide before moving to the next slide, and
   writes data-active-step="K" on the active slide. While a step is active, the
   active step's code lines stay lit and the rest dim. Theme-agnostic: uses the
   inherited --accent / --accent-soft / --fg tokens, so it looks native per theme.
   No active step (absent attr) => no match => every emphasized line lit (static). */
main.deck > .slide[` + navActiveStepAttr + `] pre.code-block[data-steps] .ts-line.emphasis {
  opacity: 0.4;
  background: transparent;
  border-left-color: transparent;
}
`)
	// Per-step re-light rules: enumerate K so CSS can match a line's data-step word
	// against the slide's active step value (CSS can't compare two attributes).
	for k := 1; k <= navStepMax; k++ {
		ks := strconv.Itoa(k)
		b.WriteString(`main.deck > .slide[` + navActiveStepAttr + `="` + ks + `"] pre.code-block[data-steps] .ts-line.emphasis[data-step~="` + ks + `"] {
  opacity: 1;
  background: var(--accent-soft, rgba(128,128,128,0.16));
  border-left-color: var(--accent, currentColor);
}
`)
	}
	return b.String()
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
//     prev, `f` -> toggle fullscreen, `o` -> open the overview grid, `p` -> open
//     the presenter window (audience view only; a no-op in the presenter window).
//     Typing in an input/textarea/select is ignored. Arrow/Space default scrolling
//     is prevented.
//   - CLICK-THROUGH CODE STEPS: a slide whose code block(s) carry data-steps="N"
//     (lowered from a `{2-3|6}` fence's `|`-groups) has N click steps. ArrowRight
//     advances the STEP within the current slide first and only moves to the next
//     slide once the steps are exhausted; ArrowLeft reverses (step down, then to
//     the previous slide's LAST step). The active step is written as
//     data-active-step on the active slide so the theme CSS spotlights that step's
//     lines. Steps are EPHEMERAL: the URL hash stays slide-only (#n), so a reload
//     lands on the slide with no step applied. A slide with no stepped block is a
//     plain one-press-per-slide slide exactly as before. This mirrors the fallback
//     lane's runtime_script.go step-then-slide model, applied to real-lane code
//     blocks. The active {index, step} syncs over the BroadcastChannel so the
//     presenter and audience step together.
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
// It exposes `window.SlidesNav = { show, next, prev, current, step, stepCount,
// openOverview, closeOverview, toggleOverview, isOverview, onChange, openPresenter,
// isPresenter }` for manual driving/debugging (and for the presenter chrome to
// drive state + subscribe to changes) and is wrapped in an IIFE so it leaks
// nothing else. It has
// NO dependency on the island runtime: hidden (display:none) slides still hydrate
// their islands on load — CSS visibility does not block JS — so toggling the active
// class (or the overview grid, or moving a section into a presenter preview) only
// changes what is shown; island state persists across navigation.
// codeCopyScript adds a hover "copy" button to every rendered code block. The
// code text is captured before the button is appended (so it never copies the
// button label), and line numbers — a CSS ::before — are excluded from innerText.
func codeCopyScript() string {
	return `(function () {
  var pres = document.querySelectorAll('main.deck pre.code-block');
  for (var i = 0; i < pres.length; i++) {
    (function (pre) {
      var code = pre.innerText;
      var btn = document.createElement('button');
      btn.type = 'button'; btn.className = 'code-copy'; btn.textContent = 'copy';
      btn.addEventListener('click', function (e) {
        e.stopPropagation();
        var done = function () { btn.textContent = 'copied'; setTimeout(function () { btn.textContent = 'copy'; }, 1200); };
        try { navigator.clipboard ? navigator.clipboard.writeText(code).then(done, done) : done(); } catch (err) { done(); }
      });
      pre.appendChild(btn);
    })(pres[i]);
  }
})();`
}

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
  // present is true when this window was opened as the PRESENTER view: either
  // ?present in the query string or #...present in the hash. The presenter chrome
  // (presenter.go) is rendered over the same page only in that case; the normal
  // (no-?present) window stays the audience view. Both still share slide state and
  // the BroadcastChannel, so prev/next in either drives the other.
  var present = /(^|[?&])present(=|&|$)/.test(location.search) || /present/.test(location.hash);
  var index = initialIndex();
  // step is the ACTIVE click-step within the current slide (0 = no step lit yet;
  // every emphasized line shows, the static-union look). It is ephemeral on
  // purpose: the URL hash stays slide-only (#n), so a deep-link/reload lands on the
  // slide with no step applied. A slide's step COUNT is the max data-steps among
  // its code blocks (0 if none) — see stepCountFor.
  var step = 0;
  var overview = false;
  var dev = deck.getAttribute('data-dev') === '1';

  // --- Audience chrome + fit-to-viewport ----------------------------------
  // A thin progress bar, a slide counter, and (dev only) an overflow badge are
  // fixed to the viewport (they escape the deck's overflow:hidden). updateChrome
  // and fitSlide run on every show() and on resize.
  function mkChrome(cls) { var e = document.createElement('div'); e.className = cls; deck.appendChild(e); return e; }
  var progress = mkChrome('deck-progress');
  var progressFill = document.createElement('div'); progressFill.className = 'deck-progress-fill'; progress.appendChild(progressFill);
  var counter = mkChrome('deck-counter');
  var overflowBadge = mkChrome('deck-overflow-badge'); overflowBadge.textContent = '⚠ overflows — split this slide';

  // fitSlide shrinks an OVERFLOWING active slide to fit the locked viewport instead
  // of clipping it (content never disappears below the fold). It resets the slide's
  // transform, measures, and scales down only when content exceeds the viewport.
  // The enter animation is opacity-only, so this transform is never fought.
  function fitSlide() {
    var s = slides[index];
    if (!s) return;
    s.style.transform = 'none';
    var avail = window.innerHeight;
    var natural = s.scrollHeight; // forces reflow -> accurate
    var overflows = natural > avail + 1;
    if (overflows) s.style.transform = 'scale(' + (avail / natural).toFixed(4) + ')';
    overflowBadge.style.display = (dev && overflows) ? 'block' : 'none';
  }
  function updateChrome() {
    var pct = slides.length > 1 ? ((index + 1) / slides.length) * 100 : 100;
    progressFill.style.width = pct.toFixed(2) + '%';
    counter.textContent = (index + 1) + ' / ' + slides.length;
  }
  var fitTimer = null;
  window.addEventListener('resize', function () { clearTimeout(fitTimer); fitTimer = setTimeout(fitSlide, 120); });
  window.addEventListener('load', fitSlide); // re-fit once webfonts settle

  // stepCountFor returns how many click steps slide i has: the MAX data-steps over
  // its code blocks (0 when the slide has no stepped code block). A slide can hold
  // several stepped fences; advancing a step advances ALL of them in lockstep
  // (they share the deck's data-active-step), so the slide's step budget is the
  // largest single block's, and shorter blocks simply have no line for the later
  // steps. Cached lazily per slide so the read happens once.
  var stepCounts = [];
  function stepCountFor(i) {
    if (i < 0 || i >= slides.length) return 0;
    if (stepCounts[i] != null) return stepCounts[i];
    var max = 0;
    var pres = slides[i].querySelectorAll('pre[data-steps]');
    for (var p = 0; p < pres.length; p++) {
      var n = parseInt(pres[p].getAttribute('data-steps'), 10) || 0;
      if (n > max) max = n;
    }
    stepCounts[i] = max;
    return max;
  }
  function maxStep() { return stepCountFor(index); }

  // Subscribers notified after every committed slide change (local, hash, or a
  // change applied from the peer window). The presenter chrome uses this to keep
  // its previews/notes/counter in lockstep; an empty list is a no-op.
  var changeSubs = [];
  function onChange(fn) { if (typeof fn === 'function') changeSubs.push(fn); }
  function notifyChange() {
    for (var s = 0; s < changeSubs.length; s++) {
      try { changeSubs[s](index); } catch (e) {}
    }
  }

  // --- Peer-to-peer sync (BroadcastChannel) --------------------------------
  // The presenter and audience windows are the SAME served page, opened twice.
  // They stay in sync with NO server/Hub: a BroadcastChannel keyed to this deck's
  // path carries the active {index, step}. Any navigation in either window posts
  // BOTH; the other applies them WITHOUT re-posting (the applying flag guards the
  // echo, so two windows can't ping-pong into a loop) — so presenter and audience
  // step through code together, not just change slides together. Older browsers
  // without BroadcastChannel degrade silently to independent per-window navigation.
  var channel = null;
  var applyingRemote = false;
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      channel = new BroadcastChannel('gosx-slides:' + location.pathname);
      channel.onmessage = function (event) {
        var data = event && event.data;
        if (!data || typeof data.index !== 'number') return;
        var remoteStep = typeof data.step === 'number' ? data.step : 0;
        if (data.index === index && remoteStep === step) return; // already there
        applyingRemote = true;
        show(data.index, remoteStep, true); // update URL hash, but don't re-broadcast
        applyingRemote = false;
      };
    }
  } catch (e) { channel = null; }

  // --- Cross-device sync (Server-Sent Events) ------------------------------
  // BroadcastChannel only reaches windows on the SAME machine. An EventSource to
  // the deck server's /presenter/events carries {index, step} ACROSS machines: the
  // presenter laptop drives audience screens and the phone /remote, all in
  // lockstep. It applies remote state through the same applyingRemote-guarded
  // show() the channel uses (so no echo loop), and on a static export (no server)
  // it simply fails quietly and the local BroadcastChannel still works.
  try {
    if (typeof EventSource !== 'undefined') {
      var sse = new EventSource('presenter/events');
      sse.addEventListener('state', function (event) {
        var data; try { data = JSON.parse(event.data); } catch (e) { return; }
        if (!data || typeof data.index !== 'number') return;
        var remoteStep = typeof data.step === 'number' ? data.step : 0;
        if (data.index === index && remoteStep === step) return; // already there
        applyingRemote = true;
        show(data.index, remoteStep, true);
        applyingRemote = false;
      });
    }
  } catch (e) {}

  function broadcast() {
    if (applyingRemote) return;
    if (channel) { try { channel.postMessage({ index: index, step: step }); } catch (e) {} }
    // Publish to the server so other machines (and the phone remote) follow. Relative
    // URL resolves against the deck page, so it works behind the --watch dev proxy.
    try { fetch('presenter/state', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ index: index, step: step }), keepalive: true }); } catch (e) {}
  }

  function initialIndex() {
    var fromHash = parseInt((location.hash || '').replace('#', '').replace('present', ''), 10);
    if (!isNaN(fromHash) && fromHash > 0) return Math.min(fromHash - 1, slides.length - 1);
    return 0;
  }

  // show(nextIndex, nextStep, push) commits a new (slide, step) position. nextStep
  // is clamped to the destination slide's step budget, so callers can pass a
  // sentinel like Infinity to mean "this slide's LAST step" (prev() uses that to
  // land on the end of the previous slide's walkthrough). It toggles the active
  // class, writes data-active-step on the active slide (and clears it elsewhere) so
  // the theme CSS spotlights the active step's lines, keeps the URL hash slide-only
  // (#n — steps are ephemeral), broadcasts {index, step}, and notifies subscribers.
  function show(nextIndex, nextStep, push) {
    var prevIndex = index, prevStep = step;
    index = Math.max(0, Math.min(slides.length - 1, nextIndex));
    var budget = stepCountFor(index);
    if (nextStep == null) nextStep = 0;
    step = Math.max(0, Math.min(budget, nextStep));
    for (var i = 0; i < slides.length; i++) {
      var on = i === index;
      slides[i].classList.toggle(ACTIVE, on);
      // Only the active slide carries data-active-step; remove it everywhere else so
      // a previously-stepped slide resets to "no step" when you leave it. step 0
      // means no spotlight yet (every emphasized line shown), so clear the attr then
      // too — the CSS treats absent/0 as "show all emphasized, dim nothing extra".
      if (on && step > 0) slides[i].setAttribute('` + navActiveStepAttr + `', String(step));
      else slides[i].removeAttribute('` + navActiveStepAttr + `');
    }
    if (!overview) fitSlide(); // scale the now-active slide to fit; skip in the grid
    updateChrome();
    if (push) history.replaceState(null, '', '#' + (index + 1) + (present ? 'present' : ''));
    broadcast();
    if (index !== prevIndex || step !== prevStep || push) notifyChange();
  }

  // next()/prev() implement STEP-THEN-SLIDE navigation (mirroring the fallback
  // lane's runtime_script.go): ArrowRight advances the click step within the
  // current slide until its steps are exhausted, and only THEN moves to the next
  // slide (starting at step 0). ArrowLeft reverses: step down within the slide,
  // and at step 0 move to the PREVIOUS slide landing on its LAST step (Infinity is
  // clamped to that slide's budget by show), so back-stepping retraces the walk.
  function next() {
    if (step < maxStep()) show(index, step + 1, true);
    else show(index + 1, 0, true);
  }
  function prev() {
    if (step > 0) show(index, step - 1, true);
    else show(index - 1, Infinity, true);
  }

  // Open a presenter window for this deck: the SAME page with ?present, in a named
  // window so a second press focuses the existing one instead of stacking copies.
  function openPresenter() {
    try {
      window.open(location.pathname + '?present', 'gosx-presenter',
        'width=1280,height=800,noopener=no');
    } catch (e) {}
  }

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
      s.style.transform = ''; // drop the fit-scale; the grid uses its own zoom
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
    fitSlide(); // re-scale the active slide now that the grid is closed
  }

  function toggleOverview() { overview ? closeOverview() : openOverview(); }

  // Jump to a slide from overview: select it, close the grid, land on its START
  // (step 0), so picking a slide always begins its walkthrough fresh.
  function jumpTo(i) {
    closeOverview();
    show(i, 0, true);
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
    // p opens the presenter window from the AUDIENCE view. In the presenter
    // window itself it is a no-op (no point opening a presenter from a presenter).
    else if ((event.key === 'p' || event.key === 'P') && !present) { event.preventDefault(); openPresenter(); }
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

  // A hash change is a slide-only deep link (#n); steps are ephemeral, so land on
  // the target slide at step 0. Pass push=false so we don't rewrite the hash we
  // just read.
  window.addEventListener('hashchange', function () { show(initialIndex(), 0, false); });

  window.SlidesNav = {
    show: show, next: next, prev: prev,
    current: function () { return index + 1; },
    // step exposes the active click step (0-based within the slide) and stepCount
    // its budget, so the presenter chrome can render "step K/N" and manual drivers
    // can inspect the walkthrough position.
    step: function () { return step; },
    stepCount: function () { return maxStep(); },
    openOverview: openOverview, closeOverview: closeOverview,
    toggleOverview: toggleOverview,
    isOverview: function () { return overview; },
    onChange: onChange,
    openPresenter: openPresenter,
    isPresenter: function () { return present; }
  };
  show(index, 0, false);

  // Presenter chrome: only when this window is the presenter view. It is handed a
  // small api so it drives slide state through the SAME functions (so its prev/next
  // broadcast to the audience) and re-renders on every change (including remote
  // ones applied from the audience window over the BroadcastChannel). getStep /
  // getStepCount let the footer counter show the live walkthrough position.
  if (present && window.SlidesPresenter && typeof window.SlidesPresenter.init === 'function') {
    window.SlidesPresenter.init({
      slides: slides,
      count: slides.length,
      getIndex: function () { return index; },
      getStep: function () { return step; },
      getStepCount: function () { return maxStep(); },
      onChange: onChange,
      show: function (i) { show(i, 0, true); },
      next: next,
      prev: prev
    });
  }
})();`
}

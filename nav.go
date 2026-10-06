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

// navOverviewClass hides live slides while the searchable text picker is open.
const navOverviewClass = "deck-overview"

// navActiveStepAttr records the active click step; the controller marks matching
// code lines without generating a stylesheet rule for every possible step.
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
    animation: slidesDeckEnter var(--slides-transition-duration, 220ms) var(--slides-transition-easing, ease) var(--slides-transition-delay, 0ms) both;
  }
  /* Per-slide overrides: a slide's own data-transition (from its transition:
     frontmatter) beats the deck-level setting in both directions. */
  main.deck > .slide.` + navActiveClass + `[data-transition="none"] { animation: none; }
  main.deck[data-transition="none"] > .slide.` + navActiveClass + `[data-transition="fade"] {
    animation: slidesDeckEnter var(--slides-transition-duration, 220ms) var(--slides-transition-easing, ease) var(--slides-transition-delay, 0ms) both;
  }
}

/* Audience chrome: a thin themed progress bar + a slide counter. navScript builds
   these and updates them in show(). Hidden in overview and print. */
main.deck .deck-progress { position: fixed; left: 0; right: 0; bottom: 0; height: 3px; z-index: 40; pointer-events: none; }
main.deck .deck-progress-fill { height: 100%; width: 0; background: var(--accent, #888); }
@media (prefers-reduced-motion: no-preference) { main.deck .deck-progress-fill { transition: width 260ms cubic-bezier(0.25,1,0.5,1); } }
main.deck .deck-counter { position: fixed; right: 1rem; bottom: 0.85rem; z-index: 40; font: 600 0.8rem/1 var(--font-mono, ui-monospace, monospace); color: var(--fg-muted, #888); opacity: 0.7; pointer-events: none; }
main.deck.` + navOverviewClass + ` .deck-progress, main.deck.` + navOverviewClass + ` .deck-counter { display: none; }
/* Diagnostic overflow cue, presenter window only (dev serve): navScript shows
   this when the active slide's content exceeds the viewport or intrudes into
   the caption-safe band (the audience view auto-scales silently). */
main.deck .deck-overflow-badge { position: fixed; left: 1rem; bottom: 0.8rem; z-index: 41; display: none; font: 700 0.78rem/1 var(--font-mono, ui-monospace, monospace); color: #ff6b6b; background: rgba(255,107,107,0.12); border: 1px solid #ff6b6b; border-radius: 999px; padding: 0.35rem 0.7rem; }
/* Print / PDF: lay every slide out one-per-page (overriding the viewport lock and
   the one-slide visibility), drop the on-screen chrome, and undo any fit-scale so
   pages print at full size. Use the browser's "Save as PDF" for a handout. */
@media print {
  html, body { height: auto !important; overflow: visible !important; }
  main.deck { display: block !important; }
  /* The second selector repeats the on-screen visibility rule's :not() shape so
     this print unhide TIES its specificity and wins by order — a bare
     main.deck > .slide is (0,2,1) and silently loses to the (0,3,1)
     display:none, which used to drop every non-active layout-default slide
     from print/PDF output. */
  main.deck > .slide, main.deck > .slide:not(.` + navActiveClass + `) { display: block !important; min-height: 100vh; max-height: 100vh; overflow: hidden; transform: none !important; box-shadow: none !important; break-after: page; page-break-after: always; animation: none !important; }
  main.deck > .slide.layout-center, main.deck > .slide.layout-title, main.deck > .slide.layout-section, main.deck > .slide.layout-quote { display: flex !important; }
  main.deck .deck-progress, main.deck .deck-counter, main.deck .deck-overflow-badge, main.deck .code-copy, main.deck .slide-notes { display: none !important; }
}

` + overviewStyle() + stepSpotlightCSS() + fragmentRevealCSS()
}

// Step selectors have constant size regardless of the authored click budget.
func stepSpotlightCSS() string {
	return `
main.deck .slide[data-active-step] pre.code-block[data-steps] .ts-line.emphasis {
  opacity: 0.4;
  background: transparent;
  border-left-color: transparent;
}
main.deck .slide[data-active-step] pre.code-block[data-steps] .ts-line.emphasis.slides-step-active {
  opacity: 1;
  background: var(--accent-soft, rgba(128,128,128,0.16));
  border-left-color: var(--accent, currentColor);
}
`
}

// Fragment zero is visible on entry; the controller reveals later fragments and
// removes hidden items from keyboard navigation. Print shows the full content.
func fragmentRevealCSS() string {
	return `
main.deck .slide [data-fragment] { opacity: 0; }
main.deck .slide:not([data-active-fragment]) [data-fragment="0"],
main.deck .slide [data-fragment].slides-fragment-visible { opacity: 1; }
@media (prefers-reduced-motion: no-preference) {
  main.deck .slide [data-fragment] { transition: opacity 220ms ease; }
}
@media print { main.deck .slide [data-fragment] { opacity: 1 !important; } }
`
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
//     Keys in native inputs, editable text, and ARIA input widgets are ignored.
//     Arrow/Space default scrolling
//     is prevented.
//   - CLICK-THROUGH CODE STEPS: a slide whose code block(s) carry data-steps="N"
//     (lowered from a `{2-3|6}` fence's `|`-groups) has N click steps. ArrowRight
//     advances the STEP within the current slide first and only moves to the next
//     slide once the steps are exhausted; ArrowLeft reverses (step down, then to
//     the previous slide's LAST step). The active step is written as
//     data-active-step on the active slide so the theme CSS spotlights that step's
//     lines. The URL records click steps (#n/k), so links and reloads restore
//     that absolute position. A slide-only link (#n) starts at step zero. A slide
//     with no stepped block is a plain one-press-per-slide slide. This mirrors the fallback
//     lane's runtime_script.go step-then-slide model, applied to real-lane code
//     blocks. The active {index, step} syncs over the BroadcastChannel so the
//     presenter and audience step together.
//   - OVERVIEW (`o` or `/`): a searchable, focus-trapped text-card picker. The
//     original slides stay hidden; DOM attributes and widget state are preserved.
//     Arrow keys select cards, Enter/Space jumps, Esc restores the current step.
//     `?` expands the shortcut reference. Hidden native surfaces pause rendering.
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

` + navLinkScript() + `
  var ACTIVE = '` + navActiveClass + `';
  var OVERVIEW = '` + navOverviewClass + `';
  // present is true when this window was opened as the PRESENTER view: either
  // ?present in the query string or #...present in the hash. The presenter chrome
  // (presenter.go) is rendered over the same page only in that case; the normal
  // (no-?present) window stays the audience view. Both still share slide state and
  // the BroadcastChannel, so prev/next in either drives the other.
  var present = /(^|[?&])present(=|&|$)/.test(location.search) || /present/.test(location.hash);
  if (deck.getAttribute('data-session-role') === 'audience') present = false;
  var initialPosition = readPosition(location.hash, slides.length);
  var index = initialPosition.index;
  // Step zero shows the static code union and the first prose fragment. Shared
  // slide/step anchors restore an absolute position; show() clamps its budget.
  var step = initialPosition.step;
  var overview = false;
  var dev = deck.getAttribute('data-dev') === '1';

  // --- Audience chrome + fit-to-viewport ----------------------------------
  // A thin progress bar, a slide counter, and (presenter-window, dev serve
  // only) an overflow badge are
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
    if (overview) return;
    var s = slides[index];
    if (!s) return;
    s.style.transform = 'none';
    var avail = window.innerHeight;
    var caption = deck.querySelector('.slides-story-caption:not([hidden])');
    if (caption && caption.offsetHeight > 0) avail = Math.max(1, Math.min(avail, caption.getBoundingClientRect().top - 12));
    var natural = s.scrollHeight; // forces reflow -> accurate
    // Content that intrudes into the slide's own bottom padding (the
    // caption-safe band) never grows scrollHeight, so also measure the real
    // bottom edge of the last content child and add the reserved space back.
    // The reserve is the LARGER of the slide's padding-bottom and the themed
    // card frame's (::before) bottom inset plus breathing room — otherwise an
    // admonition can sit inside the caption band or visually cross the card
    // border undetected.
    var last = s.lastElementChild;
    if (last) {
      var reserve = parseFloat(getComputedStyle(s).paddingBottom) || 0;
      var frame = getComputedStyle(s, '::before');
      if (frame && frame.content !== 'none') {
        var frameB = parseFloat(frame.bottom);
        if (!isNaN(frameB) && frameB > 0) reserve = Math.max(reserve, frameB + 12);
      }
      var needed = Math.ceil(last.getBoundingClientRect().bottom - s.getBoundingClientRect().top + reserve);
      if (needed > natural) natural = needed;
    }
    var overflows = natural > avail + 1;
    s.style.transformOrigin = '';
    var studio = deck.querySelector('.slides-motion-studio[open]');
    if (studio) {
      // Keep the slide's authored layout and native canvas in one lane. Fit its
      // full surface beside the inspector (or above the mobile sheet).
      var bounds = studio.getBoundingClientRect(), docked = bounds.top < 24;
      var width = docked ? bounds.left - 32 : window.innerWidth - 32;
      var height = docked ? window.innerHeight - 32 : bounds.top - 32;
      var naturalWidth = Math.max(s.offsetWidth, s.scrollWidth);
      var scale = Math.max(0.01, Math.min(1, width / naturalWidth, height / natural));
      var origin = s.getBoundingClientRect();
      var x = 16 + (width - naturalWidth * scale) / 2 - origin.left;
      var y = 16 + (height - natural * scale) / 2 - origin.top;
      s.style.transformOrigin = '0 0';
      s.style.transform = 'translate(' + x.toFixed(2) + 'px,' + y.toFixed(2) + 'px) scale(' + scale.toFixed(4) + ')';
    } else if (overflows) s.style.transform = 'scale(' + (avail / natural).toFixed(4) + ')';
    // The badge is diagnostic chrome: presenter window only (and only in dev
    // serve). The audience deck auto-scales silently.
    overflowBadge.style.display = (dev && present && overflows) ? 'block' : 'none';
  }
  function updateChrome() {
    var pct = slides.length > 1 ? ((index + 1) / slides.length) * 100 : 100;
    progressFill.style.width = pct.toFixed(2) + '%';
    counter.textContent = (index + 1) + ' / ' + slides.length;
  }
  var fitTimer = null;
  window.addEventListener('resize', function () { clearTimeout(fitTimer); fitTimer = setTimeout(fitSlide, 120); });
  window.addEventListener('load', fitSlide); // re-fit once webfonts settle
  deck.addEventListener('slides:studio-layout', fitSlide);
  deck.addEventListener('slides:caption-layout', fitSlide);

  var controls = document.createElement('nav'); controls.className = 'deck-controls'; controls.setAttribute('aria-label', 'Presentation controls');
  function control(label, text, action) {
    var button = document.createElement('button'); button.type = 'button'; button.textContent = text;
    button.className = 'deck-icon-control';
    button.title = label; button.setAttribute('aria-label', label); button.addEventListener('click', action); controls.appendChild(button); return button;
  }
  control('Previous slide (Left arrow)', '←', prev);
  control('Slide overview (O)', '▦', toggleOverview);
  control('Keyboard shortcuts (?)', '?', function () { openOverview(true); });
  control('Fullscreen (F)', '⛶', toggleFullscreen);
  control('Blank screen (B)', '◐', toggleBlank);
  control('Next slide (Right arrow)', '→', next);
  deck.appendChild(controls);
` + controlsActivityScript() + `
  var curtain = mkChrome('deck-curtain'); curtain.setAttribute('aria-hidden', 'true');
  curtain.addEventListener('click', toggleBlank);
  function toggleBlank() { blank = !blank; deck.classList.toggle('deck-blank', blank); }
  var touchStart = null;
  deck.addEventListener('touchstart', function (event) {
    if (overview || event.touches.length !== 1 || event.target.closest('button, a, input, textarea, select, [contenteditable], [data-gosx-engine], [data-gosx-island]')) { touchStart = null; return; }
    touchStart = { x: event.touches[0].clientX, y: event.touches[0].clientY };
  }, { passive: true });
  deck.addEventListener('touchcancel', function () { touchStart = null; }, { passive: true });
  deck.addEventListener('touchend', function (event) {
    if (!touchStart || !event.changedTouches.length) return;
    var dx = event.changedTouches[0].clientX - touchStart.x, dy = event.changedTouches[0].clientY - touchStart.y;
    touchStart = null;
    if (Math.abs(dx) > 60 && Math.abs(dx) > Math.abs(dy) * 1.5) { if (dx < 0) next(); else prev(); }
  }, { passive: true });

  // stepCountFor returns how many click steps slide i has. For a code-only slide
  // it is the MAX data-steps over its code blocks (0 when none). For a reveal slide
  // (containing [data-fragment] items) the budget is fragmentCount-1 (fragment 0 is
  // always visible on entry, so only N-1 presses are needed to reveal all N items).
  // When both code steps and fragments are present, the budget is max(codeSteps, N-1)
  // so neither walkthrough is skipped. Cached lazily per slide.
  var stepCounts = [];
  var fragmentCounts = [];
  var backgroundSurfaces = Array.prototype.slice.call(deck.querySelectorAll(':scope > .deck-graphics-background'));
  var blank = false;
  function stepCountFor(i) {
    if (i < 0 || i >= slides.length) return 0;
    if (stepCounts[i] != null) return stepCounts[i];
    var max = Math.max(0, (JSON.parse(slides[i].getAttribute("data-slide-cues") || "[]")).length - 1);
    var motions = slides[i].querySelectorAll("[data-slides-motion-step]");
    for (var q = 0; q < motions.length; q++) max = Math.max(max, Number(motions[q].getAttribute("data-slides-motion-step")) || 0);
    var pres = slides[i].querySelectorAll('pre[data-steps]:not(.slides-code-morph pre), .slide-graphic[data-steps], .slides-code-morph[data-steps], .slides-diagram-morph[data-steps]');
    for (var p = 0; p < pres.length; p++) {
      var n = parseInt(pres[p].getAttribute('data-steps'), 10) || 0;
      if (n > max) max = n;
    }
    // Fragment reveals: N fragments need N-1 steps (fragment 0 is shown on entry).
    var frags = slides[i].querySelectorAll('[data-fragment]');
    var fragBudget = frags.length > 0 ? frags.length - 1 : 0;
    if (fragBudget > max) max = fragBudget;
    stepCounts[i] = max;
    return max;
  }
  function maxStep() { return stepCountFor(index); }
  // fragCountFor returns the total number of [data-fragment] items in slide i.
  function fragCountFor(i) {
    if (i < 0 || i >= slides.length) return 0;
    if (fragmentCounts[i] == null) fragmentCounts[i] = slides[i].querySelectorAll('[data-fragment]').length;
    return fragmentCounts[i];
  }

` + navStepScript() + `

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
  var applyingRemote = false, initializing = true;
  var authorReload = false;
  try {
    var reloadKey = 'gosx-slides:author-reload';
    var savedReload = JSON.parse(sessionStorage.getItem(reloadKey) || 'null');
    sessionStorage.removeItem(reloadKey);
    authorReload = !!savedReload && savedReload.url === location.href && Date.now() - savedReload.time < 15000;
  } catch (e) {}
  var sourceID = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Date.now() + '-' + Math.random();
  var sequence = 0, pendingState = null, publishing = false;
  var seenSources = new Map();
  function acceptRemote(data) {
    if (!data || typeof data.index !== 'number' || data.source === sourceID) return false;
    if (data.source && typeof data.sequence === 'number') {
      if (data.sequence <= (seenSources.get(data.source) || 0)) return false;
      seenSources.set(data.source, data.sequence);
      if (seenSources.size > 128) seenSources.delete(seenSources.keys().next().value);
    }
    return true;
  }
  try {
    if (typeof BroadcastChannel !== 'undefined') {
      channel = new BroadcastChannel('gosx-slides:' + location.pathname);
      channel.onmessage = function (event) {
        var data = event && event.data;
        if (!acceptRemote(data)) return;
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
    if (typeof EventSource !== 'undefined' && deck.getAttribute('data-live-sync') === '1') {
      var sse = new EventSource('presenter/events');
      var firstServerState = true, enteredWithAnchor = !!location.hash;
      sse.addEventListener('state', function (event) {
        var data; try { data = JSON.parse(event.data); } catch (e) { return; }
        // An explicit bookmark owns the initial position; an unanchored audience joins the live room.
        if (firstServerState) { firstServerState = false; if (enteredWithAnchor) return; }
        if (!acceptRemote(data)) return;
        var remoteStep = typeof data.step === 'number' ? data.step : 0;
        if (data.index === index && remoteStep === step) return; // already there
        applyingRemote = true;
        show(data.index, remoteStep, true);
        applyingRemote = false;
      });
    }
  } catch (e) {}

  // Coalesce fast stepping into one ordered POST stream. Origin and sequence also
  // deduplicate BroadcastChannel/SSE delivery and prevent our own stale echoes.
  function publishPending() {
    if (publishing || !pendingState) return;
    var data = pendingState; pendingState = null; publishing = true;
    try {
      fetch('presenter/state', { method: 'POST', headers: window.SlidesSessionHeaders ? window.SlidesSessionHeaders({ 'Content-Type': 'application/json' }) : { 'Content-Type': 'application/json' }, body: JSON.stringify(data), keepalive: true })
        .catch(function () {}).finally(function () { publishing = false; publishPending(); });
    } catch (e) { publishing = false; }
  }
  function broadcast() {
	if (deck.getAttribute('data-session-role') === 'audience') return;
    if (applyingRemote || (initializing && (!location.hash || authorReload))) return;
    var data = { index: index, step: step, source: sourceID, sequence: ++sequence };
    if (channel) { try { channel.postMessage(data); } catch (e) {} }
    if (deck.getAttribute('data-live-sync') === '1') { pendingState = data; publishPending(); }
  }

  // show(nextIndex, nextStep, push) commits a new (slide, step) position. nextStep
  // is clamped to the destination slide's step budget, so callers can pass a
  // sentinel like Infinity to mean "this slide's LAST step" (prev() uses that to
  // land on the end of the previous slide's walkthrough). It toggles the active
  // class, writes data-active-step on the active slide (and clears it elsewhere) so
  // the theme CSS spotlights the active step's lines, keeps the URL in sync
  // (#n/k for a click step), broadcasts {index, step}, and notifies subscribers.
  function show(nextIndex, nextStep, push) {
    var prevIndex = index, prevStep = step;
    if (nextIndex !== index) deck.dispatchEvent(new CustomEvent("slides:before-change", { detail: { from: prevIndex, to: Math.max(0, Math.min(slides.length - 1, nextIndex)) } }));
    index = Math.max(0, Math.min(slides.length - 1, nextIndex));
    var budget = stepCountFor(index);
    if (nextStep == null) nextStep = 0;
    step = Math.max(0, Math.min(budget, nextStep));
    var changed = prevIndex === index ? [index] : [prevIndex, index];
    for (var c = 0; c < changed.length; c++) {
      var i = changed[c];
      var on = i === index;
      slides[i].classList.toggle(ACTIVE, on);
      // Only the active slide carries data-active-step; remove it everywhere else so
      // a previously-stepped slide resets to "no step" when you leave it. step 0
      // means no spotlight yet (every emphasized line shown), so clear the attr then
      // too — the CSS treats absent/0 as "show all emphasized, dim nothing extra".
      if (on && step > 0) slides[i].setAttribute('` + navActiveStepAttr + `', String(step));
      else slides[i].removeAttribute('` + navActiveStepAttr + `');
      // Fragment reveal: data-active-fragment="K" on the active slide reveals
      // fragments 0..K. step 0 clears the attr (CSS defaults show fragment 0 via
      // :not([data-active-fragment]) [data-fragment="0"]). Only set when there are
      // fragments; non-reveal slides never carry this attr.
      if (on && fragCountFor(i) > 0 && step > 0) {
        slides[i].setAttribute('data-active-fragment', String(step));
      } else {
        slides[i].removeAttribute('data-active-fragment');
      }
      applyStep(i, on ? step : 0);
    }
    if (!overview) fitSlide(); // scale the now-active slide to fit; skip in the grid
    updateChrome();
    if (push && !applyingRemote) revealControls();
    var source = slides[index].getAttribute('data-scene-source');
    for (var bg = 0; bg < backgroundSurfaces.length; bg++) {
      backgroundSurfaces[bg].classList.toggle('deck-background-active', backgroundSurfaces[bg].getAttribute('data-scene-source') === source && !overview);
    }
    if (prevIndex !== index) {
      var oldMedia = slides[prevIndex].querySelectorAll('video, audio');
      for (var m = 0; m < oldMedia.length; m++) oldMedia[m].pause();
    }
    if (!overview && (prevIndex !== index || push)) {
      var media = slides[index].querySelectorAll('video[autoplay], audio[autoplay]');
      for (var m = 0; m < media.length; m++) { var play = media[m].play(); if (play && play.catch) play.catch(function () {}); }
    }
    deck.dispatchEvent(new CustomEvent('slides:change', { detail: { index: index, step: step } }));
    if (push) history.replaceState(null, '', positionHash(index, step, present));
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
	if (deck.getAttribute('data-session-role') === 'audience') return;
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

` + overviewScript() + `

  document.addEventListener('keydown', function (event) {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
    if (overview) { overviewKey(event); return; }
    var target = event.target;
    // Native dialog dismissal can deliver the next key to its now-hidden input
    // before focus restoration completes. That closed control cannot own a key.
    if (target && target.closest('dialog:not([open])')) { target.blur(); target = deck; }
    if (deck.querySelector('dialog[open]')) return;
    // Editors and composite widgets own their keyboard interaction, including
    // events from nested elements and space-separated ARIA fallback roles.
    if (target && target.closest('input, textarea, select, [contenteditable], [role~="textbox"], [role~="searchbox"], [role~="combobox"], [role~="slider"], [role~="spinbutton"], [role~="scrollbar"], [role~="listbox"], [role~="option"], [role~="tablist"], [role~="tab"], [role~="checkbox"], [role~="radio"], [role~="radiogroup"], [role~="switch"], [role~="tree"], [role~="treeitem"], [role~="grid"], [role~="treegrid"], [role~="gridcell"], [role~="menu"], [role~="menubar"], [role~="menuitem"], [role~="menuitemcheckbox"], [role~="menuitemradio"]')) return;
    // Let focused controls activate themselves rather than also advancing a slide.
    if ((event.key === ' ' || event.key === 'Enter') && target && target.closest('button, a, summary, [role~="button"]')) return;
    if (event.key === 'b' || event.key === 'B' || (event.key === 'Escape' && blank)) { event.preventDefault(); toggleBlank(); return; }
    if (event.key === 'Home') { event.preventDefault(); show(0, 0, true); return; }
    if (event.key === 'End') { event.preventDefault(); show(slides.length - 1, 0, true); return; }
    if (event.key === 'PageDown') { event.preventDefault(); next(); return; }
    if (event.key === 'PageUp') { event.preventDefault(); prev(); return; }
    if (event.key === 'o' || event.key === 'O' || event.key === '/') { event.preventDefault(); openOverview(); return; }
    if (event.key === '?') { event.preventDefault(); openOverview(true); return; }

    // Single-slide navigation (overview closed).
    if (event.key === 'ArrowRight' || event.key === ' ') { event.preventDefault(); next(); }
    else if (event.key === 'ArrowLeft') { event.preventDefault(); prev(); }
    else if (event.key === 'f' || event.key === 'F') { toggleFullscreen(); }
    // p opens the presenter window from the AUDIENCE view. In the presenter
    // window itself it is a no-op (no point opening a presenter from a presenter).
    else if ((event.key === 'p' || event.key === 'P') && !present) { event.preventDefault(); openPresenter(); }
  });

  // Links and back/forward history restore the slide and its absolute click step.
  window.addEventListener('hashchange', function () {
    var target = readPosition(location.hash, slides.length);
    show(target.index, target.step, false);
  });

  window.SlidesNav = {
    show: show, next: next, prev: prev,
    // Authoring previews stay in this tab and never advance the audience.
    preview: function (nextIndex, nextStep) {
      var wasRemote = applyingRemote; applyingRemote = true;
      try { show(nextIndex, nextStep, true); } finally { applyingRemote = wasRemote; }
    },
    reloadPreview: function () {
      // Keep an authoring save at its current anchor without publishing that
      // private preview position as a new presenter action on startup.
      try { sessionStorage.setItem('gosx-slides:author-reload', JSON.stringify({url:location.href,time:Date.now()})); } catch (e) {}
      location.reload();
    },
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
  show(index, step, false);
  initializing = false;

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

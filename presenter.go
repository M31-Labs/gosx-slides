package slides

// presenter.go is the PRESENTER VIEW layer. It adds a second, presenter-only
// rendering of the SAME served page: open the deck with `?present` (or `#present`,
// or press `p` in the audience window) and the page renders presenter chrome
// instead of the plain deck — a large CURRENT slide preview, a smaller NEXT
// preview, the current slide's speaker notes, an elapsed timer, a slide counter,
// and prev/next controls.
//
// It is layered ON TOP of nav.go's controller, not a fork of it: navScript stays
// the single source of slide state (which slide is active, next/prev, hash sync),
// and the presenter chrome is driven entirely from that state. Windows stay in
// lockstep two ways, both feeding nav.go's show() through the same self-echo
// guard: a BroadcastChannel for same-machine windows, and (across machines) the
// server-backed SSE broker in present_broker.go — the presenter laptop drives
// audience screens and the phone /remote in real time.
//
// Three pieces live here, all theme-agnostic (they read the active theme's
// --bg/--surface/--accent/--fg/--line/--font-* tokens off main.deck[data-theme],
// so the chrome looks native in every theme without knowing the palette):
//
//   - extractSlideNotes: pulls a slide's speaker notes out of its mdpp subtree
//     (a <Notes>…</Notes> block or a trailing <!-- … --> comment). serve.go
//     renders these into hidden
//     <aside class="slide-notes" data-notes="N"> nodes; the presenter panel shows
//     the current slide's note, the audience CSS hides them all.
//   - presenterStyle: the .deck-presenter chrome stylesheet (appended to navStyle
//     output by serve.go).
//   - presenterScript: the chrome controller (appended to navScript output). It is
//     invoked by navScript only when the page is in present mode, and is handed an
//     api object so it never reaches into navScript's internals.

import (
	"regexp"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/mdpp"
)

// presenterModeClass is the CSS class navScript adds to main.deck when the page
// is loaded in presenter mode (?present / #present). While set, presenterStyle's
// rules build the presenter chrome over the existing deck. Kept as a const so the
// style and the script agree.
const presenterModeClass = "deck-presenter"

// notesBlockRe / notesCommentRe recognize the two speaker-note forms: a
// <Notes>…</Notes> block, or a trailing HTML comment <!-- … -->. They are
// compiled once at package load. (?is)/(?s) let them span newlines.
var (
	notesBlockRe   = regexp.MustCompile(`(?is)<Notes>(.*?)</Notes>`)
	notesCommentRe = regexp.MustCompile(`(?s)<!--(.*?)-->`)
)

// extractSlideNotes returns the speaker notes for one slide, or "" when it has
// none. It walks the slide's mdpp subtree and reads notes out of the raw literals
// mdpp preserves verbatim: a <Notes>…</Notes> block and a trailing <!-- … -->
// comment both arrive as a NodeHTMLBlock (or, inline, NodeHTMLInline / NodeText)
// whose .Literal holds the source, so scanning those literals recovers the notes
// without re-parsing the deck. Multiple note fragments on one slide are joined
// with a blank line.
//
// This is deliberately READ-ONLY over the already-parsed tree: it does not mutate
// the slide (the note literals stay in the subtree, but slidegen.go strips HTML
// comments and never emits <Notes> as markup, so they never reach the audience
// DOM — only this aside does).
func extractSlideNotes(slide IslandSlide) string {
	if slide.Node == nil {
		return ""
	}
	var notes []string
	collect := func(literal string) {
		if literal == "" {
			return
		}
		for _, m := range notesBlockRe.FindAllStringSubmatch(literal, -1) {
			if t := strings.TrimSpace(m[1]); t != "" {
				notes = append(notes, t)
			}
		}
		for _, m := range notesCommentRe.FindAllStringSubmatch(literal, -1) {
			if t := strings.TrimSpace(m[1]); t != "" {
				notes = append(notes, t)
			}
		}
	}
	slide.Node.Walk(func(n *mdpp.Node) bool {
		switch n.Type {
		case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline, mdpp.NodeText:
			collect(n.Literal)
		}
		return true
	})
	return strings.Join(notes, "\n\n")
}

// noteAsides builds one hidden <aside class="slide-notes" data-notes="N"> per
// slide that has speaker notes, carrying that slide's note text. serve.go places
// these at the end of <main.deck>; presenterStyle hides them in BOTH views, and
// the presenter chrome reads the current slide's note out of the matching aside
// by its data-notes index. The note is rendered as a gosx.Text child so it is
// HTML-escaped on output (it is opaque author prose, never markup) — a note
// containing `<`, `&`, or quotes can never inject into the page.
//
// A slide with no note contributes nothing: the presenter shows a graceful
// "No notes for this slide" placeholder for those, so the asides stay 1:1 with
// slides that actually carry a note.
func (d *IslandDeck) noteAsides() []gosx.Node {
	if d == nil {
		return nil
	}
	var nodes []gosx.Node
	for _, slide := range d.Slides {
		note := extractSlideNotes(slide)
		if note == "" {
			continue
		}
		nodes = append(nodes, gosx.El("aside",
			gosx.Attrs(
				gosx.Attr("class", "slide-notes"),
				gosx.Attr("data-notes", slide.Index),
				gosx.Attr("hidden", true),
			),
			gosx.Text(note),
		))
	}
	return nodes
}

// presenterStyle is the presenter-chrome stylesheet, returned as the inner CSS
// text (no <style> wrapper) so serve.go can place it alongside navStyle(). Every
// rule is gated under main.deck.deck-presenter so it is INERT until navScript adds
// that class on a ?present load — the audience window (no class) is completely
// untouched, and the same served page is therefore both views.
//
// Theme-agnostic by construction: it only references the theme token custom
// properties (--bg/--surface/--accent/--fg/--fg-muted/--line/--font-*/--radius)
// that every theme declares on main.deck[data-theme] (themes_css.go), so the
// chrome inherits the active palette and looks native in aurora/paper/neon/swiss
// without this file knowing any colors. Motion is gated behind
// prefers-reduced-motion: no-preference (matching nav.go); the timer/counter use
// --font-mono so the digits don't jitter as they tick.
func presenterStyle() string {
	// The audience window must NEVER show the speaker-note asides serve.go emits;
	// hide them globally (the presenter panel reads their text via JS, it does not
	// rely on them being displayed). This rule is unconditional — notes are author
	// asides, not slide content, in BOTH views.
	return `main.deck .slide-notes { display: none !important; }

/* ── Presenter chrome (?present / #present, or the 'p' key) ───────────────────
   navScript adds main.deck.` + presenterModeClass + ` on a ?present load. While
   set, the deck becomes a fixed two-region instrument panel: a large CURRENT
   slide preview, a smaller NEXT preview, a notes panel, and a footer with the
   elapsed timer, slide counter, and prev/next controls. The current/next deck
   sections are MOVED (not cloned — so live islands keep their hydration state)
   into the preview slots by the controller; everything here is scoped under the
   presenter class so the audience view is byte-for-byte unaffected. */
main.deck.` + presenterModeClass + ` {
  display: grid !important;
  grid-template-columns: minmax(0, 1.85fr) minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr) auto;
  grid-template-areas:
    "current side"
    "footer  footer";
  gap: clamp(0.75rem, 1.6vw, 1.5rem);
  padding: clamp(0.75rem, 1.6vw, 1.5rem) !important;
  max-width: none;
  margin: 0;
  width: 100vw;
  height: 100vh;
  box-sizing: border-box;
  overflow: hidden;
}
/* Hide every real slide section by default in presenter mode; the controller
   un-hides only the two it has moved into the preview stages (current + next),
   so a 50-slide deck shows exactly two previews, not 50. Beats the single-slide
   :not(.deck-active) rule with equal specificity + !important. */
main.deck.` + presenterModeClass + ` > .slide {
  display: none !important;
}

/* The chrome scaffold the controller injects (one wrapper per region). Cards use
   the theme surface so they read as panels over the theme background. A stage is a
   label header bar over a .pv-screen fill region (flex column), so the label never
   overlaps the previewed slide's own heading. */
main.deck.` + presenterModeClass + ` .pv-stage {
  background: var(--surface, rgba(128,128,128,0.08));
  border: 1px solid var(--line, rgba(128,128,128,0.25));
  border-radius: var(--radius, 12px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
  min-width: 0;
}
/* The clipped frame the scaled slide fills, below the label bar. */
main.deck.` + presenterModeClass + ` .pv-screen {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
main.deck.` + presenterModeClass + ` .pv-current { grid-area: current; }
main.deck.` + presenterModeClass + ` .pv-side {
  grid-area: side;
  display: grid;
  grid-template-rows: minmax(0, 1.25fr) minmax(0, 1fr);
  gap: clamp(0.75rem, 1.6vw, 1.5rem);
  min-height: 0;
}
main.deck.` + presenterModeClass + ` .pv-footer {
  grid-area: footer;
  display: flex;
  align-items: center;
  gap: clamp(0.75rem, 1.8vw, 1.75rem);
  background: var(--surface, rgba(128,128,128,0.08));
  border: 1px solid var(--line, rgba(128,128,128,0.25));
  border-radius: var(--radius, 12px);
  padding: clamp(0.6rem, 1.1vw, 1rem) clamp(0.9rem, 1.6vw, 1.5rem);
}

/* A small, all-caps label header bar atop each stage (CURRENT / NEXT). It sits in
   the flex column ABOVE the slide screen, so it never overlaps the slide's own
   heading. The accent backing for CURRENT pulls the eye to the live one; NEXT is a
   quieter muted bar. */
main.deck.` + presenterModeClass + ` .pv-label {
  flex: 0 0 auto;
  font-family: var(--font-display, var(--font-body, system-ui, sans-serif));
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  padding: 0.42em 0.85em;
  background: var(--accent, currentColor);
  color: var(--bg, #000);
}
main.deck.` + presenterModeClass + ` .pv-next .pv-label {
  background: transparent;
  color: var(--fg-muted, currentColor);
  border-bottom: 1px solid var(--line, rgba(128,128,128,0.25));
}

/* The moved slide section inside a screen. We scale a full slide down to fit the
   screen with CSS zoom (the same technique the overview grid uses), so the preview
   is a faithful miniature of the real slide — themed colors, real layout, real
   islands — not a re-render. The current screen gets a larger zoom than the next.
   Absolute fill keeps the scaled slide anchored to the screen's top-left. */
main.deck.` + presenterModeClass + ` .pv-screen > .slide {
  display: block !important;
  position: absolute;
  inset: 0;
  margin: 0 !important;
  min-height: 0 !important;
  height: 100%;
  width: 100%;
  overflow: hidden;
  box-sizing: border-box;
  animation: none !important; /* never run the deck-enter keyframe inside a preview */
  pointer-events: none;       /* the preview is for the presenter to READ, not click */
  -webkit-user-select: none; user-select: none;
}
main.deck.` + presenterModeClass + ` .pv-current .pv-screen > .slide { zoom: 0.5; }
main.deck.` + presenterModeClass + ` .pv-next .pv-screen > .slide { zoom: 0.32; }
/* Empty-screen placeholder (no next slide past the last slide). */
main.deck.` + presenterModeClass + ` .pv-screen[data-empty="1"]::after {
  content: attr(data-empty-label);
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--fg-muted, currentColor);
  font-family: var(--font-body, system-ui, sans-serif);
  font-size: 0.95rem;
  opacity: 0.7;
}

/* Notes panel — the bottom card in the side column. Scrolls if a note is long.
   Body font + comfortable measure so the presenter can actually read it. */
main.deck.` + presenterModeClass + ` .pv-notes {
  grid-area: auto;
  background: var(--surface, rgba(128,128,128,0.08));
  border: 1px solid var(--line, rgba(128,128,128,0.25));
  border-radius: var(--radius, 12px);
  padding: clamp(0.75rem, 1.2vw, 1.15rem);
  overflow: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
main.deck.` + presenterModeClass + ` .pv-notes-label {
  font-family: var(--font-display, var(--font-body, system-ui, sans-serif));
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--accent, currentColor);
  margin: 0;
}
main.deck.` + presenterModeClass + ` .pv-notes-body {
  font-family: var(--font-body, system-ui, sans-serif);
  font-size: clamp(0.95rem, 1.15vw, 1.15rem);
  line-height: 1.55;
  color: var(--fg, currentColor);
  margin: 0;
}
main.deck.` + presenterModeClass + ` .pv-notes-body[data-empty="1"] {
  color: var(--fg-muted, currentColor);
  font-style: italic;
  opacity: 0.8;
}
/* Rendered note markdown (renderNoteMarkdown emits these from **bold**, *italic*,
   ` + "`code`" + `, and "- " bullets). Theme-agnostic: inherits the active palette. */
main.deck.` + presenterModeClass + ` .pv-notes-body strong { color: var(--accent, currentColor); font-weight: 700; }
main.deck.` + presenterModeClass + ` .pv-notes-body em { font-style: italic; }
main.deck.` + presenterModeClass + ` .pv-notes-body code {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 0.9em;
  background: var(--bg, rgba(128,128,128,0.12));
  border: 1px solid var(--line, rgba(128,128,128,0.25));
  border-radius: 5px; padding: 0.08em 0.36em;
}
main.deck.` + presenterModeClass + ` .pv-notes-body ul { margin: 0.3em 0; padding-left: 1.2em; list-style: disc; }
main.deck.` + presenterModeClass + ` .pv-notes-body li { margin: 0.15em 0; }

/* Footer instruments: timer + counter (mono so digits don't reflow), controls. */
main.deck.` + presenterModeClass + ` .pv-timer {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: clamp(1.4rem, 2.4vw, 2.1rem);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--fg, currentColor);
  letter-spacing: 0.02em;
}
main.deck.` + presenterModeClass + ` .pv-timer[data-paused="1"] { color: var(--fg-muted, currentColor); }
main.deck.` + presenterModeClass + ` .pv-counter {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: clamp(1rem, 1.6vw, 1.35rem);
  font-variant-numeric: tabular-nums;
  color: var(--fg-muted, currentColor);
}
main.deck.` + presenterModeClass + ` .pv-counter b {
  color: var(--fg, currentColor);
  font-weight: 700;
}
/* The click-step segment of the counter ("· step 2/2"), shown only on slides that
   have stepped code blocks. Accent-tinted so the presenter sees the walkthrough
   position at a glance without it competing with the slide number. */
main.deck.` + presenterModeClass + ` .pv-counter .pv-step { color: var(--accent, currentColor); }
main.deck.` + presenterModeClass + ` .pv-spacer { flex: 1 1 auto; }
main.deck.` + presenterModeClass + ` .pv-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

/* Rehearsal-recorder indicator. The live REC badge appears next to the timer when
   recording is active: a pulsing red dot + mono "REC mm:ss" readout. The badge is
   hidden by default (display:none) and shown by JS (display:flex) only while
   recording so it never takes space when idle. The rec-btn turns accent-red when
   active so the presenter sees the armed state without looking at the badge. */
main.deck.` + presenterModeClass + ` .pv-rec-badge {
  display: none;
  align-items: center;
  gap: 0.38em;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: clamp(0.85rem, 1.2vw, 1rem);
  font-variant-numeric: tabular-nums;
  color: #e53e3e;
  font-weight: 600;
  letter-spacing: 0.03em;
}
main.deck.` + presenterModeClass + ` .pv-rec-badge[data-recording="1"] { display: flex; }
main.deck.` + presenterModeClass + ` .pv-rec-dot {
  width: 0.62em;
  height: 0.62em;
  border-radius: 50%;
  background: #e53e3e;
  flex-shrink: 0;
}
@media (prefers-reduced-motion: no-preference) {
  main.deck.` + presenterModeClass + ` .pv-rec-dot {
    animation: pv-rec-pulse 1.1s ease-in-out infinite;
  }
  @keyframes pv-rec-pulse {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.25; }
  }
}
main.deck.` + presenterModeClass + ` .pv-rec-btn[data-recording="1"] {
  border-color: #e53e3e;
  color: #e53e3e;
}
main.deck.` + presenterModeClass + ` .pv-rec-toast {
  font-family: var(--font-body, system-ui, sans-serif);
  font-size: 0.82rem;
  color: var(--fg-muted, currentColor);
  opacity: 0;
  transition: opacity 300ms ease;
  white-space: nowrap;
}
main.deck.` + presenterModeClass + ` .pv-rec-toast[data-visible="1"] { opacity: 1; }
main.deck.` + presenterModeClass + ` .pv-btn {
  font-family: var(--font-display, var(--font-body, system-ui, sans-serif));
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--fg, currentColor);
  background: transparent;
  border: 1px solid var(--line, rgba(128,128,128,0.35));
  border-radius: calc(var(--radius, 12px) * 0.6);
  padding: 0.5em 0.85em;
  cursor: pointer;
  line-height: 1;
}
main.deck.` + presenterModeClass + ` .pv-btn:hover {
  border-color: var(--accent, currentColor);
  color: var(--accent, currentColor);
}
main.deck.` + presenterModeClass + ` .pv-btn:focus-visible {
  outline: 2px solid var(--accent, currentColor);
  outline-offset: 2px;
}
main.deck.` + presenterModeClass + ` .pv-btn:active { transform: translateY(1px); }
main.deck.` + presenterModeClass + ` .pv-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
@media (prefers-reduced-motion: no-preference) {
  main.deck.` + presenterModeClass + ` .pv-btn {
    transition: border-color 160ms cubic-bezier(0.25,1,0.5,1),
                color 160ms cubic-bezier(0.25,1,0.5,1),
                transform 120ms cubic-bezier(0.25,1,0.5,1);
  }
}
/* Narrow / portrait presenter windows stack the regions so the chrome never
   crushes (e.g. a phone used as a presenter remote). */
@media (max-aspect-ratio: 1/1), (max-width: 720px) {
  main.deck.` + presenterModeClass + ` {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: minmax(0, 1.4fr) minmax(0, 1fr) auto;
    grid-template-areas:
      "current"
      "side"
      "footer";
  }
}`
}

// presenterScript is the presenter-chrome controller, returned as inner JS (no
// <script> wrapper) so serve.go can append it to navScript's output. It defines a
// SINGLE global, window.SlidesPresenter.init(api), and runs nothing on its own —
// navScript calls init(api) only when the page is in presenter mode, handing it a
// tiny api surface ({ slides, count, getIndex, getStep, getStepCount, onChange,
// show, next, prev }) so this layer never reaches into navScript's internals.
//
// What init does:
//   - builds the chrome scaffold (current stage, next stage, notes panel, footer
//     with timer + counter + prev/next/reset/pause controls) and inserts it into
//     main.deck;
//   - renders the current+next previews by MOVING the live [data-slide] sections
//     into the stages (move, not clone, so islands keep their hydrated state),
//     re-arranging them whenever the slide changes;
//   - reads the current slide's note from its hidden <aside data-notes="N"> into
//     the notes panel, rendering BASIC inline markdown (**bold**, *italic*,
//     `code`, and "- " bullet lists) XSS-safely (escape first, then a fixed small
//     set of tags), or a graceful "No notes for this slide" placeholder;
//   - ticks an elapsed mm:ss / h:mm:ss timer (rAF-free setInterval) with reset +
//     pause/resume controls, PERSISTED to localStorage keyed to the deck path so a
//     presenter-window reload resumes the elapsed time (Reset clears it);
//   - shows the current slide's CLICK-STEP position in the footer counter when the
//     slide has stepped code blocks (e.g. "3 / 4 · step 2/2"), updating live as the
//     presenter steps through (api.onChange fires on step changes too);
//   - wires its prev/next buttons to api.next/api.prev (which drive navScript's
//     real state — step-then-slide — and in turn broadcast {index, step} to the
//     audience window).
//
// It subscribes to api.onChange so the previews/notes/counter re-render on EVERY
// navigation — including ones that arrive from the OTHER window over the
// BroadcastChannel — keeping the presenter and audience perfectly in lockstep.
func presenterScript() string {
	return `(function () {
  function pad(n) { return (n < 10 ? '0' : '') + n; }
  function fmtElapsed(ms) {
    var s = Math.floor(ms / 1000);
    var h = Math.floor(s / 3600);
    var m = Math.floor((s % 3600) / 60);
    var sec = s % 60;
    return h > 0 ? (h + ':' + pad(m) + ':' + pad(sec)) : (pad(m) + ':' + pad(sec));
  }

  // renderNoteMarkdown turns a speaker note into safe HTML with BASIC inline
  // markdown: **bold**, *italic*, ` + "`code`" + `, and "- " bullet lists. It is
  // XSS-safe by construction: the raw note is HTML-ESCAPED FIRST (so any <, >, &,
  // or " in author prose becomes an entity and can never form a tag), and ONLY
  // THEN are the markdown markers rewritten into a small, fixed set of tags
  // (<strong>/<em>/<code>/<ul>/<li>/<br>). No raw HTML from the note survives —
  // there is no passthrough path. Order matters: code spans are extracted first so
  // their contents aren't re-processed; bold (**) before italic (*) so ** isn't
  // mis-read as two * emphases.
  function escapeHTML(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;')
            .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }
  function renderInline(s) {
    // Code spans first: capture, escape already-applied, leave inner verbatim.
    s = s.replace(/` + "`" + `([^` + "`" + `]+)` + "`" + `/g, function (_, code) { return '<code>' + code + '</code>'; });
    // Bold then italic (so ** wins over *). Non-greedy, no newlines inside.
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    return s;
  }
  function renderNoteMarkdown(note) {
    var lines = escapeHTML(note).split('\n');
    var out = [];
    var inList = false;
    var prose = []; // run of consecutive non-list, non-blank lines -> joined with <br>
    function flushProse() {
      if (prose.length) { out.push(prose.join('<br>')); prose = []; }
    }
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i];
      var m = /^\s*[-*]\s+(.*)$/.exec(line); // "- item" or "* item" bullet
      if (m) {
        flushProse();
        if (!inList) { out.push('<ul>'); inList = true; }
        out.push('<li>' + renderInline(m[1]) + '</li>');
        continue;
      }
      if (inList) { out.push('</ul>'); inList = false; }
      if (line.trim() === '') { flushProse(); continue; }
      prose.push(renderInline(line));
    }
    flushProse();
    if (inList) out.push('</ul>');
    return out.join('');
  }

  function init(api) {
    var deck = document.querySelector('main.deck');
    if (!deck || !api || !api.slides || !api.slides.length) return;

    deck.classList.add('` + presenterModeClass + `');

    // Map slide-index -> its speaker-note text, read from the hidden asides
    // serve.go emitted (<aside class="slide-notes" data-notes="N">). Pulled once;
    // the asides stay in the DOM (hidden by CSS) as the source of truth.
    var notesByIndex = {};
    var asides = deck.querySelectorAll('.slide-notes[data-notes]');
    for (var a = 0; a < asides.length; a++) {
      var di = parseInt(asides[a].getAttribute('data-notes'), 10);
      if (!isNaN(di)) notesByIndex[di] = (asides[a].textContent || '').trim();
    }

    // --- Build the chrome scaffold ----------------------------------------
    var current = stage('pv-current', 'Current');
    var next = stage('pv-next', 'Next');
    var notesPanel = el('div', 'pv-notes');
    var notesLabel = el('div', 'pv-notes-label'); notesLabel.textContent = 'Notes';
    var notesBody = el('div', 'pv-notes-body');
    notesPanel.appendChild(notesLabel);
    notesPanel.appendChild(notesBody);

    var side = el('div', 'pv-side');
    side.appendChild(next.wrap);
    side.appendChild(notesPanel);

    var timer = el('div', 'pv-timer');
    var counter = el('div', 'pv-counter');
    var spacer = el('div', 'pv-spacer');
    var controls = el('div', 'pv-controls');
    var resetBtn = button('Reset timer', 'Reset');
    var pauseBtn = button('Pause timer', 'Pause');
    var prevBtn = button('Previous slide', 'Prev');
    var nextBtn = button('Next slide', 'Next');
    // Rehearsal-recorder controls: a toggle and a save button, off by default.
    var recBtn = button('Toggle rehearsal recording', '● Rec');
    recBtn.className += ' pv-rec-btn';
    var saveBtn = button('Save rehearsal', 'Save');
    saveBtn.style.display = 'none';
    controls.appendChild(resetBtn);
    controls.appendChild(pauseBtn);
    controls.appendChild(prevBtn);
    controls.appendChild(nextBtn);
    controls.appendChild(recBtn);
    controls.appendChild(saveBtn);

    // Recorder status badge: pulsing red dot + "REC mm:ss" elapsed-on-slide readout.
    var recBadge = el('div', 'pv-rec-badge');
    var recDot = el('span', 'pv-rec-dot');
    var recLabel = el('span', '');
    recBadge.appendChild(recDot);
    recBadge.appendChild(recLabel);

    // Brief "saved rehearsal.json" confirmation toast shown after download.
    var recToast = el('span', 'pv-rec-toast');

    var footer = el('div', 'pv-footer');
    footer.appendChild(timer);
    footer.appendChild(recBadge);
    footer.appendChild(counter);
    footer.appendChild(recToast);
    footer.appendChild(spacer);
    footer.appendChild(controls);

    // The current stage spans the left; side (next + notes) the right; footer the
    // bottom. Append after the slide sections so the grid areas resolve.
    deck.appendChild(current.wrap);
    deck.appendChild(side);
    deck.appendChild(footer);

    function el(tag, cls) { var n = document.createElement(tag); if (cls) n.className = cls; return n; }
    function stage(cls, labelText) {
      // A stage is a label header bar over a .pv-screen region; the live slide is
      // moved into the screen (below the label), so the label never overlaps the
      // slide's own heading. The screen is the positioned/clipped frame the scaled
      // slide fills.
      var wrap = el('div', 'pv-stage ' + cls);
      var label = el('div', 'pv-label'); label.textContent = labelText;
      var screen = el('div', 'pv-screen');
      screen.setAttribute('data-empty-label', cls === 'pv-next' ? 'End of deck' : 'No slide');
      wrap.appendChild(label);
      wrap.appendChild(screen);
      return { wrap: wrap, label: label, screen: screen };
    }
    function button(aria, text) {
      var b = el('button', 'pv-btn');
      b.type = 'button';
      b.setAttribute('aria-label', aria);
      b.textContent = text;
      return b;
    }

    // --- Preview rendering: MOVE the live sections into the stages ----------
    // Moving (not cloning) preserves island hydration state. navScript's slides[]
    // array holds the same node references, so toggling its active class still
    // works wherever the node currently lives in the DOM.
    function place(screen, slideIdx) {
      // Remove any previously-placed slide from this screen (back into the deck,
      // hidden by the presenter > .slide rule, so it can be re-placed later).
      var existing = screen.querySelector(':scope > .slide');
      if (existing) deck.appendChild(existing);
      if (slideIdx == null || slideIdx < 0 || slideIdx >= api.slides.length) {
        screen.setAttribute('data-empty', '1');
        return;
      }
      screen.removeAttribute('data-empty');
      screen.appendChild(api.slides[slideIdx]);
    }

    function render(index) {
      place(current.screen, index);
      place(next.screen, index + 1 < api.slides.length ? index + 1 : null);
      var note = notesByIndex[index];
      if (note) {
        // Render basic inline markdown (bold/italic/code + - bullet lists). The
        // transform escapes first, so author prose can never inject markup.
        notesBody.innerHTML = renderNoteMarkdown(note);
        notesBody.removeAttribute('data-empty');
      } else {
        notesBody.textContent = 'No notes for this slide';
        notesBody.setAttribute('data-empty', '1');
      }
      // Counter: slide position, plus the click-step position when THIS slide has
      // steps (e.g. "3 / 4 · step 2/2"). getStep/getStepCount come from navScript;
      // guard for older api shapes so a missing fn degrades to the plain counter.
      var stepCount = api.getStepCount ? api.getStepCount() : 0;
      var stepHTML = '';
      if (stepCount > 0) {
        var st = api.getStep ? api.getStep() : 0;
        stepHTML = ' <span class="pv-step">· step ' + st + '/' + stepCount + '</span>';
      }
      counter.innerHTML = '<b>' + (index + 1) + '</b> / ' + api.count + stepHTML;
      prevBtn.disabled = index <= 0;
      nextBtn.disabled = index >= api.count - 1;
    }

    // --- Timer (persisted across reloads) ----------------------------------
    // The elapsed timer survives a presenter-window reload by persisting its state
    // to localStorage keyed to THIS deck's path. State is {startedAt, accumulated,
    // paused}: startedAt is an absolute epoch ms, so on restore the running elapsed
    // is simply now - startedAt + accumulated and continues seamlessly across the
    // reload. Reset clears the stored state (and restarts from zero). A different
    // deck path gets a different key, so two decks don't share a timer. localStorage
    // failures (private mode, disabled) degrade silently to an in-memory timer.
    var timerKey = 'gosx-slides:timer:' + location.pathname;
    function saveTimer() {
      try {
        localStorage.setItem(timerKey, JSON.stringify({
          startedAt: startedAt, accumulated: accumulated, paused: paused
        }));
      } catch (e) {}
    }
    function loadTimer() {
      try {
        var raw = localStorage.getItem(timerKey);
        if (!raw) return null;
        var s = JSON.parse(raw);
        if (s && typeof s.startedAt === 'number' && typeof s.accumulated === 'number') return s;
      } catch (e) {}
      return null;
    }

    var startedAt = Date.now();
    var accumulated = 0; // ms banked across pauses
    var paused = false;
    var restored = loadTimer();
    if (restored) {
      startedAt = restored.startedAt;
      accumulated = restored.accumulated;
      paused = !!restored.paused;
    } else {
      saveTimer(); // first open: persist the start so a reload resumes from here
    }

    function elapsed() { return accumulated + (paused ? 0 : (Date.now() - startedAt)); }
    function tickTimer() {
      timer.textContent = fmtElapsed(elapsed());
      timer.setAttribute('data-paused', paused ? '1' : '0');
    }
    if (paused) pauseBtn.textContent = 'Resume'; // reflect a restored paused state
    var timerHandle = setInterval(tickTimer, 250);
    tickTimer();

    resetBtn.addEventListener('click', function () {
      accumulated = 0; startedAt = Date.now(); paused = false;
      pauseBtn.textContent = 'Pause';
      try { localStorage.removeItem(timerKey); } catch (e) {}
      saveTimer();
      tickTimer();
    });
    pauseBtn.addEventListener('click', function () {
      if (paused) { startedAt = Date.now(); paused = false; pauseBtn.textContent = 'Pause'; }
      else { accumulated = elapsed(); paused = true; pauseBtn.textContent = 'Resume'; }
      saveTimer();
      tickTimer();
    });

    // --- Controls drive navScript's real state (which broadcasts) ----------
    prevBtn.addEventListener('click', function () { api.prev(); });
    nextBtn.addEventListener('click', function () { api.next(); });

    // --- Rehearsal recorder ------------------------------------------------
    // Records how long the presenter spends on each slide. All state is local to
    // this init() call (reset on every presenter-window open). No server endpoint
    // is used: on stop, a rehearsal.json is built in memory and offered as a
    // client-side download via a transient <a download> link.
    //
    // Per-slide title extraction: each slide section ([data-slide="N"]) may hold
    // any heading element (h1-h6). We query the first heading inside the section
    // to get a human-readable title. This is the SAME DOM node that presenterScript
    // moves into the preview stages (it is the live section, not a clone), so the
    // heading text is always accessible regardless of which stage it is currently
    // parked in — we query api.slides[i] directly (the section node reference).
    function slideTitle(index) {
      var sec = (api.slides && api.slides[index]) || null;
      if (!sec) return '';
      var h = sec.querySelector('h1,h2,h3,h4,h5,h6');
      return h ? (h.textContent || '').trim() : '';
    }

    // Recorder state: not recording by default.
    var recActive = false;
    var recSlides = [];           // [{index, title, seconds}] committed entries
    var recSlideStart = 0;        // Date.now() when current slide became active
    var recCurrentIndex = -1;     // slide index active at the moment rec started/changed

    // Update the badge's "REC mm:ss" elapsed-on-this-slide readout once per tick.
    function tickRecBadge() {
      if (!recActive) return;
      var ms = Date.now() - recSlideStart;
      var s = Math.floor(ms / 1000);
      var m = Math.floor(s / 60);
      recLabel.textContent = 'REC ' + pad(m) + ':' + pad(s % 60);
    }
    // Attach the rec tick to the same setInterval that drives the main timer, by
    // replacing the tickTimer call with a combined tick wrapper after the interval
    // is set. We extend it via a wrapper so we don't need a second interval.
    var origTickTimer = tickTimer;
    tickTimer = function () { origTickTimer(); tickRecBadge(); };
    // Flush the current slide's elapsed time into recSlides when the slide changes
    // (or when recording stops). index is the slide being LEFT.
    function flushRecSlide(index) {
      if (!recActive || index < 0 || index >= api.count) return;
      var secs = Math.round((Date.now() - recSlideStart) / 1000);
      recSlides.push({ index: index, title: slideTitle(index), seconds: secs });
    }

    function setRecording(active) {
      recActive = active;
      recBtn.setAttribute('data-recording', active ? '1' : '0');
      recBadge.setAttribute('data-recording', active ? '1' : '0');
      saveBtn.style.display = active ? '' : 'none';
      if (active) {
        // Start fresh: clear any previous session data, record the current slide.
        recSlides = [];
        recCurrentIndex = api.getIndex();
        recSlideStart = Date.now();
        tickRecBadge();
      }
    }

    function downloadRehearsal() {
      // Build the rehearsal object. Flush the last (current) slide first.
      flushRecSlide(recCurrentIndex);
      var totalSeconds = 0;
      for (var i = 0; i < recSlides.length; i++) totalSeconds += recSlides[i].seconds;
      // Deck title: prefer the first h1 in the page, fall back to document.title.
      var titleEl = document.querySelector('h1');
      var deckTitle = (titleEl ? titleEl.textContent : '') || document.title || '';
      var payload = {
        deck: deckTitle.trim(),
        recordedAtMs: Date.now(),
        totalSeconds: totalSeconds,
        slides: recSlides
      };
      var json = JSON.stringify(payload, null, 2);
      var blob = new Blob([json], { type: 'application/json' });
      var url = URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = 'rehearsal.json';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      // Show a brief confirmation toast, then fade it out after 3 s.
      recToast.textContent = 'saved rehearsal.json';
      recToast.setAttribute('data-visible', '1');
      setTimeout(function () { recToast.removeAttribute('data-visible'); }, 3000);
    }

    recBtn.addEventListener('click', function () {
      if (!recActive) {
        setRecording(true);
      } else {
        downloadRehearsal();
        setRecording(false);
      }
    });
    saveBtn.addEventListener('click', function () {
      downloadRehearsal();
      setRecording(false);
    });

    // Hook into the slide-change signal: when the slide changes while recording,
    // flush the previous slide's time and reset the slide-start clock.
    var origRender = render;
    render = function (index) {
      if (recActive && recCurrentIndex !== index) {
        flushRecSlide(recCurrentIndex);
        recSlideStart = Date.now();
        recCurrentIndex = index;
      }
      origRender(index);
    };

    // Re-render on EVERY slide change, including ones arriving from the audience
    // window over the BroadcastChannel (navScript applies those, then notifies us).
    api.onChange(render);
    render(api.getIndex());

    window.SlidesPresenter._cleanup = function () { clearInterval(timerHandle); };
  }

  window.SlidesPresenter = { init: init };
})();`
}

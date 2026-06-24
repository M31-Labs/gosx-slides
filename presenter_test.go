package slides

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestServeInjectsPresenterStyle proves the served real-lane page carries the
// presenter-chrome stylesheet, gated under the deck-presenter class so it is inert
// in the audience view, plus the unconditional rule that hides speaker-note asides
// in both views. This is what turns a ?present load into the two-region chrome.
func TestServeInjectsPresenterStyle(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)
	for _, want := range []string{
		"main.deck." + presenterModeClass,          // chrome gated under the class
		".slide-notes { display: none !important;", // notes hidden in both views
		".pv-current",            // the large current preview stage
		".pv-next",               // the next preview stage
		".pv-notes",              // the notes panel
		".pv-timer",              // the elapsed timer
		".pv-counter",            // the slide counter
		"prefers-reduced-motion", // motion is reduced-motion-safe
		"var(--accent",           // theme-agnostic: reads theme tokens
	} {
		if !strings.Contains(body, want) {
			t.Errorf("presenter style missing %q:\n%s", want, body)
		}
	}
}

// TestServeInjectsPresenterScript proves the served page carries the presenter
// controller wiring inside the same nav <script>: present-mode detection, the
// BroadcastChannel peer-to-peer sync (keyed to the deck path), the p-key trigger
// that opens the presenter window, and the timer. These are the contract surface
// the two-window sync gate exercises.
func TestServeInjectsPresenterScript(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)
	script := extractFirstScript(t, body)
	for _, want := range []string{
		"BroadcastChannel", // peer-to-peer transport
		"gosx-slides:",     // channel keyed to the deck path
		"postMessage",      // broadcasts the index on navigation
		"applyingRemote",   // self-echo guard (no ping-pong loop)
		"openPresenter",    // the p-key opens a presenter window
		"?present",         // presenter window is the same page + ?present
		presenterModeClass, // adds the presenter class in present mode
		"SlidesPresenter",  // hands off to the presenter chrome controller
		"setInterval",      // the elapsed timer ticks
		"onChange",         // re-renders on every change (incl. remote)
	} {
		if !strings.Contains(script, want) {
			t.Errorf("presenter script missing %q:\n%s", want, script)
		}
	}
}

// TestPresenterSyncDoesNotBreakSingleSlideRule proves the presenter chrome layers
// ON TOP of the one-slide-at-a-time visibility rule without removing it (mirroring
// the overview-grid invariant): nav's :not(.deck-active) display:none rule is
// still intact, and the presenter reveal is gated behind the presenter class so a
// normal (audience) load is untouched.
func TestPresenterSyncDoesNotBreakSingleSlideRule(t *testing.T) {
	css := navStyle() + "\n" + presenterStyle()
	if !strings.Contains(css, "main.deck > .slide:not(."+navActiveClass+") { display: none !important; }") {
		t.Fatalf("presenter CSS clobbered the single-slide visibility rule:\n%s", css)
	}
	// The presenter chrome container rule must be scoped under the presenter class.
	if !strings.Contains(css, "main.deck."+presenterModeClass+" {") {
		t.Errorf("presenter chrome not scoped under the presenter class:\n%s", css)
	}
}

// TestExtractSlideNotes proves the real lane recovers speaker notes from BOTH the
// trailing-comment form and the <Notes>…</Notes> block form (the same two forms
// the fallback lane's extractNotes understands), reading them out of the slide's
// mdpp subtree, and returns "" for a slide with no notes.
func TestExtractSlideNotes(t *testing.T) {
	deckMD := strings.Join([]string{
		"# Comment note",
		"",
		"prose one",
		"",
		"<!-- speak slowly on this slide -->",
		"",
		"---",
		"",
		"# Block note",
		"",
		"prose two",
		"",
		"<Notes>",
		"mention the live demo",
		"and the timer",
		"</Notes>",
		"",
		"---",
		"",
		"# No note",
		"",
		"prose three",
		"",
	}, "\n")

	dir := newDeckDirUnderModule(t, deckMD, nil)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("want 3 slides, got %d", len(deck.Slides))
	}

	if got := extractSlideNotes(deck.Slides[0]); got != "speak slowly on this slide" {
		t.Errorf("comment-form note = %q, want %q", got, "speak slowly on this slide")
	}
	if got := extractSlideNotes(deck.Slides[1]); got != "mention the live demo\nand the timer" {
		t.Errorf("block-form note = %q, want %q", got, "mention the live demo\nand the timer")
	}
	if got := extractSlideNotes(deck.Slides[2]); got != "" {
		t.Errorf("note-less slide should yield \"\", got %q", got)
	}
}

// TestServeEmitsHiddenNoteAsides proves a slide that carries a note emits a hidden
// <aside class="slide-notes" data-notes="N"> in the served page (the source the
// presenter chrome reads), that a note-less slide emits none, and that the note
// text is HTML-escaped (opaque author prose, never injected markup).
func TestServeEmitsHiddenNoteAsides(t *testing.T) {
	deckMD := "# One\n\nfirst\n\n<!-- escape <b> & \"this\" -->\n\n---\n\n# Two\n\nsecond, no note\n"
	body := serveBody(t, deckMD, nil)

	asideRe := regexp.MustCompile(`(?s)<aside[^>]*class="slide-notes"[^>]*>.*?</aside>`)
	asides := asideRe.FindAllString(body, -1)
	if len(asides) != 1 {
		t.Fatalf("want exactly 1 note aside (only slide 0 has a note), got %d:\n%v", len(asides), asides)
	}
	aside := asides[0]
	for _, want := range []string{`data-notes="0"`, "hidden", "&lt;b&gt;", "&amp;"} {
		if !strings.Contains(aside, want) {
			t.Errorf("note aside missing %q: %s", want, aside)
		}
	}
	// The raw, unescaped note markup must NOT appear (XSS-safe).
	if strings.Contains(body, "<b> &") {
		t.Errorf("note text was not escaped — raw markup leaked into the page")
	}
	// Slide 1 has no note, so there must be no data-notes="1" aside.
	if strings.Contains(body, `data-notes="1"`) {
		t.Errorf("note-less slide 1 should not emit a note aside")
	}
}

// TestPresenterChromeInertWithoutClass proves the audience view is unaffected: the
// presenter chrome stylesheet does all its work under .deck-presenter, so every
// chrome rule is gated on that class. The only UNGATED presenter rule is the one
// that hides .slide-notes (an author aside, hidden in both views) — verified to be
// the sole exception so a stray ungated chrome rule can't bleed into the audience.
func TestPresenterChromeInertWithoutClass(t *testing.T) {
	css := presenterStyle()
	// Every `main.deck ... {` selector block must either be the slide-notes hide
	// rule or be scoped under the presenter class. Walk top-level selector heads.
	for _, sel := range topLevelSelectors(css) {
		if !strings.Contains(sel, "main.deck") {
			continue // @media / @keyframes wrappers handled by their inner rules
		}
		if strings.Contains(sel, presenterModeClass) {
			continue // properly gated
		}
		if strings.Contains(sel, ".slide-notes") {
			continue // the one intentional both-views rule
		}
		t.Errorf("presenter rule not gated under .%s and not the slide-notes rule: %q", presenterModeClass, sel)
	}
}

// topLevelSelectors returns the selector text preceding each top-level `{` in css
// (a coarse split good enough to assert gating). Nested rules inside @media blocks
// are included via their own `main.deck...` heads.
func topLevelSelectors(css string) []string {
	var sels []string
	for _, chunk := range strings.Split(css, "{") {
		// The selector is the last line of the chunk before the brace.
		lines := strings.Split(strings.TrimSpace(chunk), "\n")
		sel := strings.TrimSpace(lines[len(lines)-1])
		if sel != "" {
			sels = append(sels, sel)
		}
	}
	return sels
}

// TestPresenterScriptCarriesStepCounter proves the presenter footer counter is
// step-aware: it reads getStep/getStepCount and emits a "· step K/N" segment so a
// slide with stepped code blocks shows the live walkthrough position.
func TestPresenterScriptCarriesStepCounter(t *testing.T) {
	script := presenterScript()
	for _, want := range []string{
		"getStepCount", // reads the slide's step budget from navScript
		"getStep",      // reads the active step
		"step ",        // the "· step K/N" label
		"pv-step",      // the styled step span
	} {
		if !strings.Contains(script, want) {
			t.Errorf("presenter step counter missing %q:\n%s", want, script)
		}
	}
	// The footer-counter CSS must style the step segment.
	if !strings.Contains(presenterStyle(), ".pv-step") {
		t.Errorf("presenter style missing .pv-step rule")
	}
}

// TestPresenterTimerPersistence proves the elapsed timer persists to localStorage
// keyed to the deck path: save/load/clear are wired, the key is path-scoped, and
// the stored state carries the absolute start so a reload resumes the elapsed
// time. Reset removes the key.
func TestPresenterTimerPersistence(t *testing.T) {
	script := presenterScript()
	for _, want := range []string{
		"localStorage", // persistence backend
		"gosx-slides:timer:' + location.pathname", // path-scoped key
		"setItem",    // saves state
		"getItem",    // restores state on reload
		"removeItem", // Reset clears it
		"startedAt",  // absolute start persisted
		"saveTimer",  // save helper
		"loadTimer",  // restore helper
	} {
		if !strings.Contains(script, want) {
			t.Errorf("presenter timer persistence missing %q:\n%s", want, script)
		}
	}
}

// TestPresenterNoteMarkdownRenders proves the presenter's note renderer turns
// BASIC inline markdown into the matching tags AND is XSS-safe. It EXECUTES the
// generated renderNoteMarkdown via node (skipping if node is absent) so the test
// checks real transform output, not just the source text: **bold** -> <strong>,
// `code` -> <code>, *italic* -> <em>, "- " lines -> <ul><li>, and a raw <script>
// in the note is escaped, never injected.
func TestPresenterNoteMarkdownRenders(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping note-markdown execution check")
	}
	script := presenterScript()

	// Drive renderNoteMarkdown(note) for a note exercising every feature + an XSS
	// attempt. The presenter IIFE defines window.SlidesPresenter but renderNoteMarkdown
	// is a closure inside init — so we eval the function body standalone by exposing
	// it. Simplest: re-declare a tiny harness that pastes the helper functions. We
	// extract them from the script via known markers so the test runs the REAL code.
	start := strings.Index(script, "function escapeHTML(")
	end := strings.Index(script, "function init(")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("could not locate the note-markdown helpers in presenterScript")
	}
	helpers := script[start:end]

	note := "Say **hello** and `run()` then *pause*.\n- first\n- second\n<script>alert(1)</script>"
	harness := "globalThis.localStorage = { _m:{}, getItem(k){return this._m[k]||null;}, setItem(k,v){this._m[k]=v;}, removeItem(k){delete this._m[k];} };\n" +
		"globalThis.location = { pathname: '/deck' };\n" +
		helpers +
		"\nconst note = " + jsStringLiteral(note) + ";\n" +
		"process.stdout.write(renderNoteMarkdown(note));\n"

	f := filepath.Join(t.TempDir(), "notemd.js")
	if err := os.WriteFile(f, []byte(harness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	out, err := exec.Command(node, f).CombinedOutput()
	if err != nil {
		t.Fatalf("node failed to run note-markdown harness: %v\n%s\n--- harness ---\n%s", err, out, harness)
	}
	got := string(out)

	for _, want := range []string{
		"<strong>hello</strong>",
		"<code>run()</code>",
		"<em>pause</em>",
		"<ul>", "<li>first</li>", "<li>second</li>", "</ul>",
		"&lt;script&gt;", // the XSS attempt is escaped
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered note markdown missing %q:\ngot: %s", want, got)
		}
	}
	// The raw, unescaped script tag must NOT survive.
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Fatalf("note markdown injected raw HTML (XSS): %s", got)
	}
}

// jsStringLiteral renders s as a double-quoted JS string literal safe to embed in
// generated test JS (escapes backslash, quote, and the line terminators).
func jsStringLiteral(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return `"` + r.Replace(s) + `"`
}

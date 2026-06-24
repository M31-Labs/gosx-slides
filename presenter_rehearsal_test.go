package slides

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRehearsalRecorderScriptHooks asserts that presenterScript() contains the
// key rehearsal-recorder hooks: the toggle button id/class, the download trigger,
// and the output filename so any future refactor cannot silently drop them.
func TestRehearsalRecorderScriptHooks(t *testing.T) {
	script := presenterScript()
	for _, want := range []string{
		"rehearsal.json",    // the download filename
		"pv-rec-btn",        // the record-toggle button class
		"pv-rec-badge",      // the live recording indicator badge
		"recActive",         // the recording state flag
		"flushRecSlide",     // the per-slide flush helper
		"downloadRehearsal", // the download builder
		"recordedAtMs",      // the output JSON shape (timestamp)
		"totalSeconds",      // the output JSON shape (total)
		"slideTitle",        // per-slide title extraction helper
		"recSlides",         // the in-progress recording buffer
		"recSlideStart",     // the per-slide start timestamp
		"setRecording",      // the toggle helper
		"recToast",          // the brief confirmation toast
	} {
		if !strings.Contains(script, want) {
			t.Errorf("presenterScript() missing rehearsal hook %q", want)
		}
	}
}

// TestRehearsalRecorderStyleHooks asserts that presenterStyle() contains the
// recorder-indicator CSS classes referenced by the JS, so the visual affordance
// is always present alongside its controller.
func TestRehearsalRecorderStyleHooks(t *testing.T) {
	style := presenterStyle()
	for _, want := range []string{
		".pv-rec-badge",  // the live badge container
		".pv-rec-dot",    // the pulsing dot
		".pv-rec-btn",    // the toggle button accent state
		".pv-rec-toast",  // the save confirmation toast
		"pv-rec-pulse",   // the keyframe animation for the dot
		"data-recording", // the JS-toggled attribute that shows/hides the badge
	} {
		if !strings.Contains(style, want) {
			t.Errorf("presenterStyle() missing rehearsal indicator rule/token %q", want)
		}
	}
}

// TestRehearsalRecorderChromeGating asserts every new recorder CSS rule is gated
// under the deck-presenter class (consistent with the existing chrome rules) or is
// a keyframe / animation helper, so it never bleeds into the audience view.
func TestRehearsalRecorderChromeGating(t *testing.T) {
	css := presenterStyle()
	for _, sel := range topLevelSelectors(css) {
		if !strings.Contains(sel, "main.deck") {
			continue // @media / @keyframes / @-rules are not deck selectors
		}
		if strings.Contains(sel, presenterModeClass) {
			continue // correctly gated
		}
		if strings.Contains(sel, ".slide-notes") {
			continue // intentional both-views rule
		}
		t.Errorf("presenter/recorder CSS rule not gated under .%s and not the slide-notes rule: %q",
			presenterModeClass, sel)
	}
}

// TestRehearsalDownloadBuildsPayload executes the recorder helpers via node (skipped
// when node is absent) and verifies that the downloadRehearsal logic produces a
// JSON payload with the expected shape: deck, recordedAtMs, totalSeconds, slides[].
func TestRehearsalDownloadBuildsPayload(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping rehearsal download execution check")
	}

	script := presenterScript()

	// Extract the pure recorder helper functions: slideTitle, flushRecSlide,
	// setRecording, and downloadRehearsal. We start at slideTitle and stop just
	// before the event-listener wiring (recBtn.addEventListener) that depends on
	// init-local DOM nodes. The origTickTimer reassignment is also init-local but
	// sits BETWEEN the state declarations and setRecording; we strip just that pair
	// of lines from the extracted block so the harness gets all the pure functions.
	start := strings.Index(script, "function slideTitle(")
	end := strings.Index(script, "recBtn.addEventListener(")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("could not locate recorder helpers in presenterScript (start=%d, end=%d)", start, end)
	}
	recHelpers := script[start:end]
	// Remove the origTickTimer / tickTimer reassignment lines that close over the
	// init-local tickTimer variable; all other code in recHelpers is self-contained.
	recHelpers = strings.ReplaceAll(recHelpers, "var origTickTimer = tickTimer;\n", "")
	recHelpers = strings.ReplaceAll(recHelpers, "    tickTimer = function () { origTickTimer(); tickRecBadge(); };\n", "")

	// We also need the pad() and fmtElapsed() helpers (defined at the top of the IIFE).
	padStart := strings.Index(script, "function pad(")
	padEnd := strings.Index(script, "function escapeHTML(")
	if padStart < 0 || padEnd < 0 {
		t.Fatalf("could not locate pad/fmtElapsed helpers in presenterScript")
	}
	padHelpers := script[padStart:padEnd]

	// Build a minimal harness: stub out the DOM/Browser APIs the recorder touches
	// and drive flushRecSlide + downloadRehearsal to inspect the captured JSON.
	harness := `
// --- DOM / Browser stubs ---
function makeDOMNode() {
  var attrs = {}, _style = {}, _text = '';
  return {
    textContent: '',
    style: _style,
    setAttribute: function(k,v) { attrs[k]=v; },
    getAttribute: function(k) { return attrs[k]||null; },
    removeAttribute: function(k) { delete attrs[k]; },
    click: function() {},
  };
}
globalThis.document = {
  title: 'Test Deck',
  querySelector: function(sel) {
    if (sel === 'h1') return { textContent: 'Keynote Title' };
    return null;
  },
  createElement: function(tag) {
    var n = makeDOMNode(); n._tag = tag; n.href = ''; n.download = ''; return n;
  },
  body: { appendChild: function() {}, removeChild: function() {} },
};
globalThis.URL = {
  createObjectURL: function() { return 'blob:fake'; },
  revokeObjectURL: function() {},
};
globalThis.setTimeout = function() {};
globalThis.Blob = function(parts) { this._parts = parts; };

// --- init-local DOM stubs that setRecording/downloadRehearsal reference ---
var recBtn   = makeDOMNode();
var recBadge = makeDOMNode();
var saveBtn  = { style: {} };
var recToast = makeDOMNode();
var recLabel = makeDOMNode();

` + padHelpers + `
` + recHelpers + `

// Simulate: recorder ON at slide 0, then move to slide 1 after 5 s.
var api = {
  count: 3,
  getIndex: function() { return 0; },
  slides: [
    { querySelector: function(s) { return { textContent: 'Intro' }; } },
    { querySelector: function(s) { return { textContent: 'Main' }; } },
    { querySelector: function(s) { return { textContent: 'End' }; } },
  ],
};

recActive = false;
recSlides = [];
recCurrentIndex = -1;
recSlideStart = 0;

// Set recording ON at slide 0.
setRecording(true);
recCurrentIndex = 0;
// Fake 5 seconds elapsed on slide 0.
recSlideStart = Date.now() - 5000;
// Simulate slide change to 1.
flushRecSlide(0);
recSlideStart = Date.now() - 3000;
recCurrentIndex = 1;

// Capture the Blob payload produced by downloadRehearsal.
var captured = '';
globalThis.Blob = function(parts) { captured = parts[0]; };

downloadRehearsal();

var payload = JSON.parse(captured);
// Output key facts so the test can verify them.
process.stdout.write(JSON.stringify({
  deck: payload.deck,
  hasRecordedAtMs: typeof payload.recordedAtMs === 'number',
  totalSeconds: payload.totalSeconds,
  slideCount: payload.slides.length,
  firstSlideIndex: payload.slides[0] ? payload.slides[0].index : -1,
  firstSlideTitle: payload.slides[0] ? payload.slides[0].title : '',
}));
`

	f := filepath.Join(t.TempDir(), "rec_harness.js")
	if err := os.WriteFile(f, []byte(harness), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	out, err := exec.Command(node, f).CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\n%s", err, out)
	}

	got := string(out)
	for _, want := range []string{
		`"deck":"Keynote Title"`,
		`"hasRecordedAtMs":true`,
		`"slideCount":2`,
		`"firstSlideIndex":0`,
		`"firstSlideTitle":"Intro"`,
	} {
		if !strings.Contains(strings.ReplaceAll(got, " ", ""), strings.ReplaceAll(want, " ", "")) {
			t.Errorf("rehearsal payload missing %q\ngot: %s", want, got)
		}
	}
	// totalSeconds should be approximately 5+3=8 (allow ±2 for timing slack).
	if !strings.Contains(got, `"totalSeconds":`) {
		t.Errorf("rehearsal payload missing totalSeconds\ngot: %s", got)
	}
}

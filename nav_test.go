package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// serveBody builds the real-lane App for a deck written from deckMD (+ optional
// components) and returns the body of GET /. It is the Slice-6 HTTP-contract
// harness: the nav layer is injected by renderPage, so asserting against the
// served document is the honest, end-to-end check.
func serveBody(t *testing.T, deckMD string, components map[string]string) string {
	t.Helper()
	dir := newDeckDirUnderModule(t, deckMD, components)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

const twoSlideDeck = "# One\n\nfirst slide\n\n---\n\n# Two\n\nsecond slide\n"

// TestServeInjectsSlideVisibilityStyle proves the served real-lane page carries
// the slide-visibility stylesheet: scoped under main.deck, hiding every slide and
// showing only the one with the active class. This is what turns the flat scroll
// into a one-slide-at-a-time deck.
func TestServeInjectsSlideVisibilityStyle(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)

	if !strings.Contains(body, "<style>") {
		t.Fatalf("served page has no <style> block:\n%s", body)
	}
	// The visibility rule: hide slides, show the active one, scoped to main.deck.
	for _, want := range []string{
		"main.deck",
		"display: none",
		navActiveClass,
		"display: block",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("slide-visibility style missing %q:\n%s", want, body)
		}
	}
}

// TestServeInjectsNavScript proves the served page carries the nav controller
// <script> with the load-bearing pieces: it queries data-slide sections, wires
// the Arrow/Space keyboard handlers, and syncs the URL hash via history. These
// are the contract surface the manual browser gate exercises.
func TestServeInjectsNavScript(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)

	script := extractFirstScript(t, body)
	for _, want := range []string{
		"data-slide",   // operates on the same sections the generator emits
		"ArrowRight",   // next
		"ArrowLeft",    // prev
		"keydown",      // keyboard wiring
		"location.hash", // reads the deep-link hash
		"history",      // replaceState URL sync
		navActiveClass, // toggles the active class
	} {
		if !strings.Contains(script, want) {
			t.Errorf("nav script missing %q:\n%s", want, script)
		}
	}
}

// TestServeRendersDataSlideSections proves the nav layer did not disturb the
// data-slide sections it drives: a 2-slide deck still emits both
// `<section class="slide" data-slide="N">` elements (0-based), so the controller
// has something to navigate.
func TestServeRendersDataSlideSections(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)
	for _, want := range []string{`data-slide="0"`, `data-slide="1"`} {
		if !strings.Contains(body, want) {
			t.Errorf("served page missing slide section %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `data-slide="2"`) {
		t.Errorf("served page has an unexpected third slide section:\n%s", body)
	}
}

// TestNavScriptParses validates that the injected controller is syntactically
// valid JavaScript by running `node --check` on the extracted script — the same
// "does the generated JS actually parse" gate the dev lane relies on. If node is
// not on PATH the check is skipped (the string assertions above still hold), so
// the suite stays green in a node-less CI while catching syntax errors locally.
func TestNavScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping JS parse check (string assertions still cover the script)")
	}

	// Validate the raw controller source directly (no <script> wrapper), which is
	// what node --check expects.
	script := navScript()
	f := filepath.Join(t.TempDir(), "nav.js")
	if err := os.WriteFile(f, []byte(script), 0o644); err != nil {
		t.Fatalf("write nav.js: %v", err)
	}
	out, err := exec.Command(node, "--check", f).CombinedOutput()
	if err != nil {
		t.Fatalf("node --check rejected the nav script: %v\n%s\n--- script ---\n%s", err, out, script)
	}
}

// TestServedNavScriptParses validates that the script AS SERVED (extracted from
// the page body, inside its <script> tag) parses — guarding against an injection
// bug that mangles the JS between navScript() and the rendered document.
func TestServedNavScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping served-JS parse check")
	}
	body := serveBody(t, twoSlideDeck, nil)
	script := extractFirstScript(t, body)
	f := filepath.Join(t.TempDir(), "served-nav.js")
	if err := os.WriteFile(f, []byte(script), 0o644); err != nil {
		t.Fatalf("write served-nav.js: %v", err)
	}
	out, err := exec.Command(node, "--check", f).CombinedOutput()
	if err != nil {
		t.Fatalf("node --check rejected the served nav script: %v\n%s\n--- script ---\n%s", err, out, script)
	}
}

// TestServeNavCoexistsWithIsland proves the nav layer does not break island
// hydration: a deck whose slide hosts a live <Counter/> still mounts the island
// AND carries the nav controller + visibility style on the same page. (Hidden
// slides still hydrate; the controller only toggles visibility.)
func TestServeNavCoexistsWithIsland(t *testing.T) {
	deckMD := "# One\n\n<Counter initial={3}/>\n\n---\n\n# Two\n\nsecond\n"
	body := serveBody(t, deckMD, map[string]string{"Counter": counterGSX})

	if !strings.Contains(body, `data-gosx-island="Counter"`) {
		t.Errorf("nav injection broke island mount (no live Counter):\n%s", body)
	}
	if !strings.Contains(body, "/gosx/wasm_exec.js") {
		t.Errorf("nav injection broke the client bootstrap (no wasm_exec loader)")
	}
	if !strings.Contains(body, "<style>") || !strings.Contains(body, navActiveClass) {
		t.Errorf("nav style missing on an island deck")
	}
	if !strings.Contains(extractFirstScript(t, body), "ArrowRight") {
		t.Errorf("nav script missing on an island deck")
	}
}

// scriptRe captures the inner JS of the FIRST <script>…</script> with a body.
// The PageHead bootstrap may emit script tags too, so we scan for the controller
// specifically (it is the only one that mentions data-slide / ArrowRight).
var scriptRe = regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)

// extractFirstScript returns the inner text of the nav controller's <script>
// (the one referencing data-slide). It fails the test if no such script is
// present, so a missing injection is reported sharply rather than as a confusing
// empty-string assertion.
func extractFirstScript(t *testing.T, body string) string {
	t.Helper()
	for _, m := range scriptRe.FindAllStringSubmatch(body, -1) {
		if strings.Contains(m[1], "data-slide") {
			return m[1]
		}
	}
	t.Fatalf("no nav controller <script> (referencing data-slide) found in body:\n%s", body)
	return ""
}

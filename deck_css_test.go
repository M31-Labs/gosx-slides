package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deck_css_test.go proves the first-class per-deck stylesheet lane: a deck.css
// next to deck.md (or files named by `css:` headmatter) is inlined into the
// served <head> AFTER the theme, replacing the old inject-a-<link>-via-island
// workaround.

// serveBodyWithFiles is serveBody plus extra plain files written into the deck
// dir before load (e.g. a deck.css).
func serveBodyWithFiles(t *testing.T, deckMD string, files map[string]string) string {
	t.Helper()
	dir := newDeckDirUnderModule(t, deckMD, nil)
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// TestDeckCSSAutoLoaded proves a conventional deck.css is discovered and
// inlined after the theme stylesheet.
func TestDeckCSSAutoLoaded(t *testing.T) {
	body := serveBodyWithFiles(t, "---\ntitle: T\ntheme: aurora\n---\n\n# One\n",
		map[string]string{"deck.css": ".slide h1 { color: rebeccapurple; }"})

	custom := strings.Index(body, ".slide h1 { color: rebeccapurple; }")
	if custom < 0 {
		t.Fatalf("expected deck.css content inlined in the page, got:\n%s", body)
	}
	if !strings.Contains(body, `data-deck-css`) {
		t.Errorf("expected the deck css style block to carry its marker attribute")
	}
	// Cascade position: the custom block must come after the theme block.
	theme := strings.Index(body, `data-theme="aurora"`)
	themeStyle := strings.Index(body, "main.deck[data-theme=")
	if themeStyle < 0 || custom < themeStyle {
		t.Errorf("expected deck css after the theme css (theme at %d, custom at %d, marker at %d)", themeStyle, custom, theme)
	}
}

// TestDeckCSSHeadmatterList proves `css:` headmatter names the files (comma
// separated, subdirectories allowed) and that traversal escapes are rejected.
func TestDeckCSSHeadmatterList(t *testing.T) {
	body := serveBodyWithFiles(t,
		"---\ntitle: T\ntheme: aurora\ncss: brand.css, sub/extra.css\n---\n\n# One\n",
		map[string]string{
			"brand.css":     ".brand { border: 1px solid red; }",
			"sub/extra.css": ".extra { margin: 2px; }",
			// present on disk but NOT named: must not load
			"deck.css": ".never { color: blue; }",
		})

	if !strings.Contains(body, ".brand { border: 1px solid red; }") ||
		!strings.Contains(body, ".extra { margin: 2px; }") {
		t.Fatalf("expected both named css files inlined, got:\n%s", body)
	}
	if strings.Contains(body, ".never") {
		t.Errorf("deck.css must not auto-load when css: names files explicitly")
	}
}

// TestDeckCSSTraversalRejected proves a `css:` path escaping the deck dir is
// ignored rather than read.
func TestDeckCSSTraversalRejected(t *testing.T) {
	dir := newDeckDirUnderModule(t, "---\ntitle: T\ncss: ../outside.css\n---\n\n# One\n", nil)
	outside := filepath.Join(dir, "..", "outside.css")
	if err := os.WriteFile(outside, []byte(".stolen{}"), 0o644); err != nil {
		t.Fatalf("write outside.css: %v", err)
	}
	t.Cleanup(func() { os.Remove(outside) })
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	if css := deckCustomCSS(deck); strings.Contains(css, ".stolen") {
		t.Fatalf("traversal path was read: %q", css)
	}
}

// TestDeckCSSBreakoutNeutralized proves a stylesheet containing `</style>`
// cannot terminate the inline block early.
func TestDeckCSSBreakoutNeutralized(t *testing.T) {
	body := serveBodyWithFiles(t, "---\ntitle: T\n---\n\n# One\n",
		map[string]string{"deck.css": `.x { content: "</style><script>alert(1)</script>"; }`})

	// The payload is inert while it stays INSIDE the style element's raw text;
	// breakout requires an early `</style>` immediately followed by the script.
	if strings.Contains(body, "</style><script>alert(1)") {
		t.Fatalf("deck css broke out of its style element:\n%s", body)
	}
	if !strings.Contains(body, `<\/style>`) {
		t.Errorf("expected the close-tag sequence to be neutralized in the css")
	}
}

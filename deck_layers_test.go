package slides

import (
	"strings"
	"testing"
)

// deck_layers_test.go proves the deck-level `header:` / `footer:` headmatter
// layers: persistent chrome rendered on every slide, per-slide overridable,
// sanitized through the raw-HTML lane.

// TestDeckFooterOnEverySlide proves the headmatter footer lands on each
// slide's section (and the header layer too when set).
func TestDeckFooterOnEverySlide(t *testing.T) {
	// The header uses single-quoted attributes: the minimal frontmatter parser
	// passes the value through verbatim (no YAML escape processing) and the
	// sanitizer re-emits attributes double-quoted.
	md := "---\ntitle: T\ntheme: aurora\nfooter: \"myconf 2026 · @me\"\nheader: <span class='tag'>draft</span>\n---\n\n" +
		"# One\n\n---\n\n# Two\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if got := strings.Count(html, `<div class="slide-footer">`); got != 2 {
		t.Fatalf("expected the footer on both slides, got %d:\n%s", got, html)
	}
	if !strings.Contains(html, "myconf 2026 · @me") {
		t.Errorf("expected the footer text to render, got:\n%s", html)
	}
	if got := strings.Count(html, `<span class="tag">draft</span>`); got != 2 {
		t.Errorf("expected the header's sanitized HTML on both slides, got %d", got)
	}
}

// TestSlideFooterOverride proves `footer: false` hides the deck footer on one
// slide and a non-empty per-slide value replaces it there.
func TestSlideFooterOverride(t *testing.T) {
	md := "---\ntitle: T\nfooter: global\n---\n\n" +
		"```yaml\nfooter: false\n```\n\n# Hidden\n\n---\n\n" +
		"```yaml\nfooter: replaced here\n```\n\n# Replaced\n\n---\n\n# Inherited\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if got := strings.Count(html, `<div class="slide-footer">`); got != 2 {
		t.Fatalf("expected footer on exactly 2 of 3 slides, got %d:\n%s", got, html)
	}
	if !strings.Contains(html, "replaced here") {
		t.Errorf("expected the per-slide replacement footer, got:\n%s", html)
	}
	if got := strings.Count(html, "global"); got != 1 {
		t.Errorf("expected the deck footer only on the inheriting slide, got %d occurrences", got)
	}
}

// TestLayerSanitized proves a footer cannot smuggle a script.
func TestLayerSanitized(t *testing.T) {
	md := "---\ntitle: T\nfooter: \"ok <script>alert(1)</script> text\"\n---\n\n# One\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "<script") || strings.Contains(html, "alert(1)") {
		t.Fatalf("footer script survived sanitization:\n%s", html)
	}
	if !strings.Contains(html, "ok") || !strings.Contains(html, "text") {
		t.Errorf("expected the safe footer text to survive, got:\n%s", html)
	}
}

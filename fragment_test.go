package slides

import (
	"strings"
	"testing"
)

// TestFragmentRevealDataAttributes verifies that a slide with `reveal: true`
// frontmatter and a 3-item bullet list emits data-fragment="0", "1", and "2"
// on the rendered <li> elements.
func TestFragmentRevealDataAttributes(t *testing.T) {
	src := "```yaml\nreveal: true\n```\n\n# Reveal slide\n\n- First point\n- Second point\n- Third point\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)

	for _, want := range []string{`data-fragment="0"`, `data-fragment="1"`, `data-fragment="2"`} {
		if !strings.Contains(html, want) {
			t.Errorf("reveal slide missing %q in:\n%s", want, html)
		}
	}
}

// TestFragmentRevealNotOnNonRevealSlide verifies that a slide without
// `reveal: true` does not get data-fragment attributes.
func TestFragmentRevealNotOnNonRevealSlide(t *testing.T) {
	src := "# Plain slide\n\n- Item one\n- Item two\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "data-fragment") {
		t.Errorf("plain slide should not have data-fragment attributes:\n%s", html)
	}
}

// TestFragmentRevealListAlternateValues checks that reveal: list is also
// accepted (the accepted aliases are true, list, 1, yes).
func TestFragmentRevealListAlternateValues(t *testing.T) {
	src := "```yaml\nreveal: list\n```\n\n# List reveal\n\n- Alpha\n- Beta\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `data-fragment="0"`) || !strings.Contains(html, `data-fragment="1"`) {
		t.Errorf("reveal: list did not tag list items with data-fragment:\n%s", html)
	}
}

// TestFragmentRevealNestedListNotTagged verifies that only TOP-LEVEL list items
// receive data-fragment; items inside a nested sub-list do not.
func TestFragmentRevealNestedListNotTagged(t *testing.T) {
	src := "```yaml\nreveal: true\n```\n\n# Nested\n\n- First\n  - Sub-item\n- Second\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)

	// Top-level items should be tagged.
	if !strings.Contains(html, `data-fragment="0"`) || !strings.Contains(html, `data-fragment="1"`) {
		t.Errorf("top-level items not tagged:\n%s", html)
	}
	// There should be exactly two data-fragment attributes (0 and 1), not more.
	count := strings.Count(html, "data-fragment=")
	if count != 2 {
		t.Errorf("expected 2 data-fragment attributes (top-level only), got %d:\n%s", count, html)
	}
}

// TestSlideHasRevealCSSTriggers verifies that navStyle includes per-K CSS rules
// for data-active-fragment (spot-check a few indices).
func TestSlideHasRevealCSSTriggers(t *testing.T) {
	css := navStyle()
	// At least the first few per-K rules should be present.
	for _, want := range []string{
		`[data-active-fragment="0"] [data-fragment="0"]`,
		`[data-active-fragment="1"] [data-fragment="0"]`,
		`[data-active-fragment="1"] [data-fragment="1"]`,
		`[data-active-fragment="2"] [data-fragment="2"]`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("navStyle missing per-K fragment rule %q", want)
		}
	}
}

// TestFragmentRevealWithOtherContent verifies that a reveal slide with both a
// heading and a list renders the heading normally and only the list items get
// data-fragment tags.
func TestFragmentRevealWithOtherContent(t *testing.T) {
	src := "```yaml\nreveal: true\n```\n\n# My heading\n\n- Alpha\n- Beta\n- Gamma\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "<h1>My heading</h1>") {
		t.Errorf("heading not rendered:\n%s", html)
	}
	for _, want := range []string{`data-fragment="0"`, `data-fragment="1"`, `data-fragment="2"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in:\n%s", want, html)
		}
	}
	// Heading should not carry data-fragment.
	if strings.Contains(html, `<h1 data-fragment`) {
		t.Errorf("heading should not have data-fragment:\n%s", html)
	}
}

// TestNavFragmentCSS verifies that navStyle includes the fragment-reveal CSS
// primitives: the default hide rule, the first-item-visible rule, and the
// overview/print overrides.
func TestNavFragmentCSS(t *testing.T) {
	css := navStyle()
	for _, want := range []string{
		`[data-fragment] { opacity: 0; }`,
		`:not([data-active-fragment]) [data-fragment="0"] { opacity: 1; }`,
		`data-active-fragment`,
		`opacity: 1 !important`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("navStyle missing fragment CSS %q", want)
		}
	}
}

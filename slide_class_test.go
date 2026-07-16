package slides

import (
	"strings"
	"testing"
)

// slide_class_test.go proves the per-slide `class:` and `transition:`
// frontmatter keys: authors declare a slide's identity ("dark", "centered")
// as CLASSES the deck stylesheet can key on — robust across reorders and
// nav's style-attribute rewrites (the old workaround matched the serialized
// style attribute and silently broke after hydration) — and override the
// deck-level enter transition for one slide.

// TestSlideClassFrontmatter proves `class: dark centered` lands both tokens on
// the slide's <section> alongside the layout class.
func TestSlideClassFrontmatter(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n" +
		"```yaml\nlayout: center\nclass: dark centered\n```\n\n# Classy\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `class="slide layout-center dark centered"`) {
		t.Fatalf("expected the class tokens on the section, got:\n%s", html)
	}
}

// TestSlideClassFrontmatterFiltersTokens proves malformed tokens are dropped
// while well-formed ones survive: tokens must look like CSS class names.
// (Injection is impossible regardless — the attribute is emitted as a quoted
// string expression — this keeps the class list sane.)
func TestSlideClassFrontmatterFiltersTokens(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n" +
		"```yaml\nclass: ok -bad \"quoted\" also_ok x<y\n```\n\n# Filtered\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `class="slide layout-default ok also_ok"`) {
		t.Fatalf("expected only the well-formed tokens, got:\n%s", html)
	}
}

// TestSlideTransitionFrontmatter proves `transition: none` stamps
// data-transition="none" on that slide's section (the nav CSS keys the
// per-slide enter-animation override on it) and that an unknown value stamps
// nothing.
func TestSlideTransitionFrontmatter(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n" +
		"```yaml\ntransition: none\n```\n\n# Cut\n\n---\n\n" +
		"```yaml\ntransition: wobble\n```\n\n# Unknown\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `data-transition="none"`) {
		t.Fatalf("expected data-transition=\"none\" on the first slide, got:\n%s", html)
	}
	if strings.Count(html, `data-transition=`) != 1 {
		t.Errorf("expected the unknown transition value to stamp nothing, got:\n%s", html)
	}
}

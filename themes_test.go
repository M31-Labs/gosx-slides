package slides

import (
	"strings"
	"testing"
)

// headOf returns the single <head>…</head> slice of a served document, failing
// the test if it cannot be located. Used to assert the theme <style> lands in the
// document head (not the body).
func headOf(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "<head>")
	end := strings.Index(body, "</head>")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("could not locate a single <head>…</head>:\n%s", body)
	}
	return body[start:end]
}

func TestThemesRegistryHasEntries(t *testing.T) {
	got := Themes()
	if len(got) < 3 {
		t.Fatalf("Themes() = %v, want at least 3 themes", got)
	}
	// Themes() must be sorted and every name must resolve to non-empty CSS.
	for i, name := range got {
		if i > 0 && got[i-1] > name {
			t.Errorf("Themes() not sorted: %v", got)
		}
		if strings.TrimSpace(themeCSS(name)) == "" {
			t.Errorf("themeCSS(%q) is empty", name)
		}
		// Each theme's CSS must be scoped to its own data-theme selector.
		if !strings.Contains(themeCSS(name), `data-theme="`+name+`"`) {
			t.Errorf("themeCSS(%q) is not scoped under main.deck[data-theme=%q]", name, name)
		}
	}
	// The default theme must itself be a registered theme.
	if strings.TrimSpace(themeCSS(defaultTheme)) == "" {
		t.Fatalf("defaultTheme %q has no CSS", defaultTheme)
	}
}

func TestThemeNameResolution(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", defaultTheme},
		{"  ", defaultTheme},
		{"nope-not-a-theme", defaultTheme},
		{"AURORA", "aurora"},
		{" Paper ", "paper"},
		{"neon", "neon"},
		{"swiss", "swiss"},
	}
	for _, c := range cases {
		if got := themeName(c.in); got != c.want {
			t.Errorf("themeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// themeCSS routes an unknown name to the default theme's CSS (never empty).
	if themeCSS("totally-unknown") != themeCSS(defaultTheme) {
		t.Errorf("themeCSS(unknown) did not fall back to the default theme")
	}
}

func TestLayoutClassResolution(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "layout-default"},
		{"default", "layout-default"},
		{"center", "layout-center"},
		{"title", "layout-title"},
		{"CENTER", "layout-center"},
		{"  Title ", "layout-title"},
		{"bogus", "layout-default"},
	}
	for _, c := range cases {
		if got := layoutClass(c.in); got != c.want {
			t.Errorf("layoutClass(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestServeInjectsSelectedThemeIntoHead proves a deck whose headmatter names a
// theme gets THAT theme's CSS in the single document <head> and a matching
// data-theme hook on <main class="deck"> — the whole selection contract.
func TestServeInjectsSelectedThemeIntoHead(t *testing.T) {
	body := serveBody(t, "---\ntitle: T\ntheme: neon\n---\n\n# Hello\n\nbody\n", nil)

	if !strings.Contains(body, `data-theme="neon"`) {
		t.Errorf("served <main> missing data-theme=\"neon\":\n%s", body)
	}
	head := headOf(t, body)
	if !strings.Contains(head, `main.deck[data-theme="neon"]`) {
		t.Errorf("selected theme CSS not in the single <head>:\n%s", head)
	}
	// The nav visibility style must still be present alongside the theme.
	if !strings.Contains(head, "main.deck > .slide.deck-active") {
		t.Errorf("nav visibility CSS missing from head (theme clobbered it?):\n%s", head)
	}
	// Other themes' CSS must NOT be injected — exactly one theme is served.
	if strings.Contains(head, `main.deck[data-theme="aurora"]`) {
		t.Errorf("head leaked a non-selected theme's CSS:\n%s", head)
	}
}

// TestServeUnknownThemeFallsBackToDefault proves an unknown `theme:` value (and,
// by the same path, an absent one) serves the default theme.
func TestServeUnknownThemeFallsBackToDefault(t *testing.T) {
	body := serveBody(t, "---\ntitle: T\ntheme: not-real\n---\n\n# Hi\n", nil)
	if !strings.Contains(body, `data-theme="`+defaultTheme+`"`) {
		t.Errorf("unknown theme did not fall back to default %q:\n%s", defaultTheme, body)
	}
	if !strings.Contains(headOf(t, body), `main.deck[data-theme="`+defaultTheme+`"]`) {
		t.Errorf("default theme CSS not injected for unknown theme")
	}
}

func TestServeAbsentThemeUsesDefault(t *testing.T) {
	body := serveBody(t, "---\ntitle: T\n---\n\n# Hi\n", nil)
	if !strings.Contains(body, `data-theme="`+defaultTheme+`"`) {
		t.Errorf("absent theme did not default to %q:\n%s", defaultTheme, body)
	}
}

// TestServeAppliesLayoutClass proves a slide's `layout:` frontmatter becomes a
// layout-<name> class on its <section>, and a layout-less slide gets
// layout-default.
func TestServeAppliesLayoutClass(t *testing.T) {
	// Per-slide frontmatter is authored as a leading ```yaml fence (mdpp's
	// convention; the deck-level headmatter uses the --- block).
	deckMD := "---\ntitle: T\n---\n\n```yaml\nlayout: center\n```\n\n# Centered\n\n---\n\n# Plain\n"
	body := serveBody(t, deckMD, nil)

	if !strings.Contains(body, `class="slide layout-center"`) {
		t.Errorf("layout: center slide did not get class \"slide layout-center\":\n%s", body)
	}
	if !strings.Contains(body, `class="slide layout-default"`) {
		t.Errorf("layout-less slide did not get class \"slide layout-default\":\n%s", body)
	}
}

// TestLowerSlideLayoutClassUnknownIsDefault proves the generator normalizes an
// unknown layout to layout-default at the source-gen layer (independent of the
// server), so a typo'd layout still renders a well-formed, styleable section.
func TestLowerSlideLayoutClassUnknownIsDefault(t *testing.T) {
	deck := loadDeckFromSource(t, "---\ntitle: T\n---\n\n```yaml\nlayout: zigzag\n```\n\n# X\n", nil)
	if len(deck.Slides) == 0 {
		t.Fatal("no slides parsed")
	}
	if got := slideLayoutClass(deck.Slides[0]); got != "layout-default" {
		t.Errorf("slideLayoutClass(unknown) = %q, want layout-default", got)
	}
	src := lowerSlideToGSX(deck.Slides[0])
	if !strings.Contains(src, `class="slide layout-default"`) {
		t.Errorf("lowerSlideToGSX did not emit layout-default for unknown layout:\n%s", src)
	}
}

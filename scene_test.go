package slides

import (
	"strings"
	"testing"
)

// scene_test.go proves the scene layer: a deck (or slide) names an island via
// `scene:` frontmatter and it renders full-bleed behind the content — living
// backgrounds (presets) and illustrative visual models (author islands).

// backdropGSX is a minimal author scene island.
const backdropGSX = `package main

//gosx:island
func Backdrop(props any) Node {
	return <div class="my-backdrop"> </div>
}
`

// TestSceneDeckDefaultPreset proves headmatter `scene: parse-forest` mounts
// the embedded preset behind every slide, and `scene: false` opts one slide
// out.
func TestSceneDeckDefaultPreset(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\nscene: parse-forest\n---\n\n# One\n\n---\n\n" +
		"```yaml\nscene: false\n```\n\n# Two\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if got := strings.Count(html, `<div class="slide-scene" aria-hidden="true">`); got != 1 {
		t.Fatalf("expected the scene layer on exactly 1 of 2 slides, got %d:\n%s", got, html)
	}
	if !strings.Contains(html, "scene-parse-forest") {
		t.Errorf("expected the parse-forest preset's server-rendered markup, got:\n%s", html)
	}
}

// TestSceneAuthorIsland proves `scene: Backdrop` resolves Backdrop.gsx from
// the deck dir like any island.
func TestSceneAuthorIsland(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\nscene: Backdrop\n---\n\n# One\n"
	deck := loadDeckFromSource(t, md, map[string]string{"Backdrop": backdropGSX})
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `class="slide-scene"`) || !strings.Contains(html, "my-backdrop") {
		t.Fatalf("expected the author scene island rendered in the layer, got:\n%s", html)
	}
}

// TestScenePerSlideOverride proves a slide can replace the deck scene with its
// own island (the illustrative case: a fork-tracing scene on one slide).
func TestScenePerSlideOverride(t *testing.T) {
	md := "---\ntitle: T\nscene: parse-forest\n---\n\n# One\n\n---\n\n" +
		"```yaml\nscene: Backdrop\n```\n\n# Two\n"
	deck := loadDeckFromSource(t, md, map[string]string{"Backdrop": backdropGSX})
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "scene-parse-forest") || !strings.Contains(html, "my-backdrop") {
		t.Fatalf("expected both the deck preset and the per-slide island, got:\n%s", html)
	}
}

// TestSceneUnknownValueFailsSoft proves an unknown preset key renders no scene
// layer rather than an unresolved placeholder band behind content.
func TestSceneUnknownValueFailsSoft(t *testing.T) {
	md := "---\ntitle: T\nscene: nope-nope\n---\n\n# One\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "slide-scene") {
		t.Fatalf("unknown scene value must render no layer, got:\n%s", html)
	}
}

// TestSceneLayerCSSServed proves the served page carries the layer CSS: the
// isolated stacking context, the behind-content z-index, and the
// reduced-motion hide (scenes are decorative by contract).
func TestSceneLayerCSSServed(t *testing.T) {
	body := serveBody(t, "---\ntitle: T\nscene: parse-forest\n---\n\n# One\n", nil)

	for _, want := range []string{
		"isolation: isolate",
		".slide-scene { position: absolute; inset: 0; z-index: -1;",
		"prefers-reduced-motion: reduce) { main.deck > .slide > .slide-scene { display: none; } }",
		"scene-parse-forest",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("served page missing scene CSS %q", want)
		}
	}
}

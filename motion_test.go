package slides

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
)

func TestManagedMotionMarkdownUsesNativeBootstrap(t *testing.T) {
	deck := graphicsDeck(t, "# Motion\n\n:::motion {preset=slide-up trigger=view duration=450 delay=80 distance=24 .entrance #arrival}\n## Arrive with intent\n\nA **native** entrance.\n\n- One\n- Two\n:::\n", nil)
	body := graphicsBody(t, deck)
	if !strings.Contains(body, `data-slides-motion-replay="slide"`) {
		t.Fatal("motion must replay on slide entry by default")
	}
	for _, want := range []string{`data-gosx-motion-preset="slide-up"`, `data-gosx-motion-trigger="view"`, `data-gosx-motion-duration="450"`, `data-gosx-motion-delay="80"`, `data-gosx-motion-distance="24"`, `data-gosx-motion-respect-reduced="true"`, `class="entrance"`, `id="arrival"`, `<h2>Arrive with intent</h2>`, `<strong>native</strong>`, `<li>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("managed motion missing %s", want)
		}
	}
	if strings.Contains(body, "gosx-runtime.wasm") || !strings.Contains(body, `src="/gosx/bootstrap`) {
		t.Fatal("managed DOM motion unexpectedly loads WASM")
	}
	if compiled, failures := deck.compileComponents(); len(compiled) != 0 || len(failures) != 0 {
		t.Fatalf("managed motion treated as an island: %v", failures)
	}
	// A broken unrelated island must still leave native motion and its content
	// available through the compile-failure renderer.
	fallback := gosx.RenderHTML(renderIslandSlide(island.NewRenderer("fallback"), deck.Slides[0], nil, "light"))
	for _, want := range []string{`data-gosx-motion-duration="450"`, `data-gosx-motion-trigger="view"`, `<h2>Arrive with intent</h2>`, `<strong>native</strong>`} {
		if !strings.Contains(fallback, want) {
			t.Fatalf("fallback dropped motion/content: %s", want)
		}
	}
}

func TestSlideTransitionTimingInheritanceAndOverrides(t *testing.T) {
	deck := graphicsDeck(t, "---\ntransition-duration: 650\ntransition-delay: 0.1s\ntransition-easing: ease-out\n---\n\n```yaml\ntransition-duration: 300ms\ntransition-delay: 0\ntransition-easing: linear\n```\n\n# Timed\n\nContent.\n", nil)
	body := graphicsBody(t, deck)
	for _, want := range []string{`--slides-transition-duration:650ms;`, `--slides-transition-delay:100ms;`, `--slides-transition-easing:ease-out;`, `--slides-transition-duration:300ms;`, `--slides-transition-delay:0ms;`, `--slides-transition-easing:linear;`} {
		if !strings.Contains(body, want) {
			t.Fatalf("transition timing missing %s", want)
		}
	}
	if invalid := transitionTimingStyle(map[string]any{"transition-duration": "NaN", "transition-delay": "-1s", "transition-easing": "ease;display:none"}); invalid != "" {
		t.Fatalf("invalid timing escaped normalization: %s", invalid)
	}
}

func TestManagedMotionPropertiesStayData(t *testing.T) {
	deck := graphicsDeck(t, "# Motion\n\n:::motion {preset=unknown duration=-1 delay=-3 distance=-2}\nRead **this** even without animation.\n:::\n", nil)
	body := graphicsBody(t, deck)
	for _, want := range []string{`data-gosx-motion-preset="fade"`, `data-gosx-motion-duration="220"`, `data-gosx-motion-delay="0"`, `data-gosx-motion-distance="18"`, `<strong>this</strong>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("GoSX normalization/content missing %s", want)
		}
	}
}

func TestManagedTextMotionControls(t *testing.T) {
	deck := graphicsDeck(t, "# Text\n\n:::motion {preset=fade split=word stagger=60 respect-reduced-motion=false}\nOne word at a time.\n:::\n", nil)
	body := graphicsBody(t, deck)
	for _, want := range []string{`data-gosx-motion-split="word"`, `data-gosx-motion-stagger="60"`, `data-gosx-motion-respect-reduced="false"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("text motion control missing %s", want)
		}
	}
}

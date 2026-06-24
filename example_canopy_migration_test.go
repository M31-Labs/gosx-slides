package slides

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
)

// TestCanopyMigrationDeckCompiles guards the shipped canopy-migration example:
// it must split into all 11 slides (the slide-separator absorption bug regresses
// here first — see warnAbsorbedSeparators), compile cleanly, resolve both hero
// islands as live mounts, and evaluate the title {expr}.
func TestCanopyMigrationDeckCompiles(t *testing.T) {
	deck, err := LoadIslandDeck("examples/canopy-migration")
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}

	if got := len(deck.Slides); got != 12 {
		t.Fatalf("deck split into %d slides, want 12 (mdpp thematic-break splitting regressed)", got)
	}

	cd, err := compileDeckProgram(deck)
	if err != nil {
		t.Fatalf("compileDeckProgram failed:\n%v\n--- generated source ---\n%s", err, cd.source)
	}

	compiled, failures := deck.compileComponents()
	if len(failures) != 0 {
		for name, e := range failures {
			t.Errorf("island %s.gsx failed to compile: %v", name, e)
		}
		t.FailNow()
	}
	for _, want := range []string{"HotspotExplorer", "BoundaryGate"} {
		if compiled[want] == nil {
			t.Errorf("island %s missing from compiled cache", want)
		}
	}

	r := island.NewRenderer("verify")
	for name := range compiled {
		r.SetProgramAsset(name, "/gosx/islands/"+name+".json", "json", "")
	}
	var b strings.Builder
	for _, n := range renderProgramSlides(r, deck, cd, compiled) {
		b.WriteString(gosx.RenderHTML(n))
	}
	html := b.String()

	if !strings.Contains(html, "Java to Go, with a map") {
		t.Errorf("title {deck.title} did not evaluate")
	}
	if strings.Contains(html, "data-gosx-unresolved") {
		t.Errorf("a component rendered as an unresolved placeholder")
	}
	for _, want := range []string{`data-gosx-island="HotspotExplorer"`, `data-gosx-island="BoundaryGate"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing live island mount: %s", want)
		}
	}
}

// TestIslandAnalysisIsHonest guards the re-homed lint tools: on a REAL-lane deck
// they must read the real model — the headmatter theme/title, the yaml-fence
// layouts, and the deck's actual .gsx islands — instead of the old fallback
// parser that false-flagged aurora and saw neither layouts nor components.
func TestIslandAnalysisIsHonest(t *testing.T) {
	deck, err := LoadIslandDeck("examples/canopy-migration")
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	a := Analyze(deck)

	if a.Title != "Java to Go, with a map" {
		t.Errorf("title = %q, want the headmatter title", a.Title)
	}
	if a.Theme != "aurora" {
		t.Errorf("theme = %q, want aurora", a.Theme)
	}
	for _, w := range a.Warnings {
		if strings.Contains(w, "unknown theme") || strings.Contains(w, "unknown layout") {
			t.Errorf("real deck wrongly flagged: %q", w)
		}
	}
	if a.SlideCount != 12 {
		t.Errorf("slideCount = %d, want 12", a.SlideCount)
	}
	// Real yaml-fence layouts are visible (not all "default").
	if a.Layouts["title"] != 1 || a.Layouts["center"] != 2 {
		t.Errorf("layouts = %v, want title:1 center:2 (yaml-fence layouts must be seen)", a.Layouts)
	}
	// The deck's actual islands, not a fallback registry.
	if a.Components["HotspotExplorer"] != 1 || a.Components["BoundaryGate"] != 1 {
		t.Errorf("components = %v, want the deck's real .gsx islands", a.Components)
	}

	// validate --strict must PASS on the framework's own real-lane deck.
	if r := Validate(deck, ValidateOptions{Profile: "standard"}); !r.Passed(true) {
		t.Errorf("validate --strict failed on a real deck: errors=%v warnings=%v", r.Errors, r.Warnings)
	}
}

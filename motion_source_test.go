package slides

import (
	"m31labs.dev/mdpp"
	"strings"
	"testing"
)

func TestMotionSourceByteRanges(t *testing.T) {
	source := "# Café 🌊\n\n:::motion {duration=300 easing=linear replay=once}\nA Unicode entrance.\n:::\n\n```text\n:::motion {duration=1}\n```\n\n---\n\n# Next\n\n:::motion {preset=fade duration=400}\nNext\n:::\n"
	deck, err := parseIslandDeck(t.TempDir(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	ranges := sourceMotionRanges(deck)
	if len(ranges) != 2 {
		t.Fatalf("found %d directives", len(ranges))
	}
	for i, r := range ranges {
		if string(deck.Source[r.Start:r.End]) != r.Opening || !strings.HasPrefix(r.Opening, ":::motion") {
			t.Fatal("opening range includes other source")
		}
		if r.Attrs["duration"] != []string{"300", "400"}[i] {
			t.Fatal("attributes lost")
		}
	}
	node := deck.Slides[0].Node.Find(mdpp.NodeContainerDirective)[0]
	if !strings.Contains(lowerMotionDirectiveGSX(node), "data-slides-motion-source") {
		t.Fatal("missing browser identity")
	}
}
func TestBrowserPerformanceBudgets(t *testing.T) {
	report := &BrowserBenchmark{Runs: []BrowserSample{{ReadyMillis: 100, TransferBytes: 2000, DOMNodes: 40, HeapBytes: 10000, FrameP95Millis: 17}}}
	if err := report.CheckBudget(BrowserBudget{ReadyMillis: 101, TransferBytes: 2001, DOMNodes: 41, HeapBytes: 10001, FrameP95Millis: 18}); err != nil {
		t.Fatal(err)
	}
	for _, budget := range []BrowserBudget{{ReadyMillis: 99}, {TransferBytes: 1999}, {DOMNodes: 39}, {HeapBytes: 9999}, {FrameP95Millis: 16}, {ReadyMillis: -1}} {
		if err := report.CheckBudget(budget); err == nil {
			t.Fatal("budget breach accepted")
		}
	}
}
func TestDiagramStoriesHaveNativeStatesAndSteps(t *testing.T) {
	source := "# Growth\n\n:::diagram-morph {diagram=bar duration=1000}\n```sirena\nservice a { value: 1 }\n```\n\n```sirena\nservice a { value: 4 }\nservice b { value: 2 }\n```\n:::\n"
	deck, err := parseIslandDeck(t.TempDir(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if slideMotionClicks(deck.Slides[0]) != 1 {
		t.Fatal("story not in click budget")
	}
	program := lowerSlideToGSX(deck.Slides[0], slideLayers{})
	if !strings.Contains(program, ".Morph(") {
		t.Fatal("story not lowered")
	}
}
func TestQuotedMotionEasing(t *testing.T) {
	source := []byte("# Motion\n\n:::motion {duration=400 easing=\"cubic-bezier(0.16, 1, 0.3, 1)\" custom=\"duration=5 preserved\"}\nA\n:::\n")
	deck, err := parseIslandDeck(t.TempDir(), source)
	if err != nil {
		t.Fatal(err)
	}
	ranges := sourceMotionRanges(deck)
	if len(ranges) != 1 || ranges[0].Attrs["easing"] != "cubic-bezier(0.16, 1, 0.3, 1)" || ranges[0].Attrs["custom"] != "duration=5 preserved" {
		t.Fatalf("quoted attributes truncated: %+v", ranges)
	}
}

package slides

import (
	"strings"
	"testing"
)

// minimalSirenaSrc is the smallest valid sirena diagram body: a single service
// declaration. Verified to produce non-empty SVG via fence.Render in a scratch
// test (SVG len 4765 for "service api\n").
const minimalSirenaSrc = "service api\n"

// TestSirenaDiagramRenders proves that a ```sirena fenced block produces an
// inline SVG <figure class="mdpp-diagram"> in the rendered HTML — not a blank
// node, not a mermaid <pre>, and not a diagram-error.
func TestSirenaDiagramRenders(t *testing.T) {
	src := "# Arch\n\n```sirena\n" + minimalSirenaSrc + "```\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "<svg") {
		t.Fatalf("sirena diagram did not produce inline SVG (no <svg):\n%s", html)
	}
	if !strings.Contains(html, "mdpp-diagram") {
		t.Fatalf("sirena diagram missing mdpp-diagram class:\n%s", html)
	}
	// Must NOT contain any mermaid artifact.
	if strings.Contains(html, "mermaid") {
		t.Fatalf("rendered HTML contains mermaid artifact (should be sirena only):\n%s", html)
	}
}

// TestDeckHasDiagram proves deckHasDiagram returns true when a deck contains a
// sirena fence and false when it does not.
func TestDeckHasDiagram(t *testing.T) {
	with := loadDeckFromSource(t, "# A\n\n```sirena\n"+minimalSirenaSrc+"```\n", nil)
	if !deckHasDiagram(with) {
		t.Error("deckHasDiagram should return true for a deck with a sirena fence")
	}

	without := loadDeckFromSource(t, "# B\n\nplain prose\n", nil)
	if deckHasDiagram(without) {
		t.Error("deckHasDiagram should return false for a deck without any diagram")
	}
}

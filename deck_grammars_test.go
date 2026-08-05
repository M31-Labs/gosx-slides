package slides

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

// The gophercon2026 deck ships danmuji and ferrous grammar blobs; they double
// as the fixture for the deck-local grammar lane. Real blobs, real queries —
// the same artifact contract the feature exists to demonstrate.
const deckGrammarFixtureDir = "examples/gophercon2026"

func TestDeckGrammarHighlightsDanmujiKeywords(t *testing.T) {
	src := "package cart_test\n\nimport \"testing\"\n\nunit \"Cart.Add\" {\n    given \"an empty cart\" {\n        expect cart.Count() == 1\n    }\n}"
	html, ok := deckGrammarHTML(deckGrammarFixtureDir, "danmuji", src)
	if !ok {
		t.Fatalf("danmuji grammar blob did not load from %s/grammars", deckGrammarFixtureDir)
	}
	for _, want := range []string{`<span class="ts-keyword">unit</span>`, `<span class="ts-keyword">given</span>`, `<span class="ts-string">`} {
		if !strings.Contains(html, want) {
			t.Fatalf("danmuji highlight missing %q:\n%s", want, html)
		}
	}
}

func TestDeckGrammarHighlightsFerrousKeywords(t *testing.T) {
	src := "package main\n\nimport \"os\"\n\nenum Status { Active, Suspended(string) }\n\nfunc load(p string) (string, error) {\n    let data = os.ReadFile(p)?\n    return string(data), nil\n}"
	html, ok := deckGrammarHTML(deckGrammarFixtureDir, "ferrous", src)
	if !ok {
		t.Fatalf("ferrous grammar blob did not load from %s/grammars", deckGrammarFixtureDir)
	}
	for _, want := range []string{`<span class="ts-keyword">enum</span>`, `<span class="ts-keyword">let</span>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("ferrous highlight missing %q:\n%s", want, html)
		}
	}
}

func TestDeckCodeBlockNodeFallsBackForUnknownLanguages(t *testing.T) {
	out := gosx.RenderHTML(deckCodeBlockNode(deckGrammarFixtureDir, "go", "package main", ""))
	if !strings.Contains(out, `data-lang="go"`) {
		t.Fatalf("built-in path lost its language token:\n%s", out)
	}
}

func TestDeckCodeBlockNodeUsesDeckGrammar(t *testing.T) {
	out := gosx.RenderHTML(deckCodeBlockNode(deckGrammarFixtureDir, "danmuji", "package x_test\n\nimport \"testing\"\n\nunit \"X\" {\n    given \"y\" {\n    }\n}", ""))
	if !strings.Contains(out, `data-lang="danmuji"`) || !strings.Contains(out, `<span class="ts-keyword">unit</span>`) {
		t.Fatalf("deck grammar path not taken:\n%s", out)
	}
}

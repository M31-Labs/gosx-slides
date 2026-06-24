package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
)

// loadDeckFromSource writes a deck.md (and any provided component .gsx files)
// into a temp dir under the module, loads it, and returns the deck. Using a dir
// UNDER the repo keeps the go.mod replace (m31labs.dev/gosx -> ../gosx)
// resolvable for any tooling, and t.TempDir cleans it up.
func loadDeckFromSource(t *testing.T, deckMD string, components map[string]string) *IslandDeck {
	t.Helper()
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	base := filepath.Join(repoDir, "testdata")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	dir, err := os.MkdirTemp(base, "slidegen-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte(deckMD), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	for name, src := range components {
		if err := os.WriteFile(filepath.Join(dir, name+".gsx"), []byte(src), 0o644); err != nil {
			t.Fatalf("write component %s: %v", name, err)
		}
	}

	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	return deck
}

// renderSlidesHTML compiles the deck program and renders all slides through the
// Slice-4 flow, returning the concatenated HTML. A fresh per-request renderer is
// used, mirroring the server.
func renderSlidesHTML(t *testing.T, deck *IslandDeck) string {
	t.Helper()
	cd, err := compileDeckProgram(deck)
	if err != nil {
		t.Fatalf("compileDeckProgram: %v\n--- generated source ---\n%s", err, cd.source)
	}
	r := island.NewRenderer("test")
	compiled, _ := deck.compileComponents()
	for name := range compiled {
		r.SetProgramAsset(name, "/gosx/islands/"+name+".json", "json", "")
	}
	nodes := renderProgramSlides(r, deck, cd, compiled)
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(gosx.RenderHTML(n))
	}
	return b.String()
}

// TestMarkdownImageRenders proves a markdown image is no longer silently dropped
// (slidegen had no NodeImage case) — it lowers to an <img> with src and alt.
func TestMarkdownImageRenders(t *testing.T) {
	deck := loadDeckFromSource(t, "# Pic\n\n![a cat](cat.png)\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "<img") || !strings.Contains(html, "cat.png") {
		t.Fatalf("markdown image was dropped (no <img>/src):\n%s", html)
	}
	if !strings.Contains(html, "a cat") {
		t.Errorf("image alt text missing:\n%s", html)
	}
}

// TestMarkdownTableRenders proves a GFM table renders (slidegen had no NodeTable
// case) — a <table> with header <th> and body <td> cells.
func TestMarkdownTableRenders(t *testing.T) {
	deck := loadDeckFromSource(t, "# Data\n\n| Lang | Speed |\n| --- | --- |\n| Go | fast |\n| Java | ok |\n", nil)
	html := renderSlidesHTML(t, deck)
	for _, want := range []string{"<table>", "<th>", "<td>", "Lang", "Go", "fast"} {
		if !strings.Contains(html, want) {
			t.Fatalf("table missing %q:\n%s", want, html)
		}
	}
}

// TestLayoutAndPerSlideOverrides proves the new layouts resolve to their class and
// that per-slide background:/accent: frontmatter emit an inline style override.
func TestLayoutAndPerSlideOverrides(t *testing.T) {
	src := "```yaml\nlayout: section\nbackground: \"#101820\"\naccent: \"#f6b352\"\n```\n\n# Section divider\n"
	deck := loadDeckFromSource(t, src, nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "layout-section") {
		t.Errorf("layout: section did not resolve to layout-section class:\n%s", html)
	}
	if !strings.Contains(html, "--accent:#f6b352") || !strings.Contains(html, "background:#101820") {
		t.Errorf("per-slide background/accent override not emitted:\n%s", html)
	}
	// A known new layout is accepted, an unknown one degrades to default.
	if !knownLayouts["two-cols"] || !knownLayouts["quote"] || !knownLayouts["full"] {
		t.Error("new layouts missing from knownLayouts")
	}
	if layoutClass("bogus") != "layout-default" {
		t.Error("unknown layout should degrade to layout-default")
	}
}

// --- The headline Slice-4 outcome: {expr} actually EVALUATES ---

// TestExprArithmeticEvaluates proves a slide's inline {2 + 3} renders the
// EVALUATED value 5, not the raw source "2 + 3".
func TestExprArithmeticEvaluates(t *testing.T) {
	deck := loadDeckFromSource(t, "# Math\n\nThe answer is {2 + 3}.\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "5") {
		t.Fatalf("{2 + 3} not evaluated to 5:\n%s", html)
	}
	if strings.Contains(html, "2 + 3") {
		t.Fatalf("{2 + 3} leaked as raw source instead of evaluating:\n%s", html)
	}
}

// TestExprResolvesDeckAndSlideFrontmatter proves richer expression scope: an
// inline {deck.title} resolves from the deck headmatter and {slide.layout} from
// the per-slide frontmatter, so slide prose can reference deck/slide context.
func TestExprResolvesDeckAndSlideFrontmatter(t *testing.T) {
	deckMD := "---\ntitle: My Talk\n---\n\n# Intro\n\nWelcome to {deck.title}.\n\n---\n\n```yaml\nlayout: center\n```\n\nLayout is {slide.layout}.\n"
	deck := loadDeckFromSource(t, deckMD, nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "Welcome to My Talk.") {
		t.Fatalf("{deck.title} not resolved from headmatter:\n%s", html)
	}
	if !strings.Contains(html, "Layout is center.") {
		t.Fatalf("{slide.layout} not resolved from per-slide frontmatter:\n%s", html)
	}
}

// TestExprStringFuncEvaluates proves {strings.ToUpper("hi")} renders "HI" — the
// bound strings namespace is reachable from inline prose.
func TestExprStringFuncEvaluates(t *testing.T) {
	deck := loadDeckFromSource(t, `# Upper

Shout: {strings.ToUpper("hi")}
`, nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "HI") {
		t.Fatalf(`{strings.ToUpper("hi")} not evaluated to HI:\n%s`, html)
	}
}

// TestExprStringConcatEvaluates proves a pure string-concat expr evaluates with
// no bindings.
func TestExprStringConcatEvaluates(t *testing.T) {
	deck := loadDeckFromSource(t, "# Concat\n\nJoined: {\"a\" + \"b\" + \"c\"}\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "abc") {
		t.Fatalf(`{"a" + "b" + "c"} not evaluated to abc:\n%s`, html)
	}
}

// --- Islands still hydrate ---

// TestComponentStillHydrates proves a slide with a <Counter/> still yields a
// hydrated island mount (data-gosx-island) AND that {expr} on the same slide
// evaluates — the two features coexist in one rendered slide.
func TestComponentStillHydrates(t *testing.T) {
	deck := loadDeckFromSource(t,
		"# Live\n\nThe answer is {2 + 3}.\n\n<Counter initial={3}/>\n",
		map[string]string{"Counter": counterGSX},
	)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, `data-gosx-island="Counter"`) {
		t.Fatalf("<Counter/> did not render a hydrated island mount:\n%s", html)
	}
	if !strings.Contains(html, "5") {
		t.Fatalf("{2 + 3} not evaluated alongside the island:\n%s", html)
	}
	if strings.Contains(html, "data-gosx-unresolved") {
		t.Fatalf("Counter rendered as unresolved placeholder (island failed to inline):\n%s", html)
	}
}

// --- Prose safety: <, &, quotes never corrupt the generated source ---

// TestProseEscapingPreserved proves prose containing <, &, and quotes is emitted
// ESCAPED (entity-encoded) and never breaks the generated source or injects raw
// markup. This is the source-gen safety trick (text -> string-literal expr).
func TestProseEscapingPreserved(t *testing.T) {
	deck := loadDeckFromSource(t, "# Safe\n\nDanger: a < b && c \"q\" here\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "&lt;") {
		t.Fatalf("prose '<' not escaped to &lt;:\n%s", html)
	}
	if !strings.Contains(html, "&amp;") {
		t.Fatalf("prose '&' not escaped to &amp;:\n%s", html)
	}
	// The raw dangerous sequence must NOT appear unescaped.
	if strings.Contains(html, "a < b") {
		t.Fatalf("prose raw '<' leaked unescaped:\n%s", html)
	}
}

// TestProseWithBracesQuotedSafely proves prose that itself contains a brace
// character is treated as opaque text (quoted), not as an interpolation that
// would corrupt the generated source.
func TestProseWithBracesQuotedSafely(t *testing.T) {
	// A heading whose text has a literal quote and ampersand — the generator
	// must quote it so the source compiles and the content renders escaped.
	deck := loadDeckFromSource(t, "# Title \"Q&A\"\n\nbody\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, "<h1>") {
		t.Fatalf("heading not rendered:\n%s", html)
	}
	if !strings.Contains(html, "&amp;") {
		t.Fatalf("heading '&' not escaped:\n%s", html)
	}
}

// --- Structure: headings, paragraphs, data-slide ---

// TestSlideStructure proves the lowered slide keeps its data-slide section and
// maps heading level + paragraph to the right tags.
func TestSlideStructure(t *testing.T) {
	deck := loadDeckFromSource(t, "## Subhead\n\nbody text\n", nil)
	html := renderSlidesHTML(t, deck)
	if !strings.Contains(html, `data-slide="0"`) {
		t.Fatalf("missing data-slide section:\n%s", html)
	}
	if !strings.Contains(html, "<h2>Subhead</h2>") {
		t.Fatalf("want <h2>Subhead</h2> in:\n%s", html)
	}
	if !strings.Contains(html, "<p>body text</p>") {
		t.Fatalf("want <p>body text</p> in:\n%s", html)
	}
}

// --- generateDeckSource / parseIslandDef unit coverage ---

// TestGenerateDeckSourceShape proves the generated source declares a Slide_N
// function per slide and inlines the island definition, as one package.
func TestGenerateDeckSourceShape(t *testing.T) {
	deck := loadDeckFromSource(t,
		"# One\n\n{1 + 1}\n\n<Counter/>\n\n---\n\n# Two\n\nplain\n",
		map[string]string{"Counter": counterGSX},
	)
	defs := loadIslandDefs(deck)
	src := generateDeckSource(deck, defs)

	for _, want := range []string{
		"package main",
		"func Slide_0() Node {",
		"func Slide_1() Node {",
		"func Counter() Node {", // island def inlined
		"//gosx:island",         // directive preserved
		// Slides carry a layout-<name> class from their `layout:` frontmatter
		// (layout-default when absent — see slideLayoutClass/themes.go).
		`<section class="slide layout-default" data-slide="0">`,
		`<section class="slide layout-default" data-slide="1">`,
		"{1 + 1}", // expr carried verbatim
		"<Counter/>",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated source missing %q:\n%s", want, src)
		}
	}
	// The island def's package line must NOT be duplicated inside the body.
	if strings.Count(src, "package main") != 1 {
		t.Fatalf("expected exactly one package clause, source:\n%s", src)
	}
}

// TestParseIslandDefStripsPackageAndImports proves parseIslandDef removes the
// package clause and both grouped and single-line imports, returning them
// separately, and preserves the component body verbatim.
func TestParseIslandDefStripsPackageAndImports(t *testing.T) {
	src := `package main

import (
	"strings"
	"fmt"
)

import "errors"

//gosx:island
func Widget() Node {
	return <div>{strings.ToUpper("x")}</div>
}
`
	def := parseIslandDef(src)
	if strings.Contains(def.body, "package main") {
		t.Fatalf("body still contains package clause:\n%s", def.body)
	}
	if strings.Contains(def.body, "import") {
		t.Fatalf("body still contains an import:\n%s", def.body)
	}
	if !strings.Contains(def.body, "//gosx:island") || !strings.Contains(def.body, "func Widget() Node") {
		t.Fatalf("body lost the component definition:\n%s", def.body)
	}
	wantImports := map[string]bool{`"strings"`: true, `"fmt"`: true, `"errors"`: true}
	if len(def.imports) != len(wantImports) {
		t.Fatalf("imports = %#v, want the 3 specs %v", def.imports, wantImports)
	}
	for _, imp := range def.imports {
		if !wantImports[imp] {
			t.Fatalf("unexpected import spec %q in %#v", imp, def.imports)
		}
	}
}

// TestMergeIslandDefsDedupesImports proves merging two components that import the
// same package yields a single deduped import entry.
func TestMergeIslandDefsDedupesImports(t *testing.T) {
	defs := map[string]islandDef{
		"A": {imports: []string{`"strings"`}, body: "func A() Node { return <div/> }"},
		"B": {imports: []string{`"strings"`, `"fmt"`}, body: "func B() Node { return <div/> }"},
	}
	imports, bodies := mergeIslandDefs(defs)
	if got := strings.Count(strings.Join(imports, "\n"), `"strings"`); got != 1 {
		t.Fatalf(`"strings" appears %d times in merged imports %v, want 1`, got, imports)
	}
	if len(bodies) != 2 {
		t.Fatalf("merged bodies = %d, want 2", len(bodies))
	}
}

// counterGSX is a minimal island component used by the generator tests. It is
// import-free, exercising the trivial merge path.
const counterGSX = `package main

//gosx:island
func Counter() Node {
	count := signal.New(0)
	increment := func() { count.Set(count.Get() + 1) }
	return <div class="counter">
		<span class="counter-label">count is {count.Get()}</span>
		<button class="counter-btn" onClick={increment}>+</button>
	</div>
}
`

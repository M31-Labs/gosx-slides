package slides

import (
	"strings"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
	"m31labs.dev/mdpp"
)

// --- Work item 1: block-level <Component/> recognition ---

// TestCollectComponentRefsBlockLevel proves that a standalone <Counter/> written
// on its own blank-line-delimited line — which mdpp parses as a NodeHTMLBlock,
// NOT a folded NodeComponent (the Slice-1 lesson) — is still discovered as a
// component reference by the real lane.
func TestCollectComponentRefsBlockLevel(t *testing.T) {
	// Heading + prose + standalone component: the component lands in a
	// NodeHTMLBlock (verified against mdpp), so the inline-only walk misses it.
	src := []byte("# Live Counter\n\nClick to drive a real island.\n\n<Counter initial={3}/>\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)

	slide := doc.Slides()[0]
	refs := collectComponentRefs(slide)
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1 (block-level <Counter/>); got %#v", len(refs), refs)
	}
	if refs[0].Name != "Counter" {
		t.Fatalf("ref name = %q, want Counter", refs[0].Name)
	}
	if refs[0].Props != "initial={3}" {
		t.Fatalf("ref props = %q, want %q", refs[0].Props, "initial={3}")
	}
}

// TestCollectComponentRefsPairedBlock proves a paired block-level component
// <Note>...</Note> on its own line is discovered (the open tag arrives in a
// NodeText/HTMLBlock, not folded).
func TestCollectComponentRefsPairedBlock(t *testing.T) {
	src := []byte("# T\n\nlead\n\n<Note kind=\"tip\">be careful</Note>\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1; got %#v", len(refs), refs)
	}
	if refs[0].Name != "Note" {
		t.Fatalf("name = %q, want Note", refs[0].Name)
	}
	if refs[0].Props != `kind="tip"` {
		t.Fatalf("props = %q, want %q", refs[0].Props, `kind="tip"`)
	}
}

// TestCollectComponentRefsInlineStillWorks guards the Slice-1 path: an inline
// <Counter/> embedded in prose (folded to NodeComponent) is still found, and is
// not double-counted by the new block scan.
func TestCollectComponentRefsInlineStillWorks(t *testing.T) {
	src := []byte("# T\n\nlive island here: <Counter/> in prose.\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want exactly 1 (no double count); got %#v", len(refs), refs)
	}
	if refs[0].Name != "Counter" {
		t.Fatalf("name = %q, want Counter", refs[0].Name)
	}
}

// TestCollectComponentRefsLowercaseIgnored proves ordinary lowercase HTML in a
// block is never mistaken for a component.
func TestCollectComponentRefsLowercaseIgnored(t *testing.T) {
	src := []byte("# T\n\n<div class=\"x\">plain html</div>\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 0 {
		t.Fatalf("refs = %d, want 0 (lowercase html is not a component); got %#v", len(refs), refs)
	}
}

// --- Work item 2: prop lowering ---

func TestParseProps(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]any
	}{
		{"empty", "", map[string]any{}},
		{"int", "initial={3}", map[string]any{"initial": 3}},
		{"string", `label="hi"`, map[string]any{"label": "hi"}},
		{"bool-true", "live={true}", map[string]any{"live": true}},
		{"bool-false", "disabled={false}", map[string]any{"disabled": false}},
		{"bare-bool", "live", map[string]any{"live": true}},
		{"negative-int", "delta={-2}", map[string]any{"delta": -2}},
		{"multiple", `initial={3} label="hi" live`, map[string]any{"initial": 3, "label": "hi", "live": true}},
		{"string-braced", `title={"Q3"}`, map[string]any{"title": "Q3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseProps(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("parseProps(%q) = %#v, want %#v", c.in, got, c.want)
			}
			for k, wv := range c.want {
				gv, ok := got[k]
				if !ok {
					t.Fatalf("parseProps(%q) missing key %q; got %#v", c.in, k, got)
				}
				if gv != wv {
					t.Fatalf("parseProps(%q)[%q] = %#v (%T), want %#v (%T)", c.in, k, gv, gv, wv, wv)
				}
			}
		})
	}
}

// --- Work item 3: prose -> static gosx.Node ---

// TestRenderSlideProseAndIsland proves a slide of heading + prose + a live
// <Counter/> lowers to a node tree that contains the heading text, the prose,
// and an actual island mount (the renderer's data-gosx-island marker).
func TestRenderSlideProseAndIsland(t *testing.T) {
	deck, err := LoadIslandDeck("testdata/island-deck")
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	r := island.NewRenderer("test")
	// Register the program asset so the island mount carries a programRef.
	prog, _, err := deck.CompileComponent("Counter")
	if err != nil {
		t.Fatalf("CompileComponent: %v", err)
	}
	r.SetProgramAsset("Counter", "/gosx/islands/Counter.json", "json", "")

	node := renderIslandSlide(r, deck.Slides[0], map[string]*compiledComponent{
		"Counter": {prog: prog},
	})
	html := gosx.RenderHTML(node)

	if !strings.Contains(html, "Hello") {
		t.Fatalf("rendered slide missing heading text 'Hello':\n%s", html)
	}
	if !strings.Contains(html, "Welcome to the real lane") {
		t.Fatalf("rendered slide missing prose:\n%s", html)
	}
	if !strings.Contains(html, `data-gosx-island="Counter"`) {
		t.Fatalf("rendered slide missing live island mount:\n%s", html)
	}
}

// --- HTML escaping: deck content must never be injected raw ---

// renderSlideHTML parses one slide of markdown and renders it through the real
// lane, returning the HTML string. Components are unresolved (nil map) — these
// tests exercise the prose lane only.
func renderSlideHTML(t *testing.T, src string) string {
	t.Helper()
	doc, err := mdpp.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	r := island.NewRenderer("test")
	node := renderIslandSlide(r, IslandSlide{Index: 0, Node: doc.Slides()[0]}, nil)
	return gosx.RenderHTML(node)
}

// TestRenderEscapesHeadingText proves a heading whose text contains &, <, and "
// is emitted escaped — the dangerous characters are entity-encoded, not injected
// as raw markup.
func TestRenderEscapesHeadingText(t *testing.T) {
	html := renderSlideHTML(t, "# A & B < C \"q\"\n")
	if !strings.Contains(html, "&amp;") {
		t.Errorf("heading did not escape & to &amp;:\n%s", html)
	}
	if !strings.Contains(html, "&lt;") {
		t.Errorf("heading did not escape < to &lt;:\n%s", html)
	}
	// The literal `<h1>` open tag is fine; what must NOT appear is the content's
	// raw `< C` sequence (it must have become `&lt; C`).
	if strings.Contains(html, "< C") {
		t.Errorf("heading content's raw '<' leaked unescaped:\n%s", html)
	}
}

// TestRenderEscapesParagraphText proves paragraph prose containing & and " is
// emitted escaped (no raw injection from text runs).
func TestRenderEscapesParagraphText(t *testing.T) {
	html := renderSlideHTML(t, "plain & dangerous \"quotes\" here\n")
	if !strings.Contains(html, "&amp;") {
		t.Errorf("paragraph did not escape & to &amp;:\n%s", html)
	}
	if !strings.Contains(html, "&#34;") {
		t.Errorf("paragraph did not escape \" to &#34;:\n%s", html)
	}
	if strings.Contains(html, ` & `) {
		t.Errorf("paragraph's raw '&' leaked unescaped:\n%s", html)
	}
}

// TestRenderEscapesExpression proves an inline {expr} whose source contains < and
// & is emitted escaped — the expression source is rendered as escaped text, never
// raw markup.
func TestRenderEscapesExpression(t *testing.T) {
	html := renderSlideHTML(t, "Value: {a < b && c}\n")
	if !strings.Contains(html, "&lt;") {
		t.Errorf("expression did not escape < to &lt;:\n%s", html)
	}
	if !strings.Contains(html, "&amp;&amp;") {
		t.Errorf("expression did not escape && to &amp;&amp;:\n%s", html)
	}
	if strings.Contains(html, "a < b") {
		t.Errorf("expression's raw '<' leaked unescaped:\n%s", html)
	}
}

// TestRenderDoesNotInjectRawScript proves a raw <script> written in deck prose is
// never emitted as a live <script> tag (it arrives as an HTMLInline node, which
// the prose lane drops; either way no executable script reaches the output).
func TestRenderDoesNotInjectRawScript(t *testing.T) {
	html := renderSlideHTML(t, "danger <script>alert(1)</script> end\n")
	if strings.Contains(html, "<script>") {
		t.Errorf("raw <script> tag leaked into rendered output:\n%s", html)
	}
}

// TestRenderSlideHeadingLevels proves heading level maps to the right tag.
func TestRenderSlideHeadingLevels(t *testing.T) {
	src := []byte("## Subhead\n\nbody text\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	r := island.NewRenderer("test")
	node := renderIslandSlide(r, IslandSlide{Index: 0, Node: doc.Slides()[0]}, nil)
	html := gosx.RenderHTML(node)
	if !strings.Contains(html, "<h2>Subhead</h2>") {
		t.Fatalf("want <h2>Subhead</h2> in:\n%s", html)
	}
	if !strings.Contains(html, "<p>body text</p>") {
		t.Fatalf("want <p>body text</p> in:\n%s", html)
	}
}

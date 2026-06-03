package slides

import (
	"testing"

	"m31labs.dev/mdpp"
)

// TestLoadIslandDeckAndCompile proves Phase 1 Slice 1's lowering core: a deck's
// inline <Component/> becomes a real GoSX island program (bytecode). It loads a
// deck via the mdpp-backed loader, finds the inline <Counter/> reference, then
// drives the proven gosx Compile -> LowerIsland -> EncodeJSON pipeline and
// asserts the resulting island *program.Program is well-formed.
func TestLoadIslandDeckAndCompile(t *testing.T) {
	const dir = "testdata/island-deck"

	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck(%q): %v", dir, err)
	}

	// The deck has exactly one slide (no `---` separators in deck.md, so mdpp's
	// SplitSlides yields a single uniform slide).
	if got := len(deck.Slides); got != 1 {
		t.Fatalf("slide count = %d, want 1", got)
	}

	// That slide references a component named "Counter".
	slide := deck.Slides[0]
	if got := len(slide.Components); got != 1 {
		t.Fatalf("slide 0 component count = %d, want 1", got)
	}
	if name := slide.Components[0].Name; name != "Counter" {
		t.Fatalf("component name = %q, want %q", name, "Counter")
	}

	// Compiling that component yields a valid island program (real bytecode).
	prog, wire, err := deck.CompileComponent("Counter")
	if err != nil {
		t.Fatalf("CompileComponent(Counter): %v", err)
	}
	if prog == nil {
		t.Fatal("CompileComponent returned nil program")
	}
	if prog.Name != "Counter" {
		t.Fatalf("program Name = %q, want %q", prog.Name, "Counter")
	}

	// A signal named "count" exists (counter.gsx declares `count := signal.New(0)`).
	var signalNames []string
	for _, s := range prog.Signals {
		signalNames = append(signalNames, s.Name)
	}
	if !hasSignal(signalNames, "count") {
		t.Fatalf("island signals = %v, want one named %q", signalNames, "count")
	}

	// At least one handler exists (increment/decrement).
	if len(prog.Handlers) == 0 {
		t.Fatal("island has no handlers, want >= 1 (increment/decrement)")
	}

	// The program has lowered DOM nodes.
	if len(prog.Nodes) == 0 {
		t.Fatal("island has no nodes, want > 0")
	}

	// The JSON wire form is non-empty (this is what the dev socket ships).
	if len(wire) == 0 {
		t.Fatal("CompileComponent returned empty JSON wire bytes")
	}
}

// --- I1.1: component refs inside HTML comments must not leak ---

// TestCollectComponentRefsIgnoresHTMLComment is the I1 regression: mdpp passes an
// HTML comment through as a NodeHTMLBlock whose .Literal holds the comment text,
// including any <Tag/> written inside it. Scanning that literal naively yields a
// bogus ref (here <Ghost/>), which then fails the whole deck at compile time.
// A component tag that lives only inside a comment must contribute ZERO refs.
func TestCollectComponentRefsIgnoresHTMLComment(t *testing.T) {
	src := []byte("# T\n\n<!-- TODO: add <Ghost/> later -->\n\nreal prose\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 0 {
		t.Fatalf("refs = %d, want 0 (tag is only inside an HTML comment); got %#v", len(refs), refs)
	}
}

// TestCollectComponentRefsMultiLineHTMLComment proves multi-line comments are
// stripped too: a <Tag/> spanning into a multi-line <!-- ... --> block yields no
// ref.
func TestCollectComponentRefsMultiLineHTMLComment(t *testing.T) {
	src := []byte("# T\n\n<!--\nnotes:\n<Ghost initial={3}/>\n-->\n\nbody\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 0 {
		t.Fatalf("refs = %d, want 0 (tag inside multi-line comment); got %#v", len(refs), refs)
	}
}

// TestCollectComponentRefsCommentDoesNotMaskRealTag proves stripping comments does
// not swallow a genuine adjacent component: a real <Counter/> next to a commented
// <Ghost/> still yields exactly the Counter ref.
func TestCollectComponentRefsCommentDoesNotMaskRealTag(t *testing.T) {
	src := []byte("# T\n\n<!-- <Ghost/> -->\n\n<Counter initial={3}/>\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1 (only the real Counter); got %#v", len(refs), refs)
	}
	if refs[0].Name != "Counter" {
		t.Fatalf("ref name = %q, want Counter; got %#v", refs[0].Name, refs)
	}
}

// TestCollectComponentRefsCodeUnaffected guards that uppercase tags inside inline
// code (`<Counter/>`) and fenced code blocks are NOT picked up as refs — those
// arrive as NodeCodeSpan / NodeCodeBlock, which the ref scan never inspects.
func TestCollectComponentRefsCodeUnaffected(t *testing.T) {
	src := []byte("# T\n\nInline code `<Counter/>` is literal.\n\n```\n<Counter/>\n```\n")
	doc, err := mdpp.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mdpp.SplitSlides(doc)
	refs := collectComponentRefs(doc.Slides()[0])
	if len(refs) != 0 {
		t.Fatalf("refs = %d, want 0 (tags are inside code, not real refs); got %#v", len(refs), refs)
	}
}

// hasSignal reports whether names contains want.
func hasSignal(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

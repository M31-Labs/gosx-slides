package slides

import (
	"testing"
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

// hasSignal reports whether names contains want.
func hasSignal(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

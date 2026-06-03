package slides

// render_program.go wires the Slice-4 render flow: lower the deck to one GoSX
// source (slidegen.go), compile it ONCE, then render each slide via
// route.RenderProgramComponent so inline {expr} actually EVALUATES server-side
// and inline <Component/> tags hydrate as real islands (inlined through the
// per-request island renderer).
//
// This supersedes render_island.go's hand-built lowering for the SERVE path
// (serve.go's renderPage now calls renderProgramSlides). render_island.go's
// node lowering stays in the tree — it still backs render_island_test.go's
// prose/escaping/island unit coverage and documents the original lane — but the
// live deck server renders through the compiler now, which is what makes {expr}
// real.

import (
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/ir"
	"m31labs.dev/gosx/island"
	"m31labs.dev/gosx/route"
)

// compiledDeck is the result of lowering + compiling a whole deck to one program
// for the Slice-4 render flow. The single program declares every Slide_N
// function plus the island components they reference; RenderProgramComponent
// renders an individual slide from it.
type compiledDeck struct {
	// prog is the compiled program for the generated deck source (all Slide_N
	// funcs + merged island defs). nil if compilation failed.
	prog *ir.Program
	// slideCount is the number of Slide_N functions generated (== len(deck.Slides)).
	slideCount int
	// source is the generated GoSX source, retained for diagnostics.
	source string
}

// compileDeckProgram lowers the deck to one GoSX source and compiles it once.
//
// For each distinct referenced component it reads <Name>.gsx from the deck dir
// and parses it into an islandDef (package + imports stripped) so its
// definition can be inlined into the deck source; a component whose file is
// missing or unreadable is simply omitted (it then renders fail-soft as the
// renderer's unresolved/empty path). On a compile failure the returned
// compiledDeck has a nil prog and the error is returned, so callers can fall
// back to the previous render lane rather than 500.
func compileDeckProgram(deck *IslandDeck) (*compiledDeck, error) {
	defs := loadIslandDefs(deck)
	source := generateDeckSource(deck, defs)
	prog, err := gosx.Compile([]byte(source))
	if err != nil {
		return &compiledDeck{slideCount: len(deck.Slides), source: source}, err
	}
	return &compiledDeck{prog: prog, slideCount: len(deck.Slides), source: source}, nil
}

// loadIslandDefs reads and parses the <Name>.gsx definition for every distinct
// component referenced anywhere in the deck. A component whose file is missing,
// unreadable, or empty is omitted (it renders fail-soft). The map is keyed by
// component name.
func loadIslandDefs(deck *IslandDeck) map[string]islandDef {
	defs := map[string]islandDef{}
	for _, slide := range deck.Slides {
		for _, ref := range slide.Components {
			if _, ok := defs[ref.Name]; ok {
				continue
			}
			source, err := deck.readComponentSource(ref.Name)
			if err != nil || strings.TrimSpace(source) == "" {
				continue
			}
			def := parseIslandDef(source)
			if def.body == "" {
				continue
			}
			defs[ref.Name] = def
		}
	}
	return defs
}

// renderProgramSlides renders every slide of the deck to HTML via the Slice-4
// flow and returns one gosx.Node per slide (a RawHTML node wrapping the rendered
// section). Inline {expr} is evaluated by the gosx compiler/renderer; inline
// <Component/> tags hydrate as real islands via the per-request renderer r
// (RenderIslandFromProgram registers each on r so PageHead sees them).
//
// The render env binds a small, safe expression namespace (exprFuncs) so prose
// can call e.g. strings.ToUpper; pure exprs ({2 + 3}) need no bindings. An
// unresolved identifier renders empty (fail-soft), never an error.
//
// If the deck failed to compile (cd.prog == nil), it falls back to the original
// hand-built lane (renderIslandSlide) so a transient bad slide never blanks the
// page — {expr} degrades to raw text there, but prose and islands still render.
func renderProgramSlides(r *island.Renderer, deck *IslandDeck, cd *compiledDeck, compiled map[string]*compiledComponent) []gosx.Node {
	if cd == nil || cd.prog == nil {
		// Fallback lane: compile failed; render each slide the hand-built way so
		// the page still serves (prose + islands; {expr} as raw text).
		var nodes []gosx.Node
		for _, slide := range deck.Slides {
			nodes = append(nodes, renderIslandSlide(r, slide, compiled))
		}
		return nodes
	}

	env := route.ProgramRenderEnv{
		Funcs:        exprFuncs(),
		RenderIsland: r.RenderIslandFromProgram,
	}

	var nodes []gosx.Node
	for _, slide := range deck.Slides {
		html, err := route.RenderProgramComponent(cd.prog, slideFuncName(slide.Index), env)
		if err != nil {
			// A single slide failing to render must not blank the deck: fall back
			// to the hand-built lane for just this slide.
			nodes = append(nodes, renderIslandSlide(r, slide, compiled))
			continue
		}
		nodes = append(nodes, gosx.RawHTML(html))
	}
	return nodes
}

// exprFuncs is the safe expression-evaluation namespace bound for slide {expr}.
// It is intentionally SMALL and explicit: a slide can call these from inline
// prose (e.g. {strings.ToUpper("hi")}), and nothing else from a package is
// reachable. Pure exprs ({2 + 3}, {"a" + "b"}) need no entry here. Extend this
// map to widen the surface (see hand-off notes: deck/slide vars, signals).
func exprFuncs() map[string]any {
	return map[string]any{
		"strings": map[string]any{
			"ToUpper":   strings.ToUpper,
			"ToLower":   strings.ToLower,
			"TrimSpace": strings.TrimSpace,
			"Title":     strings.Title,
			"Repeat":    strings.Repeat,
			"Join":      strings.Join,
		},
	}
}

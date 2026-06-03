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
	"m31labs.dev/gosx/highlight"
	"m31labs.dev/gosx/ir"
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
func renderProgramSlides(r islandMounter, deck *IslandDeck, cd *compiledDeck, compiled map[string]*compiledComponent) []gosx.Node {
	if cd == nil || cd.prog == nil {
		// Fallback lane: compile failed; render each slide the hand-built way so
		// the page still serves (prose + islands; {expr} as raw text).
		var nodes []gosx.Node
		for _, slide := range deck.Slides {
			nodes = append(nodes, renderIslandSlide(r, slide, compiled))
		}
		return nodes
	}

	deckVals := deckFrontmatterValues(deck)
	funcs := exprFuncs()

	var nodes []gosx.Node
	for _, slide := range deck.Slides {
		// Per-slide expression scope: prose can reference {deck.<key>} (headmatter)
		// and {slide.<key>} (this slide's frontmatter, plus its index). Keys are the
		// lowercase YAML keys as authored; an unknown key resolves empty (fail-soft).
		env := route.ProgramRenderEnv{
			Values: map[string]any{
				"deck":  deckVals,
				"slide": slideFrontmatterValues(slide),
			},
			Funcs:        funcs,
			RenderIsland: r.RenderIslandFromProgram,
		}
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
		// codeNS backs the generated `{` + codeBlockFunc + `(lang, src)}` call that
		// slidegen lowers a fenced code block to. It returns a gosx RawHTML Node
		// (not a string) so the syntax-highlighted span markup survives the
		// expression evaluator unescaped — a plain string would be HTML-escaped by
		// the renderer (kindExpr), turning the <span> tokens into visible text. See
		// codeBlockNode for the rationale and the probe that proved RawHTML rides
		// the eval path; the highlighter itself escapes the code text, so the output
		// is always safe.
		codeNamespace: map[string]any{
			codeBlockFunc: codeBlockNode,
		},
	}
}

// codeNamespace / codeBlockFunc name the bound expression function slidegen emits
// for a fenced code block (e.g. `{__slidesCode.Block("go", "…")}`). They are
// consts so the generator (slidegen.go) and this binding can never drift; the
// namespace is `__`-prefixed so it cannot collide with a deck author's own
// identifier in prose.
const (
	codeNamespace = "__slidesCode"
	codeBlockFunc = "Block"
)

// codeBlockNode renders a fenced code block to a syntax-highlighted
// `<pre class="code-block" data-lang="…"><code>…</code></pre>` Node. It is the
// real-lane code-block renderer: slidegen lowers a ```lang fence to a call to
// this (via the codeNamespace binding), so the gosx compiler evaluates the call
// at render time and the returned RawHTML Node emits token <span>s the themes
// style. Returning a Node (not a string) is load-bearing — a string would be
// escaped by the expression renderer; the highlighter escapes the code text, so
// the tokens are the only markup and the block is XSS-safe.
//
// lang is the fence info-string language (e.g. "go"); highlight.NormalizeLanguage
// canonicalizes it (unknown -> plain escaped text). The trailing newline a fence
// commonly carries is trimmed so the <pre> has no dangling blank last line.
func codeBlockNode(lang, source string) gosx.Node {
	source = strings.TrimRight(source, "\n")
	// NormalizeLanguage returns one of a fixed, attribute-safe token set
	// (go/gosx/javascript/json/bash/text), so it needs no escaping in the data-lang
	// attribute. highlight.HTML escapes the code text itself.
	normalized := highlight.NormalizeLanguage(lang)
	var b strings.Builder
	b.WriteString(`<pre class="code-block" data-lang="`)
	b.WriteString(normalized)
	b.WriteString(`"><code>`)
	b.WriteString(highlight.HTML(normalized, source))
	b.WriteString(`</code></pre>`)
	return gosx.RawHTML(b.String())
}

// deckFrontmatterValues parses the deck's headmatter (the leading `---` block of
// deck.md) into the value map bound as `deck` for slide expressions, so prose can
// reference {deck.title}, {deck.theme}, etc. Keys are the lowercase YAML keys as
// authored. A deck with no headmatter yields an empty map (refs resolve empty).
func deckFrontmatterValues(deck *IslandDeck) map[string]any {
	if deck == nil {
		return map[string]any{}
	}
	headmatter, _, err := splitHeadmatter(string(deck.Source))
	if err != nil {
		return map[string]any{}
	}
	return stringMapToAny(parseFrontmatter(headmatter))
}

// slideFrontmatterValues builds the value map bound as `slide` for one slide's
// expressions: its per-slide frontmatter keys (e.g. {slide.layout}) plus its
// 0-based {slide.index}. The index is always present; frontmatter keys override
// nothing reserved here.
func slideFrontmatterValues(slide IslandSlide) map[string]any {
	vals := map[string]any{"index": slide.Index}
	if slide.Node != nil {
		for k, v := range parseFrontmatter(slide.Node.Attr("frontmatter")) {
			vals[k] = v
		}
	}
	return vals
}

// stringMapToAny widens a string map to an any map so the route expression
// evaluator can resolve member access (e.g. deck.title) against it.
func stringMapToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

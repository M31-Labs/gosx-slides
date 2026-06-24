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
	"strconv"
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
// titleCase upper-cases the first letter of each whitespace-separated word — a
// Unicode-safe replacement for the deprecated strings.Title, bound as
// {strings.Title(...)} in deck expressions.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		words[i] = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return strings.Join(words, " ")
}

func exprFuncs() map[string]any {
	return map[string]any{
		"strings": map[string]any{
			"ToUpper":   strings.ToUpper,
			"ToLower":   strings.ToLower,
			"TrimSpace": strings.TrimSpace,
			"Title":     titleCase,
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
		// diagramNS backs the generated `{__slidesDiagram.Render(src, theme, view)}`
		// call that slidegen lowers a sirena fence to. renderSirenaDiagram calls
		// fence.Render server-side and returns a gosx.Node (inline SVG <figure> or
		// a degrade <pre>), so the SVG rides the eval path as a Node — never
		// HTML-escaped. Pure server-side: no JavaScript, no CDN.
		diagramNamespace: map[string]any{
			diagramRenderFunc: slidesDiagram{}.Render,
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

// diagramNamespace / diagramRenderFunc name the bound expression function
// slidegen emits for a sirena fence (e.g. `{__slidesDiagram.Render(src, "", "")}`).
// They are consts so the generator (slidegen.go) and this binding never drift;
// the `__`-prefix prevents collisions with deck author identifiers.
const (
	diagramNamespace  = "__slidesDiagram"
	diagramRenderFunc = "Render"
)

// slidesDiagram is the runtime helper type whose Render method is bound as
// __slidesDiagram in the expression scope. It wraps renderSirenaDiagram so the
// gosx expression evaluator can call it from generated slide code.
type slidesDiagram struct{}

// Render calls renderSirenaDiagram and returns the resulting gosx.Node. It is
// the implementation behind `{__slidesDiagram.Render(source, theme, view)}` in
// the generated deck source — the gosx expression evaluator calls this at render
// time, so the inline SVG is produced server-side with no JavaScript required.
func (slidesDiagram) Render(source, theme, view string) gosx.Node {
	return renderSirenaDiagram(source, theme, view, "")
}

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
//
// highlights is the fence's `{…}` line-range meta (mdpp's Attrs["highlights"], raw
// e.g. "2-3|6" or "2,4" or "all"; "" for a plain fence). When non-empty, the block
// renders PER LINE (highlight.HTMLLines: each line wrapped in
// `<span class="ts-line" data-line="N">`) and the lines in the spec get an extra
// `emphasis` class; the rest are dimmed by the theme CSS. A `data-emphasized`
// marker on the <pre> lets the CSS dim only when a spec is present, so a plain
// fence (no spec) renders every line at full opacity exactly as before.
//
// CLICK-THROUGH STEPS: the `|`-separated groups are ordered click STEPS, not a
// flat union. Each emphasized line is tagged `data-step="K"` (1-based) with the
// step(s) it belongs to (SPACE-joined if a line recurs across groups, e.g.
// "1 3", so the theme CSS can match the active step with a `~=` word selector),
// and the <pre> records the total step count as `data-steps="N"`. A
// single-group spec like `{1-3}` is one step (every matched line data-step="1"),
// so the existing STATIC emphasis still lights exactly those lines. navScript
// reads data-steps to drive the step model and sets data-active-step on the deck;
// the theme CSS spotlights the active step. With no active step (data-steps="0",
// or before the first ArrowRight on a stepped slide) every emphasized line shows,
// so a stepped block reads like the old static union until you start stepping.
func codeBlockNode(lang, source, highlights string) gosx.Node {
	source = strings.TrimRight(source, "\n")
	// NormalizeLanguage returns one of a fixed, attribute-safe token set
	// (go/gosx/javascript/json/bash/text), so it needs no escaping in the data-lang
	// attribute. highlight.HTML / highlight.HTMLLines escape the code text itself.
	normalized := highlight.NormalizeLanguage(lang)
	isDiff := strings.EqualFold(lang, "diff")

	steps := parseHighlightSteps(highlights)

	var b strings.Builder
	b.WriteString(`<pre class="code-block" data-lang="`)
	b.WriteString(normalized)
	// When a (valid) spec is present, mark the block so the theme CSS dims the
	// non-emphasized lines, and record the ordered step count so navScript can
	// advance through the groups. Absent/garbage spec -> no markers -> every line
	// full opacity, no stepping.
	if len(steps) > 0 {
		b.WriteString(`" data-emphasized="true" data-steps="`)
		b.WriteString(strconv.Itoa(len(steps)))
	}
	b.WriteString(`"><code>`)
	if !isDiff && len(steps) == 0 {
		// No emphasis, not a diff: the original single-string path (one highlighted
		// block, no per-line wrappers) — byte-identical to the pre-emphasis behavior.
		b.WriteString(highlight.HTML(normalized, source))
	} else {
		// Per-line wrappers: needed for emphasis stepping OR diff coloring. The
		// highlighter already escaped the code; we only ADD classes/data attrs to
		// individual lines, so the markup stays XSS-safe.
		for i, line := range highlight.HTMLLines(normalized, source) {
			// Apply emphasis stepping first (it widens class="ts-line" ->
			// class="ts-line emphasis"); diff class injection runs after so it
			// can locate the now-widened class prefix safely.
			if len(steps) > 0 {
				line = emphasizeLineSteps(line, steps)
			}
			if isDiff {
				line = addDiffClass(line, i, source)
			}
			b.WriteString(line)
		}
	}
	b.WriteString(`</code></pre>`)
	return gosx.RawHTML(b.String())
}

// addDiffClass injects a diff-coloring class into the `class="ts-line"` attribute
// of one highlight.HTMLLines fragment based on the first non-space character of
// the corresponding raw source line. lineIdx is the 0-based index into
// highlight.HTMLLines (which does not itself expose the raw text).
//
//   - `+added` (but not `+++`) -> ts-diff-add (green background)
//   - `-removed` (but not `---`) -> ts-diff-del (red background)
//   - `@@…` hunk header         -> ts-diff-meta (accent color)
//   - everything else            -> unchanged
//
// The class is appended AFTER the existing `ts-line` class so it is purely
// additive; emphasis and step attributes added later by emphasizeLineSteps are
// unaffected. The raw source is needed to inspect the actual first character
// because the highlighted span markup has already been HTML-entity-encoded.
func addDiffClass(line string, lineIdx int, rawSource string) string {
	rawLines := strings.Split(rawSource, "\n")
	if lineIdx >= len(rawLines) {
		return line
	}
	raw := strings.TrimLeft(rawLines[lineIdx], " \t")
	var cls string
	switch {
	case strings.HasPrefix(raw, "+++") || strings.HasPrefix(raw, "---"):
		// file header lines — leave uncolored (they look like meta but are prose)
	case strings.HasPrefix(raw, "+"):
		cls = "ts-diff-add"
	case strings.HasPrefix(raw, "-"):
		cls = "ts-diff-del"
	case strings.HasPrefix(raw, "@@"):
		cls = "ts-diff-meta"
	}
	if cls == "" {
		return line
	}
	// Match the open class prefix so the replacement works whether emphasizeLineSteps
	// has already widened class="ts-line" to class="ts-line emphasis ..." or not.
	// We replace `class="ts-line` with `class="ts-line <cls>` in one pass.
	return strings.Replace(line, `class="ts-line`, `class="ts-line `+cls, 1)
}

// emphasizeLineSteps adds the `emphasis` class AND a `data-step="K"` attribute to
// a single `<span class="ts-line" data-line="N">…` fragment from
// highlight.HTMLLines when N belongs to one or more ordered click STEPS, and
// returns it unchanged otherwise. steps is the ordered slice parseHighlightSteps
// produced (one set per `|`-group); the step value(s) are 1-based group indices,
// SPACE-joined when a line recurs across groups (e.g. data-step="1 3"), so the
// theme CSS can match the active step with a `[data-step~="K"]` word selector. A
// group that is the "all" sentinel matches every real line.
//
// It edits only the well-known opening class attribute the highlighter emits
// (`class="ts-line"`), so it never touches the inner token markup and cannot
// unbalance the spans. An emphasized line is lit when no step is active (the
// static-union look) and spotlit/dimmed by data-active-step once stepping begins.
func emphasizeLineSteps(line string, steps []map[int]bool) string {
	n := lineNumberOf(line)
	if n == 0 {
		return line
	}
	var in []string
	for i, set := range steps {
		if set[allLinesSentinel] || set[n] {
			in = append(in, strconv.Itoa(i+1)) // 1-based step index
		}
	}
	if len(in) == 0 {
		return line // not in any step's spec -> plain, dimmed by the data-emphasized CSS
	}
	// highlight.HTMLLines always opens the line with the exact literal
	// `class="ts-line"`; widen it to add the emphasis hook + the step tag. Replace
	// once so a data-line value that happened to contain the substring can't be
	// touched.
	return strings.Replace(line, `class="ts-line"`,
		`class="ts-line emphasis" data-step="`+strings.Join(in, " ")+`"`, 1)
}

// lineNumberOf extracts the 1-based N from a `data-line="N"` attribute in a
// highlight.HTMLLines fragment, returning 0 when absent or unparseable. The value
// is machine-generated (strconv.Itoa) so it is always a bare integer.
func lineNumberOf(line string) int {
	const marker = `data-line="`
	i := strings.Index(line, marker)
	if i < 0 {
		return 0
	}
	rest := line[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return 0
	}
	n, err := strconv.Atoi(rest[:j])
	if err != nil {
		return 0
	}
	return n
}

// parseHighlightLines parses the fence line-range mini-DSL mdpp stores in a code
// block's Attrs["highlights"] into the flat SET of 1-based line numbers to
// emphasize. The grammar (matching mdpp's fence meta and gosx's own range parser):
//
//	groups   separated by '|'   — each '|' group is a future "click step"; for
//	                              STATIC emphasis we union every group's lines.
//	items    separated by ','   — within a group
//	item     N         a single 1-based line
//	         N-M       an inclusive range (M >= N)
//	         all       every line (the sentinel)
//
// Whitespace around separators/items is ignored. "all" (case-insensitive) anywhere
// yields the sentinel map{-1:true} so the caller can treat the whole block as
// emphasized regardless of line count. Empty/garbage input yields an empty map, so
// the caller falls back to rendering every line at full opacity (no emphasis).
//
// NOTE: click-through STEPPING (advancing through the '|' groups one at a time on
// keypress) is a deliberate future slice — this flattens the groups to a single
// static emphasis set. See the hand-off in the task report.
func parseHighlightLines(spec string) map[int]bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	lines := map[int]bool{}
	for _, group := range strings.Split(spec, "|") {
		for _, item := range strings.Split(group, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if strings.EqualFold(item, "all") {
				return map[int]bool{allLinesSentinel: true}
			}
			if lo, hi, ok := parseLineRange(item); ok {
				for n := lo; n <= hi; n++ {
					lines[n] = true
				}
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// parseHighlightSteps parses the same fence mini-DSL as parseHighlightLines but
// PRESERVES the `|`-group ORDER as click STEPS instead of unioning them. Each
// `|`-group becomes one set of 1-based line numbers (in input order); an empty or
// all-garbage group is DROPPED (so a stray `||` never creates an empty step), and
// an "all" group becomes the sentinel set {-1:true} matching every line. The
// returned slice's length is the number of steps the block advances through
// (data-steps); an empty/garbage spec yields nil (no steps, plain block).
//
// Examples:
//
//	"2-3|6" -> [{2,3}, {6}]            (two steps)
//	"1-3"   -> [{1,2,3}]              (one step — the static-emphasis case)
//	"1|all" -> [{1}, {-1}]            (second step lights every line)
//	"|2|"   -> [{2}]                  (empty groups dropped)
//	""      -> nil                    (plain block, no stepping)
//
// This is the ordered counterpart kept beside parseHighlightLines (which stays for
// the flat static-union semantics its callers/tests rely on); the gosx highlighter
// has the same model internally (highlight.HighlightStep), but its parser is
// unexported, so the slides DSL mirrors it here.
func parseHighlightSteps(spec string) []map[int]bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	var steps []map[int]bool
	for _, group := range strings.Split(spec, "|") {
		set := map[int]bool{}
		all := false
		for _, item := range strings.Split(group, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if strings.EqualFold(item, "all") {
				all = true
				continue
			}
			if lo, hi, ok := parseLineRange(item); ok {
				for n := lo; n <= hi; n++ {
					set[n] = true
				}
			}
		}
		switch {
		case all:
			// "all" subsumes any explicit lines in the same group.
			steps = append(steps, map[int]bool{allLinesSentinel: true})
		case len(set) > 0:
			steps = append(steps, set)
		}
		// A group that contributed nothing (empty/garbage) is dropped, so it never
		// becomes a dead step you'd have to click past.
	}
	if len(steps) == 0 {
		return nil
	}
	return steps
}

// allLinesSentinel is the key parseHighlightLines / parseHighlightSteps set when a
// group is "all": emphasize every line. It is a value no real 1-based line number
// can be, so a lookup for any positive N falls through to the sentinel branch.
const allLinesSentinel = -1

// parseLineRange parses one "N" or "N-M" item into an inclusive [lo, hi]. It
// returns ok == false for anything malformed, a non-positive line, or an inverted
// range (M < N) — matching gosx highlight's parseRangeItem so the slides DSL and
// the gosx one accept exactly the same inputs.
func parseLineRange(item string) (lo, hi int, ok bool) {
	if dash := strings.IndexByte(item, '-'); dash >= 0 {
		a := strings.TrimSpace(item[:dash])
		b := strings.TrimSpace(item[dash+1:])
		lo, err1 := strconv.Atoi(a)
		hi, err2 := strconv.Atoi(b)
		if err1 != nil || err2 != nil || lo < 1 || hi < lo {
			return 0, 0, false
		}
		return lo, hi, true
	}
	n, err := strconv.Atoi(item)
	if err != nil || n < 1 {
		return 0, 0, false
	}
	return n, n, true
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

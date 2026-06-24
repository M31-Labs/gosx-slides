package slides

// bridge.go is the entry point of gosx-slides' render pipeline: it loads a deck
// directory and composes two already-shipped seams to turn a deck's inline
// <Component/> into a genuine GoSX island program (bytecode):
//
//   - mdpp parses the deck's Markdown and splits it into slides, with uppercase
//     inline tags reinterpreted as mdpp.NodeComponent leaves.
//   - gosx compiles the referenced .gsx component and lowers the island to an
//     island/program.Program, the same wire form `gosx dev` hot-swaps.
//
// This file is the lowering CORE: load a deck, find its component references, and
// compile a referenced component to island bytecode. Prose/static-node lowering,
// HTTP serving, and hydration live in slidegen.go / render_program.go / serve.go.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/ir"
	"m31labs.dev/gosx/island/program"
	"m31labs.dev/mdpp"
)

// DeckFileName is the deck entry file LoadIslandDeck reads from a deck directory.
const DeckFileName = "deck.md"

// ComponentRef is a single inline <Name .../> reference found on a slide. It
// records the component name and the raw, opaque props string exactly as mdpp
// captured it (e.g. `initial={3}`); props are NOT parsed or lowered in Slice 1.
type ComponentRef struct {
	// Name is the component function name, e.g. "Counter". It is the value of
	// the mdpp NodeComponent's "name" attribute and the name of the island this
	// reference resolves to (and the <Name>.gsx file that defines it).
	Name string

	// Props is the raw props source mdpp captured for the tag (the value of the
	// NodeComponent's "props" attribute), or "" when the tag had none. Carried
	// verbatim for a later slice to parse and lower into the island program.
	Props string
}

// IslandSlide is one deck slide in the real lane: the mdpp slide node plus the
// component references discovered anywhere in its subtree.
type IslandSlide struct {
	// Index is the slide's 0-based position in the deck.
	Index int

	// Node is the underlying mdpp NodeSlide. The full parsed subtree is retained
	// so later slices can lower its prose/static nodes to .gsx without re-parsing.
	Node *mdpp.Node

	// Components are the inline component references found in this slide, in
	// document order.
	Components []ComponentRef
}

// IslandDeck is the parsed model for the real lane: a deck directory, its mdpp
// document, and its slides with their component references. Compilation of a
// referenced component to island bytecode is done on demand via CompileComponent.
type IslandDeck struct {
	// Dir is the deck directory. <Name>.gsx component sources are resolved here.
	Dir string

	// Source is the raw deck.md bytes.
	Source []byte

	// Document is the parsed (and slide-split) mdpp document.
	Document *mdpp.Document

	// Slides are the deck's slides with their component references.
	Slides []IslandSlide
}

// LoadIslandDeck reads <dir>/deck.md, parses it with mdpp, splits it into slides
// (opt-in via mdpp.SplitSlides), and collects the inline component references on
// each slide. It does not compile anything — call CompileComponent for that.
func LoadIslandDeck(dir string) (*IslandDeck, error) {
	path := filepath.Join(dir, DeckFileName)
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read deck %s: %w", path, err)
	}

	doc, err := mdpp.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse deck %s: %w", path, err)
	}

	// Slide splitting is opt-in in mdpp; turn the flat document into a uniform
	// NodeSlide layer (idempotent) before we walk it.
	mdpp.SplitSlides(doc)

	deck := &IslandDeck{
		Dir:      dir,
		Source:   src,
		Document: doc,
	}

	for i, slideNode := range doc.Slides() {
		deck.Slides = append(deck.Slides, IslandSlide{
			Index:      i,
			Node:       slideNode,
			Components: collectComponentRefs(slideNode),
		})
	}

	warnAbsorbedSeparators(dir, doc)

	return deck, nil
}

// warnAbsorbedSeparators logs when slide separators were swallowed instead of
// splitting slides. For some multi-block slide sequences, mdpp's markdown parser
// parses a standalone `---`/`***`/`___` as a literal paragraph rather than a
// NodeThematicBreak, silently merging two slides into one longer slide. The deck
// still serves, so this is easy to miss — surface it with a concrete fix. (A `---`
// inside a code fence lives in a CodeBlock node, not a Paragraph, so it is not
// counted: no false positives from sample code.)
func warnAbsorbedSeparators(dir string, doc *mdpp.Document) {
	absorbed := 0
	for _, p := range doc.AST().Find(mdpp.NodeParagraph) {
		switch strings.TrimSpace(p.Text()) {
		case "---", "***", "___":
			absorbed++
		}
	}
	if absorbed > 0 {
		log.Printf("slides: deck %q has %d slide separator(s) that did not split a slide "+
			"(parsed as text, not a slide break) — slides were silently merged. End the slide "+
			"above each with a trailing block (an HTML comment / <!-- speaker note --> works) to force the split.",
			dir, absorbed)
	}
}

// CompileComponent resolves <name>.gsx in the deck directory and compiles the
// named island to its island program plus JSON wire form. It is the public
// entry to the real lane's lowering core.
func (d *IslandDeck) CompileComponent(name string) (*program.Program, []byte, error) {
	if d == nil {
		return nil, nil, fmt.Errorf("compile component %q: nil deck", name)
	}
	return compileIslandComponent(d.Dir, name)
}

// componentNamePat is the MDX component-name rule (mirrors mdpp/components.go):
// an uppercase initial, followed by identifier chars and optional dotted member
// access (<Foo.Bar/>). Gating on an uppercase initial is what distinguishes a
// GoSX component from ordinary HTML (<div>, <span>); kept local so mdpp stays
// unmodified.
const componentNamePat = `[A-Z][A-Za-z0-9_.]*`

// blockComponentRe matches a component tag — self-closing (<Name .../>) or an
// opening tag (<Name ...>) — anywhere inside a node's raw Literal. The props
// group is everything between the name and the closing `>`/`/>`, excluding angle
// brackets so a match cannot run across adjacent tags. Group 1 = name, group 2 =
// raw props, group 3 = "/" when self-closing.
//
// Why scan literals at all: mdpp folds only INLINE uppercase tags into
// NodeComponent. A component written on its own blank-line-delimited line — the
// common case for real decks — arrives as a NodeHTMLBlock (self-closing) or as a
// NodeText holding the opening tag plus a NodeHTMLInline close (paired). Neither
// is a NodeComponent, so the inline-only walk misses it; this regex recovers it.
var blockComponentRe = regexp.MustCompile(`<(` + componentNamePat + `)((?:\s[^<>]*?)?)\s*(/?)>`)

// htmlCommentRe matches an HTML comment, including multi-line ones (the `(?s)`
// flag lets `.` span newlines). mdpp passes `<!-- ... -->` through verbatim as a
// NodeHTMLBlock whose .Literal holds the comment text, so a component tag written
// inside a comment (`<!-- TODO: add <Ghost/> later -->`) would otherwise be
// scanned as a real reference and brick the deck. We strip comments from a literal
// before scanning it for component tags.
var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// stripHTMLComments removes HTML comments from a raw literal so component tags
// that exist only inside a comment are never mistaken for references.
func stripHTMLComments(literal string) string {
	if !strings.Contains(literal, "<!--") {
		return literal
	}
	return htmlCommentRe.ReplaceAllString(literal, "")
}

// collectComponentRefs walks a slide subtree in document order and returns every
// component reference it finds: both mdpp's folded inline NodeComponent leaves
// (Slice 1) and block-level tags that mdpp left as raw HTML literals (Slice 2).
//
// A folded NodeComponent's own tag is consumed into the node (it has no child
// literal carrying it), so scanning the literals of non-component nodes never
// double-counts an inline component. We still skip the subtree under a
// NodeComponent to avoid scanning any folded inner content.
func collectComponentRefs(slide *mdpp.Node) []ComponentRef {
	var refs []ComponentRef
	slide.Walk(func(n *mdpp.Node) bool {
		switch n.Type {
		case mdpp.NodeComponent:
			refs = append(refs, ComponentRef{
				Name:  n.Attr("name"),
				Props: n.Attr("props"),
			})
			// Don't descend: a folded paired component's inner content is its
			// children; any nested components there are out of Slice-2 scope and
			// would otherwise be matched twice if they surfaced as literals.
			return false
		case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline, mdpp.NodeText:
			refs = append(refs, scanLiteralComponents(n.Literal)...)
		}
		return true
	})
	return refs
}

// scanLiteralComponents finds component open/self-closing tags inside a raw
// literal and returns them as refs in document order. Each opening tag matches
// once (a paired <Name>…</Name> contributes a single ref from its open tag; the
// closing tag has no name-with-props match). The uppercase-initial gate in the
// pattern means lowercase HTML (<div>, <br/>) never matches.
func scanLiteralComponents(literal string) []ComponentRef {
	if literal == "" || !strings.Contains(literal, "<") {
		return nil
	}
	// Drop HTML comments first: a `<Tag/>` written inside `<!-- ... -->` is not a
	// real component reference (mdpp keeps comments verbatim in the literal).
	literal = stripHTMLComments(literal)
	var refs []ComponentRef
	for _, m := range blockComponentRe.FindAllStringSubmatch(literal, -1) {
		refs = append(refs, ComponentRef{
			Name:  m[1],
			Props: strings.TrimSpace(m[2]),
		})
	}
	return refs
}

// parseProps parses a component's raw props source into a structured map for
// lowering into the island program. Slice-2 scope: literal int, string, and
// bool props only — the forms a static deck needs.
//
//	initial={3}        -> {"initial": 3}      (int literal in braces)
//	delta={-2}         -> {"delta": -2}       (negative int)
//	label="hi"         -> {"label": "hi"}     (quoted string)
//	title={"Q3"}       -> {"title": "Q3"}     (quoted string in braces)
//	live={true}        -> {"live": true}      (bool literal in braces)
//	live               -> {"live": true}      (bare attribute = true)
//
// Richer expressions (signals, identifiers, arithmetic, objects) are NOT
// evaluated here — a value that is none of the above is carried through as its
// raw string so nothing is silently dropped. Real expression evaluation is a
// follow-up (see hand-off notes).
func parseProps(raw string) map[string]any {
	props := map[string]any{}
	for _, tok := range splitPropTokens(raw) {
		name, value, hasValue := strings.Cut(tok, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !hasValue {
			// Bare attribute: <Counter live/> -> live=true.
			props[name] = true
			continue
		}
		props[name] = parsePropValue(strings.TrimSpace(value))
	}
	return props
}

// parsePropValue lowers a single raw prop value (the right-hand side of `=`) to
// a Go value. Brace-wrapped expressions ({…}) are unwrapped first; then int,
// bool, and quoted-string literals are recognized. Anything else is returned
// verbatim as a string (no silent drop).
func parsePropValue(v string) any {
	v = strings.TrimSpace(v)
	// Unwrap a JSX-style {expr}.
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
		v = strings.TrimSpace(v[1 : len(v)-1])
	}
	// Quoted string.
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	// Bool literal.
	switch v {
	case "true":
		return true
	case "false":
		return false
	}
	// Integer literal.
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	// Float literal.
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	// Unrecognized: carry the raw source through (follow-up: real expr eval).
	return v
}

// splitPropTokens splits a raw props string into individual `name`, `name=val`,
// or `name={expr}` tokens, honoring quoted strings and brace groups so spaces
// inside "..." or {...} do not split a token.
func splitPropTokens(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tokens []string
	var cur strings.Builder
	depth := 0     // brace nesting
	var quote byte // active quote char, 0 when not in a string
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == '{':
			depth++
			cur.WriteByte(c)
		case c == '}':
			if depth > 0 {
				depth--
			}
			cur.WriteByte(c)
		case depth == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return tokens
}

// readComponentSource reads the raw <Name>.gsx source for a component from the
// deck directory. It is the source the Slice-4 generator inlines (after stripping
// the package clause + imports via parseIslandDef) so the component's
// //gosx:island definition can be compiled together with the generated Slide_N
// funcs in a single deck program.
func (d *IslandDeck) readComponentSource(name string) (string, error) {
	if d == nil {
		return "", fmt.Errorf("read component %q: nil deck", name)
	}
	path := filepath.Join(d.Dir, name+".gsx")
	source, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read component %s: %w", path, err)
	}
	return string(source), nil
}

// compileIslandComponent reads <dir>/<name>.gsx and runs the proven GoSX
// pipeline — Compile -> LowerIsland -> EncodeJSON — for the island matching
// name. It returns the lowered island program and its JSON wire bytes. This is
// the same pipeline `gosx dev` runs on every island edit, so the program here is
// byte-identical to the one the dev socket hot-swaps in.
func compileIslandComponent(dir, name string) (*program.Program, []byte, error) {
	path := filepath.Join(dir, name+".gsx")
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read component %s: %w", path, err)
	}

	irProg, err := gosx.Compile(source)
	if err != nil {
		return nil, nil, fmt.Errorf("compile %s: %w", path, err)
	}

	for i, comp := range irProg.Components {
		if !comp.IsIsland || comp.Name != name {
			continue
		}
		isl, err := ir.LowerIsland(irProg, i)
		if err != nil {
			return nil, nil, fmt.Errorf("lower island %s: %w", name, err)
		}
		data, err := program.EncodeJSON(isl)
		if err != nil {
			return nil, nil, fmt.Errorf("encode island %s: %w", name, err)
		}
		return isl, data, nil
	}

	return nil, nil, fmt.Errorf("island %q not found in %s", name, path)
}

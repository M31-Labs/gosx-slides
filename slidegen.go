package slides

// slidegen.go is the real lane's source generator (Phase 1, Slice 4). It is the
// heart of REAL `{expr}` evaluation: instead of rendering a slide's mdpp AST to
// gosx.Nodes by hand (render_island.go, where an inline {expr} could only be
// emitted as raw text), it LOWERS each slide to generated GoSX (.gsx) source —
// one `func Slide_N() Node { … }` per slide — and lets the gosx compiler +
// route.RenderProgramComponent actually evaluate the expressions server-side.
//
// The whole deck becomes a SINGLE compiled source: the referenced island
// component definitions (read from <Name>.gsx and merged) plus every Slide_N
// function. A single gosx.Compile over that source declares all of them, and
// cross-references (a Slide_N referencing <Counter/>) resolve at render time —
// the model proven by gosx's route.RenderProgramComponent test.
//
// Why source-gen and not hand-built nodes: the gosx compiler is the only thing
// that can evaluate `{2 + 3}` or `{strings.ToUpper("hi")}`. By turning a slide
// into source, inline expressions ride the real evaluator for free, while
// <Component/> tags resolve to hydrated islands via the same render call. The
// key safety trick is that PROSE is emitted as a Go string-literal EXPRESSION
// (`{"…"}`), never as raw source, so text containing `<`, `{`, `&`, or quotes
// can never corrupt the generated program — the compiler treats it as opaque
// string data and the renderer escapes it on output.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"m31labs.dev/mdpp"
)

// slideFuncName is the generated component function name for the slide at index
// i (e.g. 0 -> "Slide_0"). RenderProgramComponent renders the component by this
// name, so the generator and the renderer must agree on it.
func slideFuncName(i int) string {
	return "Slide_" + strconv.Itoa(i)
}

// generateDeckSource builds the single GoSX source string for the whole deck:
// `package main`, the merged island component definitions for every distinct
// referenced component that has a compilable <Name>.gsx, then one Slide_N
// function per slide. islandDefs maps a component name to the body of its .gsx
// (its `package` line already stripped); a name with no entry is still
// referenced in slide bodies but renders as a fail-soft empty/placeholder via
// the renderer's unresolved path.
//
// The returned source is what gosx.Compile consumes; compiling it once yields a
// program that declares all Slide_N funcs plus the island components they
// reference, with cross-references resolved at render time.
func generateDeckSource(deck *IslandDeck, islandDefs map[string]islandDef) string {
	var b strings.Builder
	b.WriteString("package main\n\n")

	// Merge + dedupe imports across all island defs into one block at the top,
	// then emit each def's body (imports already stripped).
	imports, bodies := mergeIslandDefs(islandDefs)
	if len(imports) > 0 {
		b.WriteString("import (\n")
		for _, imp := range imports {
			b.WriteString("\t")
			b.WriteString(imp)
			b.WriteString("\n")
		}
		b.WriteString(")\n\n")
	}
	for _, body := range bodies {
		b.WriteString(body)
		b.WriteString("\n\n")
	}

	layers := deckSlideLayers(deck)
	for _, slide := range deck.Slides {
		b.WriteString("func ")
		b.WriteString(slideFuncName(slide.Index))
		b.WriteString("() Node {\n\treturn ")
		b.WriteString(lowerSlideToGSX(slide, layers))
		b.WriteString("\n}\n\n")
	}

	return b.String()
}

// slideLayers carries the deck-level `header:` / `footer:` headmatter values
// rendered onto every slide (Slidev's global layers, minus the extra files):
// small persistent chrome — event name, speaker handle, a link — that themes
// style via .slide-header / .slide-footer. Either may be empty.
type slideLayers struct {
	Header string
	Footer string
}

// deckSlideLayers reads the deck's header:/footer: headmatter.
func deckSlideLayers(deck *IslandDeck) slideLayers {
	if deck == nil {
		return slideLayers{}
	}
	return slideLayers{
		Header: strings.TrimSpace(deckFrontmatterString(deck, "header")),
		Footer: strings.TrimSpace(deckFrontmatterString(deck, "footer")),
	}
}

// resolveSlideLayer applies a slide's own frontmatter to one deck-level layer
// value: `footer: false` (or none/off) hides it on that slide, any other
// non-empty value replaces it, absence inherits the deck value.
func resolveSlideLayer(slide IslandSlide, key, deckValue string) string {
	if slide.Node == nil {
		return deckValue
	}
	fm := parseFrontmatter(slide.Node.Attr("frontmatter"))
	v, present := fm[key]
	if !present {
		return deckValue
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "none", "off", "":
		return ""
	default:
		return v
	}
}

// islandDef is the parsed form of a component's <Name>.gsx: its import specs and
// the remaining body (everything that is not the `package` line or an import),
// so many island defs can be merged into one source with a single import block.
type islandDef struct {
	// imports are the individual import spec lines, e.g. `"strings"` or
	// `foo "example.com/foo"`. Empty for an import-free component (the common
	// case, e.g. the example Counter).
	imports []string
	// body is the component source with its `package` line and import block(s)
	// removed: comments + the `//gosx:island func Name() …` definition.
	body string
}

// parseIslandDef splits a component .gsx source into its imports and its body
// (package line + import declarations removed). It handles both grouped
// `import ( … )` blocks and single `import "…"` lines, and a source with no
// imports at all. The body is returned verbatim otherwise, so the component
// definition (including its //gosx:island directive and any leading doc
// comments) is preserved exactly for inlining into the deck source.
func parseIslandDef(source string) islandDef {
	lines := strings.Split(source, "\n")
	var imports []string
	var bodyLines []string

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Drop the package clause.
		if strings.HasPrefix(trimmed, "package ") {
			i++
			continue
		}

		// Grouped import block: import ( … ).
		if trimmed == "import (" || strings.HasPrefix(trimmed, "import (") {
			i++
			for i < len(lines) {
				inner := strings.TrimSpace(lines[i])
				if inner == ")" {
					i++
					break
				}
				if inner != "" {
					imports = append(imports, inner)
				}
				i++
			}
			continue
		}

		// Single-line import: import "pkg" or import alias "pkg".
		if strings.HasPrefix(trimmed, "import ") {
			imports = append(imports, strings.TrimSpace(strings.TrimPrefix(trimmed, "import")))
			i++
			continue
		}

		bodyLines = append(bodyLines, line)
		i++
	}

	return islandDef{
		imports: imports,
		body:    strings.TrimSpace(strings.Join(bodyLines, "\n")),
	}
}

// mergeIslandDefs combines the island defs into one deduped, sorted import list
// and the ordered list of component bodies (sorted by name for a stable source).
// Deduping imports by their exact spec keeps the merged block valid even when
// two components import the same package.
func mergeIslandDefs(defs map[string]islandDef) (imports []string, bodies []string) {
	seen := map[string]bool{}
	names := make([]string, 0, len(defs))
	for name := range defs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := defs[name]
		for _, imp := range def.imports {
			if imp == "" || seen[imp] {
				continue
			}
			seen[imp] = true
			imports = append(imports, imp)
		}
		if def.body != "" {
			bodies = append(bodies, def.body)
		}
	}
	sort.Strings(imports)
	return imports, bodies
}

// lowerSlideToGSX lowers one slide's mdpp subtree to a GoSX element expression:
// `<section class="slide layout-<name>" data-slide="N"> …children… </section>`.
// Each child node is lowered by lowerNodeToGSX. The `.slide`/data-slide shape
// matches the original hand-built lane (renderIslandSlide) so existing nav/lookup
// keyed on them keeps working; the extra `layout-<name>` class comes from the
// slide's `layout:` frontmatter (default | center | title — see layoutClass) so
// themes (themes.go) can style per-slide layouts. The class always carries
// exactly one well-formed layout token (unknown/absent -> layout-default).
//
// If the slide's per-slide frontmatter contains `reveal: true` (or `reveal: list`),
// each top-level list item (`<li>`) is tagged with a `data-fragment="K"` attribute
// (0-based, in document order). navScript then reveals them one per step as the
// presenter advances through the slide (see nav.go stepCountFor + show logic).
func lowerSlideToGSX(slide IslandSlide, layers slideLayers) string {
	var b strings.Builder
	b.WriteString(`<section class="slide `)
	b.WriteString(slideLayoutClass(slide))
	// Per-slide `class:` frontmatter: extra identity tokens ("dark",
	// "centered") the deck stylesheet keys on. Class-based styling is the
	// robust alternative to matching the serialized style attribute (which
	// nav's fit-scaler rewrites).
	for _, cls := range slideExtraClasses(slide) {
		b.WriteByte(' ')
		b.WriteString(cls)
	}
	fmt.Fprintf(&b, `" data-slide="%d"`, slide.Index)
	// Per-slide `transition:` frontmatter overrides the deck-level enter
	// animation for this one slide (fade | none; anything else stamps nothing).
	if tr := slideTransition(slide); tr != "" {
		fmt.Fprintf(&b, ` data-transition="%s"`, tr)
	}
	// Per-slide overrides: `background:` and `accent:` frontmatter set an inline
	// style (a background and/or an --accent token override that cascades to the
	// slide's content). Emitted as a quoted string-literal attribute expression so
	// an author value can never corrupt the generated source.
	if style := slideOverrideStyle(slide); style != "" {
		b.WriteString(" style={")
		b.WriteString(strconv.Quote(style))
		b.WriteString("}")
	}
	b.WriteString(">")
	if slide.Node != nil {
		// slideReveal determines whether this slide has fragment reveal enabled.
		// We thread a counter through the lowering so every top-level <li> in
		// the slide gets a unique 0-based data-fragment index.
		reveal := slideHasReveal(slide)
		fragIdx := 0
		for _, child := range slide.Node.Children {
			b.WriteString(lowerNodeToGSXReveal(child, reveal, &fragIdx))
		}
	}
	// Persistent layers: deck-level header:/footer: headmatter, per-slide
	// overridable (footer: false hides; any other value replaces). Content goes
	// through the sanitized raw-HTML lane so a layer can carry a link or a span
	// but never a script.
	if h := resolveSlideLayer(slide, "header", layers.Header); h != "" {
		b.WriteString(`<div class="slide-header">` + rawHTMLCallGSX(h) + `</div>`)
	}
	if f := resolveSlideLayer(slide, "footer", layers.Footer); f != "" {
		b.WriteString(`<div class="slide-footer">` + rawHTMLCallGSX(f) + `</div>`)
	}
	b.WriteString("</section>")
	return b.String()
}

// slideHasReveal reports whether the slide has `reveal: true` or `reveal: list`
// in its per-slide frontmatter. Parsed the same way `layout:` is parsed (see
// slideLayoutClass / parseFrontmatter). Unknown values are treated as false so
// authors can use `reveal: false` to explicitly opt out.
func slideHasReveal(slide IslandSlide) bool {
	if slide.Node == nil {
		return false
	}
	v := parseFrontmatter(slide.Node.Attr("frontmatter"))["reveal"]
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "true" || v == "list" || v == "1" || v == "yes"
}

// slideClassTokenRe is the shape a `class:` frontmatter token must have to land
// on the slide's <section>: a plain CSS class name (leading letter or
// underscore, then word characters and dashes). Anything else is dropped —
// injection is impossible regardless (the attribute is emitted as a quoted
// string), this just keeps the class list sane.
var slideClassTokenRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// slideExtraClasses returns the well-formed tokens of the slide's `class:`
// frontmatter, in authored order (empty when absent).
func slideExtraClasses(slide IslandSlide) []string {
	if slide.Node == nil {
		return nil
	}
	raw := parseFrontmatter(slide.Node.Attr("frontmatter"))["class"]
	var out []string
	for _, tok := range strings.Fields(raw) {
		if slideClassTokenRe.MatchString(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// slideTransition returns the slide's `transition:` frontmatter when it names
// a known transition (fade | none), "" otherwise. The value is stamped as
// data-transition on the section; nav's CSS keys the per-slide enter-animation
// override on it.
func slideTransition(slide IslandSlide) string {
	if slide.Node == nil {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(parseFrontmatter(slide.Node.Attr("frontmatter"))["transition"]))
	if v == "fade" || v == "none" {
		return v
	}
	return ""
}

// slideOverrideStyle builds the inline style for a slide's `background:` / `accent:`
// frontmatter (empty when neither is set). The values are CSS fragments placed in
// a style attribute (no selector context, so they cannot escape into arbitrary
// rules); strconv.Quote at the call site keeps them from breaking the source.
func slideOverrideStyle(slide IslandSlide) string {
	fm := slideFrontmatterValues(slide)
	var style strings.Builder
	if bg, ok := fm["background"].(string); ok {
		if bg = strings.TrimSpace(bg); bg != "" {
			style.WriteString("background:")
			style.WriteString(bg)
			style.WriteString(";")
		}
	}
	if accent, ok := fm["accent"].(string); ok {
		if accent = strings.TrimSpace(accent); accent != "" {
			style.WriteString("--accent:")
			style.WriteString(accent)
			style.WriteString(";")
		}
	}
	return style.String()
}

// slideLayoutClass returns the `layout-<name>` class for a slide, read from its
// `layout:` frontmatter (parsed the same way slideFrontmatterValues does) and
// normalized through layoutClass so an absent/unknown layout becomes
// "layout-default". Centralizing the read here keeps lowerSlideToGSX simple and
// guarantees the class agrees with the layouts every theme styles.
func slideLayoutClass(slide IslandSlide) string {
	layout := ""
	if slide.Node != nil {
		layout = parseFrontmatter(slide.Node.Attr("frontmatter"))["layout"]
	}
	return layoutClass(layout)
}

// lowerNodeToGSX lowers a single mdpp node to GoSX source text. The rules:
//
//   - NodeText        -> {<quoted>}  — prose as a Go string-literal EXPRESSION,
//     never raw, so `<`, `{`, `&`, quotes cannot corrupt the source.
//   - NodeExpression  -> {<literal>} — the expr source verbatim, so the gosx
//     compiler EVALUATES it ({2+3} -> 5, {strings.ToUpper("hi")} -> HI).
//   - NodeComponent   -> <Name propsraw/> — props carried through verbatim
//     (already name={…}/name="…" shaped) so the island hydrates.
//   - NodeHTMLBlock/Inline -> any embedded component tag(s) emitted as
//     <Name props/>; otherwise dropped (no raw HTML injection).
//   - structural prose nodes (heading, paragraph, emphasis, strong, code span,
//     link, list, blockquote) -> the matching HTML element wrapping lowered
//     children.
//   - unknown/other kinds -> {<quoted node text>} so nothing is silently
//     dropped and the source can never break.
func lowerNodeToGSX(n *mdpp.Node) string {
	if n == nil {
		return ""
	}
	switch n.Type {
	case mdpp.NodeText:
		// A text literal may embed a block-level component tag (the open tag
		// landed inside a text run). Split those out so the component still
		// mounts; surrounding prose stays quoted.
		return lowerTextLiteralToGSX(n.Literal)

	case mdpp.NodeExpression:
		// Verbatim expr source: this is what makes {expr} actually evaluate.
		return "{" + n.Literal + "}"

	case mdpp.NodeComponent:
		return componentTagGSX(n.Attr("name"), n.Attr("props"))

	case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline:
		// mdpp folds only INLINE uppercase tags into NodeComponent; a block-level
		// component arrives as a raw HTML literal. Emit any component tags it
		// holds; ordinary HTML PASSES THROUGH sanitized (html_raw.go) so authors
		// compose with real markup — div grids, spans, <br>, per-slide <style>.
		return lowerLiteralHTML(n.Literal)

	case mdpp.NodeHeading:
		level := n.Level()
		if level < 1 || level > 6 {
			level = 1
		}
		tag := "h" + strconv.Itoa(level)
		return wrapChildrenGSX(tag, n)

	case mdpp.NodeParagraph:
		// A paragraph whose only content is a block-level component renders bare
		// (no wrapping <p>), matching the hand-built lane.
		if isBlockComponentParagraph(n) {
			return lowerChildrenGSX(n)
		}
		return wrapChildrenGSX("p", n)

	case mdpp.NodeEmphasis:
		return wrapChildrenGSX("em", n)

	case mdpp.NodeStrong:
		return wrapChildrenGSX("strong", n)

	case mdpp.NodeStrikethrough:
		return wrapChildrenGSX("del", n)

	case mdpp.NodeCodeSpan:
		// Code span text is opaque: quote it as a string-literal expression.
		return "<code>{" + strconv.Quote(n.Text()) + "}</code>"

	case mdpp.NodeCodeBlock:
		// A fenced ```lang block. We CANNOT emit highlighted span-HTML as text
		// (prose is quoted with strconv.Quote and would escape the spans), so
		// instead we lower to a CALL to the bound codeNamespace.codeBlockFunc
		// (render_program.go) — `{__slidesCode.Block("<lang>", "<code>", "<hl>")}`.
		// The gosx compiler evaluates that call at render time and the function
		// returns a RawHTML <pre class="code-block">…tokens…</pre> Node that rides
		// the eval path unescaped (proven safe: the highlighter escapes the code
		// text). All three args are Go string literals, so a fence containing `<`,
		// `{`, `"` etc. can never corrupt the generated source. Language is mdpp's
		// Attrs["language"] (empty for a bare fence -> plain escaped text via
		// NormalizeLanguage); highlights is mdpp's Attrs["highlights"] — the fence's
		// `{2-3|6}` line-range meta (empty for a plain fence -> no emphasis, every
		// line full opacity). codeBlockNode parses the spec into ordered click STEPS:
		// it tags each emphasized line data-step="K" and records data-steps="N" on the
		// <pre>, so navScript can advance through the `|`-groups one ArrowRight at a
		// time (a single group like `{1-3}` is one step = the old static emphasis).
		lang := n.Attr("language")
		highlights := n.Attr("highlights")
		// Snippet import: a fence whose body is a `<<< ./path` directive pulls
		// the code from a file next to the deck at render time (Slidev's own
		// snippet syntax), so talks show real source that never drifts from the
		// repo. Optional trailing line window: `<<< ./x.go 10-20`. Lowered to
		// the bound File call (render_program.go) which sandboxes the path to
		// the deck directory.
		if path, window, ok := parseSnippetDirective(n.Literal); ok {
			return "{" + codeNamespace + "." + codeFileFunc + "(" +
				strconv.Quote(lang) + ", " + strconv.Quote(path) + ", " +
				strconv.Quote(window) + ", " + strconv.Quote(highlights) + ")}"
		}
		return "{" + codeNamespace + "." + codeBlockFunc + "(" +
			strconv.Quote(lang) + ", " + strconv.Quote(n.Literal) + ", " +
			strconv.Quote(highlights) + ")}"

	case mdpp.NodeLink:
		href := n.Attr("href")
		inner := lowerChildrenGSX(n)
		if inner == "" {
			inner = "{" + strconv.Quote(n.Text()) + "}"
		}
		return "<a href={" + strconv.Quote(href) + "}>" + inner + "</a>"

	case mdpp.NodeBlockquote:
		return wrapChildrenGSX("blockquote", n)

	case mdpp.NodeList:
		// ordered vs unordered: mdpp records an "ordered" attr when applicable.
		tag := "ul"
		if n.Attr("ordered") == "true" {
			tag = "ol"
		}
		return wrapChildrenGSX(tag, n)

	case mdpp.NodeListItem, mdpp.NodeTaskListItem:
		return wrapChildrenGSX("li", n)

	case mdpp.NodeImage:
		// `![alt](src)` — mdpp carries alt/src as attrs (no children). Emit an
		// <img> with both as string-literal attribute expressions so a src/alt
		// containing `<`, `{`, `"` can never corrupt the generated source. Themes
		// constrain it to the slide with object-fit (see themes_css.go).
		return "<img src={" + strconv.Quote(n.Attr("src")) + "} alt={" + strconv.Quote(n.Attr("alt")) + "}/>"

	case mdpp.NodeTable:
		return lowerTableGSX(n)

	case mdpp.NodeDiagram:
		// A sirena diagram fence: emit a call to the bound __slidesDiagram.Render
		// helper (render_program.go), which calls fence.Render server-side and returns
		// an inline-SVG <figure> node. All three args are Go string literals so `<`,
		// `{`, `"` etc. in the fence source can never corrupt the generated GoSX
		// source. n.Attr("theme") and n.Attr("view") are "" for a plain sirena fence
		// (the attrs live in the language info-string, not as parsed attrs on the node;
		// the helper passes "" to fence.Options which selects the default theme).
		return "{" + diagramNamespace + "." + diagramRenderFunc + "(" +
			strconv.Quote(n.Literal) + ", " + strconv.Quote(n.Attr("theme")) + ", " +
			strconv.Quote(n.Attr("view")) + ")}"

	case mdpp.NodeSoftBreak:
		return "{\" \"}"

	case mdpp.NodeHardBreak:
		return "<br/>"

	default:
		// Unmodeled node: if it has children, lower them so their prose/components
		// are not dropped; otherwise emit its plain text quoted (fail-soft, never
		// breaks the generated source).
		if len(n.Children) > 0 {
			return lowerChildrenGSX(n)
		}
		if t := n.Text(); t != "" {
			return "{" + strconv.Quote(t) + "}"
		}
		return ""
	}
}

// lowerNodeToGSXReveal is like lowerNodeToGSX but handles fragment-reveal markup.
// When reveal is true and n is a NodeList (top-level), its NodeListItem /
// NodeTaskListItem children each get a `data-fragment="K"` attribute (0-based,
// across the slide). The fragIdx pointer is shared across all nodes in the slide
// so the indices are globally monotone. Items nested inside a reveal item do NOT
// get additional data-fragment attributes; we pass reveal=false when descending
// into a list item's own children so only the top-level bullets are tagged.
func lowerNodeToGSXReveal(n *mdpp.Node, reveal bool, fragIdx *int) string {
	if n == nil {
		return ""
	}
	if !reveal {
		return lowerNodeToGSX(n)
	}
	// Only NodeList at the top-level of a reveal slide needs special treatment:
	// tag each of its direct NodeListItem / NodeTaskListItem children with
	// data-fragment="K". All other node types delegate to lowerNodeToGSX unchanged
	// (paragraphs, headings, code blocks, etc. are not affected by reveal).
	switch n.Type {
	case mdpp.NodeList:
		tag := "ul"
		if n.Attr("ordered") == "true" {
			tag = "ol"
		}
		var b strings.Builder
		b.WriteString("<" + tag + ">")
		for _, child := range n.Children {
			// Pass reveal=true so each immediate list item is tagged.
			b.WriteString(lowerNodeToGSXReveal(child, true, fragIdx))
		}
		b.WriteString("</" + tag + ">")
		return b.String()

	case mdpp.NodeListItem, mdpp.NodeTaskListItem:
		// Top-level list item in a reveal slide: emit with data-fragment="K".
		// Children of the item are lowered with reveal=false so nested sub-lists
		// are never tagged (only the top-level bullets carry data-fragment).
		k := *fragIdx
		*fragIdx++
		var b strings.Builder
		b.WriteString(`<li data-fragment="`)
		b.WriteString(strconv.Itoa(k))
		b.WriteString(`">`)
		for _, child := range n.Children {
			b.WriteString(lowerNodeToGSX(child)) // reveal=false for nested content
		}
		b.WriteString("</li>")
		return b.String()

	default:
		return lowerNodeToGSX(n)
	}
}

// wrapChildrenGSX lowers n's children and wraps them in <tag>…</tag>.
func wrapChildrenGSX(tag string, n *mdpp.Node) string {
	return "<" + tag + ">" + lowerChildrenGSX(n) + "</" + tag + ">"
}

// lowerTableGSX lowers a GFM table (NodeTable -> NodeTableRow -> NodeTableCell).
// mdpp builds the header row first and does not otherwise mark it, so the first
// row's cells become <th> and the rest <td>. Cells lower their inline children
// (so **bold**, `code`, links inside a cell render). Themes style the table.
func lowerTableGSX(n *mdpp.Node) string {
	var b strings.Builder
	b.WriteString("<table>")
	for ri, row := range n.Children {
		if row == nil || row.Type != mdpp.NodeTableRow {
			continue
		}
		cellTag := "td"
		if ri == 0 {
			cellTag = "th"
		}
		b.WriteString("<tr>")
		for _, cell := range row.Children {
			if cell == nil || cell.Type != mdpp.NodeTableCell {
				continue
			}
			b.WriteString("<" + cellTag + ">" + lowerChildrenGSX(cell) + "</" + cellTag + ">")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	return b.String()
}

// lowerChildrenGSX lowers all of n's children to GoSX source, concatenated.
func lowerChildrenGSX(n *mdpp.Node) string {
	var b strings.Builder
	for _, child := range n.Children {
		b.WriteString(lowerNodeToGSX(child))
	}
	return b.String()
}

// componentTagGSX builds a self-closing component tag `<Name props/>`. props is
// the raw mdpp-captured props string (already name={…}/name="…" shaped); it is
// carried through verbatim so the island receives exactly the authored props.
func componentTagGSX(name, props string) string {
	if name == "" {
		return ""
	}
	props = strings.TrimSpace(props)
	if props == "" {
		return "<" + name + "/>"
	}
	return "<" + name + " " + props + "/>"
}

// lowerTextLiteralToGSX lowers a raw text literal to GoSX source. If the literal
// embeds a block-level component tag (an open tag that landed inside a text run),
// the component is emitted as a tag and the surrounding text is quoted as a
// string-literal expression; otherwise the whole literal is one quoted
// expression. HTML comments are stripped first so a `<Tag/>` inside `<!-- … -->`
// is never mounted.
func lowerTextLiteralToGSX(literal string) string {
	if literal == "" {
		return ""
	}
	literal = stripHTMLComments(literal)
	if literal == "" {
		return ""
	}
	locs := blockComponentRe.FindAllStringSubmatchIndex(literal, -1)
	if len(locs) == 0 {
		return lowerProseSegment(literal)
	}
	var b strings.Builder
	pos := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > pos {
			b.WriteString(lowerProseSegment(literal[pos:start]))
		}
		name := literal[loc[2]:loc[3]]
		props := ""
		if loc[4] >= 0 {
			props = strings.TrimSpace(literal[loc[4]:loc[5]])
		}
		b.WriteString(componentTagGSX(name, props))
		pos = end
	}
	if pos < len(literal) {
		b.WriteString(lowerProseSegment(literal[pos:]))
	}
	return b.String()
}

// parseSnippetDirective recognizes a fence body that is a snippet import:
// its first non-blank line is `<<< <path>` with an optional `<lo>-<hi>` line
// window, and nothing else follows. Returns ok == false for a normal fence.
func parseSnippetDirective(body string) (path, window string, ok bool) {
	trimmed := strings.TrimSpace(body)
	if !strings.HasPrefix(trimmed, "<<<") || strings.ContainsRune(trimmed, '\n') {
		return "", "", false
	}
	fields := strings.Fields(strings.TrimPrefix(trimmed, "<<<"))
	switch len(fields) {
	case 1:
		return fields[0], "", true
	case 2:
		if _, _, rangeOK := parseLineRange(fields[1]); rangeOK {
			return fields[0], fields[1], true
		}
	}
	return "", "", false
}

// htmlTagRe matches one HTML tag-shaped run inside prose: an open, close, or
// self-closing lowercase-led tag. mdpp hands INLINE OPEN TAGS to the lowering
// inside NodeText literals (only the close arrives as NodeHTMLInline), so
// prose segments must route tag-shaped runs through the sanitizing raw lane —
// this is also CommonMark's own semantics for raw inline HTML. The leading
// [a-z] keeps uppercase component tags (handled by blockComponentRe first)
// and accidental `a < b` prose (no tag name) out.
var htmlTagRe = regexp.MustCompile(`</?[a-z][a-zA-Z0-9-]*(?:\s[^<>]*)?/?>`)

// lowerProseSegment lowers one non-component prose run: HTML tag-shaped
// substrings go through the sanitized raw lane (rawHTMLCallGSX), everything
// between stays a quoted string expression exactly as before. Prose with no
// tags takes the single-quoted fast path.
func lowerProseSegment(seg string) string {
	locs := htmlTagRe.FindAllStringIndex(seg, -1)
	if len(locs) == 0 {
		return quoteTextExpr(seg)
	}
	var b strings.Builder
	pos := 0
	for _, loc := range locs {
		if loc[0] > pos {
			b.WriteString(quoteTextExpr(seg[pos:loc[0]]))
		}
		b.WriteString(rawHTMLCallGSX(seg[loc[0]:loc[1]]))
		pos = loc[1]
	}
	if pos < len(seg) {
		b.WriteString(quoteTextExpr(seg[pos:]))
	}
	return b.String()
}

// lowerLiteralHTML lowers a raw HTML literal to GoSX source: component tags
// (<Name …/>) are emitted as component tags so islands still mount, and every
// non-component segment is emitted as a `{__slidesHTML.Raw("…")}` call so the
// markup renders after render-time sanitization (html_raw.go). The literal is
// carried as a quoted Go string, so no author markup can corrupt the generated
// source. Comments are stripped first (the trailing-comment speaker-note form
// is consumed upstream; the sanitizer would drop them anyway).
func lowerLiteralHTML(literal string) string {
	if literal == "" {
		return ""
	}
	literal = stripHTMLComments(literal)
	if strings.TrimSpace(literal) == "" {
		return ""
	}
	locs := blockComponentRe.FindAllStringSubmatchIndex(literal, -1)
	var b strings.Builder
	pos := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if seg := literal[pos:start]; strings.TrimSpace(seg) != "" {
			b.WriteString(rawHTMLCallGSX(seg))
		}
		name := literal[loc[2]:loc[3]]
		props := ""
		if loc[4] >= 0 {
			props = strings.TrimSpace(literal[loc[4]:loc[5]])
		}
		b.WriteString(componentTagGSX(name, props))
		pos = end
	}
	if seg := literal[pos:]; strings.TrimSpace(seg) != "" {
		b.WriteString(rawHTMLCallGSX(seg))
	}
	return b.String()
}

// rawHTMLCallGSX emits the bound sanitizing passthrough call for one raw HTML
// segment: `{__slidesHTML.Raw(<quoted segment>)}` (see render_program.go for
// the binding and html_raw.go for the sanitizer).
func rawHTMLCallGSX(seg string) string {
	return "{" + htmlNamespace + "." + htmlRawFunc + "(" + strconv.Quote(seg) + ")}"
}

// lowerLiteralComponents emits the component tags found in a raw HTML literal
// (block-level <Name/> tags mdpp left unfolded), dropping ordinary HTML and
// closing tags. Comments are stripped first.
func lowerLiteralComponents(literal string) string {
	if literal == "" {
		return ""
	}
	literal = stripHTMLComments(literal)
	var b strings.Builder
	for _, m := range blockComponentRe.FindAllStringSubmatch(literal, -1) {
		b.WriteString(componentTagGSX(m[1], strings.TrimSpace(m[2])))
	}
	return b.String()
}

// quoteTextExpr turns a prose segment into a Go string-literal EXPRESSION
// `{"…"}`. This is the safety trick: the segment becomes opaque string data
// the compiler never parses as markup, and the renderer escapes it on output.
//
// A whitespace-ONLY segment is collapsed to a single space rather than
// dropped: mdpp emits the joining space between adjacent inline nodes as its
// own text node, so dropping it fuses `**strong** *em*` into "strongem".
// Stray spaces at block level are inert (HTML whitespace processing ignores
// them, including between flex/grid children).
func quoteTextExpr(seg string) string {
	if seg == "" {
		return ""
	}
	if strings.TrimSpace(seg) == "" {
		return `{" "}`
	}
	return "{" + strconv.Quote(seg) + "}"
}

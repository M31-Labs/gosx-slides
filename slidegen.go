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
	"sort"
	"strconv"
	"strings"

	"m31labs.dev/mdpp"
)

// slideFuncName is the generated component function name for the slide at index
// i (e.g. 0 -> "Slide_0"). RenderProgramComponent renders the component by this
// name, so the generator and the renderer must agree on it.
func slideFuncName(i int) string {
	return "Slide_" + itoa(i)
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

	for _, slide := range deck.Slides {
		b.WriteString("func ")
		b.WriteString(slideFuncName(slide.Index))
		b.WriteString("() Node {\n\treturn ")
		b.WriteString(lowerSlideToGSX(slide))
		b.WriteString("\n}\n\n")
	}

	return b.String()
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
func lowerSlideToGSX(slide IslandSlide) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="slide %s" data-slide="%d">`, slideLayoutClass(slide), slide.Index)
	if slide.Node != nil {
		for _, child := range slide.Node.Children {
			b.WriteString(lowerNodeToGSX(child))
		}
	}
	b.WriteString("</section>")
	return b.String()
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
		// holds; ordinary HTML / closing tags contribute nothing (no raw inject).
		return lowerLiteralComponents(n.Literal)

	case mdpp.NodeHeading:
		level := n.Level()
		if level < 1 || level > 6 {
			level = 1
		}
		tag := "h" + itoa(level)
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
		return quoteTextExpr(literal)
	}
	var b strings.Builder
	pos := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > pos {
			b.WriteString(quoteTextExpr(literal[pos:start]))
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
		b.WriteString(quoteTextExpr(literal[pos:]))
	}
	return b.String()
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
// `{"…"}`. Whitespace-only segments are dropped (they carry no prose and only
// add noise). This is the safety trick: the segment becomes opaque string data
// the compiler never parses as markup, and the renderer escapes it on output.
func quoteTextExpr(seg string) string {
	if strings.TrimSpace(seg) == "" {
		return ""
	}
	return "{" + strconv.Quote(seg) + "}"
}

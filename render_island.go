package slides

import (
	"strconv"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island/program"
	"m31labs.dev/mdpp"
)

// render_island.go is the hand-built node lowering and COMPILE-FAILURE SAFETY
// NET: it turns a parsed deck slide into a live gosx.Node tree without the
// source-gen + compile path. Prose nodes (headings, paragraphs, text) lower to
// STATIC server HTML (zero client cost) while component references render through
// the island renderer so they hydrate into real interactive GoSX islands.
//
// The production path is the source-gen lane (render_program.go), which compiles
// the whole deck once so inline {expr} EVALUATES. When that compile fails, slides
// fall back to this hand-built lowering (prose + islands still render; {expr}
// degrades to raw text) so a transient bad deck never blanks the page.

// islandMounter is the minimal island-rendering surface the lowering lanes
// need: register a compiled island program and return its server-rendered,
// hydratable shell. Both *island.Renderer (via RenderIslandFromProgram) and the
// page-runtime adapter (serve.go's runtimeMounter, which delegates to
// server.PageRuntime.Island) satisfy it, so the same lowering code drives the
// standalone unit lane and the live deck server without depending on a concrete
// renderer.
type islandMounter interface {
	RenderIslandFromProgram(prog *program.Program, props any) gosx.Node
}

// compiledComponent is a once-compiled island program cached by component name.
// CompileComponent recompiles on every call, so the server compiles each
// distinct component a single time and shares the result across every slide that
// mounts it.
type compiledComponent struct {
	prog *program.Program
	// json is the JSON wire form served at /gosx/islands/<Name>.json. It is the
	// byte-identical program the dev socket hot-swaps.
	json []byte
}

// renderIslandSlide lowers one slide to a gosx.Node. components maps a component
// name to its compiled program; a component with no entry (or a nil map) renders
// as an inert placeholder so an unresolved reference never panics the page.
func renderIslandSlide(r islandMounter, slide IslandSlide, components map[string]*compiledComponent, diagramTheme string) gosx.Node {
	var children []gosx.Node
	if slide.Node != nil {
		for _, child := range slide.Node.Children {
			children = append(children, lowerNode(r, child, components, diagramTheme)...)
		}
	}
	return gosx.El("section",
		gosx.Attrs(
			gosx.Attr("class", "slide"),
			gosx.Attr("data-slide", slide.Index),
		),
		gosx.Fragment(children...),
	)
}

// lowerNode lowers a single mdpp node to zero or more gosx nodes. Block-level
// literals that are really a component tag are rendered as islands; ordinary
// prose lowers to static HTML elements. diagramTheme is the deck's resolved
// sirena theme, used as the default when a sirena fence does not name its own.
func lowerNode(r islandMounter, n *mdpp.Node, components map[string]*compiledComponent, diagramTheme string) []gosx.Node {
	if n == nil {
		return nil
	}
	switch n.Type {
	case mdpp.NodeComponent:
		// Folded inline component (Slice-1 path).
		return []gosx.Node{renderComponentRef(r, ComponentRef{
			Name:  n.Attr("name"),
			Props: n.Attr("props"),
		}, components)}

	case mdpp.NodeHeading:
		level := n.Level()
		if level < 1 || level > 6 {
			level = 1
		}
		return []gosx.Node{gosx.El("h"+strconv.Itoa(level), gosx.Text(n.Text()))}

	case mdpp.NodeParagraph:
		inline := lowerInline(r, n, components)
		// A paragraph that is nothing but a block-level component (the common
		// "<Counter/> on its own line" shape, where the open tag landed in a
		// paragraph's text) renders as a bare island, not wrapped in <p>.
		if len(inline) == 1 && isBlockComponentParagraph(n) {
			return inline
		}
		return []gosx.Node{gosx.El("p", gosx.Fragment(inline...))}

	case mdpp.NodeText:
		return lowerTextLiteral(r, n.Literal, components)

	case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline:
		// Block-level component tags arrive as raw HTML literals (mdpp folds only
		// inline tags). If the literal is a component, mount the island; closing
		// tags and ordinary HTML pass through (closing tags contribute nothing).
		refs := scanLiteralComponents(n.Literal)
		if len(refs) > 0 {
			var out []gosx.Node
			for _, ref := range refs {
				out = append(out, renderComponentRef(r, ref, components))
			}
			return out
		}
		return nil

	case mdpp.NodeDiagram:
		// Render the sirena fence server-side to inline SVG via renderSirenaDiagram.
		// This is the hand-built fallback lane (compile failed); the source-gen lane
		// uses __slidesDiagram.Render via the render_program.go binding. A fence may
		// name its own theme; otherwise fall back to the deck's resolved theme.
		theme := n.Attr("theme")
		if theme == "" {
			theme = diagramTheme
		}
		return []gosx.Node{renderSirenaDiagram(n.Literal, theme, n.Attr("view"), "")}

	case mdpp.NodeExpression:
		// DEGRADE PATH ONLY: this hand-built lowering renders the expression's
		// source as raw text. The production source-gen lane (render_program.go)
		// EVALUATES {expr}; this path is reached only when that lane's compile fails,
		// as a degrade so the page still serves.
		return []gosx.Node{gosx.Text(n.Literal)}

	case mdpp.NodeImage:
		return []gosx.Node{gosx.El("img", gosx.Attrs(
			gosx.Attr("src", n.Attr("src")),
			gosx.Attr("alt", n.Attr("alt")),
		))}

	case mdpp.NodeTable:
		return []gosx.Node{lowerTableNode(r, n, components)}

	default:
		// Lower any descendant prose/components of unmodeled container nodes so
		// nothing in the subtree is silently dropped.
		var out []gosx.Node
		for _, child := range n.Children {
			out = append(out, lowerNode(r, child, components, diagramTheme)...)
		}
		return out
	}
}

// lowerTableNode builds a <table> from a NodeTable (first row -> <th>, rest -> <td>),
// lowering each cell's inline children. The degrade-path counterpart to
// slidegen's lowerTableGSX.
func lowerTableNode(r islandMounter, n *mdpp.Node, components map[string]*compiledComponent) gosx.Node {
	var rows []gosx.Node
	for ri, row := range n.Children {
		if row == nil || row.Type != mdpp.NodeTableRow {
			continue
		}
		cellTag := "td"
		if ri == 0 {
			cellTag = "th"
		}
		var cells []gosx.Node
		for _, cell := range row.Children {
			if cell == nil || cell.Type != mdpp.NodeTableCell {
				continue
			}
			cells = append(cells, gosx.El(cellTag, gosx.Fragment(lowerInline(r, cell, components)...)))
		}
		rows = append(rows, gosx.El("tr", gosx.Fragment(cells...)))
	}
	return gosx.El("table", gosx.Fragment(rows...))
}

// lowerInline lowers the inline children of a prose container (paragraph) to
// gosx nodes: text segments (which may themselves embed a block-level component
// tag) and folded inline components.
func lowerInline(r islandMounter, parent *mdpp.Node, components map[string]*compiledComponent) []gosx.Node {
	var out []gosx.Node
	for _, child := range parent.Children {
		switch child.Type {
		case mdpp.NodeComponent:
			out = append(out, renderComponentRef(r, ComponentRef{
				Name:  child.Attr("name"),
				Props: child.Attr("props"),
			}, components))
		case mdpp.NodeText:
			out = append(out, lowerTextLiteral(r, child.Literal, components)...)
		case mdpp.NodeExpression:
			// Degrade-path raw text; the source-gen lane evaluates {expr} (see the
			// NodeExpression case in lowerNode).
			out = append(out, gosx.Text(child.Literal))
		case mdpp.NodeHTMLInline, mdpp.NodeHTMLBlock:
			for _, ref := range scanLiteralComponents(child.Literal) {
				out = append(out, renderComponentRef(r, ref, components))
			}
		default:
			// Strong/emphasis/links/etc: lower as their plain text for Slice 2.
			if t := child.Text(); t != "" {
				out = append(out, gosx.Text(t))
			}
		}
	}
	return out
}

// lowerTextLiteral lowers a raw text literal. If the literal embeds a component
// tag (a block-level component whose opening tag landed inside a text run), the
// component is mounted as an island and the surrounding text is kept as text;
// otherwise the whole literal is a single text node.
func lowerTextLiteral(r islandMounter, literal string, components map[string]*compiledComponent) []gosx.Node {
	if literal == "" {
		return nil
	}
	// Strip HTML comments so a `<Tag/>` inside `<!-- ... -->` is not mounted as an
	// island at render time (it was already excluded from ref collection).
	literal = stripHTMLComments(literal)
	locs := blockComponentRe.FindAllStringSubmatchIndex(literal, -1)
	if len(locs) == 0 {
		return []gosx.Node{gosx.Text(literal)}
	}
	var out []gosx.Node
	pos := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > pos {
			if seg := literal[pos:start]; strings.TrimSpace(seg) != "" {
				out = append(out, gosx.Text(seg))
			}
		}
		name := literal[loc[2]:loc[3]]
		props := ""
		if loc[4] >= 0 {
			props = strings.TrimSpace(literal[loc[4]:loc[5]])
		}
		out = append(out, renderComponentRef(r, ComponentRef{Name: name, Props: props}, components))
		pos = end
	}
	if pos < len(literal) {
		if seg := literal[pos:]; strings.TrimSpace(seg) != "" {
			out = append(out, gosx.Text(seg))
		}
	}
	return out
}

// renderComponentRef mounts a component reference as a live island, lowering its
// props. An unresolved component (not compiled / nil map) renders as an inert
// span so the page degrades instead of panicking.
func renderComponentRef(r islandMounter, ref ComponentRef, components map[string]*compiledComponent) gosx.Node {
	cc := components[ref.Name]
	if cc == nil || cc.prog == nil {
		return gosx.El("span",
			gosx.Attrs(
				gosx.Attr("data-gosx-unresolved", ref.Name),
				// A class as well as the attribute: the dev-mode loud-placeholder CSS
				// targets this class so its selector never contains the literal
				// "data-gosx-unresolved" string (which tests grep for to detect a
				// real placeholder).
				gosx.Attr("class", "gosx-unresolved"),
			),
			gosx.Text("["+ref.Name+"]"),
		)
	}
	return r.RenderIslandFromProgram(cc.prog, parseProps(ref.Props))
}

// isBlockComponentParagraph reports whether a paragraph's only meaningful
// content is a single block-level component tag (so it should render bare,
// without a wrapping <p>). It is true when the paragraph's collected text, with
// any component tags removed, is empty.
func isBlockComponentParagraph(p *mdpp.Node) bool {
	refs := collectComponentRefs(p)
	if len(refs) != 1 {
		return false
	}
	stripped := blockComponentRe.ReplaceAllString(p.Text(), "")
	return strings.TrimSpace(stripped) == ""
}

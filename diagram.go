package slides

// diagram.go provides sirena diagram rendering support: helpers to detect
// whether a deck contains any diagram nodes, the CSS that sizes and centres
// them, and the server-side SVG renderer that turns a sirena fence body into
// an inline SVG <figure> with no JavaScript and no CDN dependencies.
//
// NodeDiagram is produced by mdpp's codeBlockToDiagram whenever a fenced block
// carries a recognised diagram language (sirena/sir). Because rendering is
// server-side (fence.Render calls the layout engine and emits SVG bytes), the
// inline SVG is present in the initial HTML and works with display:none slides.

import (
	"m31labs.dev/gosx"
	"m31labs.dev/mdpp"
	"m31labs.dev/sirena/fence"
	_ "m31labs.dev/sirena/layout" // registers the layout engine into sirena.Render
)

// renderSirenaDiagram renders a sirena diagram source to an inline SVG <figure>
// via fence.Render (pure Go, server-side, no JavaScript). On success with
// non-empty SVG it returns a <figure class="mdpp-diagram mdpp-diagram-sirena">
// wrapping the inline SVG. On error or empty SVG it returns a visible degrade
// node so a broken fence never produces a blank slide.
func renderSirenaDiagram(source, theme, view, workspaceRoot string) gosx.Node {
	res, err := fence.Render([]byte(source), fence.Options{
		Theme:         theme,
		ViewRef:       view,
		WorkspaceRoot: workspaceRoot,
	})
	if err != nil {
		return gosx.El("pre",
			gosx.Attrs(gosx.Attr("class", "diagram-error")),
			gosx.Text("diagram error: "+err.Error()),
		)
	}
	if len(res.SVG) == 0 {
		msg := "diagram error: empty output"
		if len(res.Diagnostics) > 0 {
			msg = "diagram error: " + res.Diagnostics[0].Message
		}
		return gosx.El("pre",
			gosx.Attrs(gosx.Attr("class", "diagram-error")),
			gosx.Text(msg),
		)
	}
	return gosx.El("figure",
		gosx.Attrs(
			gosx.Attr("class", "mdpp-diagram mdpp-diagram-sirena"),
			gosx.Attr("data-diagram-syntax", "sirena"),
		),
		gosx.RawHTML(string(res.SVG)),
	)
}

// deckHasDiagram reports whether any slide in d contains at least one
// NodeDiagram node. It is used to gate the diagram CSS injection so
// non-diagram decks never include unnecessary styles.
func deckHasDiagram(d *IslandDeck) bool {
	for _, slide := range d.Slides {
		if slide.Node == nil {
			continue
		}
		if len(slide.Node.Find(mdpp.NodeDiagram)) > 0 {
			return true
		}
	}
	return false
}

// baseDiagramStyle returns the CSS that presents sirena diagrams. Sirena's only
// theme today (earth-default) paints a light canvas (--sirena-bg: #f5f1e8) with
// dark glyph labels, so on a dark deck the diagram reads best as a deliberate
// light "card": a fit-content panel whose padding matches the SVG's own bg
// (#f5f1e8 — seamless), centred, lifted off the deck with a soft shadow, and
// height-capped so it never overflows the locked viewport. The selectors are
// scoped under main.deck so they never leak outside the deck.
func baseDiagramStyle() string {
	return `main.deck .mdpp-diagram { width: fit-content; max-width: 100%; margin: var(--sp-3, 1rem) auto; padding: var(--sp-4, 1.25rem); background: #f5f1e8; border-radius: var(--radius, 12px); box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35); }
main.deck .mdpp-diagram svg { display: block; max-width: 100%; max-height: 54vh; height: auto; }
main.deck pre.diagram-error { color: #ff6b6b; font: 600 0.9rem var(--font-mono, ui-monospace, monospace); }`
}

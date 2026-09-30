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
	"strings"
)

// renderSirenaDiagram renders a sirena diagram source to an inline SVG <figure>
// via fence.Render (pure Go, server-side, no JavaScript). On success with
// non-empty SVG it returns a <figure class="mdpp-diagram mdpp-diagram-sirena">
// wrapping the inline SVG. On error or empty SVG it returns a visible degrade
// node so a broken fence never produces a blank slide.
func renderSirenaDiagram(source, theme, view, workspaceRoot string, diagram ...string) gosx.Node {
	kind := ""
	if len(diagram) > 0 {
		kind = diagram[0]
	}
	res, err := fence.Render([]byte(source), fence.Options{
		Diagram:       kind,
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

// sirenaThemeForDeck maps a deck's visual theme to the sirena diagram theme that
// matches it. Dark decks (aurora, neon) get sirena's "midnight" so a diagram
// reads as part of the page rather than a bright card; light decks (paper, swiss)
// use sirena's own default (returned as "" so it always tracks DefaultThemeName).
// A fence that names its own theme overrides this (see slidesDiagram.Render).
func sirenaThemeForDeck(d *IslandDeck) string {
	switch themeName(deckTheme(d)) {
	case "aurora", "neon":
		return "midnight"
	default:
		return ""
	}
}

// baseDiagramStyle returns the CSS that presents sirena diagrams. The SVG paints
// its OWN canvas (svg { background: var(--sirena-bg) }) — light for earth-default,
// dark for midnight — so the frame is theme-agnostic: a fit-content panel, clipped
// to a rounded corner (overflow:hidden, so the SVG's own bg becomes the card),
// lifted with a soft shadow and a hairline border, height-capped so it never
// overflows the locked viewport. No background of its own, so a dark diagram on a
// dark deck blends and a light diagram on a light deck reads as a clean card.
// The selectors are scoped under main.deck so they never leak outside the deck.
func baseDiagramStyle() string {
	return `main.deck .mdpp-diagram { width: 100%; max-width: 100%; margin: var(--sp-3, 1rem) auto; border-radius: var(--radius, 12px); overflow: hidden; box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3); border: 1px solid var(--line, rgba(128, 128, 128, 0.25)); }
main.deck .mdpp-diagram svg { display: block; width: 100%; max-width: 100%; max-height: 56vh; height: auto; }
main.deck pre.diagram-error { color: #ff6b6b; font: 600 0.9rem var(--font-mono, ui-monospace, monospace); }`
}

// mdpp's fast and grammar paths retain different amounts of fence info. Read
// each already-parsed diagram's own opening line, never scan prose/code bodies.
func retainDiagramFenceOptions(doc *mdpp.Document) {
	for _, node := range doc.AST().Find(mdpp.NodeDiagram) {
		if node.Attr("syntax") != "sirena" {
			continue
		}
		start := node.Range.StartByte
		if start < 0 || start >= len(doc.Source) {
			continue
		}
		opening := strings.SplitN(string(doc.Source[start:]), "\n", 2)[0]
		info := strings.Fields(strings.TrimLeft(strings.TrimSpace(opening), "`~"))
		if len(info) < 2 {
			continue
		}
		for _, field := range info[1:] {
			field = strings.Trim(field, "{}")
			key, value, has := strings.Cut(field, "=")
			if !has {
				key, value = "diagram", field
			}
			value = strings.Trim(value, "\"'")
			if key == "diagram" || key == "theme" {
				node.Attrs[key] = value
			}
		}
	}
}

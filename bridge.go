package slides

// bridge.go is the entry point of gosx-slides' REAL rendering lane (Phase 1).
//
// Where the fallback lane (deck.go / parse.go / render.go) turns Markdown into
// HTML strings with zero dependencies, the real lane composes two already-shipped
// seams to turn a deck's inline <Component/> into a genuine GoSX island program
// (bytecode):
//
//   - mdpp parses the deck's Markdown and (opt-in) splits it into slides, with
//     uppercase inline tags reinterpreted as mdpp.NodeComponent leaves.
//   - gosx compiles the referenced .gsx component and lowers the island to an
//     island/program.Program, the same wire form `gosx dev` hot-swaps.
//
// Slice 1 is the lowering CORE only: load a deck, find its component references,
// and compile a referenced component to island bytecode. It deliberately does
// NOT generate .gsx for prose/static nodes, lower props into the island, serve
// HTTP, or hydrate in the browser — those are later slices. It also leaves the
// fallback lane completely untouched (this file is the only consumer of the new
// gosx/mdpp dependencies).

import (
	"fmt"
	"os"
	"path/filepath"

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

	return deck, nil
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

// collectComponentRefs walks a slide subtree in document order and returns every
// inline component reference (mdpp.NodeComponent) it finds.
func collectComponentRefs(slide *mdpp.Node) []ComponentRef {
	var refs []ComponentRef
	slide.Walk(func(n *mdpp.Node) bool {
		if n.Type == mdpp.NodeComponent {
			refs = append(refs, ComponentRef{
				Name:  n.Attr("name"),
				Props: n.Attr("props"),
			})
		}
		return true
	})
	return refs
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

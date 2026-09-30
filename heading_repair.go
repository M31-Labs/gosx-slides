package slides

import (
	"m31labs.dev/mdpp"
	"strings"
)

// mdpp v0.4.8's slow-path heading protection can shift repair ordinals when a
// document mixes raw HTML blocks and inline HTML in ATX headings. Its source
// ranges remain correct. Reparse only mismatched headings from those ranges;
// matching headings retain their document-scoped references and attributes.
func repairDeckHeadings(doc *mdpp.Document) {
	for _, heading := range doc.AST().Find(mdpp.NodeHeading) {
		span := heading.Range
		if span.StartByte < 0 || span.EndByte > len(doc.Source) || span.StartByte >= span.EndByte {
			continue
		}
		source := doc.Source[span.StartByte:span.EndByte]
		if !strings.HasPrefix(strings.TrimLeft(string(source), " \t"), "#") {
			continue
		}
		parsed, err := mdpp.Parse(source)
		if err != nil {
			continue
		}
		headings := parsed.AST().Find(mdpp.NodeHeading)
		if len(headings) != 1 || headings[0].Text() == heading.Text() {
			continue
		}
		heading.Children = headings[0].Children
		for _, child := range heading.Children {
			child.Walk(func(n *mdpp.Node) bool {
				n.Range.StartByte += span.StartByte
				n.Range.EndByte += span.StartByte
				n.Range.StartLine += span.StartLine - 1
				n.Range.EndLine += span.StartLine - 1
				return true
			})
		}
	}
}

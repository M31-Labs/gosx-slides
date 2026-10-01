package slides

import (
	"m31labs.dev/mdpp"
	"strconv"
	"strings"
)

func lowerCodeMorphGSX(n *mdpp.Node) string {
	var body strings.Builder
	count := 0
	for _, child := range n.Children {
		if child.Type != mdpp.NodeCodeBlock {
			continue
		}
		count++
		body.WriteString("{" + codeNamespace + "." + codeBlockFunc + "(" + strconv.Quote(child.Attr("language")) + ", " + strconv.Quote(child.Literal) + ", \"all\")}")
	}
	return `<div class="slides-code-morph" data-steps="` + strconv.Itoa(max(0, count-1)) + `">` + body.String() + `</div>`
}

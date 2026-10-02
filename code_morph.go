package slides

import (
	"encoding/json"
	"m31labs.dev/mdpp"
	"strconv"
	"strings"
)

func codeMorphTiming(n *mdpp.Node) (int, string) {
	var attrs map[string]string
	_ = json.Unmarshal([]byte(n.Attr("attrs")), &attrs)
	duration, err := strconv.Atoi(attrs["duration"])
	if err != nil || duration < 1 || duration > 600000 {
		duration = 450
	}
	easing := attrs["easing"]
	switch easing {
	case "linear", "ease", "ease-in", "ease-out", "ease-in-out":
	default:
		easing = "cubic-bezier(.25,1,.5,1)"
	}
	return duration, easing
}

func lowerCodeMorphGSX(n *mdpp.Node) string {
	duration, easing := codeMorphTiming(n)
	var body strings.Builder
	count := 0
	for _, child := range n.Children {
		if child.Type != mdpp.NodeCodeBlock {
			continue
		}
		count++
		body.WriteString("{" + codeNamespace + "." + codeBlockFunc + "(" + strconv.Quote(child.Attr("language")) + ", " + strconv.Quote(child.Literal) + ", \"all\")}")
	}
	return `<div class="slides-code-morph" data-code-duration="` + strconv.Itoa(duration) + `" data-code-easing="` + easing + `" data-steps="` + strconv.Itoa(max(0, count-1)) + `">` + body.String() + `</div>`
}

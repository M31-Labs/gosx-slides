package slides

import (
	"encoding/json"
	"m31labs.dev/mdpp"
	"regexp"
	"strconv"
	"strings"
)

type motionSourceRange struct {
	Start   int               `json:"start"`
	End     int               `json:"end"`
	Opening string            `json:"opening"`
	Attrs   map[string]string `json:"attrs"`
}

// Byte offsets come from parsed directives. Never scan arbitrary fenced code or
// prose for openings; the browser checks the rendered source revision before saving.
func sourceMotionRanges(deck *IslandDeck) []motionSourceRange {
	var ranges []motionSourceRange
	for _, slide := range deck.Slides {
		if slide.Node == nil {
			continue
		}
		for _, node := range slide.Node.Find(mdpp.NodeContainerDirective) {
			if node.Attr("name") != "motion" {
				continue
			}
			start := node.Range.StartByte
			if start < 0 || start >= len(deck.Source) {
				continue
			}
			end := start
			for end < len(deck.Source) && deck.Source[end] != '\n' {
				end++
			}
			opening := string(deck.Source[start:end])
			if !strings.HasPrefix(strings.TrimSpace(opening), ":::motion") {
				continue
			}
			ranges = append(ranges, motionSourceRange{Start: start, End: end, Opening: opening, Attrs: motionDirectiveAttrs(node)})
		}
	}
	return ranges
}

var sourceDirectiveAttribute = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_-]*)\s*=\s*("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s}]+)`)

// Recover quoted timing values from each parsed opening, preserving spaces in
// cubic-bezier/steps expressions that mdpp's info tokenization can truncate.
func retainMotionFenceOptions(doc *mdpp.Document) {
	for _, node := range doc.AST().Find(mdpp.NodeContainerDirective) {
		if node.Attr("name") != "motion" && node.Attr("name") != "diagram-morph" && node.Attr("name") != "code-morph" {
			continue
		}
		start := node.Range.StartByte
		if start < 0 || start >= len(doc.Source) {
			continue
		}
		opening := strings.SplitN(string(doc.Source[start:]), "\n", 2)[0]
		brace := strings.Index(opening, "{")
		if brace < 0 {
			continue
		}
		attrs := map[string]string{}
		json.Unmarshal([]byte(node.Attr("attrs")), &attrs)
		for _, match := range sourceDirectiveAttribute.FindAllStringSubmatch(opening[brace:], -1) {
			value := match[2]
			if strings.HasPrefix(value, `"`) {
				if decoded, err := strconv.Unquote(value); err == nil {
					value = decoded
				}
			} else if strings.HasPrefix(value, "'") {
				value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
			}
			attrs[match[1]] = value
		}
		data, _ := json.Marshal(attrs)
		node.Attrs["attrs"] = string(data)
	}
}

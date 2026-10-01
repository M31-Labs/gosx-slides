package slides

import (
	"encoding/json"
	"m31labs.dev/gosx"
	"m31labs.dev/mdpp"
	"m31labs.dev/sirena/fence"
	"strconv"
	"strings"
)

func diagramMorphArgs(n *mdpp.Node) (string, string, string, int, string) {
	var sources []string
	for _, child := range n.Children {
		if child.Type == mdpp.NodeDiagram && child.Attr("syntax") == "sirena" {
			sources = append(sources, child.Literal)
		}
	}
	attrs := map[string]string{}
	json.Unmarshal([]byte(n.Attr("attrs")), &attrs)
	duration := 700
	if v, err := strconv.Atoi(attrs["duration"]); err == nil && v >= 0 && v <= 10000 {
		duration = v
	}
	easing := attrs["easing"]
	if !transitionEasing.MatchString(easing) {
		easing = "ease-in-out"
	}
	data, _ := json.Marshal(sources)
	return string(data), attrs["diagram"], attrs["theme"], duration, easing
}
func lowerDiagramMorphGSX(n *mdpp.Node) string {
	source, kind, theme, duration, easing := diagramMorphArgs(n)
	return "{" + diagramNamespace + ".Morph(" + strconv.Quote(source) + ", " + strconv.Quote(kind) + ", " + strconv.Quote(theme) + ", " + strconv.Itoa(duration) + ", " + strconv.Quote(easing) + ")}"
}
func (d slidesDiagram) Morph(source, kind, theme string, duration int, easing string) gosx.Node {
	var states []string
	if err := json.Unmarshal([]byte(source), &states); err != nil {
		return diagramMorphError(err.Error())
	}
	var bodies [][]byte
	for _, s := range states {
		bodies = append(bodies, []byte(s))
	}
	if theme == "" {
		theme = d.deckTheme
	}
	frames, err := fence.Storyboard(bodies, fence.Options{Diagram: kind, Theme: theme, StrictBudget: true})
	if err != nil {
		return diagramMorphError(err.Error())
	}
	var children []gosx.Node
	for i, frame := range frames {
		children = append(children, gosx.El("figure", gosx.Attrs(gosx.Attr("class", "mdpp-diagram mdpp-diagram-sirena"), gosx.Attr("data-diagram-state", i)), gosx.RawHTML(strings.TrimPrefix(string(frame), `<?xml version="1.0" encoding="UTF-8"?>`))))
	}
	return gosx.El("div", gosx.Attrs(gosx.Attr("class", "slides-diagram-morph"), gosx.Attr("data-steps", len(frames)-1), gosx.Attr("data-diagram-duration", duration), gosx.Attr("data-diagram-easing", easing)), gosx.Fragment(children...))
}
func diagramMorphError(message string) gosx.Node {
	return gosx.El("pre", gosx.Attrs(gosx.Attr("class", "diagram-error")), gosx.Text("diagram story error: "+message))
}

func deckDiagramMotionScript(deck *IslandDeck) string {
	for _, slide := range deck.Slides {
		if slide.Node == nil {
			continue
		}
		for _, node := range slide.Node.Find(mdpp.NodeContainerDirective) {
			if node.Attr("name") == "diagram-morph" {
				return diagramMotionScript
			}
		}
	}
	return ""
}

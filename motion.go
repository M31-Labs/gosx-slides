package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

// Markdown containers use the same managed Motion primitive as GoSX components.
// Keep author properties declarative; do not introduce another animation loop.
func motionDirectiveAttrs(n *mdpp.Node) map[string]string {
	attrs := map[string]string{}
	_ = json.Unmarshal([]byte(n.Attr("attrs")), &attrs)
	if attrs["trigger"] == "" {
		attrs["trigger"] = "view"
	}
	switch attrs["replay"] {
	case "once", "step":
	default:
		attrs["replay"] = "slide"
	}
	return attrs
}

func deckHasManagedMotion(deck *IslandDeck) bool {
	for _, slide := range deck.Slides {
		if slide.Node == nil {
			continue
		}
		for _, node := range slide.Node.Find(mdpp.NodeContainerDirective) {
			if strings.EqualFold(node.Attr("name"), "motion") {
				return true
			}
		}
	}
	return false
}

func lowerMotionDirectiveGSX(n *mdpp.Node) string {
	attrs := motionDirectiveAttrs(n)
	var b strings.Builder
	b.WriteString("<Motion")
	b.WriteString(" data-slides-motion-replay=" + strconv.Quote(attrs["replay"]))
	for _, key := range []string{"preset", "trigger", "duration", "delay", "easing", "distance", "respect-reduced-motion"} {
		if value := attrs[key]; value != "" {
			prop := key
			if key == "respect-reduced-motion" {
				prop = "respectReducedMotion"
			}
			b.WriteString(" " + prop + "=" + strconv.Quote(value))
		}
	}
	for _, key := range []string{"class", "id"} {
		if value := n.Attr(key); value != "" {
			b.WriteString(" " + key + "=" + strconv.Quote(value))
		}
	}
	for _, key := range []string{"split", "stagger"} {
		if value := attrs[key]; value != "" {
			b.WriteString(" data-gosx-motion-" + key + "=" + strconv.Quote(value))
		}
	}
	b.WriteString(">" + lowerChildrenGSX(n) + "</Motion>")
	return b.String()
}

func lowerMotionDirectiveNode(n *mdpp.Node, children []gosx.Node) gosx.Node {
	attrs := motionDirectiveAttrs(n)
	duration, _ := strconv.Atoi(attrs["duration"])
	delay, _ := strconv.Atoi(attrs["delay"])
	distance, _ := strconv.ParseFloat(attrs["distance"], 64)
	props := server.MotionProps{
		Preset: server.MotionPreset(attrs["preset"]), Trigger: server.MotionTrigger(attrs["trigger"]),
		Duration: duration, Delay: delay, Distance: distance, Easing: attrs["easing"],
	}
	if value, err := strconv.ParseBool(attrs["respect-reduced-motion"]); err == nil {
		props.RespectReducedMotion = &value
	}
	extra := gosx.Attrs(gosx.Attr("class", n.Attr("class")), gosx.Attr("id", n.Attr("id")), gosx.Attr("data-slides-motion-replay", attrs["replay"]))
	for _, key := range []string{"split", "stagger"} {
		if value := attrs[key]; value != "" {
			extra = append(extra, gosx.Attr("data-gosx-motion-"+key, value))
		}
	}
	return server.Motion(props, extra, gosx.Fragment(children...))
}

var transitionEasing = regexp.MustCompile(`^(ease|ease-in|ease-out|ease-in-out|linear|step-start|step-end|cubic-bezier\([-+0-9., ]+\)|steps\([0-9]+(?:, *(?:start|end|jump-start|jump-end|jump-none|jump-both))?\))$`)

// CSS variables inherit deck timings while allowing individual slide overrides.
// Numeric times are milliseconds; ms/s suffixes are also accepted.
func transitionTimingStyle(values map[string]any) string {
	var style strings.Builder
	for _, key := range []string{"duration", "delay"} {
		value := strings.TrimSpace(fmt.Sprint(values["transition-"+key]))
		unit := 1.0
		if strings.HasSuffix(value, "ms") {
			value = strings.TrimSuffix(value, "ms")
		} else if strings.HasSuffix(value, "s") {
			value = strings.TrimSuffix(value, "s")
			unit = 1000
		}
		time, err := strconv.ParseFloat(value, 64)
		time *= unit
		if err == nil && !math.IsNaN(time) && !math.IsInf(time, 0) && time >= 0 && time <= 600_000 {
			style.WriteString("--slides-transition-" + key + ":" + strconv.FormatFloat(time, 'f', -1, 64) + "ms;")
		}
	}
	if easing := strings.TrimSpace(fmt.Sprint(values["transition-easing"])); len(easing) <= 128 && transitionEasing.MatchString(easing) {
		style.WriteString("--slides-transition-easing:" + easing + ";")
	}
	return style.String()
}

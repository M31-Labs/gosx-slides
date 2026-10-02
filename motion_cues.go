package slides

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"m31labs.dev/mdpp"
)

var cueNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// A slide ID and ordered cue names are stable public addresses. Index zero is
// the initial pose, shared with code, list, and Scene3D step navigation.
func slideCueNames(slide IslandSlide) []string {
	var cues []string
	value, _ := slideFrontmatterValues(slide)["cues"].(string)
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if cueNamePattern.MatchString(name) {
			cues = append(cues, name)
		}
	}
	cues = uniqueStrings(cues)
	next := max(0, len(cues)-1)
	if slide.Node != nil {
		for _, node := range slide.Node.Find(mdpp.NodeContainerDirective) {
			if node.Attr("name") != "motion" {
				continue
			}
			attrs := motionCueAttrs(node)
			name := attrs["data-slides-motion-cue"]
			if value, exists := attrs["data-slides-motion-step"]; exists {
				step, _ := strconv.Atoi(value)
				next = max(next, step)
				if name != "" {
					for len(cues) <= step {
						cues = append(cues, "")
					}
					for i, old := range cues {
						if old == name {
							cues[i] = ""
						}
					}
					cues[step] = name
				}
			} else if name != "" {
				found := -1
				for i, cue := range cues {
					if cue == name {
						found = i
						break
					}
				}
				if found >= 0 {
					next = max(next, found)
				} else {
					next++
					for len(cues) <= next {
						cues = append(cues, "")
					}
					cues[next] = name
				}
			}
		}
	}
	return cues
}

func slideIdentityAttrs(slide IslandSlide) map[string]string {
	attrs := map[string]string{}
	if id, _ := slideFrontmatterValues(slide)["id"].(string); cueNamePattern.MatchString(id) {
		attrs["data-slide-id"] = id
	}
	if n, err := strconv.Atoi(fmt.Sprint(slideFrontmatterValues(slide)["morph-duration"])); err == nil && n >= 0 && n <= 600000 {
		attrs["data-morph-duration"] = strconv.Itoa(n)
	}
	if n, err := strconv.Atoi(fmt.Sprint(slideFrontmatterValues(slide)["motion-duration"])); err == nil && n > 0 && n <= 600000 {
		attrs["data-motion-duration"] = strconv.Itoa(n)
	}
	if names := slideCueNames(slide); len(names) > 0 {
		data, _ := json.Marshal(names)
		attrs["data-slide-cues"] = string(data)
	}
	return attrs
}

func motionCueAttrs(n *mdpp.Node) map[string]string {
	attrs, out := motionDirectiveAttrs(n), map[string]string{}
	for _, key := range []string{"cue", "after", "group"} {
		if cueNamePattern.MatchString(attrs[key]) {
			out["data-slides-motion-"+key] = attrs[key]
		}
	}
	if step, err := strconv.Atoi(attrs["step"]); err == nil && step >= 0 && step <= 10000 {
		out["data-slides-motion-step"] = strconv.Itoa(step)
	}
	return out
}

func slideMotionClicks(slide IslandSlide) int {
	budget := max(0, len(slideCueNames(slide))-1)
	if slide.Node == nil {
		return budget
	}
	next, named := 0, map[string]int{}
	for i, name := range slideCueNames(slide) {
		named[name] = i
	}
	for _, node := range slide.Node.Find(mdpp.NodeContainerDirective) {
		if node.Attr("name") == "diagram-morph" {
			budget = max(budget, len(node.Find(mdpp.NodeDiagram))-1)
		}
		if node.Attr("name") == "code-morph" {
			budget = max(budget, len(node.Find(mdpp.NodeCodeBlock))-1)
		}
		if node.Attr("name") != "motion" {
			continue
		}
		attrs := motionCueAttrs(node)
		if value, exists := attrs["data-slides-motion-step"]; exists {
			step, _ := strconv.Atoi(value)
			next = max(next, step)
		} else if name := attrs["data-slides-motion-cue"]; name != "" {
			if step, exists := named[name]; exists {
				next = max(next, step)
			} else {
				next++
				named[name] = next
			}
		}
		budget = max(budget, next)
	}
	return budget
}

package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"m31labs.dev/mdpp"
)

var storySurfaceName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type storySurface struct {
	name, containerID string
	node              *mdpp.Node
}

// Containers give each surface an authored identity and a precise DOM boundary.
// The existing native container lowering preserves the internal id attribute.
func storySurfaces(slide IslandSlide) ([]storySurface, error) {
	var surfaces []storySurface
	seen := map[string]bool{}
	var walk func(*mdpp.Node, bool) error
	walk = func(node *mdpp.Node, inside bool) error {
		if node.Type == mdpp.NodeContainerDirective && node.Attr("name") == "story-surface" {
			if inside {
				return fmt.Errorf("story surfaces cannot be nested")
			}
			var attrs map[string]string
			if err := json.Unmarshal([]byte(node.Attr("attrs")), &attrs); err != nil {
				return fmt.Errorf("story-surface needs {name=surface}")
			}
			name := attrs["name"]
			if !storySurfaceName.MatchString(name) || seen[name] {
				return fmt.Errorf("invalid or duplicate story surface %q", name)
			}
			seen[name] = true
			id := fmt.Sprintf("slides-story-surface-%d-%s", slide.Index, name)
			if authored := node.Attr("id"); authored != "" {
				id = authored
			}
			surfaces = append(surfaces, storySurface{name, id, node})
			inside = true
		}
		for _, child := range node.Children {
			if err := walk(child, inside); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(slide.Node, false); err != nil {
		return nil, err
	}
	if len(surfaces) > 8 {
		return nil, fmt.Errorf("story supports at most 8 named surfaces per slide")
	}
	if len(surfaces) > 0 {
		// A named deck must account for every surface. An unwrapped diagram may
		// share actor IDs and must never accidentally receive a named pose.
		var outside func(*mdpp.Node) *mdpp.Node
		outside = func(node *mdpp.Node) *mdpp.Node {
			copy := *node
			copy.Children = nil
			if node.Type == mdpp.NodeContainerDirective && node.Attr("name") == "story-surface" {
				return &mdpp.Node{}
			}
			for _, child := range node.Children {
				copy.Children = append(copy.Children, outside(child))
			}
			return &copy
		}
		node := outside(slide.Node)
		if storySurfaceCount(node, collectComponentRefs(node)) != 0 {
			return nil, fmt.Errorf("every Sirena surface must have a named story-surface container")
		}
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].name < surfaces[j].name })
	return surfaces, nil
}

func storySurfaceCount(node *mdpp.Node, refs []ComponentRef) int {
	count := 0
	morph := map[*mdpp.Node]bool{}
	for _, container := range node.Find(mdpp.NodeContainerDirective) {
		if container.Attr("name") != "diagram-morph" {
			continue
		}
		found := false
		for _, diagram := range container.Find(mdpp.NodeDiagram) {
			morph[diagram] = true
			found = found || diagram.Attr("syntax") == "sirena"
		}
		if found {
			count++
		}
	}
	for _, diagram := range node.Find(mdpp.NodeDiagram) {
		if diagram.Attr("syntax") == "sirena" && !morph[diagram] {
			count++
		}
	}
	for _, ref := range refs {
		if ref.Name == "Scene3D" && strings.HasSuffix(strings.ToLower(graphicString(parseProps(ref.Props), "Src", "")), ".sir") {
			count++
		}
	}
	return count
}

func storyGraphStep(node *mdpp.Node, step int) int {
	for _, container := range node.Find(mdpp.NodeContainerDirective) {
		if container.Attr("name") == "diagram-morph" {
			return max(0, min(step, len(container.Find(mdpp.NodeDiagram))-1))
		}
	}
	return 0
}

func compileStoryBeatGraphs(deck *IslandDeck, story *CompiledStory, index, step int, beat StoryBeat) (string, map[string]string, error) {
	slide := deck.Slides[index]
	surfaces, err := storySurfaces(slide)
	if err != nil {
		return "", nil, err
	}
	if len(surfaces) == 0 {
		if len(beat.Surfaces) != 0 {
			return "", nil, fmt.Errorf("surface poses need named story-surface containers")
		}
		key := fmt.Sprintf("%d:%d", index, storyGraphStep(slide.Node, step))
		if _, ok := story.Graphs[key]; !ok {
			graph, err := storyGraph(deck, slide, storyGraphStep(slide.Node, step))
			if err != nil {
				return "", nil, err
			}
			story.Graphs[key] = graph
		}
		return key, nil, nil
	}
	if beat.Focus != nil || beat.Reveal != nil || beat.Trace != nil || beat.Camera != nil {
		return "", nil, fmt.Errorf("named surfaces require scoped poses in surfaces; actor expectations use surface/actor")
	}
	keys := map[string]string{}
	key := fmt.Sprintf("%d:named", index)
	aggregate := StoryGraph{SlideIndex: index, Actors: []StoryActor{}, Edges: [][2]string{}, TraceTargets: map[string]string{}}
	for _, surface := range surfaces {
		scoped := slide
		scoped.Node = surface.node
		scoped.Components = collectComponentRefs(surface.node)
		if storySurfaceCount(scoped.Node, scoped.Components) != 1 {
			return "", nil, fmt.Errorf("surface %s needs exactly one Sirena SVG/morph or Scene3D", surface.name)
		}
		graphKey := fmt.Sprintf("%d:%s:%d", index, surface.name, storyGraphStep(scoped.Node, step))
		graph, ok := story.Graphs[graphKey]
		if !ok {
			graph, err = storyGraph(deck, scoped, storyGraphStep(scoped.Node, step))
			if err != nil {
				return "", nil, fmt.Errorf("surface %s: %w", surface.name, err)
			}
			graph.Surface, graph.ContainerID = surface.name, surface.containerID
			story.Graphs[graphKey] = graph
		}
		keys[surface.name] = graphKey
		key += "/" + graphKey
		if pose, ok := beat.Surfaces[surface.name]; ok {
			if err := validateStorySurfacePose(graph, pose); err != nil {
				return "", nil, fmt.Errorf("surface %s: %w", surface.name, err)
			}
		}
		for _, actor := range graph.Actors {
			actor.ID = surface.name + "/" + actor.ID
			aggregate.Actors = append(aggregate.Actors, actor)
		}
	}
	for name := range beat.Surfaces {
		if _, ok := keys[name]; !ok {
			return "", nil, fmt.Errorf("unknown story surface %q", name)
		}
	}
	if _, exists := story.Graphs[key]; !exists {
		story.Graphs[key] = aggregate
	}
	return key, keys, nil
}

func validateStorySurfacePose(graph StoryGraph, pose StorySurfacePose) error {
	actors := map[string]bool{}
	for _, actor := range graph.Actors {
		actors[actor.ID] = true
	}
	for _, group := range [][]string{pose.Focus, pose.Reveal, pose.Trace} {
		for _, id := range group {
			if !actors[id] {
				return fmt.Errorf("unknown actor %q", id)
			}
		}
	}
	for i := 1; i < len(pose.Trace); i++ {
		from, to := pose.Trace[i-1], pose.Trace[i]
		if !storyHasEdge(graph, from, to) {
			return fmt.Errorf("trace has no relationship %s → %s", from, to)
		}
		if graph.TraceTargets[from+"->"+to] == "" {
			return fmt.Errorf("trace relationship is ambiguous: %s → %s", from, to)
		}
	}
	if pose.Camera != nil {
		if !graph.Scene {
			return fmt.Errorf("camera needs a Sirena Scene3D component")
		}
		for _, v := range []float64{pose.Camera.X, pose.Camera.Y, pose.Camera.Z, pose.Camera.FOV} {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 10000 {
				return fmt.Errorf("camera values must be finite and bounded")
			}
		}
		if pose.Camera.FOV <= 0 || pose.Camera.FOV >= 180 {
			return fmt.Errorf("camera fov must be between 0 and 180")
		}
	}
	return nil
}

func storyPoseForGraph(beat CompiledStoryBeat, graph StoryGraph) StorySurfacePose {
	if graph.Surface != "" {
		return beat.Surfaces[graph.Surface]
	}
	return StorySurfacePose{beat.Focus, beat.Reveal, beat.Trace, beat.Camera}
}

func storyActorVisible(story *CompiledStory, beat CompiledStoryBeat, id string) bool {
	if len(beat.SurfaceGraphKeys) == 0 {
		return beat.Reveal == nil || containsString(beat.Reveal, id)
	}
	name, actor, ok := strings.Cut(id, "/")
	if !ok {
		return false
	}
	pose := beat.Surfaces[name]
	return pose.Reveal == nil || containsString(pose.Reveal, actor)
}

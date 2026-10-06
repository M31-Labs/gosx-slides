package slides

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"m31labs.dev/gosx/scene"
	"m31labs.dev/mdpp"
	"m31labs.dev/sirena"
	"m31labs.dev/sirena/fence"
	sirenascene "m31labs.dev/sirena/render/scene3d"
)

// StoryBeat describes one absolute narrative pose, addressed by authored IDs.
// Missing effects restore baseline state; effects never accumulate on navigation.
type StoryBeat struct {
	Slide      string                      `json:"slide" yaml:"slide"`
	Cue        string                      `json:"cue" yaml:"cue"`
	Caption    string                      `json:"caption,omitempty" yaml:"caption,omitempty"`
	DurationMS int                         `json:"durationMs" yaml:"durationMs"`
	Focus      []string                    `json:"focus,omitempty" yaml:"focus,omitempty"`
	Reveal     []string                    `json:"reveal" yaml:"reveal,omitempty"`
	Trace      []string                    `json:"trace,omitempty" yaml:"trace,omitempty"`
	Camera     *StoryCamera                `json:"camera,omitempty" yaml:"camera,omitempty"`
	Code       *StoryCode                  `json:"code,omitempty" yaml:"code,omitempty"`
	Show       []string                    `json:"show,omitempty" yaml:"show,omitempty"`
	Hide       []string                    `json:"hide,omitempty" yaml:"hide,omitempty"`
	Expect     *StoryExpectation           `json:"expect,omitempty" yaml:"expect,omitempty"`
	Surfaces   map[string]StorySurfacePose `json:"surfaces,omitempty" yaml:"surfaces,omitempty"`
}

// StorySurfacePose is an absolute pose scoped to one named authored surface.
// Actor IDs remain local here; expectations qualify them as surface/actor.
type StorySurfacePose struct {
	Focus  []string     `json:"focus,omitempty" yaml:"focus,omitempty"`
	Reveal []string     `json:"reveal" yaml:"reveal,omitempty"`
	Trace  []string     `json:"trace,omitempty" yaml:"trace,omitempty"`
	Camera *StoryCamera `json:"camera,omitempty" yaml:"camera,omitempty"`
}
type StoryCamera struct {
	X   float64 `json:"x" yaml:"x"`
	Y   float64 `json:"y" yaml:"y"`
	Z   float64 `json:"z" yaml:"z"`
	FOV float64 `json:"fov" yaml:"fov"`
}
type StoryCode struct {
	Block int   `json:"block" yaml:"block"`
	Lines []int `json:"lines" yaml:"lines"`
}
type StoryExpectation struct {
	Visible []string          `json:"visible,omitempty" yaml:"visible,omitempty"`
	Hidden  []string          `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Labels  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}
type CompiledStoryBeat struct {
	StoryBeat
	SlideIndex       int               `json:"slideIndex"`
	Step             int               `json:"step"`
	GraphKey         string            `json:"graphKey"`
	SurfaceGraphKeys map[string]string `json:"surfaceGraphKeys,omitempty"`
	Source           SourceDiagnostic  `json:"source"`
}
type StoryActor struct {
	Name  string     `json:"name"`
	ID    string     `json:"id"`
	Label string     `json:"label"`
	File  string     `json:"file"`
	Range mdpp.Range `json:"range"`
}
type StoryGraph struct {
	TraceTargets map[string]string `json:"traceTargets,omitempty"`
	SlideIndex   int               `json:"slideIndex"`
	Actors       []StoryActor      `json:"actors"`
	Edges        [][2]string       `json:"edges"`
	Scene        bool              `json:"scene"`
	Source       string            `json:"source,omitempty"`
	Surface      string            `json:"surface,omitempty"`
	ContainerID  string            `json:"containerId,omitempty"`
}
type CompiledStory struct {
	Version int                   `json:"version"`
	File    string                `json:"file"`
	Beats   []CompiledStoryBeat   `json:"beats"`
	Graphs  map[string]StoryGraph `json:"graphs"`
}

// Private parsed-AST marker requests line wrappers without numeric click steps.
const storyCodeLines = "__slides_story_lines"

// CompileStory reads a bounded local YAML/JSON manifest and resolves every
// narrative address against mdpp slides and Sirena's parsed actor model.
func CompileStory(deck *IslandDeck) (*CompiledStory, error) {
	if deck == nil {
		return nil, fmt.Errorf("story requires a deck")
	}
	file := deckFrontmatterString(deck, "story")
	if file == "" {
		return nil, nil
	}
	path, err := safeAuthorPath(deck.Dir, file)
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
		return nil, fmt.Errorf("story: manifest must be a regular file below 1 MiB")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var input struct {
		Version int         `yaml:"version"`
		Beats   []StoryBeat `yaml:"beats"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	decoder.KnownFields(true)
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("%s: expected one story manifest", file)
	}
	if input.Version != 1 || len(input.Beats) == 0 || len(input.Beats) > 1000 {
		return nil, fmt.Errorf("%s: story needs version 1 and 1–1000 beats", file)
	}
	var tree yaml.Node
	if err := yaml.Unmarshal(source, &tree); err != nil {
		return nil, err
	}
	var nodes []*yaml.Node
	for i := 0; i+1 < len(tree.Content[0].Content); i += 2 {
		if tree.Content[0].Content[i].Value == "beats" {
			nodes = tree.Content[0].Content[i+1].Content
		}
	}
	story := &CompiledStory{Version: 1, File: file, Beats: []CompiledStoryBeat{}, Graphs: map[string]StoryGraph{}}
	ids := map[string]int{}
	for i, s := range deck.Slides {
		id, _ := slideFrontmatterValues(s)["id"].(string)
		if id != "" {
			if _, duplicate := ids[id]; duplicate {
				return nil, fmt.Errorf("story: ambiguous slide ID %q", id)
			}
			ids[id] = i
		}
	}
	seen := map[string]bool{}
	for i, beat := range input.Beats {
		if deck.storyExcludedSlides[beat.Slide] {
			continue
		}
		start := yamlSourceOffset(source, nodes[i].Line, nodes[i].Column)
		end := len(source)
		if i+1 < len(nodes) {
			end = yamlSourceOffset(source, nodes[i+1].Line, nodes[i+1].Column)
		}
		rangeValue := mdpp.Range{StartByte: start, EndByte: end}
		rangeValue.StartLine, rangeValue.StartCol = sourcePosition(source, start)
		rangeValue.EndLine, rangeValue.EndCol = sourcePosition(source, end)
		fail := func(message string) (*CompiledStory, error) {
			return nil, fmt.Errorf("%s:%d:%d: %s", file, rangeValue.StartLine, rangeValue.StartCol, message)
		}
		slideIndex, exists := ids[beat.Slide]
		if !exists {
			return fail("unknown slide " + strconv.Quote(beat.Slide))
		}
		step := -1
		for j, name := range slideCueNames(deck.Slides[slideIndex]) {
			if name == beat.Cue {
				step = j
				break
			}
		}
		if step < 0 {
			return fail("unknown cue " + strconv.Quote(beat.Cue) + " on slide " + beat.Slide)
		}
		key := beat.Slide + "/" + beat.Cue
		if seen[key] {
			return fail("duplicate story beat " + key)
		}
		seen[key] = true
		if beat.DurationMS < 0 || beat.DurationMS > 600000 || len(beat.Caption) > 4096 {
			return fail("durationMs must be 0–600000 and caption at most 4096 bytes")
		}
		graphKey, surfaceKeys, err := compileStoryBeatGraphs(deck, story, slideIndex, step, beat)
		if err != nil {
			return fail(err.Error())
		}
		graph := story.Graphs[graphKey]
		actors := map[string]StoryActor{}
		for _, actor := range graph.Actors {
			actors[actor.ID] = actor
		}
		for _, group := range [][]string{beat.Focus, beat.Reveal, beat.Trace} {
			for _, id := range group {
				if _, ok := actors[id]; !ok {
					return fail("unknown actor " + strconv.Quote(id))
				}
			}
		}
		for j := 1; j < len(beat.Trace); j++ {
			if !storyHasEdge(graph, beat.Trace[j-1], beat.Trace[j]) {
				return fail("trace has no relationship " + beat.Trace[j-1] + " → " + beat.Trace[j])
			}
			if graph.TraceTargets[beat.Trace[j-1]+"->"+beat.Trace[j]] == "" {
				return fail("trace relationship is ambiguous: " + beat.Trace[j-1] + " → " + beat.Trace[j])
			}
		}
		if beat.Camera != nil {
			if !graph.Scene {
				return fail("camera needs a Sirena Scene3D component")
			}
			for _, v := range []float64{beat.Camera.X, beat.Camera.Y, beat.Camera.Z, beat.Camera.FOV} {
				if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 10000 {
					return fail("camera values must be finite and bounded")
				}
			}
			if beat.Camera.FOV <= 0 || beat.Camera.FOV >= 180 {
				return fail("camera fov must be between 0 and 180")
			}
		}
		if beat.Code != nil {
			blocks := deck.Slides[slideIndex].Node.Find(mdpp.NodeCodeBlock)
			if beat.Code.Block < 0 || beat.Code.Block >= len(blocks) {
				return fail("unknown code block")
			}
			code, err := storyCodeSource(deck, blocks[beat.Code.Block])
			if err != nil {
				return fail(err.Error())
			}
			lines := len(strings.Split(strings.TrimRight(code, "\n"), "\n"))
			for _, line := range beat.Code.Lines {
				if line < 1 || line > lines {
					return fail("code line outside block")
				}
			}
		}
		dom := storyDOMTargets(deck.Slides[slideIndex])
		for id, target := range dom {
			if target.Count > 1 {
				return fail("ambiguous DOM target " + strconv.Quote(id))
			}
			if _, exists := actors[id]; exists {
				return fail("DOM and actor identity collide: " + id)
			}
		}
		for _, group := range [][]string{beat.Show, beat.Hide} {
			for _, id := range group {
				if _, exists := dom[id]; !exists {
					return fail("unknown DOM target " + strconv.Quote(id))
				}
			}
		}
		for _, id := range beat.Show {
			if containsString(beat.Hide, id) {
				return fail("DOM target is both shown and hidden: " + id)
			}
		}
		if beat.Expect != nil {
			for _, group := range [][]string{beat.Expect.Visible, beat.Expect.Hidden} {
				for _, id := range group {
					if _, ok := actors[id]; !ok && dom[id].Count == 0 {
						return fail("unknown asserted target " + strconv.Quote(id))
					}
				}
			}
			for id, label := range beat.Expect.Labels {
				if a, ok := actors[id]; !ok || a.Label != label {
					return fail("asserted actor label differs: " + id)
				}
			}
		}
		story.Beats = append(story.Beats, CompiledStoryBeat{StoryBeat: beat, SlideIndex: slideIndex, Step: step, GraphKey: graphKey, SurfaceGraphKeys: surfaceKeys, Source: SourceDiagnostic{File: file, Range: rangeValue}})
	}
	sort.SliceStable(story.Beats, func(i, j int) bool {
		a, b := story.Beats[i], story.Beats[j]
		return a.SlideIndex < b.SlideIndex || a.SlideIndex == b.SlideIndex && a.Step < b.Step
	})
	compiled, err := json.Marshal(story)
	if err != nil || len(compiled) > 16<<20 {
		return nil, fmt.Errorf("story: compiled metadata exceeds 16 MiB")
	}
	return story, nil
}

func yamlSourceOffset(source []byte, line, column int) int {
	i := 0
	for row := 1; row < line && i < len(source); row++ {
		for i < len(source) && source[i] != '\n' {
			i++
		}
		if i < len(source) {
			i++
		}
	}
	for col := 1; col < column && i < len(source); col++ {
		_, size := utf8.DecodeRune(source[i:])
		i += size
	}
	return i
}

func storyHasEdge(graph StoryGraph, from, to string) bool {
	for _, edge := range graph.Edges {
		if edge == [2]string{from, to} {
			return true
		}
	}
	return false
}

func storyCodeSource(deck *IslandDeck, node *mdpp.Node) (string, error) {
	path, window, imported := parseSnippetDirective(node.Literal)
	if !imported {
		return node.Literal, nil
	}
	file, err := safeAuthorPath(deck.Dir, path)
	if err != nil {
		return "", fmt.Errorf("code source: %w", err)
	}
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
		return "", fmt.Errorf("code source %q must be a regular file below 1 MiB", path)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("code source exceeds 1 MiB")
	}
	source := strings.TrimRight(string(data), "\n")
	if window != "" {
		lo, hi, _ := parseLineRange(window)
		lines := strings.Split(source, "\n")
		if lo > len(lines) || hi > len(lines) {
			return "", fmt.Errorf("code source line window outside %q", path)
		}
		source = strings.Join(lines[lo-1:hi], "\n")
	}
	return source, nil
}

type storyDOMTarget struct {
	Hidden bool
	Count  int
}

func storyDOMTargets(slide IslandSlide) map[string]storyDOMTarget {
	ids := map[string]storyDOMTarget{}
	slide.Node.Walk(func(n *mdpp.Node) bool {
		if n.Type != mdpp.NodeHTMLBlock && n.Type != mdpp.NodeHTMLInline && n.Type != mdpp.NodeText {
			return true
		}
		tokenizer := html.NewTokenizer(strings.NewReader(sanitizeDeckHTML(n.Literal)))
		for {
			kind := tokenizer.Next()
			if kind == html.ErrorToken {
				break
			}
			if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
				continue
			}
			hidden := false
			names := map[string]bool{}
			for _, attr := range tokenizer.Token().Attr {
				if attr.Key == "hidden" {
					hidden = true
				}
				if attr.Key == "id" || attr.Key == "data-story-id" {
					names[attr.Val] = true
				}
			}
			for id := range names {
				target := ids[id]
				target.Hidden = hidden
				target.Count++
				ids[id] = target
			}
		}
		return true
	})
	return ids
}

func storyGraph(deck *IslandDeck, slide IslandSlide, step int) (StoryGraph, error) {
	graph := StoryGraph{SlideIndex: slide.Index, Actors: []StoryActor{}, Edges: [][2]string{}, TraceTargets: map[string]string{}}
	surfaces := 0
	morphNodes := map[*mdpp.Node]bool{}
	for _, container := range slide.Node.Find(mdpp.NodeContainerDirective) {
		if container.Attr("name") != "diagram-morph" {
			continue
		}
		found := false
		for _, node := range container.Find(mdpp.NodeDiagram) {
			morphNodes[node] = true
			found = found || node.Attr("syntax") == "sirena"
		}
		if found {
			surfaces++
		}
	}
	for _, node := range slide.Node.Find(mdpp.NodeDiagram) {
		if node.Attr("syntax") == "sirena" && !morphNodes[node] {
			surfaces++
		}
	}
	for _, ref := range slide.Components {
		if ref.Name == "Scene3D" && strings.HasSuffix(strings.ToLower(graphicString(parseProps(ref.Props), "Src", "")), ".sir") {
			surfaces++
		}
	}
	if surfaces > 1 {
		return graph, fmt.Errorf("story currently needs one Sirena surface per slide")
	}
	var source []byte
	file := DeckFileName
	offset := 0
	var originalSource []byte
	var fenceBody []byte
	visibleIDs := map[string]bool{}
	for _, ref := range slide.Components {
		if ref.Name != "Scene3D" {
			continue
		}
		props := parseProps(ref.Props)
		src := graphicString(props, "Src", "")
		if !strings.HasSuffix(strings.ToLower(src), ".sir") {
			continue
		}
		if source != nil {
			return graph, fmt.Errorf("story currently needs one Sirena surface per slide")
		}
		if graphicString(props, "Steps", "") != "" {
			return graph, fmt.Errorf("story and authored Scene3D Steps are mutually exclusive")
		}
		var err error
		source, err = readGraphicFile(deck.Dir, src)
		if err != nil {
			return graph, err
		}
		file = src
		graph.Scene = true
		graph.Source = src
		originalSource = source
		payload, err := compileSirenaGraphic(deck.Dir, props, source, nil)
		if err != nil {
			return graph, err
		}
		objects, _ := payload["scene"].(map[string]any)["objects"].([]any)
		for _, raw := range objects {
			object, _ := raw.(map[string]any)
			id, _ := object["id"].(string)
			visibleIDs[id] = true
		}
	}
	if source == nil {
		nodes := slide.Node.Find(mdpp.NodeDiagram)
		for _, container := range slide.Node.Find(mdpp.NodeContainerDirective) {
			if container.Attr("name") == "diagram-morph" {
				states := container.Find(mdpp.NodeDiagram)
				if len(states) > 0 {
					nodes = []*mdpp.Node{states[max(0, min(step, len(states)-1))]}
				}
			}
		}
		for _, node := range nodes {
			if node.Attr("syntax") != "sirena" {
				continue
			}
			if source != nil {
				return graph, fmt.Errorf("story currently needs one Sirena surface per slide")
			}
			source = []byte(node.Literal)
			if where, ok := deck.SourceLocation(node.Range.StartByte, node.Range.StartByte); ok {
				file = where.File
				originalSource = deck.Source
				if file != DeckFileName {
					path, err := safeAuthorPath(deck.Dir, file)
					if err != nil {
						return graph, err
					}
					originalSource, err = os.ReadFile(path)
					if err != nil {
						return graph, err
					}
				}
				// Parse the origin independently: include padding may extend the
				// composed node range, but its actual fence must live in one file.
				origin, err := mdpp.Parse(originalSource)
				if err != nil {
					return graph, err
				}
				matched := false
				cr := markdownCRPositions(originalSource)
				for _, candidate := range origin.AST().Find(mdpp.NodeDiagram) {
					start := originalMarkdownOffset(cr, candidate.Range.StartByte)
					if start != where.StartByte || candidate.Literal != node.Literal {
						continue
					}
					end := min(len(originalSource), originalMarkdownOffset(cr, candidate.Range.EndByte))
					offset = start + bytes.IndexByte(originalSource[start:end], '\n') + 1
					fenceBody = originalSource[offset:end]
					matched = true
					break
				}
				if !matched {
					return graph, fmt.Errorf("diagram crosses authored file boundaries")
				}
			} else {
				return graph, fmt.Errorf("diagram crosses authored file boundaries")
			}
			res, err := fence.Render(source, fence.Options{Theme: sirenaThemeForDeck(deck), ViewRef: node.Attr("view"), Diagram: node.Attr("diagram"), WorkspaceRoot: deck.Dir})
			if err != nil {
				return graph, err
			}
			decoder := xml.NewDecoder(bytes.NewReader(res.SVG))
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return graph, err
				}
				if tag, ok := token.(xml.StartElement); ok {
					for _, attr := range tag.Attr {
						if attr.Name.Local == "data-sirena-id" {
							visibleIDs[attr.Value] = true
						}
					}
				}
			}
		}
	}
	if source == nil {
		return graph, nil
	}
	workspace, err := sirena.NewFenceWorkspace(source, sirena.FenceOptions{})
	if err != nil {
		return graph, err
	}
	doc := workspace.Files[0].Document
	for _, d := range append(doc.Diagnostics(), workspace.ResolveDiagnostics()...) {
		if d.Severity == sirena.SeverityError {
			return graph, fmt.Errorf("Sirena: %s", d.Message)
		}
	}
	identity := map[string]string{}
	var add func([]sirena.Node)
	add = func(nodes []sirena.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *sirena.Element:
				id := n.Name
				if sid, ok := n.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
					id = sid.Value
				}
				identity[n.Name] = id
				if visibleIDs[id] {
					start, end := n.Range.Start, n.Range.End
					if fenceBody != nil {
						start, end = storyFenceOffset(fenceBody, source, start), storyFenceOffset(fenceBody, source, end)
					}
					rangeValue := mdpp.Range{StartByte: offset + start, EndByte: offset + end}
					rangeValue.StartLine, rangeValue.StartCol = sourcePosition(originalSource, rangeValue.StartByte)
					rangeValue.EndLine, rangeValue.EndCol = sourcePosition(originalSource, rangeValue.EndByte)
					graph.Actors = append(graph.Actors, StoryActor{Name: n.Name, ID: id, Label: n.DisplayLabel(), File: file, Range: rangeValue})
				}
			case *sirena.Boundary:
				add(n.Children)
			}
		}
	}
	for _, system := range doc.Systems {
		nodes := []sirena.Node{}
		for _, element := range system.Elements {
			nodes = append(nodes, element)
		}
		for _, boundary := range system.Boundaries {
			nodes = append(nodes, boundary)
		}
		add(nodes)
	}
	var edges func([]sirena.Node)
	edges = func(nodes []sirena.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *sirena.Edge:
				a, b := identity[n.From], identity[n.To]
				if !visibleIDs[a] || !visibleIDs[b] {
					continue
				}
				if n.Direction == sirena.DirReverse {
					a, b = b, a
				}
				alias := n.From + "->" + n.To
				if sid, ok := n.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
					alias = sid.Value
				}
				if _, duplicate := graph.TraceTargets[a+"->"+b]; duplicate {
					alias = ""
				}
				graph.Edges = append(graph.Edges, [2]string{a, b})
				graph.TraceTargets[a+"->"+b] = alias
				if n.Direction == sirena.DirBidirectional && a != b {
					graph.Edges = append(graph.Edges, [2]string{b, a})
					if _, duplicate := graph.TraceTargets[b+"->"+a]; duplicate {
						alias = ""
					}
					graph.TraceTargets[b+"->"+a] = alias
				}
			case *sirena.Boundary:
				edges(n.Children)
			}
		}
	}
	for _, system := range doc.Systems {
		nodes := []sirena.Node{}
		for _, edge := range system.Edges {
			nodes = append(nodes, edge)
		}
		for _, boundary := range system.Boundaries {
			nodes = append(nodes, boundary)
		}
		edges(nodes)
	}
	seen := map[string]bool{}
	if len(graph.Actors) > 256 {
		return graph, fmt.Errorf("story surface exceeds 256 actors")
	}
	for _, actor := range graph.Actors {
		if seen[actor.ID] {
			return graph, fmt.Errorf("ambiguous actor identity %q", actor.ID)
		}
		seen[actor.ID] = true
	}
	return graph, nil
}

// mdpp normalizes line endings and removes fence indentation. Translate Sirena
// offsets by matching each retained source line to the original fenced line.
func storyFenceOffset(original, literal []byte, offset int) int {
	start, rawStart := 0, 0
	for start <= len(literal) && rawStart <= len(original) {
		end := start + bytes.IndexByte(literal[start:], '\n')
		if end < start {
			end = len(literal)
		}
		rawEnd := rawStart + bytes.IndexByte(original[rawStart:], '\n')
		if rawEnd < rawStart {
			rawEnd = len(original)
		}
		if offset <= end {
			rawLine := bytes.TrimSuffix(original[rawStart:rawEnd], []byte{'\r'})
			line := literal[start:end]
			indent := 0
			if len(line) > 0 && bytes.HasSuffix(rawLine, line) {
				indent = len(rawLine) - len(line)
			}
			return min(len(original), rawStart+indent+offset-start)
		}
		start, rawStart = end+1, rawEnd+1
	}
	return len(original)
}

func attachSemanticStory(deck *IslandDeck) error {
	story, err := CompileStory(deck)
	if err != nil {
		return err
	}
	deck.Story = story
	if story == nil {
		return nil
	}
	deck.storySceneSteps = map[string][]byte{}
	for _, slide := range deck.Slides {
		surfaces, err := storySurfaces(slide)
		if err != nil {
			return err
		}
		for _, surface := range surfaces {
			if surface.node.Attrs == nil {
				surface.node.Attrs = map[string]string{}
			}
			surface.node.Attrs["id"] = surface.containerID
		}
	}
	for _, beat := range story.Beats {
		if beat.Code != nil {
			node := deck.Slides[beat.SlideIndex].Node.Find(mdpp.NodeCodeBlock)[beat.Code.Block]
			if node.Attr("highlights") == "" {
				if node.Attrs == nil {
					node.Attrs = map[string]string{}
				}
				node.Attrs["highlights"] = storyCodeLines
			}
		}
	}
	for _, graph := range story.Graphs {
		slideIndex := graph.SlideIndex
		if !graph.Scene {
			continue
		}
		steps := make([]sirenascene.Step, len(slideCueNames(deck.Slides[slideIndex])))
		for i, name := range slideCueNames(deck.Slides[slideIndex]) {
			steps[i].Label = name
		}
		for _, beat := range story.Beats {
			if beat.SlideIndex != slideIndex {
				continue
			}
			pose := storyPoseForGraph(beat, graph)
			step := sirenascene.Step{Label: beat.Caption, DurationMS: beat.DurationMS, Focus: pose.Focus, Reveal: pose.Reveal}
			for i := 1; i < len(pose.Trace); i++ {
				step.Trace = append(step.Trace, graph.TraceTargets[pose.Trace[i-1]+"->"+pose.Trace[i]])
			}
			if pose.Camera != nil {
				step.Camera = &scene.IRCamera{Kind: "perspective", X: pose.Camera.X, Y: pose.Camera.Y, Z: pose.Camera.Z, FOV: pose.Camera.FOV, Near: .1, Far: 1000}
			}
			steps[beat.Step] = step
		}
		raw, _ := json.Marshal(steps)
		var authored []map[string]any
		json.Unmarshal(raw, &authored)
		for i := range authored {
			authored[i]["durationMs"] = steps[i].DurationMS
			if steps[i].Reveal != nil {
				authored[i]["reveal"] = steps[i].Reveal
			}
		}
		raw, _ = json.Marshal(authored)
		updated := false
		bind := func(props string) string {
			if graphicString(parseProps(props), "Src", "") != graph.Source {
				return props
			}
			props = strings.TrimSpace(props) + " StorySlide=" + strconv.Quote(strconv.Itoa(slideIndex))
			if graph.Surface != "" {
				props += " StorySurface=" + strconv.Quote(graph.Surface)
			}
			ref := ComponentRef{Name: "Scene3D", Props: props}
			deck.storySceneSteps[graphicsKey(ref.Name, ref.Props)] = raw
			updated = true
			return props
		}
		root := deck.Slides[slideIndex].Node
		if graph.Surface != "" {
			for _, container := range root.Find(mdpp.NodeContainerDirective) {
				if container.Attr("id") == graph.ContainerID {
					root = container
					break
				}
			}
			if root == deck.Slides[slideIndex].Node {
				return fmt.Errorf("story surface %s container disappeared", graph.Surface)
			}
		}
		root.Walk(func(node *mdpp.Node) bool {
			if node.Type == mdpp.NodeCodeBlock {
				return false
			}
			if node.Type == mdpp.NodeComponent && node.Attr("name") == "Scene3D" {
				node.Attrs["props"] = bind(node.Attr("props"))
			}
			if node.Type == mdpp.NodeHTMLBlock || node.Type == mdpp.NodeHTMLInline || node.Type == mdpp.NodeText {
				node.Literal = blockComponentRe.ReplaceAllStringFunc(node.Literal, func(tag string) string {
					match := blockComponentRe.FindStringSubmatch(tag)
					if match[1] != "Scene3D" {
						return tag
					}
					props := bind(match[2])
					return "<Scene3D " + props + " " + match[3] + ">"
				})
			}
			return true
		})
		if !updated {
			return fmt.Errorf("story scene could not bind its parsed Scene3D component")
		}
		deck.Slides[slideIndex].Components = collectComponentRefs(deck.Slides[slideIndex].Node)
	}
	return nil
}

// Story assertions check the compiled absolute states and authored actor labels.
type StoryAssertionReport struct {
	Beats          int      `json:"beats"`
	Checks         int      `json:"checks"`
	RenderedStates int      `json:"renderedStates,omitempty"`
	Errors         []string `json:"errors"`
}

func AssertStory(deck *IslandDeck) (StoryAssertionReport, error) {
	story, err := CompileStory(deck)
	if err != nil {
		return StoryAssertionReport{}, err
	}
	report := StoryAssertionReport{Errors: []string{}}
	if story == nil {
		return report, nil
	}
	report.Beats = len(story.Beats)
	for _, diagnostic := range deckSourceDiagnostics(deck) {
		report.Checks++
		if diagnostic.Severity == "error" || strings.HasPrefix(diagnostic.Code, "STORY-") {
			report.Errors = append(report.Errors, fmt.Sprintf("%s:%d: %s", diagnostic.File, diagnostic.Range.StartLine, diagnostic.Message))
		}
	}
	for _, slide := range deck.Slides {
		for _, node := range slide.Node.Find(mdpp.NodeCodeBlock) {
			if _, _, imported := parseSnippetDirective(node.Literal); imported {
				report.Checks++
				if _, err := storyCodeSource(deck, node); err != nil {
					report.Errors = append(report.Errors, err.Error())
				}
			}
		}
		for _, node := range slide.Node.Find(mdpp.NodeLink) {
			href := node.Attr("href")
			reference, err := url.Parse(href)
			if err != nil {
				report.Errors = append(report.Errors, "invalid authored link: "+href)
				continue
			}
			// Fragment addresses use mdpp's story index. Remote URLs remain external.
			if reference.IsAbs() || reference.Host != "" || reference.Path == "" || reference.Path == "/" || reference.Path == "/remote" || reference.Path == "/presenter" {
				continue
			}
			report.Checks++
			file, err := safeAuthorPath(deck.Dir, strings.TrimPrefix(reference.Path, "/"))
			if err == nil {
				_, err = os.Stat(file)
			}
			if err != nil {
				report.Errors = append(report.Errors, "authored link "+href+": "+err.Error())
			}
		}
	}
	for _, beat := range story.Beats {
		if beat.Expect == nil {
			continue
		}
		graph := story.Graphs[beat.GraphKey]
		dom := storyDOMTargets(deck.Slides[beat.SlideIndex])
		for _, group := range []struct {
			ids     []string
			visible bool
		}{{beat.Expect.Visible, true}, {beat.Expect.Hidden, false}} {
			for _, id := range group.ids {
				report.Checks++
				visible := !dom[id].Hidden
				if containsString(beat.Show, id) {
					visible = true
				}
				if containsString(beat.Hide, id) {
					visible = false
				}
				for _, a := range graph.Actors {
					if a.ID == id {
						visible = storyActorVisible(story, beat, id)
					}
				}
				if visible != group.visible {
					report.Errors = append(report.Errors, fmt.Sprintf("%s/%s: %s visibility expected %t", beat.Slide, beat.Cue, id, group.visible))
				}
			}
		}
		report.Checks += len(beat.Expect.Labels)
	}
	return report, nil
}

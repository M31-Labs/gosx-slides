package slides

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"m31labs.dev/mdpp"
)

// AudienceSelectionError reports source-addressable links into omitted slides.
// Selection fails before serve/export can publish a broken variant.
type AudienceSelectionError struct {
	Diagnostics []SourceDiagnostic
}

func (err *AudienceSelectionError) Error() string {
	var messages []string
	for _, diagnostic := range err.Diagnostics {
		messages = append(messages, diagnosticMessage(diagnostic))
	}
	return strings.Join(messages, "\n")
}

// LoadIslandDeckAudience loads a variant before resolving components and story
// beats. An empty audience loads the complete deck, as LoadIslandDeck does.
func LoadIslandDeckAudience(dir, audience string) (*IslandDeck, error) {
	source, err := os.ReadFile(filepath.Join(dir, DeckFileName))
	if err != nil {
		return nil, fmt.Errorf("read deck %s: %w", filepath.Join(dir, DeckFileName), err)
	}
	return parseIslandDeckAudience(dir, source, audience)
}

// DeckAudiences lists declared audiences, or discovers them from slide tags.
// Names are case sensitive and follow the same syntax as stable slide IDs.
// When deck audiences is declared, every slide tag must name one of its values.
func DeckAudiences(deck *IslandDeck) ([]string, error) {
	names, _, err := deckAudienceTags(deck)
	return names, err
}

func deckAudienceTags(deck *IslandDeck) ([]string, [][]string, error) {
	if deck == nil || deck.Document == nil {
		return nil, nil, fmt.Errorf("audience requires a parsed deck")
	}
	head, _, _ := splitHeadmatter(string(deck.Source))
	names, declared, err := parseAudienceMetadata(head)
	if err != nil {
		return nil, nil, fmt.Errorf("deck audiences: %w", err)
	}
	if deck.Audience != "" {
		names = slices.Clone(deck.audienceNames)
	}
	known := map[string]bool{}
	for _, name := range names {
		known[name] = true
	}
	tags := make([][]string, len(deck.Slides))
	for i, slide := range deck.Slides {
		if slide.Node == nil {
			return nil, nil, fmt.Errorf("audience: slide %d has no parsed node", i+1)
		}
		values, _, err := parseAudienceMetadata(slide.Node.Attr("frontmatter"))
		if err != nil {
			return nil, nil, fmt.Errorf("slide %d audiences: %w", slide.Index+1, err)
		}
		for _, name := range values {
			if declared && !known[name] {
				return nil, nil, fmt.Errorf("slide %d audiences: undeclared audience %q", slide.Index+1, name)
			}
			if !known[name] {
				known[name] = true
				names = append(names, name)
				if len(names) > 128 {
					return nil, nil, fmt.Errorf("at most 128 audiences are supported")
				}
			}
		}
		tags[i] = values
	}
	return slices.Clone(names), tags, nil
}

// Parse nodes rather than decoded aliases. Audience metadata accepts a string
// (including a comma list) or a sequence of strings; empty values, collections,
// numbers and aliases are errors. Bound metadata before handing it to YAML.
func parseAudienceMetadata(source string) ([]string, bool, error) {
	if strings.TrimSpace(source) == "" {
		return nil, false, nil
	}
	if len(source) > 65536 {
		return nil, false, fmt.Errorf("metadata exceeds 64 KiB")
	}
	var tree yaml.Node
	if err := yaml.Unmarshal([]byte(source), &tree); err != nil {
		return nil, false, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(tree.Content) == 0 {
		return nil, false, nil
	}
	if len(tree.Content) != 1 || tree.Content[0].Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("metadata must be a YAML mapping")
	}
	var value *yaml.Node
	root := tree.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "audiences" {
			if value != nil {
				return nil, true, fmt.Errorf("duplicate audiences key")
			}
			value = root.Content[i+1]
		}
	}
	if value == nil {
		return nil, false, nil
	}
	var names []string
	add := func(node *yaml.Node, commaList bool) error {
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return fmt.Errorf("audiences must contain strings")
		}
		values := []string{node.Value}
		if commaList {
			values = strings.Split(node.Value, ",")
		}
		for _, raw := range values {
			name := strings.TrimSpace(raw)
			if !cueNamePattern.MatchString(name) {
				return fmt.Errorf("invalid audience %q (use a letter followed by up to 63 letters, digits, underscores or hyphens)", name)
			}
			names = append(names, name)
			if len(names) > 128 {
				return fmt.Errorf("at most 128 audiences are supported")
			}
		}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			if err := add(item, false); err != nil {
				return nil, true, err
			}
		}
	} else if err := add(value, true); err != nil {
		return nil, true, err
	}
	if len(names) == 0 {
		return nil, true, fmt.Errorf("audiences cannot be empty; omit the key for shared slides")
	}
	return uniqueStrings(names), true, nil
}

// SelectAudience returns an independent render model containing shared slides
// and slides tagged with audience. It preserves original source bytes/ranges,
// stable IDs, cues and component origins while reindexing visible slides.
// Select from the complete deck; changing an already selected audience requires
// reloading it. An unregistered name and a variant with no slides are errors.
func SelectAudience(deck *IslandDeck, audience string) (*IslandDeck, error) {
	if !cueNamePattern.MatchString(audience) {
		return nil, fmt.Errorf("invalid audience %q", audience)
	}
	if deck != nil && deck.Audience != "" {
		return nil, fmt.Errorf("deck already selects audience %q; reload the complete deck before selecting a variant", deck.Audience)
	}
	names, tags, err := deckAudienceTags(deck)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(names, audience) {
		return nil, fmt.Errorf("unknown audience %q (available: %s)", audience, strings.Join(names, ", "))
	}
	selected := map[int]int{}
	for i, values := range tags {
		if len(values) == 0 || slices.Contains(values, audience) {
			selected[i] = len(selected)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("audience %q selects no slides", audience)
	}
	out := *deck
	out.Audience, out.audienceNames = audience, names
	out.Source, out.ExpandedSource = slices.Clone(deck.Source), slices.Clone(deck.ExpandedSource)
	out.Includes = slices.Clone(deck.Includes)
	out.sourceSegments = slices.Clone(deck.sourceSegments)
	out.sourceCRPositions = slices.Clone(deck.sourceCRPositions)
	out.componentSources, out.compositionAssets = maps.Clone(deck.componentSources), maps.Clone(deck.compositionAssets)
	out.packLayouts = maps.Clone(deck.packLayouts)
	out.storyExcludedSlides = maps.Clone(deck.storyExcludedSlides)
	if out.storyExcludedSlides == nil {
		out.storyExcludedSlides = map[string]bool{}
	}
	out.Packs = slices.Clone(deck.Packs)
	for i := range out.Packs {
		out.Packs[i].CSS = slices.Clone(deck.Packs[i].CSS)
		out.Packs[i].Layouts = slices.Clone(deck.Packs[i].Layouts)
		out.Packs[i].Components = maps.Clone(deck.Packs[i].Components)
		out.Packs[i].css = slices.Clone(deck.Packs[i].css)
	}
	parseSource := deck.ExpandedSource
	if len(parseSource) == 0 {
		parseSource = deck.Source
	}
	if len(parseSource) == 0 {
		parseSource = deck.Document.Source
	}
	doc, err := mdpp.Parse(parseSource)
	if err != nil {
		return nil, fmt.Errorf("audience: copy parsed metadata: %w", err)
	}
	// Parse privately copies hidden frontmatter/diagnostic metadata; the cloned
	// root retains include rebasing and already-compiled story annotations.
	doc.Root = cloneAudienceNode(deck.Document.Root)
	out.Document, out.Slides = doc, nil
	cloned := doc.Slides()
	if len(cloned) != len(deck.Slides) {
		return nil, fmt.Errorf("audience: parsed slide model does not match the document")
	}
	for i, slide := range deck.Slides {
		index, keep := selected[i]
		if !keep {
			if id := slideIdentityAttrs(slide)["data-slide-id"]; id != "" {
				out.storyExcludedSlides[id] = true
			}
			continue
		}
		slide.Index, slide.Node = index, cloned[i]
		slide.Components, slide.packLayouts = slices.Clone(slide.Components), out.packLayouts
		out.Slides = append(out.Slides, slide)
	}
	// Excluded duplicate IDs must not suppress a retained slide's story beats.
	for _, slide := range out.Slides {
		delete(out.storyExcludedSlides, slideIdentityAttrs(slide)["data-slide-id"])
	}
	children := doc.Root.Children[:0]
	index := 0
	for _, child := range doc.Root.Children {
		if child.Type == mdpp.NodeSlide {
			_, keep := selected[index]
			index++
			if !keep {
				continue
			}
		}
		children = append(children, child)
	}
	doc.Root.Children = children
	if err := out.filterAudienceDiagnostics(selected, len(deck.Slides)); err != nil {
		return nil, err
	}
	out.rewriteAudienceLinks(deck, selected)
	if len(out.audienceDiagnostics) > 0 {
		var diagnostics []SourceDiagnostic
		for _, diagnostic := range deckSourceDiagnostics(&out) {
			if diagnostic.Code == "AUDIENCE-LINK" {
				diagnostics = append(diagnostics, diagnostic)
			}
		}
		return nil, &AudienceSelectionError{Diagnostics: diagnostics}
	}
	if deck.Story != nil {
		data, err := json.Marshal(deck.Story)
		if err != nil {
			return nil, fmt.Errorf("audience: copy compiled story: %w", err)
		}
		out.Story = &CompiledStory{}
		if err := json.Unmarshal(data, out.Story); err != nil {
			return nil, err
		}
		beats := out.Story.Beats[:0]
		for _, beat := range out.Story.Beats {
			if index, keep := selected[beat.SlideIndex]; keep {
				beat.SlideIndex = index
				beats = append(beats, beat)
			}
		}
		out.Story.Beats = beats
		for key, graph := range out.Story.Graphs {
			if index, keep := selected[graph.SlideIndex]; keep {
				graph.SlideIndex = index
				out.Story.Graphs[key] = graph
			} else {
				delete(out.Story.Graphs, key)
			}
		}
	}
	out.storySceneSteps = map[string][]byte{}
	for _, slide := range out.Slides {
		for _, ref := range slide.Components {
			key := graphicsKey(ref.Name, ref.Props)
			if steps, ok := deck.storySceneSteps[key]; ok {
				out.storySceneSteps[key] = slices.Clone(steps)
			}
		}
	}
	if err := FilterSimulations(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func cloneAudienceNode(node *mdpp.Node) *mdpp.Node {
	if node == nil {
		return nil
	}
	out := *node
	out.Attrs = maps.Clone(node.Attrs)
	out.Children = make([]*mdpp.Node, len(node.Children))
	for i, child := range node.Children {
		out.Children[i] = cloneAudienceNode(child)
	}
	return &out
}

// IndexStory recovers lifted metadata by reparsing split documents. Supply an
// unsplit selected AST so indexing cannot silently restore omitted slides.
func (deck *IslandDeck) filterAudienceDiagnostics(selected map[int]int, count int) error {
	doc, err := mdpp.Parse(deck.Document.Source)
	if err != nil {
		return fmt.Errorf("audience: parse source diagnostics: %w", err)
	}
	groups := [][]*mdpp.Node{{}}
	var separators []*mdpp.Node
	var head []*mdpp.Node
	for _, node := range doc.Root.Children {
		if node.Type == mdpp.NodeFrontmatter && len(groups) == 1 && len(groups[0]) == 0 {
			head = append(head, node)
			deck.audienceRanges = append(deck.audienceRanges, node.Range)
		} else if node.Type == mdpp.NodeThematicBreak {
			groups = append(groups, nil)
			separators = append(separators, node)
		} else {
			groups[len(groups)-1] = append(groups[len(groups)-1], node)
		}
	}
	if len(groups) != count {
		return fmt.Errorf("audience: parsed source slide count changed")
	}
	doc.Root.Children = head
	kept := 0
	for i, nodes := range groups {
		if _, keep := selected[i]; !keep {
			continue
		}
		if kept > 0 {
			doc.Root.Children = append(doc.Root.Children, separators[i-1])
		}
		kept++
		doc.Root.Children = append(doc.Root.Children, nodes...)
		for _, node := range nodes {
			deck.audienceRanges = append(deck.audienceRanges, node.Range)
		}
	}
	deck.audienceDocument = doc
	return nil
}

func (deck *IslandDeck) audienceContainsRange(where mdpp.Range) bool {
	if where.StartByte == 0 && where.EndByte == 0 {
		return true
	}
	for _, active := range deck.audienceRanges {
		if where.StartByte >= active.StartByte && where.StartByte < active.EndByte {
			return true
		}
	}
	return false
}

var audienceNumericLink = regexp.MustCompile(`^#([0-9]+)((?:/[0-9]+)?(?:present)?)$`)

func (deck *IslandDeck) rewriteAudienceLinks(original *IslandDeck, selected map[int]int) {
	ids := map[string]int{}
	for i, slide := range original.Slides {
		if id := slideIdentityAttrs(slide)["data-slide-id"]; id != "" {
			ids[id] = i
		}
	}
	for _, slide := range deck.Slides {
		slide.Node.Walk(func(node *mdpp.Node) bool {
			rewrite := func(href string) string {
				target, found := -1, false
				numeric := audienceNumericLink.FindStringSubmatch(href)
				if numeric != nil {
					value, err := strconv.Atoi(numeric[1])
					if err == nil && value > 0 && value <= len(original.Slides) {
						target, found = value-1, true
					}
				} else if strings.HasPrefix(href, "#") {
					name, _, _ := strings.Cut(href[1:], "/")
					target, found = ids[name]
				}
				if !found {
					return href
				}
				index, keep := selected[target]
				if !keep {
					deck.audienceDiagnostics = append(deck.audienceDiagnostics, SourceDiagnostic{
						Code: "AUDIENCE-LINK", Severity: "error", Message: fmt.Sprintf("audience %q omits slide targeted by %s", deck.Audience, href), Range: node.Range,
					})
					return href
				}
				if numeric != nil {
					return "#" + strconv.Itoa(index+1) + numeric[2]
				}
				return href
			}
			if node.Type == mdpp.NodeLink {
				node.Attrs["href"] = rewrite(node.Attr("href"))
			}
			if node.Type == mdpp.NodeHTMLBlock || node.Type == mdpp.NodeHTMLInline {
				var result strings.Builder
				tokens := html.NewTokenizer(strings.NewReader(node.Literal))
				changed := false
				for {
					kind := tokens.Next()
					if kind == html.ErrorToken {
						if tokens.Err() != io.EOF {
							return true
						}
						break
					}
					raw := string(tokens.Raw())
					if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
						token := tokens.Token()
						if token.Data == "a" {
							for i, attr := range token.Attr {
								if attr.Key == "href" {
									value := rewrite(attr.Val)
									if value != attr.Val {
										token.Attr[i].Val = value
										raw, changed = token.String(), true
									}
								}
							}
						}
					}
					result.WriteString(raw)
				}
				if changed {
					node.Literal = result.String()
				}
			}
			return node.Type != mdpp.NodeCodeBlock
		})
	}
}

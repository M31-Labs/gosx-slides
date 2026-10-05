package slides

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"m31labs.dev/sirena"
)

type ArchitectureChangeTour struct {
	Added                []string `json:"added"`
	Removed              []string `json:"removed"`
	Changed              []string `json:"changed"`
	RelationshipsAdded   []string `json:"relationshipsAdded"`
	RelationshipsRemoved []string `json:"relationshipsRemoved"`
	Markdown             string   `json:"markdown"`
	StoryYAML            string   `json:"storyYAML"`
}

// ArchitectureTour derives a ready deck and semantic captions from two parsed
// Sirena snapshots. Stable sid metadata survives renames and layout changes.
func ArchitectureTour(before, after []byte) (ArchitectureChangeTour, error) {
	type snapshot struct{ actors, relationships map[string]string }
	actorSnapshot := func(source []byte) (*snapshot, error) {
		if len(source) > maxSourceBytes {
			return nil, fmt.Errorf("architecture snapshot exceeds 1 MiB")
		}
		ws, err := sirena.NewFenceWorkspace(source, sirena.FenceOptions{})
		if err != nil {
			return nil, err
		}
		doc := ws.Files[0].Document
		for _, d := range append(doc.Diagnostics(), ws.ResolveDiagnostics()...) {
			if d.Severity == sirena.SeverityError {
				return nil, fmt.Errorf("Sirena: %s", d.Message)
			}
		}
		out := &snapshot{map[string]string{}, map[string]string{}}
		identities := map[string]string{}
		edges := []*sirena.Edge{}
		var visit func([]sirena.Node, string) error
		visit = func(nodes []sirena.Node, boundary string) error {
			for _, node := range nodes {
				switch n := node.(type) {
				case *sirena.Element:
					id := n.Name
					if sid, ok := n.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
						id = sid.Value
					}
					if _, exists := out.actors[id]; exists {
						return fmt.Errorf("ambiguous actor identity %q", id)
					}
					data, _ := json.Marshal(struct {
						Kind     sirena.ElementKind
						Metadata map[string]sirena.Value
						Boundary string
						Label    string
					}{n.Kind, storySemanticMetadata(n.Metadata), boundary, n.DisplayLabel()})
					out.actors[id] = string(data)
					identities[n.Name] = id
				case *sirena.Boundary:
					id := n.Name
					if sid, ok := n.Metadata["sid"].(sirena.String); ok && sid.Value != "" {
						id = sid.Value
					}
					if err := visit(n.Children, boundary+"/"+id); err != nil {
						return err
					}
				case *sirena.Edge:
					edges = append(edges, n)
				}
			}
			return nil
		}
		for _, system := range doc.Systems {
			nodes := []sirena.Node{}
			for _, n := range system.Elements {
				nodes = append(nodes, n)
			}
			for _, n := range system.Boundaries {
				nodes = append(nodes, n)
			}
			for _, n := range system.Edges {
				nodes = append(nodes, n)
			}
			if err := visit(nodes, ""); err != nil {
				return nil, err
			}
		}
		for _, edge := range edges {
			from, to := identities[edge.From], identities[edge.To]
			if from == "" {
				from = edge.From
			}
			if to == "" {
				to = edge.To
			}
			data, _ := json.Marshal(struct {
				From, To  string
				Kind      sirena.EdgeKind
				Direction sirena.Direction
				Label     string
				Metadata  map[string]sirena.Value
			}{from, to, edge.Kind, edge.Direction, edge.Label, storySemanticMetadata(edge.Metadata)})
			label := from + " → " + to + " (" + edge.Kind.String() + ")"
			if edge.Label != "" {
				label += ": " + edge.Label
			}
			out.relationships[string(data)] = label
		}
		return out, nil
	}
	a, err := actorSnapshot(before)
	if err != nil {
		return ArchitectureChangeTour{}, err
	}
	b, err := actorSnapshot(after)
	if err != nil {
		return ArchitectureChangeTour{}, err
	}
	tour := ArchitectureChangeTour{Added: []string{}, Removed: []string{}, Changed: []string{}, RelationshipsAdded: []string{}, RelationshipsRemoved: []string{}}
	for id, value := range a.actors {
		if next, exists := b.actors[id]; !exists {
			tour.Removed = append(tour.Removed, id)
		} else if next != value {
			tour.Changed = append(tour.Changed, id)
		}
	}
	for id := range b.actors {
		if _, exists := a.actors[id]; !exists {
			tour.Added = append(tour.Added, id)
		}
	}
	for signature, label := range a.relationships {
		if _, exists := b.relationships[signature]; !exists {
			tour.RelationshipsRemoved = append(tour.RelationshipsRemoved, label)
		}
	}
	for signature, label := range b.relationships {
		if _, exists := a.relationships[signature]; !exists {
			tour.RelationshipsAdded = append(tour.RelationshipsAdded, label)
		}
	}
	sort.Strings(tour.Added)
	sort.Strings(tour.Removed)
	sort.Strings(tour.Changed)
	sort.Strings(tour.RelationshipsAdded)
	sort.Strings(tour.RelationshipsRemoved)
	describe := func(verb string, ids []string) string {
		if len(ids) == 0 {
			return ""
		}
		return verb + " " + strings.Join(ids, ", ") + "."
	}
	previous := strings.TrimSpace(describe("Remove", tour.Removed) + " " + describe("Remove relationships", tour.RelationshipsRemoved))
	if previous == "" {
		previous = "Start with the current architecture."
	}
	caption := strings.TrimSpace(describe("Add", tour.Added) + " " + describe("Change", tour.Changed) + " " + describe("Add relationships", tour.RelationshipsAdded))
	if caption == "" {
		caption = "The remaining architecture keeps its stable identities."
	}
	manifest := struct {
		Version int         `yaml:"version"`
		Beats   []StoryBeat `yaml:"beats"`
	}{1, []StoryBeat{{Slide: "changes", Cue: "before", Caption: previous, DurationMS: 0, Focus: tour.Removed}, {Slide: "changes", Cue: "after", Caption: caption, DurationMS: 700, Focus: append(append([]string{}, tour.Added...), tour.Changed...)}}}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return ArchitectureChangeTour{}, err
	}
	tour.StoryYAML = string(data)
	fence := func(source []byte) string {
		longest, run := 0, 0
		for _, c := range source {
			if c == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
		mark := strings.Repeat("`", max(3, longest+1))
		return mark + "sirena\n" + strings.TrimSpace(string(source)) + "\n" + mark + "\n"
	}
	tour.Markdown = "---\ntitle: Architecture changes\nstory: story.yaml\n---\n\n```yaml\nid: changes\ncues: before, after\n```\n\n# Architecture changes\n\n:::diagram-morph {duration=700}\n" + fence(before) + "\n" + fence(after) + ":::\n\n<!-- Compare actor identities, then explain the architectural change. -->\n"
	return tour, nil
}

// Source positions are provenance, not an architectural change. Strip them
// recursively while preserving metadata types and values for comparison.
func storySemanticMetadata(metadata map[string]sirena.Value) map[string]sirena.Value {
	var value func(sirena.Value) sirena.Value
	value = func(raw sirena.Value) sirena.Value {
		switch v := raw.(type) {
		case sirena.String:
			return sirena.String{Value: v.Value}
		case sirena.Number:
			return sirena.Number{Value: v.Value}
		case sirena.Ident:
			return sirena.Ident{Value: v.Value}
		case sirena.List:
			values := make([]sirena.Value, len(v.Values))
			for i, item := range v.Values {
				values[i] = value(item)
			}
			return sirena.List{Values: values}
		}
		return raw
	}
	out := map[string]sirena.Value{}
	for key, raw := range metadata {
		out[key] = value(raw)
	}
	return out
}

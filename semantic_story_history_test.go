package slides

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestArchitectureHistoryPreservesExplicitCurationAndStableRevisions(t *testing.T) {
	before := ArchitectureSnapshot{ID: "v1", Source: "PRIVATE-source-path", Label: "PRIVATE-label", Sirena: []byte("service old { sid: \"api\" label: \"API\" }\n")}
	after := ArchitectureSnapshot{ID: "v2", Sirena: []byte("service renamed { sid: \"api\" label: \"API\" }\njob worker\nrenamed -> worker\n")}
	latest := ArchitectureSnapshot{ID: "v3", Sirena: []byte("service renamed { sid: \"api\" label: \"API\" }\njob worker\ndatabase db\nrenamed -> worker\nworker -> db\n")}
	text, empty, orphan := "We added a worker to isolate bursts. <authored> & explicit.", "", "Keep this removed revision explanation."
	curation := ArchitectureTourCuration{Version: 1, Explanations: map[string]ArchitectureTourExplanation{"v2": {FromID: "v1", BeforeCaption: &empty, AfterCaption: &text}, "retired": {AfterCaption: &orphan}}}
	tour, err := ArchitectureTourHistory([]ArchitectureSnapshot{before, after, latest}, ArchitectureTourOptions{Curation: &curation})
	if err != nil {
		t.Fatal(err)
	}
	if len(tour.Transitions) != 2 || len(tour.Snapshots) != 3 || tour.Transitions[0].Slide != "revision-v2" || len(tour.Transitions[0].Change.Changed) != 0 {
		t.Fatalf("unstable rename/revision comparison: %+v", tour.Transitions)
	}
	if strings.Contains(tour.Markdown, "PRIVATE") || strings.Contains(tour.StoryYAML, "PRIVATE") || len(tour.Snapshots[0].SHA256) != 64 {
		t.Fatal("private provenance published or hash missing")
	}
	if strings.Join(tour.OrphanedExplanations, ",") != "retired" || *tour.Curation.Explanations["retired"].AfterCaption != orphan {
		t.Fatal("orphan curation was silently dropped")
	}
	var manifest struct {
		Version int         `yaml:"version"`
		Beats   []StoryBeat `yaml:"beats"`
	}
	if err := yaml.Unmarshal([]byte(tour.StoryYAML), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Beats) != 4 || manifest.Beats[0].Caption != "" || manifest.Beats[1].Caption != text {
		t.Fatal("curated blank/text was overwritten", manifest)
	}
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": tour.Markdown, "story.yaml": tour.StoryYAML})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal("generated multi-revision tour does not compile", err)
	}
	if len(deck.Slides) != 2 || deck.Story.Beats[2].Slide != "revision-v3" {
		t.Fatal("absorbed transition separator or unstable slide address")
	}
	report, err := AssertStory(deck)
	if err != nil || len(report.Errors) > 0 {
		t.Fatal(report, err)
	}
	// Appending history and regenerating must preserve curation, without aliasing it.
	more := ArchitectureSnapshot{ID: "v4", Sirena: latest.Sirena}
	again, err := ArchitectureTourHistory([]ArchitectureSnapshot{before, after, latest, more}, ArchitectureTourOptions{Curation: &tour.Curation})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(again.StoryYAML, "We added a worker") || again.Transitions[0].Slide != tour.Transitions[0].Slide {
		t.Fatal("regeneration lost curation/address")
	}
	*tour.Curation.Explanations["v2"].AfterCaption = "changed result"
	if text != "We added a worker to isolate bursts. <authored> & explicit." || !strings.Contains(again.StoryYAML, text) {
		t.Fatal("curation result aliases author state")
	}
}

func TestArchitectureHistoryRejectsReorderedCurationAndBounds(t *testing.T) {
	base := []ArchitectureSnapshot{{ID: "v1", Sirena: []byte("service api\n")}, {ID: "v2", Sirena: []byte("service api\n")}}
	caption := "curated"
	for _, tc := range []struct {
		name      string
		snapshots []ArchitectureSnapshot
		options   ArchitectureTourOptions
		want      string
	}{
		{"count", base[:1], ArchitectureTourOptions{}, "2–32"},
		{"duplicate", []ArchitectureSnapshot{base[0], base[0]}, ArchitectureTourOptions{}, "unique"},
		{"source-bound", []ArchitectureSnapshot{base[0], {ID: "v2", Sirena: []byte(strings.Repeat(" ", maxSourceBytes+1))}}, ArchitectureTourOptions{}, "limits"},
		{"ambiguous-source", []ArchitectureSnapshot{base[0], {ID: "v2", Sirena: []byte("service api { sid: \"same\" }\nservice duplicate { sid: \"same\" }\n")}}, ArchitectureTourOptions{}, "v1 → v2"},
		{"reordered", base, ArchitectureTourOptions{Curation: &ArchitectureTourCuration{Version: 1, Explanations: map[string]ArchitectureTourExplanation{"v2": {FromID: "other", AfterCaption: &caption}}}}, "expects predecessor"},
		{"version", base, ArchitectureTourOptions{Curation: &ArchitectureTourCuration{Version: 2}}, "version 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ArchitectureTourHistory(tc.snapshots, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}

func TestArchitectureHistoryRejectsOutputBeyondAuthoringLimit(t *testing.T) {
	// Both inputs fit individually. Repeating snapshots in the rendered history
	// must not create a deck that its own source editor refuses to open or save.
	source := []byte("service api\n" + strings.Repeat("// This is an authored snapshot comment.\n", 14000))
	if len(source) > maxSourceBytes {
		t.Fatal("fixture exceeds individual source limit")
	}
	_, err := ArchitectureTourHistory([]ArchitectureSnapshot{{ID: "v1", Sirena: source}, {ID: "v2", Sirena: source}}, ArchitectureTourOptions{})
	if err == nil || !strings.Contains(err.Error(), "per-file authoring limit") {
		t.Fatal("oversized generated deck accepted", err)
	}
}

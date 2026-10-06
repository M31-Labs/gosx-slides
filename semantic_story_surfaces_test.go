package slides

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"m31labs.dev/mdpp"
)

func multiSurfaceFixture(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, name := range []string{"deck.md", "story.yaml", "runtime.sir", "deck.css"} {
		data, err := os.ReadFile(filepath.Join("examples/multi-surface-story", name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = strings.ReplaceAll(string(data), "\r\n", "\n")
	}
	return files
}

func TestSemanticStoryNamedSurfacesCompileAndQualifyOrigins(t *testing.T) {
	deck, err := LoadIslandDeck("examples/multi-surface-story")
	if err != nil {
		t.Fatal(err)
	}
	beat := deck.Story.Beats[1]
	if len(beat.SurfaceGraphKeys) != 3 || len(deck.Story.Graphs[beat.GraphKey].Actors) != 10 {
		t.Fatalf("lost surface graph ownership: %+v", beat)
	}
	for name, key := range beat.SurfaceGraphKeys {
		graph := deck.Story.Graphs[key]
		if graph.Surface != name || graph.ContainerID == "" {
			t.Fatalf("missing container identity: %+v", graph)
		}
		for _, actor := range graph.Actors {
			data, err := os.ReadFile(filepath.Join(deck.Dir, actor.File))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data[actor.Range.StartByte:actor.Range.EndByte]), actor.Name) {
				t.Fatalf("incorrect %s origin: %+v", name, actor)
			}
		}
	}
	assertions, err := AssertStory(deck)
	if err != nil || len(assertions.Errors) > 0 {
		t.Fatal(assertions, err)
	}
	graphics := compileDeckGraphics(deck)
	for _, ref := range deck.Slides[0].Components {
		if ref.Name != "Scene3D" {
			continue
		}
		cfg := graphics[graphicsKey(ref.Name, ref.Props)]
		if cfg.err != nil {
			t.Fatal(cfg.err)
		}
		var timeline graphicTimeline
		json.Unmarshal([]byte(cfg.config.MountAttrs["data-slide-steps"].(string)), &timeline)
		if len(timeline.Frames) != 3 || *timeline.Frames[1].DurationMS != 1200 {
			t.Fatal("native surface did not receive shared beat timing")
		}
	}
	// Same filename and actor IDs in two scenes must still have distinct commands.
	files := multiSurfaceFixture(t)
	// Build a small two-scene fixture without altering the comprehensive example.
	files["deck.md"] = "---\nstory: story.yaml\n---\n\n```yaml\nid: pressure\ncues: baseline, burst\n```\n\n:::story-surface {name=left}\n<Scene3D Src=\"runtime.sir\" />\n:::\n\n:::story-surface {name=right}\n<Scene3D Src=\"runtime.sir\" />\n:::\n"
	files["story.yaml"] = "version: 1\nbeats:\n  - slide: pressure\n    cue: burst\n    durationMs: 600\n    surfaces:\n      left: {camera: {x: 0, y: 0, z: 8, fov: 45}, reveal: [api]}\n      right: {camera: {x: 0, y: 0, z: 16, fov: 50}, reveal: [worker]}\n    expect:\n      visible: [left/api, right/worker]\n      hidden: [left/worker, right/api]\n"
	dir := t.TempDir()
	writeCompositionFiles(t, dir, files)
	two, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(two.storySceneSteps) != 2 {
		t.Fatal("same source scenes incorrectly shared pose program")
	}
	refs := two.Slides[0].Components
	if graphicsKey(refs[0].Name, refs[0].Props) == graphicsKey(refs[1].Name, refs[1].Props) {
		t.Fatal("named scene cache collision")
	}
	report, err := AssertStory(two)
	if err != nil || len(report.Errors) > 0 {
		t.Fatal(report, err)
	}
}

func TestSemanticStoryNamedSurfacesRejectAmbiguousTargets(t *testing.T) {
	for _, tc := range []struct{ name, file, old, new, want string }{
		{"unknown-surface", "story.yaml", "      load:\n        focus: [api]", "      ghost:\n        focus: [api]", "unknown story surface"},
		{"unqualified-assertion", "story.yaml", "map/api, runtime/api, load/api", "api, runtime/api, load/api", "unknown asserted target"},
		{"wrong-qualified-label", "story.yaml", "runtime/api: API", "runtime/api: Wrong label", "asserted actor label differs"},
		{"cross-surface-actor", "story.yaml", "focus: [api, buffer]", "focus: [runtime/api]", "unknown actor"},
		{"svg-camera", "story.yaml", "      load:\n        focus: [api]", "      load:\n        camera: {x: 0, y: 0, z: 10, fov: 45}", "camera needs"},
		{"duplicate-name", "deck.md", "name=map", "name=runtime", "duplicate story surface"},
		{"extra-unwrapped", "deck.md", "# The queue buys us time", "# The queue buys us time\n\n```sirena\nservice extra\n```", "every Sirena surface"},
		{"legacy-fields-on-named", "story.yaml", "    durationMs: 1200", "    durationMs: 1200\n    focus: [api]", "require scoped poses"},
		{"nested", "deck.md", ":::story-surface {name=map}", ":::story-surface {name=outer}\n:::story-surface {name=map}", "cannot be nested"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := multiSurfaceFixture(t)
			if !strings.Contains(files[tc.file], tc.old) {
				t.Fatal("fixture mutation did not match")
			}
			files[tc.file] = strings.Replace(files[tc.file], tc.old, tc.new, 1)
			dir := t.TempDir()
			writeCompositionFiles(t, dir, files)
			_, err := LoadIslandDeck(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSemanticStoryNamedSurfacesCRLFAndAbsoluteDefaults(t *testing.T) {
	files := multiSurfaceFixture(t)
	for name, source := range files {
		files[name] = strings.ReplaceAll(source, "\n", "\r\n")
	}
	dir := t.TempDir()
	writeCompositionFiles(t, dir, files)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	baseline := deck.Story.Beats[0]
	for name, key := range baseline.SurfaceGraphKeys {
		if len(storyPoseForGraph(baseline, deck.Story.Graphs[key]).Focus) > 0 {
			t.Fatalf("%s inherited a later pose", name)
		}
	}
	for _, actor := range deck.Story.Graphs[deck.Story.Beats[1].GraphKey].Actors {
		data := []byte(files[actor.File])
		if !strings.Contains(string(data[actor.Range.StartByte:actor.Range.EndByte]), actor.Name) {
			t.Fatal("CRLF origin lost", actor)
		}
	}
	files["story.yaml"] = strings.Replace(files["story.yaml"], "focus: [api]", "reveal: []", 1)
	writeCompositionFiles(t, dir, files)
	deck, err = LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if storyActorVisible(deck.Story, deck.Story.Beats[1], "load/api") {
		t.Fatal("explicit empty reveal became baseline")
	}
}

func TestSemanticStoryAudiencePayloadOmitsPrivateProvenanceAndNotes(t *testing.T) {
	files := multiSurfaceFixture(t)
	files["deck.md"] += "\n<Notes>PRIVATE presenter instructions that must never become captions.</Notes>\n"
	dir := t.TempDir()
	writeCompositionFiles(t, dir, files)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	assets := semanticStoryAssets(deck)
	for _, private := range []string{"PRIVATE", "runtime.sir", `"file":`, `"range":`} {
		if strings.Contains(assets, private) {
			t.Fatalf("audience metadata contains %q", private)
		}
	}
	if !strings.Contains(assets, `"runtime"`) || !strings.Contains(assets, "The buffer protects the worker") {
		t.Fatal("privacy filtering removed authored semantic effects/captions")
	}
	if deck.Story.File != "story.yaml" || deck.Story.Beats[1].Source.File != "story.yaml" {
		t.Fatal("public payload mutation destroyed inspect provenance")
	}
}

func TestSemanticStoryNamedGraphSharingAndAudienceRemap(t *testing.T) {
	files := multiSurfaceFixture(t)
	// Many cues on one static surface share canonical graph data.
	var cues []string
	var beats strings.Builder
	beats.WriteString("version: 1\nbeats:\n")
	for i := 0; i < 100; i++ {
		cue := "cue" + strconv.Itoa(i)
		cues = append(cues, cue)
		beats.WriteString("  - slide: pressure\n    cue: " + cue + "\n    durationMs: 0\n")
	}
	files["deck.md"] = "---\nstory: story.yaml\n---\n\n```yaml\nid: pressure\ncues: " + strings.Join(cues, ", ") + "\n```\n\n:::story-surface {name=map}\n```sirena\nservice api\n```\n:::\n"
	files["story.yaml"] = beats.String()
	dir := t.TempDir()
	writeCompositionFiles(t, dir, files)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Story.Graphs) != 2 {
		t.Fatal("static surfaces duplicate actor metadata at every cue", len(deck.Story.Graphs))
	}
	files = multiSurfaceFixture(t)
	files["deck.md"] = strings.Replace(files["deck.md"], "```yaml\nid: pressure", "```yaml\nid: intro\naudiences: [leaders]\n```\n\n# Leaders\n\n<!-- Intro -->\n\n---\n\n```yaml\nid: pressure\naudiences: [engineers]", 1)
	dir = t.TempDir()
	writeCompositionFiles(t, dir, files)
	deck, err = LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := SelectAudience(deck, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Slides) != 1 || filtered.Story.Beats[1].SlideIndex != 0 {
		t.Fatal("named story was not reindexed")
	}
	report, err := AssertStory(filtered)
	if err != nil || len(report.Errors) > 0 {
		t.Fatal("qualified targets failed after audience selection", report, err)
	}
	for _, key := range filtered.Story.Beats[1].SurfaceGraphKeys {
		graph := filtered.Story.Graphs[key]
		if graph.SlideIndex != 0 {
			t.Fatal("surface retained original slide index")
		}
		found := false
		for _, node := range filtered.Slides[0].Node.Find(mdpp.NodeContainerDirective) {
			found = found || node.Attr("id") == graph.ContainerID
		}
		if !found {
			t.Fatal("audience selection lost the authored surface boundary")
		}
	}
}

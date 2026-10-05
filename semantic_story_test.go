package slides

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticStoryCompilesNativeTimelineAndOrigins(t *testing.T) {
	deck, err := LoadIslandDeck("examples/semantic-story")
	if err != nil {
		t.Fatal(err)
	}
	if deck.Story == nil || len(deck.Story.Beats) != 3 {
		t.Fatal("missing story manifest")
	}
	beat := deck.Story.Beats[1]
	if beat.SlideIndex != 0 || beat.Step != 1 || beat.Source.File != "story.yaml" || beat.Source.Range.StartLine != 12 {
		t.Fatalf("bad beat address/source %+v", beat)
	}
	graph := deck.Story.Graphs[beat.GraphKey]
	if !graph.Scene || len(graph.Actors) != 4 || !storyHasEdge(graph, "api", "worker") {
		t.Fatal("missing parsed graph", graph)
	}
	source, _ := os.ReadFile(filepath.Join(deck.Dir, "request.sir"))
	for _, actor := range graph.Actors {
		if !strings.Contains(string(source[actor.Range.StartByte:actor.Range.EndByte]), actor.Name) || actor.Range.StartLine < 1 {
			t.Fatal("bad actor source range", actor)
		}
	}
	graphics := compileDeckGraphics(deck)
	ref := deck.Slides[0].Components[0]
	cfg := graphics[graphicsKey(ref.Name, ref.Props)]
	if cfg.err != nil {
		t.Fatal(cfg.err)
	}
	raw, _ := cfg.config.MountAttrs["data-slide-steps"].(string)
	var timeline graphicTimeline
	if err := json.Unmarshal([]byte(raw), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Frames) != 3 || timeline.Frames[1].DurationMS == nil || *timeline.Frames[1].DurationMS != 1200 || timeline.Frames[0].DurationMS == nil || *timeline.Frames[0].DurationMS != 0 {
		t.Fatal("lost generated absolute timing", timeline)
	}
	found := false
	for _, command := range timeline.Frames[1].Commands {
		if command.ObjectID == "api" {
			found = true
		}
	}
	if !found {
		t.Fatal("story did not generate actor commands")
	}
	assertions, err := AssertStory(deck)
	if err != nil || len(assertions.Errors) != 0 || assertions.Checks < 10 {
		t.Fatal("valid assertions failed", assertions, err)
	}
	if Analyze(deck).Story == nil || !containsString(Analyze(deck).SourceFiles, "story.yaml") {
		t.Fatal("inspect lost compiled story")
	}
}

func TestSemanticStoryRejectsInvalidReferencesAndBoundaries(t *testing.T) {
	baseDeck, _ := os.ReadFile("examples/semantic-story/deck.md")
	manifest, _ := os.ReadFile("examples/semantic-story/story.yaml")
	graph, _ := os.ReadFile("examples/semantic-story/request.sir")
	for _, tc := range []struct{ name, source, want string }{
		{"slide", strings.Replace(string(manifest), "slide: request", "slide: missing", 1), "unknown slide"},
		{"cue", strings.Replace(string(manifest), "cue: accepted", "cue: missing", 1), "unknown cue"},
		{"actor", strings.Replace(string(manifest), "focus: [api]", "focus: [ghost]", 1), "unknown actor"},
		{"path", strings.Replace(string(manifest), "trace: [browser, api, worker]", "trace: [browser, db]", 1), "no relationship"},
		{"code", strings.Replace(string(manifest), "lines: [1, 2]", "lines: [90]", 1), "code line outside"},
		{"dom", strings.Replace(string(manifest), "hide: [completion]", "hide: [missing]", 1), "unknown DOM"},
		{"camera", strings.Replace(string(manifest), "fov: 45", "fov: .nan", 1), "finite"},
		{"unknown", string(manifest) + "\nunknown: true\n", "field unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCompositionFiles(t, dir, map[string]string{"deck.md": string(baseDeck), "story.yaml": tc.source, "request.sir": string(graph)})
			_, err := LoadIslandDeck(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal("invalid story accepted", err)
			}
			if tc.name != "unknown" && !strings.Contains(err.Error(), "story.yaml:") {
				t.Fatal("finding lost manifest source", err)
			}
		})
	}
	dir := t.TempDir()
	outside := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": string(baseDeck), "request.sir": string(graph)})
	os.WriteFile(filepath.Join(outside, "story.yaml"), manifest, 0644)
	os.Symlink(filepath.Join(outside, "story.yaml"), filepath.Join(dir, "story.yaml"))
	if _, err := LoadIslandDeck(dir); err == nil {
		t.Fatal("story manifest escaped deck via symlink")
	}
}

func TestArchitectureTourUsesStableIDsAndCompiles(t *testing.T) {
	before := []byte("service oldname { sid: \"api\" label: \"Old API\" }\njob gone\noldname -> gone\n")
	after := []byte("service newname { sid: \"api\" label: \"New API\" }\njob worker\nnewname -> worker\n")
	tour, err := ArchitectureTour(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(tour.Added, ",") != "worker" || strings.Join(tour.Removed, ",") != "gone" || strings.Join(tour.Changed, ",") != "api" {
		t.Fatal("wrong structural actor changes", tour)
	}
	if len(tour.RelationshipsAdded) != 1 || len(tour.RelationshipsRemoved) != 1 || !strings.Contains(tour.StoryYAML, "relationships") {
		t.Fatal("tour lost dependency changes", tour)
	}
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": tour.Markdown, "story.yaml": tour.StoryYAML})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal("generated tour does not compile", err)
	}
	if len(deck.Story.Beats) != 2 || deck.Story.Beats[0].GraphKey == deck.Story.Beats[1].GraphKey {
		t.Fatal("tour snapshots lost distinct graphs")
	}
	for _, beat := range deck.Story.Beats {
		for _, actor := range deck.Story.Graphs[beat.GraphKey].Actors {
			if actor.File != DeckFileName || !strings.Contains(tour.Markdown[actor.Range.StartByte:actor.Range.EndByte], actor.Name) {
				t.Fatal("tour actor range lost source", actor)
			}
		}
	}
	if _, err := ArchitectureTour([]byte("service a { sid: \"same\" }\nservice b { sid: \"same\" }"), after); err == nil {
		t.Fatal("ambiguous stable IDs accepted")
	}
}

func TestSemanticStoryInlineIncludedCRLFOrigins(t *testing.T) {
	graph := "  service api { label: \"API\" }\r\n  job worker\r\n  api -> worker\r\n"
	fragment := "```yaml\r\nid: request\r\ncues: overview, accepted\r\n```\r\n\r\n# Inline story\r\n\r\n  ```sirena\r\n" + graph + "  ```\r\n"
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":             "---\r\nstory: story.yaml\r\n---\r\n\r\n<!-- slides:include chapters/request.md -->\r\n",
		"chapters/request.md": fragment,
		"story.yaml":          "version: 1\r\nbeats:\r\n  - slide: request\r\n    cue: accepted\r\n    focus: [api]\r\n    trace: [api, worker]\r\n"})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, actor := range deck.Story.Graphs[deck.Story.Beats[0].GraphKey].Actors {
		if actor.File != "chapters/request.md" || !strings.HasPrefix(fragment[actor.Range.StartByte:actor.Range.EndByte], map[string]string{"api": "service api", "worker": "job worker"}[actor.ID]) {
			t.Fatal("fenced actor range lost indentation/CRLF origin", actor)
		}
	}
	if deck.Story.Beats[0].Source.Range.StartLine != 3 {
		t.Fatal("manifest CRLF source range lost")
	}
}

func TestSemanticStoryAssertionsValidateLinksSnippetsAndBaseline(t *testing.T) {
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":    "---\nstory: story.yaml\n---\n\n```yaml\nid: request\ncues: overview, accepted\n```\n\n# Assertions\n\n<p id=\"initially-hidden\" hidden>Hidden</p>\n\n```go\n<<< ./sample.go 2-3\n```\n\n[Missing asset](./missing.txt)\n\n[Broken cue](#request/nope)\n",
		"sample.go":  "one\ntwo\nthree\n",
		"story.yaml": "version: 1\nbeats:\n  - slide: request\n    cue: overview\n    code: {block: 0, lines: [1, 2]}\n    expect: {hidden: [initially-hidden]}\n  - slide: request\n    cue: accepted\n    show: [initially-hidden]\n    expect: {visible: [initially-hidden]}\n"})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	report, err := AssertStory(deck)
	if err != nil || len(report.Errors) != 2 || !strings.Contains(strings.Join(report.Errors, " "), "nope") || !strings.Contains(strings.Join(report.Errors, " "), "missing.txt") {
		t.Fatal("assertions missed source/link failures", report, err)
	}
	os.Remove(filepath.Join(dir, "sample.go"))
	if _, err := CompileStory(deck); err == nil || !strings.Contains(err.Error(), "code source") {
		t.Fatal("missing code source accepted", err)
	}
}

func TestSemanticStoryRejectsAmbiguousAndSanitizedDOMTargets(t *testing.T) {
	for _, content := range []string{"<p id=\"target\">One</p>\n\n<p id=\"target\">Two</p>", "<script id=\"target\">unsafe()</script>"} {
		dir := t.TempDir()
		writeCompositionFiles(t, dir, map[string]string{"deck.md": "---\nstory: story.yaml\n---\n\n```yaml\nid: request\ncues: overview\n```\n\n# Targets\n\n" + content + "\n", "story.yaml": "version: 1\nbeats:\n  - slide: request\n    cue: overview\n    show: [target]\n"})
		if _, err := LoadIslandDeck(dir); err == nil {
			t.Fatal("invalid DOM target accepted", content)
		}
	}
}

func TestSemanticStoryExplicitEmptyReveal(t *testing.T) {
	deckSource, _ := os.ReadFile("examples/semantic-story/deck.md")
	manifest, _ := os.ReadFile("examples/semantic-story/story.yaml")
	graph, _ := os.ReadFile("examples/semantic-story/request.sir")
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": string(deckSource), "story.yaml": strings.Replace(string(manifest), "reveal: [browser, api, worker]", "reveal: []", 1), "request.sir": string(graph)})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref := deck.Slides[0].Components[0]
	var steps []map[string]any
	json.Unmarshal(deck.storySceneSteps[graphicsKey(ref.Name, ref.Props)], &steps)
	reveal, ok := steps[1]["reveal"].([]any)
	if !ok || len(reveal) != 0 {
		t.Fatal("empty reveal must differ from omitted reveal", steps[1])
	}
	report, err := AssertStory(deck)
	if err != nil || len(report.Errors) == 0 {
		t.Fatal("assertions should catch explicitly hidden actors", report, err)
	}
}

func TestSemanticStoryRenderedAssertions(t *testing.T) {
	if _, err := findChrome(); err != nil {
		t.Skip("Chrome is unavailable")
	}
	deck, err := LoadIslandDeck("examples/semantic-story")
	if err != nil {
		t.Fatal(err)
	}
	report, err := AssertStoryBrowser(deck)
	if err != nil || len(report.Errors) != 0 || report.RenderedStates != 18 {
		t.Fatal("rendered story assertions failed", report, err)
	}
}

func TestArchitectureTourDetectsBoundaryMovesAndStableRename(t *testing.T) {
	before := []byte("boundary deployment \"oldzone\" { sid: \"cluster\" } {\n service a { sid: \"api\" label: \"API\" }\n}\njob b\na -> b\n")
	after := []byte("boundary deployment \"newzone\" { sid: \"cluster\" } {\n service renamed { sid: \"api\" label: \"API\" }\n}\njob b\nrenamed -> b\n")
	tour, err := ArchitectureTour(before, after)
	if err != nil || len(tour.Changed) != 0 || len(tour.RelationshipsAdded) != 0 || len(tour.RelationshipsRemoved) != 0 {
		t.Fatal("stable rename must preserve structural identity", tour, err)
	}
	moved := []byte("boundary deployment \"other\" {\n service renamed { sid: \"api\" label: \"API\" }\n}\njob b\nrenamed -> b\n")
	tour, err = ArchitectureTour(before, moved)
	if err != nil || strings.Join(tour.Changed, ",") != "api" {
		t.Fatal("actor boundary move must be explained", tour, err)
	}
}

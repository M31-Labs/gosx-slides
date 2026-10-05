package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

func TestStoryCaptionsUseExactStepAndPreserveExplicitEmpty(t *testing.T) {
	deck, err := parseIslandDeck(t.TempDir(), []byte("```yaml\ncaption: Slide fallback\n```\n\n# Caption\n\n<!-- private notes -->\n"))
	if err != nil {
		t.Fatal(err)
	}
	deck.Story = &CompiledStory{Beats: []CompiledStoryBeat{
		{StoryBeat: StoryBeat{Caption: "First cue </script>"}, SlideIndex: 0, Step: 0},
		{StoryBeat: StoryBeat{Caption: ""}, SlideIndex: 0, Step: 1},
	}}
	for step, expected := range map[int]string{0: "First cue </script>", 1: "", 2: "Slide fallback"} {
		if actual := deckRecordingCaption(deck, deck.Slides[0], step); actual != expected {
			t.Fatalf("step %d: caption %q != %q", step, actual, expected)
		}
	}
	markup := gosx.RenderHTML(recordingMetadata(deck))
	if strings.Contains(markup, "private notes") || strings.Contains(markup, "First cue </script>") || !strings.Contains(markup, `"beats":{"0":`) {
		t.Fatal("recording metadata must retain addressed captions and safe script escaping", markup)
	}
}

func TestAudienceProgramStagingRemovesExcludedOrphans(t *testing.T) {
	dir := t.TempDir()
	source := "# Shared\n\n<Counter/>\n\n<!-- split -->\n\n---\n\n```yaml\naudiences: engineers\n```\n\n# Engineering\n\n<Secret/>\n"
	for name, data := range map[string]string{
		"deck.md":     source,
		"Counter.gsx": "package main\n//gosx:island\nfunc Counter(props any) Node {\n return <span>Public</span>\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Declare another audience so its variant consists of shared content only.
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte("---\naudiences: engineers, leaders\n---\n\n"+source), 0644); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeckAudience(dir, "leaders")
	if err != nil {
		t.Fatal(err)
	}
	islands := filepath.Join(dir, "build", "islands")
	if err := os.MkdirAll(islands, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(islands, "Secret.json"), []byte("old private program"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stageDeckIslandPrograms(deck); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(islands, "Secret.json")); !os.IsNotExist(err) {
		t.Fatal("excluded component remains publishable", err)
	}
	if _, err := os.Stat(filepath.Join(islands, "Counter.json")); err != nil {
		t.Fatal("shared live component lost its staged program", err)
	}
}

func TestAudienceLiveRuntimeCannotResurrectExcludedPrograms(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte("---\naudiences: leaders, engineers\n---\n\n# Shared\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"build/islands/Secret.json", "build/islands/nested/Secret.json", "assets/islands/hash.json", "assets/css/Secret.css", "build/css/Secret.css"} {
		name := filepath.Join(dir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte("EXCLUDED PRIVATE PROGRAM"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	deck, err := LoadIslandDeckAudience(dir, "leaders")
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	app.SetRuntimeRoot(dir)
	handler := app.Build()
	for _, route := range []string{"/gosx/islands/Secret.json", "/gosx/islands/nested/Secret.json", "/gosx/assets/islands/hash.json", "/gosx/assets/css/Secret.css", "/gosx/css/Secret.css"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "EXCLUDED PRIVATE PROGRAM") {
			t.Fatal("selected deck exposed a cached full-deck asset", route, recorder.Code)
		}
	}
}

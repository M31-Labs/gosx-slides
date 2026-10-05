package slides

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const storySource = "---\ntitle: Story\n---\n\n```yaml\nid: request\ncues: arrived, accepted\n```\n\n# Café 🦊\n\n:::motion {cue=accepted after=arrived}\nA cue.\n:::\n\n[Jump](#request/accepted)\n\n```text\naccepted request worker\n```\n\n```sirena\narchitecture {\n  service worker \"Keep worker label\"\n  service api\n  worker -> api\n}\n```\n\n<!-- notes -->\n\n---\n\n```yaml\nid: recovery\ncues: accepted\n```\n\n# Recovery\n\n:::motion {cue=accepted}\nOther scope.\n:::\n\n```sirena\narchitecture {\n  service worker\n}\n```\n"

func TestSourceStoryAnalysisAndScopedRename(t *testing.T) {
	report, idx, err := analyzeSource(storySource)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Outline) != 2 || report.Outline[0].Title != "Café 🦊" || report.Outline[1].Title != "Recovery" {
		t.Fatalf("wrong outline: %+v", report.Outline)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("valid story diagnostics: %+v", report.Diagnostics)
	}
	start := strings.Index(storySource, "accepted after")
	target, ok := idx.At(start)
	if !ok || target.Name != "accepted" || target.Kind != "cue" {
		t.Fatal("missing ranged cue")
	}
	if got := len(idx.References(target, true)); got != 3 {
		t.Fatalf("cue occurrences: %d", got)
	}
	renamed, err := renameSource(storySource, start, "processed")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"cues: arrived, processed", "cue=processed after=arrived", "#request/processed", "```text\naccepted request worker", "id: recovery\ncues: accepted", "cue=accepted}\nOther scope"} {
		if !strings.Contains(renamed, part) {
			t.Fatalf("rename lost scope or formatting: %q", part)
		}
	}
	renamed, err = renameSource(storySource, strings.Index(storySource, "service worker")+len("service "), "runner")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(renamed, "service runner \"Keep worker label\"") || !strings.Contains(renamed, "runner -> api") || !strings.Contains(renamed, "architecture {\n  service worker\n}") {
		t.Fatal("actor rename escaped its parsed fence", renamed)
	}
	renamed, err = renameSource(storySource, strings.Index(storySource, "id: request")+len("id: "), "incoming")
	if err != nil || !strings.Contains(renamed, "#incoming/accepted") || !strings.Contains(renamed, "```text\naccepted request worker") {
		t.Fatal("slide rename did not preserve code", err)
	}
	for _, attempt := range []struct {
		source string
		offset int
		name   string
	}{
		{storySource, start, "arrived"},
		{storySource, start, "bad name"},
		{storySource, strings.Index(storySource, "```text") + 8, "renamed"},
		{strings.ReplaceAll(storySource, "\n", "\r\n"), start, "renamed"},
		{strings.Replace(storySource, "id: recovery", "id: request", 1), strings.Index(storySource, "id: request") + 4, "renamed"},
	} {
		if _, err := renameSource(attempt.source, attempt.offset, attempt.name); err == nil {
			t.Fatal("unsafe rename succeeded", attempt)
		}
	}
}

func TestSourceStoryDiagnosticsReachInspectAndValidate(t *testing.T) {
	source := strings.Replace(storySource, "after=arrived", "after=missing", 1)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, DeckFileName), []byte(source), 0644)
	d, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(d, ValidateOptions{})
	found := false
	for _, diagnostic := range report.Analysis.Diagnostics {
		if diagnostic.Code == "STORY-UNRESOLVED" && strings.Contains(diagnostic.Message, "missing") {
			found = true
			if diagnostic.Range.StartByte != strings.Index(source, "after=missing")+6 {
				t.Fatal("diagnostic range does not select dependency")
			}
		}
	}
	if !found || report.Passed(true) {
		t.Fatal("strict validation missed unresolved cue", report)
	}
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "STORY-UNRESOLVED") {
		t.Fatal("CLI warnings missed story diagnostic")
	}
}

func TestSourceToolsAnalyzeDraftWithoutWritesAndSecureOrigin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DeckFileName)
	os.WriteFile(path, []byte("# Saved\n"), 0644)
	d, _ := LoadIslandDeck(dir)
	app, err := d.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/_slides/source", nil))
	var state map[string]string
	json.Unmarshal(get.Body.Bytes(), &state)
	request := func(host, origin, token string, payload any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", "http://"+host+"/_slides/analyze", bytes.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Slides-Token", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	input := map[string]any{"source": storySource, "rename": map[string]any{"offset": strings.Index(storySource, "accepted after"), "name": "processed"}}
	for _, auth := range [][3]string{{"evil.example", "http://evil.example", state["token"]}, {"localhost", "http://evil.example", state["token"]}, {"localhost", "http://localhost", ""}, {"localhost", "", state["token"]}} {
		if w := request(auth[0], auth[1], auth[2], input); w.Code != 403 {
			t.Fatalf("draft origin bypass: %d", w.Code)
		}
	}
	w := request("localhost", "http://localhost", state["token"], input)
	var result sourceAnalysis
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Source == nil || !strings.Contains(*result.Source, "#request/processed") {
		t.Fatal("draft rename failed", w.Code, w.Body.String())
	}
	saved, _ := os.ReadFile(path)
	if string(saved) != "# Saved\n" {
		t.Fatal("draft analysis wrote to disk")
	}
	input["extra"] = true
	if w := request("localhost", "http://localhost", state["token"], input); w.Code != 400 {
		t.Fatal("unexpected JSON fields accepted")
	}
	for _, opts := range []ServeOptions{{}, {Edit: true, Static: true}} {
		app, _ := d.NewServer(opts)
		w := httptest.NewRecorder()
		app.Build().ServeHTTP(w, httptest.NewRequest("POST", "http://localhost/_slides/analyze", nil))
		if w.Code == 200 || w.Code == 403 {
			t.Fatal("draft endpoint exposed without editor")
		}
	}
}

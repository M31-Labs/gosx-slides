package slides

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackgroundPresetsCompileBothBackends(t *testing.T) {
	for _, preset := range BackgroundPresets() {
		t.Run(preset.Name, func(t *testing.T) {
			options := preset.Defaults
			options.Speed = 0
			options.Ink, options.Glow = "#123456", "#abcdef"
			config, err := compileGraphic(t.TempDir(), shaderBackgroundRef("shader:"+preset.Name, backgroundValues(options)))
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(config.Props, &payload); err != nil {
				t.Fatal(err)
			}
			object := payload["scene"].(map[string]any)["objects"].([]any)[0].(map[string]any)
			if object["customFragment"] == "" || object["customFragmentWGSL"] == "" || object["shaderBackend"] != "selena" {
				t.Fatal("missing native shaders", object)
			}
		})
	}
}

func TestBackgroundPresetsShareOnlyEqualSettings(t *testing.T) {
	if shaderBackgroundRef("shader:aurora", nil) != shaderBackgroundRef("shader:aurora", map[string]string{"shader-speed": "0.1800", "shader-ink": "#08121F"}) {
		t.Fatal("equal effective settings should share a surface")
	}
	deck := graphicsDeck(t, "---\nscene: shader:aurora\nshader-speed: 0\n---\n\n# One\n\n<!-- notes -->\n\n---\n\n# Two\n\n<!-- notes -->\n\n---\n\n```yaml\nshader-glow: '#eeaa88'\n```\n\n# Three\n", nil)
	refs := deckGraphicRefs(deck)
	if len(refs) != 3 || refs[0] != refs[1] || refs[0] == refs[2] {
		t.Fatalf("background identity: %+v", refs)
	}
	body := graphicsBody(t, deck)
	if strings.Count(body, `class="deck-graphics-background"`) != 2 {
		t.Fatal("distinct backgrounds must each mount once")
	}
	if strings.Contains(body, `class="graphic-error"`) || graphicsManifest(t, body)["runtime"].(map[string]any)["path"] != "" {
		t.Fatal("preset compilation failed or needs WASM")
	}
}

func TestBackgroundWizardRefusesAmbiguousMetadata(t *testing.T) {
	for _, source := range []string{"scene: [one, two]\n", "scene: |\n  custom\n", "scene: false\nscene: true\n", "{scene: false, title: Talk}\n"} {
		if _, err := updateBackgroundYAML(source, backgroundValues(BackgroundPresets()[0].Defaults)); err == nil {
			t.Fatalf("ambiguous metadata accepted: %s", source)
		}
	}
	deck := graphicsDeck(t, "# Windows\r\n", nil)
	project, _ := NewAuthorProject(deck.Dir)
	if _, err := findBackgroundTarget(project, "slide", 0); err == nil || !strings.Contains(err.Error(), "LF line endings") {
		t.Fatalf("expected an explicit CRLF diagnostic, got %v", err)
	}
}

func TestBackgroundWizardKeepsOriginalMetadataAndIncludes(t *testing.T) {
	root := "---\ntitle: 'A café 🦊' # stay\nscene: parse-forest # replace\ncustom: [one, two]\n---\n\n# Root\n\n<!-- private note -->\n\n---\n\n<!-- slides:include parts/body.md -->\n"
	part := "```yaml\nid: details\nlayout: center # layout\nscene: false # scene\n```\n\n# Details\n\n```yaml\nscene: example-in-code\n```\n\n<!-- keep this note -->\n"
	deck := graphicsDeck(t, root, map[string]string{"parts/body.md": part})
	project, err := NewAuthorProject(deck.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"deck", "slide"} {
		target, err := findBackgroundTarget(project, scope, 1)
		if err != nil {
			t.Fatal(err)
		}
		source, err := backgroundEditedSource(target, BackgroundPresets()[1].Defaults, scope)
		if err != nil {
			t.Fatal(err)
		}
		if scope == "deck" {
			if target.document.Path != "deck.md" || !strings.Contains(source, "title: 'A café 🦊' # stay") || !strings.Contains(source, "custom: [one, two]") || !strings.Contains(source, "# replace") || !strings.Contains(source, "<!-- private note -->") {
				t.Fatal("lost deck metadata", source)
			}
		} else {
			if target.document.Path != "parts/body.md" || !strings.Contains(source, "id: details\nlayout: center # layout") || !strings.Contains(source, "scene: example-in-code") || !strings.Contains(source, "<!-- keep this note -->") {
				t.Fatal("lost included source", source, target.document.Path)
			}
		}
		_, err = project.Write(ProjectEdit{File: target.document.Path, Source: source, Revision: target.document.Revision, ContextRevision: target.document.ContextRevision})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestBackgroundWizardInsertsMissingMetadata(t *testing.T) {
	for _, scope := range []string{"deck", "slide"} {
		deck := graphicsDeck(t, "# Café 🦊\n\nSome text\n\n<!-- keep -->\n\n---\n\n# Two\n", nil)
		project, _ := NewAuthorProject(deck.Dir)
		target, err := findBackgroundTarget(project, scope, 0)
		if err != nil {
			t.Fatal(err)
		}
		source, err := backgroundEditedSource(target, BackgroundPresets()[0].Defaults, scope)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(source, "# Café 🦊\n\nSome text") || !strings.Contains(source, "<!-- keep -->") {
			t.Fatal("changed content", source)
		}
		fresh, err := parseIslandDeck(deck.Dir, []byte(source))
		if err != nil || len(fresh.Slides) != 2 {
			t.Fatalf("source edit lost slides: %v\n%s", err, source)
		}
		if _, err := compileDeckProgram(fresh); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBackgroundWizardRejectsInvalidAndConflictingWrites(t *testing.T) {
	deck := graphicsDeck(t, "# Background\n\n<!-- note -->\n", nil)
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", "http://127.0.0.1/_slides/background?scope=slide&slide=0", nil))
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	var state struct{ File, Revision, ContextRevision, Token string }
	if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"scope": "slide", "slide": 0, "file": state.File, "revision": state.Revision, "contextRevision": state.ContextRevision, "options": BackgroundPresets()[0].Defaults}
	write := func(origin, token string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("PUT", "http://127.0.0.1/_slides/background", strings.NewReader(string(raw)))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Slides-Token", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := write("http://foreign.example", state.Token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := write("http://127.0.0.1", "bad"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	validWrite := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("PUT", "http://127.0.0.1/_slides/background", strings.NewReader(string(raw)))
		r.Header.Set("Origin", "http://127.0.0.1")
		r.Header.Set("X-Slides-Token", state.Token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	invalid := BackgroundPresets()[0].Defaults
	invalid.Preset = "../outside"
	body["options"] = invalid
	if w := validWrite(); w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	body["options"] = BackgroundPresets()[0].Defaults
	if err := os.WriteFile(filepath.Join(deck.Dir, "deck.md"), []byte("# External edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if w := validWrite(); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, options := range []BackgroundOptions{{Preset: "aurora", Ink: "red", Glow: "#ffffff", Scale: 1}, {Preset: "aurora", Ink: "#ffffff", Glow: "#000000", Scale: 1, Speed: 3}} {
		if _, err := BackgroundShaderSource(options); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}

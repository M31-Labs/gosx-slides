package slides

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/fsnotify/fsnotify"
)

const testShader = `material Ink { param gain : float = 1.0 surface(geo) -> color { return rgb(geo.uv.x * gain, 0.4, 0.8) } }`

func graphicsDeck(t *testing.T, source string, files map[string]string) *IslandDeck {
	t.Helper()
	dir := newDeckDirUnderModule(t, source, nil)
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	return deck
}
func graphicsBody(t *testing.T, deck *IslandDeck) string {
	t.Helper()
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	return rec.Body.String()
}
func graphicsManifest(t *testing.T, body string) map[string]any {
	t.Helper()
	match := regexp.MustCompile(`(?s)<script[^>]*id="gosx-manifest"[^>]*>(.*?)</script>`).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatal("missing graphics manifest")
	}
	var manifest map[string]any
	if err := json.Unmarshal([]byte(match[1]), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}
func TestShaderGraphicsTransportAndSharedBackground(t *testing.T) {
	deck := graphicsDeck(t, "---\nscene: ink.sel\n---\n\n# One\n\n<Shader Src=\"ink.sel\" Shape=\"sphere\" Uniforms=\"uniforms.json\" />\n\n<!-- note -->\n\n---\n\n# Two\n", map[string]string{"ink.sel": testShader, "uniforms.json": `{"gain":0.75}`})
	body := graphicsBody(t, deck)
	if strings.Contains(body, "graphic error:") || strings.Contains(body, "data-gosx-unresolved") {
		t.Fatal("graphic failed to render")
	}
	manifest := graphicsManifest(t, body)
	engines := manifest["engines"].([]any)
	if len(engines) != 2 {
		t.Fatalf("engines=%d, want one illustration and one shared background", len(engines))
	}
	for _, raw := range engines {
		props := raw.(map[string]any)["props"].(map[string]any)
		if props["maxFrameRate"] != float64(30) || props["maxPixels"] != float64(2_000_000) {
			t.Fatal("missing graphics budgets")
		}
		object := props["scene"].(map[string]any)["objects"].([]any)[0].(map[string]any)
		if object["shaderBackend"] != "selena" || object["customFragment"] == "" || object["customFragmentWGSL"] == "" {
			t.Fatal("shader stages missing from engine payload")
		}
	}
	if manifest["runtime"].(map[string]any)["path"] != "" {
		t.Fatal("native shader deck unexpectedly loads WASM bridge")
	}
	if len(Analyze(deck).Graphics) != 2 {
		t.Fatal("graphics absent from inventory")
	}
}
func TestSceneShaderTargets(t *testing.T) {
	deck := graphicsDeck(t, "# Diagram\n\n<Scene3D Src=\"graph.json\" Shader=\"ink.sel\" Targets=\"a\" />\n", map[string]string{"ink.sel": testShader, "graph.json": `{"scene":{"objects":[{"id":"a","kind":"sphere","radius":1},{"id":"b","kind":"box","size":1}]}}`})
	manifest := graphicsManifest(t, graphicsBody(t, deck))
	props := manifest["engines"].([]any)[0].(map[string]any)["props"].(map[string]any)
	objects := props["scene"].(map[string]any)["objects"].([]any)
	if objects[0].(map[string]any)["shaderBackend"] != "selena" {
		t.Fatal("target shader not applied")
	}
	if _, ok := objects[1].(map[string]any)["shaderBackend"]; ok {
		t.Fatal("non-target object changed")
	}
	if _, ok := props["scene"].(map[string]any)["backendCaps"]; !ok {
		t.Fatal("shader backend contract missing")
	}
}

func TestSceneShaderRejectsMalformedTargetIDs(t *testing.T) {
	for _, id := range []string{`[]`, `{}`, `17`, `null`} {
		t.Run(id, func(t *testing.T) {
			deck := graphicsDeck(t, "# Invalid\n", map[string]string{"ink.sel": testShader, "graph.json": `{"scene":{"objects":[{"id":` + id + `,"kind":"sphere","radius":1}]}}`})
			_, err := compileGraphic(deck.Dir, ComponentRef{Name: "Scene3D", Props: `Src="graph.json" Shader="ink.sel" Targets="a"`})
			if err == nil || !strings.Contains(err.Error(), "object id must be a string") {
				t.Fatalf("malformed target id %s: %v", id, err)
			}
		})
	}
}
func TestGraphicFailuresAndPathBoundary(t *testing.T) {
	for _, files := range []map[string]string{{"bad.sel": "material Broken {"}, {"bad.sel": testShader}} {
		source := "# Bad\n\n<Shader Src=\"bad.sel\" Shape=\"unknown\" />\n"
		deck := graphicsDeck(t, source, files)
		report, err := Doctor(deck.Dir)
		if err != nil || !report.HasFailures() {
			t.Fatalf("doctor did not flag graphic failure: %v", err)
		}
		if !strings.Contains(graphicsBody(t, deck), "graphic error:") {
			t.Fatal("graphic failure was silent")
		}
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.sel")
	if err := os.WriteFile(outside, []byte(testShader), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readGraphicFile(root, "../outside.sel"); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.sel")); err == nil {
		if _, err := readGraphicFile(root, "escape.sel"); err == nil {
			t.Fatal("symlink escape accepted")
		}
	}
}
func TestProductionDeckCachesProgramAndSupportsConcurrentRequests(t *testing.T) {
	deck := graphicsDeck(t, "# Cached\n\n<Card/>\n", map[string]string{"Card.gsx": "package main\n//gosx:island\nfunc Card() Node {\n return <div>original</div>\n}\n"})
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	if err := os.WriteFile(filepath.Join(deck.Dir, "Card.gsx"), []byte("package main\nfunc Card() Node {\n return <div>changed</div>\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if !strings.Contains(rec.Body.String(), ">original</div>") {
				t.Error("production request recompiled the edited source")
			}
		}()
	}
	group.Wait()
}
func TestMixedHTMLHeadingsRetainSourceText(t *testing.T) {
	deck := graphicsDeck(t, "<p>First eyebrow</p>\n\n# First<br>heading.\n\n<!-- note -->\n\n---\n\n<p>Second eyebrow</p>\n\n# Second heading.\n", nil)
	if len(deck.Slides) != 2 || slideTitle(deck.Slides[0]) != "Firstheading." || slideTitle(deck.Slides[1]) != "Second heading." {
		t.Fatal("mixed HTML heading repair lost source text")
	}
	body := graphicsBody(t, deck)
	if !strings.Contains(body, "<h1>First<br/>heading.</h1>") || !strings.Contains(body, "<h1>Second heading.</h1>") {
		t.Fatalf("heading repair lost inline HTML: %s", regexp.MustCompile(`(?s)<h1.*?</h1>`).FindAllString(body, -1))
	}
}
func TestGraphicSourcesTriggerReload(t *testing.T) {
	for _, name := range []string{"deck.md", "shaders/ink.sel", "scenes/graph.json"} {
		if !isMarkdownWriteEvent(fsnotify.Event{Name: name, Op: fsnotify.Write}) {
			t.Fatalf("source %s does not trigger reload", name)
		}
	}
	if isMarkdownWriteEvent(fsnotify.Event{Name: "build/Counter.js", Op: fsnotify.Write}) {
		t.Fatal("unrelated JS handled by source watcher")
	}
}
func TestSnapshotExportNeedsNoModuleOrRuntime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte("# Snapshot\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := ExportStatic(dir, ExportOptions{Format: "single", OutDir: out}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `data-live-sync="0"`) {
		t.Fatal("snapshot still attempts server sync")
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
		t.Fatal("snapshot needlessly staged a runtime")
	}
}

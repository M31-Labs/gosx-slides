package slides

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceEditorSaveValidationAndRevision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DeckFileName)
	initial := "# Original\n\nOriginal text\n"
	if err := os.WriteFile(path, []byte(initial), 0640); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", "http://example.com/_slides/source", nil))
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	var state map[string]string
	json.Unmarshal(get.Body.Bytes(), &state)
	put := func(source, revision, token, origin string) int {
		data, _ := json.Marshal(map[string]string{"source": source, "revision": revision})
		req := httptest.NewRequest("PUT", "http://example.com/_slides/source", bytes.NewReader(data))
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Slides-Token", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	for _, input := range []struct {
		source, token, origin string
		code                  int
	}{{"# Forged", state["token"], "http://evil.example", 403}, {"# Forged", "", "http://example.com", 403}, {"# Forged", state["token"], "", 403}, {"# Broken\n\n<Missing/>", state["token"], "http://example.com", 422}, {strings.Repeat("x", maxSourceBytes+1), state["token"], "http://example.com", 400}} {
		if code := put(input.source, state["revision"], input.token, input.origin); code != input.code {
			t.Fatalf("got %d want %d", code, input.code)
		}
		saved, _ := os.ReadFile(path)
		if string(saved) != initial {
			t.Fatal("rejected save changed file")
		}
	}
	updated := "# Persistent\n\nSaved in the browser\n"
	if code := put(updated, state["revision"], state["token"], "http://example.com"); code != 200 {
		t.Fatal(code)
	}
	saved, _ := os.ReadFile(path)
	if string(saved) != updated {
		t.Fatal("source was not persisted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("changed file permissions")
	}
	if code := put("# Stale", state["revision"], state["token"], "http://example.com"); code != 409 {
		t.Fatal("stale edit overwrote current source")
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(page.Body.String(), "Saved in the browser") || !strings.Contains(page.Body.String(), `name="slides-edit"`) {
		t.Fatal("saved content not served without watch")
	}
	for _, opts := range []ServeOptions{{}, {Edit: true, Static: true}} {
		app, err := deck.NewServer(opts)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		app.Build().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_slides/source", nil))
		if w.Code == 200 {
			t.Fatal("editing exposed without enabled server")
		}
	}
}
func TestSourceEditorServesNewComponent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, DeckFileName), []byte("# Initial"), 0644)
	source, err := os.ReadFile("testdata/island-deck/Counter.gsx")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "Counter.gsx"), source, 0644)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	h := app.Build()
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest("GET", "http://example.com/_slides/source", nil))
	var state map[string]string
	json.Unmarshal(get.Body.Bytes(), &state)
	data, _ := json.Marshal(map[string]string{"source": "# Live\n\n<Counter/>\n", "revision": state["revision"]})
	req := httptest.NewRequest("PUT", "http://example.com/_slides/source", bytes.NewReader(data))
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("X-Slides-Token", state["token"])
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	program := httptest.NewRecorder()
	h.ServeHTTP(program, httptest.NewRequest("GET", "/gosx/islands/Counter.json", nil))
	if program.Code != 200 || !json.Valid(program.Body.Bytes()) {
		t.Fatal("new island unavailable", program.Code, program.Body.String())
	}
}

func TestSourceEditorRefusesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "actual.md")
	os.WriteFile(target, []byte("# Source"), 0644)
	os.Symlink(target, filepath.Join(dir, DeckFileName))
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	app.Build().ServeHTTP(w, httptest.NewRequest("GET", "/_slides/source", nil))
	if w.Code != 400 {
		t.Fatal("symlink deck could be replaced")
	}
}

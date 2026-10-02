package slides

import (
	"bytes"
	"encoding/json"
	"errors"
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
	handler.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/_slides/source", nil))
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	var state map[string]string
	json.Unmarshal(get.Body.Bytes(), &state)
	put := func(source, revision, token, origin string) int {
		data, _ := json.Marshal(map[string]string{"source": source, "revision": revision})
		req := httptest.NewRequest("PUT", "http://localhost/_slides/source", bytes.NewReader(data))
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Slides-Token", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	for _, input := range []struct {
		source, token, origin string
		code                  int
	}{{"# Forged", state["token"], "http://evil.example", 403}, {"# Forged", "", "http://localhost", 403}, {"# Forged", state["token"], "", 403}, {"# Broken\n\n<Missing/>", state["token"], "http://localhost", 422}, {strings.Repeat("x", maxSourceBytes+1), state["token"], "http://localhost", 400}} {
		if code := put(input.source, state["revision"], input.token, input.origin); code != input.code {
			t.Fatalf("got %d want %d", code, input.code)
		}
		saved, _ := os.ReadFile(path)
		if string(saved) != initial {
			t.Fatal("rejected save changed file")
		}
	}
	updated := "# Persistent\n\nSaved in the browser\n"
	if code := put(updated, state["revision"], state["token"], "http://localhost"); code != 200 {
		t.Fatal(code)
	}
	saved, _ := os.ReadFile(path)
	if string(saved) != updated {
		t.Fatal("source was not persisted")
	}
	previous, err := filepath.Glob(filepath.Join(dir, ".slides-history-*", DeckFileName))
	if err != nil || len(previous) != 1 {
		t.Fatal("save did not retain the displaced revision", previous, err)
	}
	recovery, err := os.ReadFile(previous[0])
	if err != nil || string(recovery) != initial {
		t.Fatal("saved recovery differs from displaced source", err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("changed file permissions")
	}
	if code := put("# Stale", state["revision"], state["token"], "http://localhost"); code != 409 {
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

func TestFocusedMotionSavePreservesSourceAndRejectsStaleTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DeckFileName)
	initial := "# Café\n\n```text\n:::motion {duration=10}\n```\n\n:::motion {  duration='800'   easing=\"ease-out\" cue=beat }\nKeep **this body** and its spacing.\n:::\n"
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
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/_slides/source?motion=1", nil))
	var state struct{ Token, Revision string }
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	start := strings.LastIndex(initial, ":::motion")
	edit := map[string]any{"start": start, "attrs": map[string]string{"duration": "1100", "replay": "once"}}
	put := func(input map[string]any) int {
		data, _ := json.Marshal(input)
		req := httptest.NewRequest("PUT", "http://localhost/_slides/source", bytes.NewReader(data))
		req.Header.Set("Origin", "http://localhost")
		req.Header.Set("X-Slides-Token", state.Token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	for _, edits := range [][]any{
		{map[string]any{"start": strings.Index(initial, ":::motion"), "attrs": map[string]string{"duration": "1100"}}},
		{edit, edit},
		{map[string]any{"start": start, "attrs": map[string]string{"cue": "renamed"}}},
	} {
		if code := put(map[string]any{"motions": edits, "revision": state.Revision}); code != 400 {
			t.Fatalf("invalid motion targets: %d", code)
		}
		saved, _ := os.ReadFile(path)
		if string(saved) != initial {
			t.Fatal("rejected motion edit changed source")
		}
	}
	if code := put(map[string]any{"motions": []any{edit}, "source": "# Mixed", "revision": state.Revision}); code != 400 {
		t.Fatal(code)
	}
	if code := put(map[string]any{"motions": []any{edit}, "revision": state.Revision}); code != 200 {
		t.Fatal(code)
	}
	saved, _ := os.ReadFile(path)
	want := strings.Replace(initial, "duration='800'", "duration='1100'", 1)
	want = strings.Replace(want, "cue=beat }", "cue=beat  replay=\"once\"}", 1)
	if string(saved) != want {
		t.Fatalf("formatting changed:\ngot %s\nwant %s", saved, want)
	}
	if code := put(map[string]any{"motions": []any{edit}, "revision": state.Revision}); code != 409 {
		t.Fatalf("stale motion edit: %d", code)
	}
}

func TestSourceSavePreservesConcurrentExternalWrites(t *testing.T) {
	const initial = "# Original\n"
	const external = "# External\n"
	const browser = "# Browser\n"
	for _, scenario := range []string{"in-place before capture", "replacement before capture", "recreated after capture", "open writer after capture"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, DeckFileName)
			if err := os.WriteFile(path, []byte(initial), 0640); err != nil {
				t.Fatal(err)
			}
			save, err := stageSourceEdit(path, browser, 0640)
			if err != nil {
				t.Fatal(err)
			}
			defer save.cleanup()
			write := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(external), 0640); err != nil {
					t.Fatal(err)
				}
			}
			var conflict error
			wantCurrent, wantPrevious := external, external
			switch scenario {
			case "in-place before capture":
				write(path) // Interleaves after validation and candidate fsync.
				conflict = save.capture(sourceRevision([]byte(initial)))
			case "replacement before capture":
				other := filepath.Join(dir, "external.md")
				write(other)
				if err := os.Rename(other, path); err != nil {
					t.Fatal(err)
				}
				conflict = save.capture(sourceRevision([]byte(initial)))
			case "recreated after capture":
				if err := save.capture(sourceRevision([]byte(initial))); err != nil {
					t.Fatal(err)
				}
				write(path) // Must not be clobbered by publication or rollback.
				conflict = save.publish(sourceRevision([]byte(initial)))
				wantPrevious = initial
			case "open writer after capture":
				file, err := os.OpenFile(path, os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := save.capture(sourceRevision([]byte(initial))); err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate(0); err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString(external); err != nil {
					t.Fatal(err)
				}
				conflict = save.publish(sourceRevision([]byte(initial)))
				wantCurrent = browser
			}
			var typed *sourceEditConflict
			if !errors.As(conflict, &typed) || !strings.Contains(conflict.Error(), filepath.Base(save.dir)) {
				t.Fatal("concurrent write did not surface a recoverable conflict", conflict)
			}
			save.cleanup()
			for name, want := range map[string]string{path: wantCurrent, save.previous: wantPrevious} {
				got, err := os.ReadFile(name)
				if err != nil || string(got) != want {
					t.Fatalf("%s: got %q, want %q (%v)", name, got, want, err)
				}
			}
			if _, err := os.Stat(save.prepared); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("candidate temp file remained", err)
			}
		})
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
	h.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/_slides/source", nil))
	var state map[string]string
	json.Unmarshal(get.Body.Bytes(), &state)
	data, _ := json.Marshal(map[string]string{"source": "# Live\n\n<Counter/>\n", "revision": state["revision"]})
	req := httptest.NewRequest("PUT", "http://localhost/_slides/source", bytes.NewReader(data))
	req.Header.Set("Origin", "http://localhost")
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
	app.Build().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/_slides/source", nil))
	if w.Code != 400 {
		t.Fatal("symlink deck could be replaced")
	}
}

func TestSourceEditorRejectsReboundHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DeckFileName)
	initial := "# Original\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
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
	h := app.Build()
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/_slides/source", nil))
	var state map[string]string
	if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil || get.Code != 200 {
		t.Fatal("trusted source unavailable", get.Code, err)
	}
	for _, host := range []string{"attacker.example:8100", "localhost.evil:8100", "127.0.0.1.evil:8100"} {
		for _, method := range []string{"GET", "PUT"} {
			data, _ := json.Marshal(map[string]string{"source": "# Injected", "revision": state["revision"]})
			req := httptest.NewRequest(method, "http://"+host+"/_slides/source", bytes.NewReader(data))
			req.Header.Set("Origin", "http://"+host)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("X-Slides-Token", state["token"])
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 403 || strings.Contains(w.Body.String(), state["token"]) {
				t.Fatalf("%s hostile authority %s: %d %s", method, host, w.Code, w.Body.String())
			}
		}
	}
	for _, host := range []string{"localhost", "LOCALHOST", "localhost:8100", "127.0.0.1", "127.0.0.1:8100", "[::1]", "[::1]:8100"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://"+host+"/_slides/source", nil))
		if w.Code != 200 {
			t.Fatalf("trusted authority %s: %d %s", host, w.Code, w.Body.String())
		}
	}
	for _, host := range []string{"attacker.example:8100", "evil@localhost", "localhost/evil", "localhost?x=1", "localhost#fragment", "localhost:bad", "localhost:0", "localhost:65536", "localhost:"} {
		req := httptest.NewRequest("GET", "http://localhost/_slides/source", nil)
		req.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req) // Even an Origin-less token request must be rejected.
		if w.Code != 403 {
			t.Fatalf("untrusted authority %q accepted without Origin: %d", host, w.Code)
		}
	}
	src, err := os.ReadFile(path)
	if err != nil || string(src) != initial {
		t.Fatal("host rejection changed source", err)
	}
}

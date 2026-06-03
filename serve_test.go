package slides

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// realDeckDir is the shipped end-to-end example: prose + a standalone <Counter/>.
const realDeckDir = "examples/real-deck"

// TestNewServerRendersIslandPage proves the real lane serves a deck whose slide
// hosts a LIVE GoSX island: GET / returns a full HTML document containing the
// prose (static) and the island mount markup (hydratable), with the client
// bootstrap wired in the head.
func TestNewServerRendersIslandPage(t *testing.T) {
	deck, err := LoadIslandDeck(realDeckDir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// Prose is server-rendered (static lane).
	if !strings.Contains(body, "Live Counter") {
		t.Errorf("GET / missing prose heading 'Live Counter'")
	}
	// The island is mounted (real hydratable island, not a placeholder).
	if !strings.Contains(body, `data-gosx-island="Counter"`) {
		t.Errorf("GET / missing live island mount markup")
	}
	if strings.Contains(body, `data-gosx-unresolved`) {
		t.Errorf("GET / has an unresolved component (island failed to compile)")
	}
	// The client runtime is wired: PageHead emits the wasm_exec loader, and the
	// browser bootstrap fetches /gosx/runtime.wasm from there.
	if !strings.Contains(body, "/gosx/wasm_exec.js") {
		t.Errorf("GET / head missing /gosx/wasm_exec.js bootstrap loader")
	}
	// The island's program asset URL is referenced for hydration.
	if !strings.Contains(body, "/gosx/islands/Counter.json") {
		t.Errorf("GET / missing island program asset reference")
	}
}

// TestNewServerServesIslandJSON proves the compiled island program is served as
// JSON at its mount path, and is the real Counter program (the dev-socket wire
// form).
func TestNewServerServesIslandJSON(t *testing.T) {
	deck, err := LoadIslandDeck(realDeckDir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gosx/islands/Counter.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET island JSON status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("island JSON Content-Type = %q, want application/json", ct)
	}
	var prog struct {
		Name  string `json:"name"`
		Nodes []any  `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prog); err != nil {
		t.Fatalf("island JSON not valid program JSON: %v", err)
	}
	if prog.Name != "Counter" {
		t.Errorf("island program Name = %q, want Counter", prog.Name)
	}
	if len(prog.Nodes) == 0 {
		t.Errorf("island program has no nodes")
	}
}

// TestNewServerServesRuntimeAssets proves the standalone server stages and
// serves the client runtime: /gosx/runtime.wasm (application/wasm) and
// /gosx/wasm_exec.js — the assets the browser needs to hydrate the island
// without `gosx dev`. Building runtime.wasm is slow, so it is staged once and
// cached; if staging is unavailable the wasm assertions are skipped, but the
// JSON+page assertions above still hold.
func TestNewServerServesRuntimeAssets(t *testing.T) {
	deck, err := LoadIslandDeck(realDeckDir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	buildDir := filepath.Join(realDeckDir, "build")
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })

	app, err := deck.NewServer(ServeOptions{StageRuntime: true})
	if err != nil {
		t.Fatalf("NewServer(StageRuntime): %v", err)
	}
	handler := app.Build()

	// wasm_exec.js comes straight from the Go toolchain — always stageable.
	t.Run("wasm_exec.js", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gosx/wasm_exec.js", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /gosx/wasm_exec.js status = %d, want 200", rec.Code)
		}
		if n, _ := io.Copy(io.Discard, rec.Body); n == 0 {
			t.Fatalf("/gosx/wasm_exec.js is empty")
		}
	})

	t.Run("runtime.wasm", func(t *testing.T) {
		if !isFileExists(filepath.Join(buildDir, "gosx-runtime.wasm")) {
			t.Skip("runtime.wasm not staged (GOOS=js build unavailable in this env); slides serve stages it on demand")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gosx/runtime.wasm", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /gosx/runtime.wasm status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/wasm") {
			t.Errorf("runtime.wasm Content-Type = %q, want application/wasm", ct)
		}
	})
}

// TestNewServerConcurrentRequestsAreIdenticalAndRaceClean is the C1 regression:
// the deck server must serve a stable, correct page under concurrent load. Before
// the fix, NewServer captured ONE shared island.Renderer in the "/" closure, so
// every request mutated shared renderer state (r.manifest.AddIsland, r.counter)
// unguarded — a data race under `go test -race`, and the page accumulated stale
// islands across requests (the body grew with each GET). After the fix the
// renderer is constructed per-request, so all responses are byte-identical to a
// single-request baseline and the handler is race-clean.
func TestNewServerConcurrentRequestsAreIdenticalAndRaceClean(t *testing.T) {
	deck, err := LoadIslandDeck(realDeckDir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	// The gosx server mints a unique per-request ID (gosx-<unixnano>-<seq>) and
	// embeds it in the page's manifest JSON; that value is *correctly* different
	// on every request and is unrelated to C1, so normalize it before comparing.
	// The C1 symptom (stale-island accumulation) grows the manifest's island list
	// and balloons the body by kilobytes — far beyond this one ID — so masking it
	// keeps the regression sharp.
	requestIDRe := regexp.MustCompile(`"requestID":"[^"]*"`)
	get := func() (int, string) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec.Code, requestIDRe.ReplaceAllString(rec.Body.String(), `"requestID":"X"`)
	}

	// Baseline: the correct single-request output. It must mount exactly one
	// live island (the deck has exactly one <Counter/>).
	wantCode, want := get()
	if wantCode != http.StatusOK {
		t.Fatalf("baseline GET / status = %d, want 200", wantCode)
	}
	if got := strings.Count(want, `data-gosx-island="Counter"`); got != 1 {
		t.Fatalf("baseline GET / mounts %d Counter islands, want exactly 1:\n%s", got, want)
	}

	const n = 32
	bodies := make([]string, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			codes[i], bodies[i] = get()
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if codes[i] != http.StatusOK {
			t.Fatalf("concurrent GET / [%d] status = %d, want 200", i, codes[i])
		}
		if bodies[i] != want {
			// Surface the accumulation symptom directly: island count and size.
			t.Fatalf("concurrent GET / [%d] body differs from baseline (Counter mounts: got %d want %d; size got %d want %d) — shared renderer accumulated state across requests",
				i,
				strings.Count(bodies[i], `data-gosx-island="Counter"`),
				strings.Count(want, `data-gosx-island="Counter"`),
				len(bodies[i]), len(want),
			)
		}
	}
}

// TestNewServerSoftDegradesMissingComponent is the I1.2 regression: a deck that
// references a component whose <Name>.gsx does not exist (a typo or a not-yet-
// created component) must NOT fail NewServer or 500 the presentation. It serves
// 200 with an INERT placeholder for that one component, while the rest of the
// deck renders normally. Before the I1.2 fix, compileComponents treated the
// missing file as a hard error and the whole deck failed to build.
//
// Slice 4 note: slides now render through the source-gen lane, so an unresolved
// <Missing/> is emitted by the gosx route renderer as its standard inert
// component placeholder `<div data-gosx-component="Missing" …>` (no source to
// inline, so it falls through to the default rendering) rather than the old
// slides-specific data-gosx-unresolved span. The CONTRACT is unchanged: 200, an
// inert non-hydrating placeholder, surrounding prose intact, and crucially NOT a
// live island mount for a component with no source.
func TestNewServerSoftDegradesMissingComponent(t *testing.T) {
	dir := t.TempDir()
	// A deck referencing a component that has no .gsx source.
	deckMD := "# Degrade\n\nprose before\n\n<Missing initial={3}/>\n\nprose after\n"
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte(deckMD), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}

	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer must not fail on a missing component (got %v)", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200 (missing component must degrade, not 500)", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-gosx-component="Missing"`) {
		t.Fatalf("GET / missing the inert placeholder for the unresolved component:\n%s", body)
	}
	// Surrounding prose still renders.
	if !strings.Contains(body, "prose before") || !strings.Contains(body, "prose after") {
		t.Fatalf("GET / dropped surrounding prose around the unresolved component:\n%s", body)
	}
	// It must be an INERT placeholder, not a live island mount.
	if strings.Contains(body, `data-gosx-island="Missing"`) {
		t.Fatalf("GET / mounted a live island for a component with no source:\n%s", body)
	}
}

// TestStageRuntimeAssetsRebuild is the I2 regression: runtime.wasm is
// existence-cached (so a normal serve reuses it), but rebuild=true must force a
// fresh GOOS=js build even when a cached artifact already exists — otherwise a
// gosx runtime change is never picked up. We plant a tiny sentinel where the wasm
// is cached, then assert: rebuild=false leaves the sentinel untouched (cache hit),
// and rebuild=true replaces it with a real (much larger) wasm artifact. The
// GOOS=js build is slow; if it is unavailable in this environment the rebuild leg
// is skipped, but the cache-hit leg still holds.
func TestStageRuntimeAssetsRebuild(t *testing.T) {
	// The build runs with cmd.Dir = deckDir and must resolve m31labs.dev/gosx via
	// the repo's go.mod replace, so use a deck dir UNDER the module (t.TempDir() is
	// outside it and would not resolve the replace).
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	deckDir := filepath.Join(repoDir, "testdata", "island-deck")
	buildDir := filepath.Join(deckDir, "build")
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}
	wasmPath := filepath.Join(buildDir, "gosx-runtime.wasm")
	sentinel := []byte("STALE-SENTINEL-NOT-A-REAL-WASM")
	if err := os.WriteFile(wasmPath, sentinel, 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// Cache hit: rebuild=false must NOT touch the existing (stale) artifact.
	if _, err := StageRuntimeAssets(deckDir, false); err != nil {
		t.Fatalf("StageRuntimeAssets(rebuild=false): %v", err)
	}
	got, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("read wasm after cache hit: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Fatalf("rebuild=false replaced the cached wasm (len %d); existence-cache must reuse it", len(got))
	}

	// Forced rebuild: rebuild=true must replace the stale sentinel with a real
	// build, even though the file already exists.
	if _, err := StageRuntimeAssets(deckDir, true); err != nil {
		t.Skipf("rebuild=true failed (GOOS=js build unavailable in this env): %v", err)
	}
	rebuilt, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("read wasm after rebuild: %v", err)
	}
	if string(rebuilt) == string(sentinel) {
		t.Fatalf("rebuild=true did NOT rebuild: the stale sentinel survived (len %d)", len(rebuilt))
	}
	// A real runtime.wasm is large; the sentinel is tiny. Guard against a
	// degenerate replacement.
	if len(rebuilt) < 1024 {
		t.Fatalf("rebuilt wasm is implausibly small (%d bytes); expected a real GOOS=js artifact", len(rebuilt))
	}
}

// TestNewServerEvaluatesExpressions is the Slice-4 headline outcome through the
// real HTTP handler: a deck whose slides carry inline {expr} serves a page where
// those expressions are EVALUATED server-side — {2 + 3} -> "5",
// {strings.ToUpper("hi")} -> "HI" (strings is the bound namespace), and a pure
// string concat -> "abc" — not rendered as their raw source. This is the whole
// point of the slice: a slide's {expr} runs through the gosx evaluator, not the
// old raw-text path.
func TestNewServerEvaluatesExpressions(t *testing.T) {
	dir := newDeckDirUnderModule(t,
		"# Eval\n\nThe answer is {2 + 3}.\n\nShout {strings.ToUpper(\"hi\")} now.\n\nJoined {\"a\" + \"b\" + \"c\"}.\n",
		nil)

	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "The answer is 5.") {
		t.Errorf("GET / did not evaluate {2 + 3} to 5:\n%s", body)
	}
	if strings.Contains(body, "2 + 3") {
		t.Errorf("GET / leaked raw expr source {2 + 3} instead of evaluating:\n%s", body)
	}
	if !strings.Contains(body, "HI") {
		t.Errorf("GET / did not evaluate {strings.ToUpper(\"hi\")} to HI:\n%s", body)
	}
	if !strings.Contains(body, "abc") {
		t.Errorf("GET / did not evaluate {\"a\" + \"b\" + \"c\"} to abc:\n%s", body)
	}
}

// TestNewServerExprAndIslandCoexist proves a single served page evaluates an
// inline {expr} AND hydrates a real <Counter/> island on the same slide, with
// the island program served as JSON — the two Slice-4 capabilities together
// through the HTTP server.
func TestNewServerExprAndIslandCoexist(t *testing.T) {
	dir := newDeckDirUnderModule(t,
		"# Both\n\nThe answer is {2 + 3}.\n\n<Counter initial={3}/>\n",
		map[string]string{"Counter": counterGSX})

	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "The answer is 5.") {
		t.Errorf("GET / did not evaluate {2 + 3} alongside the island:\n%s", body)
	}
	if !strings.Contains(body, `data-gosx-island="Counter"`) {
		t.Errorf("GET / missing live island mount for <Counter/>:\n%s", body)
	}
	if strings.Contains(body, "data-gosx-unresolved") || strings.Contains(body, `data-gosx-component="Counter"`) {
		t.Errorf("GET / Counter did not inline as a live island (rendered as a placeholder):\n%s", body)
	}

	// The island program is served as JSON for hydration.
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/gosx/islands/Counter.json", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET island JSON status = %d, want 200", rec2.Code)
	}
	if ct := rec2.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("island JSON Content-Type = %q, want application/json", ct)
	}
}

// TestNewServerServesSingleDocument is the nested-document hydration regression:
// the deck must be served as ONE HTML document with the island runtime ENABLED,
// not a document nested inside another document. Before the fix, the "/" route was
// registered with App.Route and the handler returned its own server.HTMLDocument,
// which the App then wrapped in its OWN document — so the outer <html> reported
// the runtime OFF (the App never saw the islands, which were rendered on a private
// renderer) while the real manifest/bootstrap sat illegally nested inside the
// outer <body>, and nothing hydrated. After the fix the deck routes through
// App.Page and registers islands on ctx.Runtime(), so the App emits exactly one
// document whose single <head> carries the runtime-enabled contract, the manifest
// (with a fetchable programRef), and the bootstrap.
func TestNewServerServesSingleDocument(t *testing.T) {
	deck, err := LoadIslandDeck(realDeckDir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler := app.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// Exactly one of each document-shell token — no nesting.
	for _, tok := range []string{"<!DOCTYPE", "<html", "<head>", "<body", "</head>", "</body>", "</html>"} {
		if got := strings.Count(strings.ToLower(body), strings.ToLower(tok)); got != 1 {
			t.Errorf("served page has %d %q, want exactly 1 (nested document?):\n%s", got, tok, body)
		}
	}

	// The document contract must report the runtime ENABLED (the bug left it off).
	if !strings.Contains(body, `"runtime":true`) {
		t.Errorf("document contract does not enable the runtime (expected \"runtime\":true):\n%s", body)
	}
	if !strings.Contains(body, `"manifest":true`) {
		t.Errorf("document contract reports no manifest (expected \"manifest\":true):\n%s", body)
	}
	if strings.Contains(body, `"bootstrapMode":"none"`) {
		t.Errorf("document contract bootstrapMode is \"none\" — runtime layer is OFF:\n%s", body)
	}

	// The manifest + bootstrap + wasm_exec must all live in the SINGLE <head>.
	headStart := strings.Index(body, "<head>")
	headEnd := strings.Index(body, "</head>")
	if headStart < 0 || headEnd < 0 || headEnd < headStart {
		t.Fatalf("could not locate a single <head>…</head> in the served page")
	}
	head := body[headStart:headEnd]
	for _, want := range []string{`id="gosx-manifest"`, "wasm_exec.js", "bootstrap-runtime.js"} {
		if !strings.Contains(head, want) {
			t.Errorf("single <head> missing %q (runtime not wired into the document head):\n%s", want, head)
		}
	}

	// The island manifest must carry a fetchable programRef (empty -> no
	// hydration). The manifest JSON may be compact or pretty-printed, so match the
	// key and value independently of inter-token whitespace.
	if !strings.Contains(body, `"programRef"`) || !strings.Contains(body, "/gosx/islands/Counter.json") {
		t.Errorf("manifest island has no fetchable programRef to /gosx/islands/Counter.json:\n%s", body)
	}
}

// newDeckDirUnderModule writes a deck.md (+ optional component .gsx files) into a
// fresh temp dir UNDER the module (so the go.mod replace resolves for any
// tooling) and returns the dir. t.Cleanup removes it.
func newDeckDirUnderModule(t *testing.T, deckMD string, components map[string]string) string {
	t.Helper()
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	base := filepath.Join(repoDir, "testdata")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	dir, err := os.MkdirTemp(base, "serve-eval-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte(deckMD), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	for name, src := range components {
		if err := os.WriteFile(filepath.Join(dir, name+".gsx"), []byte(src), 0o644); err != nil {
			t.Fatalf("write component %s: %v", name, err)
		}
	}
	return dir
}

func isFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

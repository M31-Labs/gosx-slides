package slides

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dev_test.go verifies the real-lane hot-swap dev loop (Slice 3) over real
// HTTP/SSE — the headless half of the verify gate. The actual in-browser live
// swap (island updates, signal state survives, no flash) is the MANUAL gate and
// is NOT asserted here.

// newTempDeck copies the deck at srcDir into a fresh directory UNDER the module
// (testdata/) so (a) edits don't dirty the shipped example and (b) the GOOS=js
// runtime build's `go list -m` resolves the repo's replace directives — a
// t.TempDir() outside the module would not. It returns the temp deck dir and
// registers cleanup.
func newTempDeck(t *testing.T, srcDir string) string {
	t.Helper()
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	base := filepath.Join(repoDir, "testdata")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	dst, err := os.MkdirTemp(base, "devdeck-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dst) })

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read src deck %s: %v", srcDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue // decks are flat (deck.md + *.gsx); skip any build/ etc.
		}
		data, err := os.ReadFile(filepath.Join(srcDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.Name(), err)
		}
	}
	return dst
}

// startDevLoopForTest stands up the dev loop on deckDir and starts the dev proxy
// front on a free public port, returning the public base URL. It waits until the
// proxy answers, and registers cleanup (proxy shutdown + internal server close).
//
// Building the GOOS=js runtime.wasm is slow and may be unavailable in CI; if the
// stage step fails for that reason the whole test is skipped (the proxy/SSE
// behavior is what we are verifying, not the wasm toolchain).
func startDevLoopForTest(t *testing.T, deckDir string) (publicURL string, loop *DevLoop) {
	t.Helper()
	loop, err := StartDevLoop(deckDir, DevLoopConfig{
		Logf: func(format string, args ...any) { t.Logf("devloop: "+format, args...) },
	})
	if err != nil {
		if strings.Contains(err.Error(), "runtime wasm") || strings.Contains(err.Error(), "stage runtime assets") {
			t.Skipf("dev loop unavailable (GOOS=js build): %v", err)
		}
		t.Fatalf("StartDevLoop: %v", err)
	}
	t.Cleanup(func() { _ = loop.Close() })

	port, err := pickFreePort()
	if err != nil {
		t.Fatalf("pick public port: %v", err)
	}
	publicAddr := "127.0.0.1:" + strconv.Itoa(port)
	publicURL = "http://" + publicAddr

	go func() { _ = loop.Server.ListenAndServe(publicAddr) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = loop.Server.Shutdown(ctx)
	})

	if err := waitForReady(publicURL, 20*time.Second); err != nil {
		t.Fatalf("wait for dev proxy ready: %v", err)
	}
	return publicURL, loop
}

// TestDevLoopProxiesDeckWithInjectedScript verifies (a) of the gate: the public
// proxy serves the deck HTML (prose + live island mount) with the gosx dev
// hot-swap client script injected.
func TestDevLoopProxiesDeckWithInjectedScript(t *testing.T) {
	deckDir := newTempDeck(t, realDeckDir)
	publicURL, _ := startDevLoopForTest(t, deckDir)

	resp, err := http.Get(publicURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)

	// Prose is server-rendered (proxied from the deck server).
	if !strings.Contains(html, "Live Counter") {
		t.Errorf("proxied / missing prose heading 'Live Counter'")
	}
	// The live island is mounted (not a placeholder).
	if !strings.Contains(html, `data-gosx-island="Counter"`) {
		t.Errorf("proxied / missing live island mount markup")
	}
	if strings.Contains(html, `data-gosx-unresolved`) {
		t.Errorf("proxied / has an unresolved component (island failed to compile)")
	}
	// The dev proxy injected its hot-swap client script.
	if !strings.Contains(html, "data-gosx-dev-reload") {
		t.Errorf("proxied / missing injected dev hot-swap script")
	}
	if !strings.Contains(html, "/gosx/dev/events") {
		t.Errorf("proxied / injected script missing the SSE endpoint reference")
	}
}

// TestDevLoopServesStagedIslandJSON verifies the dev proxy front serves the
// staged island program at /gosx/islands/<Name>.json (it SHADOWS the proxied
// deck server, so the program must be staged on disk for the initial load).
func TestDevLoopServesStagedIslandJSON(t *testing.T) {
	deckDir := newTempDeck(t, realDeckDir)
	publicURL, _ := startDevLoopForTest(t, deckDir)

	resp, err := http.Get(publicURL + "/gosx/islands/Counter.json")
	if err != nil {
		t.Fatalf("GET island JSON: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /gosx/islands/Counter.json status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var prog struct {
		Name  string `json:"name"`
		Nodes []any  `json:"nodes"`
	}
	if err := json.Unmarshal(body, &prog); err != nil {
		t.Fatalf("staged island JSON not valid program JSON: %v", err)
	}
	if prog.Name != "Counter" {
		t.Errorf("staged island Name = %q, want Counter", prog.Name)
	}
	if len(prog.Nodes) == 0 {
		t.Errorf("staged island program has no nodes")
	}
}

// TestDevLoopGSXChangeEmitsProgramNoReload verifies (b) of the gate: editing a
// component .gsx emits a "program" SSE event for that component carrying fresh
// bytecode, and does NOT emit a "reload" (the live island hot-swaps in place).
func TestDevLoopGSXChangeEmitsProgramNoReload(t *testing.T) {
	deckDir := newTempDeck(t, realDeckDir)
	publicURL, _ := startDevLoopForTest(t, deckDir)

	stream := openSSE(t, publicURL)
	defer stream.Close()
	stream.drainConnected(t)

	// Edit the island source: change the label text. Track B recompiles just this
	// island and ships a "program" event keyed by component name.
	counterPath := filepath.Join(deckDir, "Counter.gsx")
	src, err := os.ReadFile(counterPath)
	if err != nil {
		t.Fatalf("read Counter.gsx: %v", err)
	}
	edited := strings.Replace(string(src), "count is", "clicks:", 1)
	if edited == string(src) {
		t.Fatalf("test fixture changed: 'count is' not found in Counter.gsx")
	}
	writeAfterSettle(t, counterPath, []byte(edited))

	name, data := stream.awaitFrame(t, 8*time.Second, map[string]bool{"program": true, "reload": true})
	if name == "reload" {
		t.Fatalf("a component .gsx edit must NOT emit reload (it must hot-swap); got reload")
	}
	var payload struct {
		Component string `json:"component"`
		Format    string `json:"format"`
		Program   string `json:"program"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("program frame not JSON: %v (%s)", err, data)
	}
	if payload.Component != "Counter" {
		t.Fatalf("program frame component = %q, want Counter", payload.Component)
	}
	if payload.Format != "json" {
		t.Fatalf("program frame format = %q, want json", payload.Format)
	}
	// The inline bytecode must be the fresh Counter program.
	var isl struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(payload.Program), &isl); err != nil || isl.Name != "Counter" {
		t.Fatalf("program frame missing fresh Counter bytecode: name=%q err=%v", isl.Name, err)
	}
}

// TestDevLoopDeckMDChangeEmitsReloadAndNewContent verifies (c) of the gate:
// editing deck.md emits a "reload" SSE event, and a subsequent GET / reflects the
// new content (the dev deck server re-loads deck.md per request).
func TestDevLoopDeckMDChangeEmitsReloadAndNewContent(t *testing.T) {
	deckDir := newTempDeck(t, realDeckDir)
	publicURL, _ := startDevLoopForTest(t, deckDir)

	// Confirm the new heading is not present yet.
	const newHeading = "Edited Live Deck Heading"
	if before := getBody(t, publicURL+"/"); strings.Contains(before, newHeading) {
		t.Fatalf("precondition: new heading already present before edit")
	}

	stream := openSSE(t, publicURL)
	defer stream.Close()
	stream.drainConnected(t)

	deckPath := filepath.Join(deckDir, DeckFileName)
	src, err := os.ReadFile(deckPath)
	if err != nil {
		t.Fatalf("read deck.md: %v", err)
	}
	edited := strings.Replace(string(src), "# Live Counter", "# "+newHeading, 1)
	if edited == string(src) {
		t.Fatalf("test fixture changed: '# Live Counter' not found in deck.md")
	}
	writeAfterSettle(t, deckPath, []byte(edited))

	name, _ := stream.awaitFrame(t, 8*time.Second, map[string]bool{"program": true, "reload": true})
	if name != "reload" {
		t.Fatalf("a deck.md edit must emit reload; got %q", name)
	}

	// After the reload, the dev deck server (ServeOptions.Dev) re-loads deck.md
	// per request, so the new content is served. Poll briefly: OnChange re-stages
	// before the broadcast, but the next GET reads disk fresh.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if strings.Contains(getBody(t, publicURL+"/"), newHeading) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET / after reload did not reflect the new deck.md heading %q", newHeading)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// --- SSE test helpers ---

// frame is one parsed SSE event:/data: pair (or a read error).
type frame struct {
	name string
	data string
	err  error
}

// sseStream is an open SSE connection to the dev proxy's /gosx/dev/events, with a
// single background goroutine parsing frames into a channel. One reader goroutine
// per stream (started at open) avoids two goroutines racing on the same
// bufio.Reader, while still letting awaitFrame's timeout fire between heartbeats.
type sseStream struct {
	resp   *http.Response
	frames chan frame
}

func openSSE(t *testing.T, baseURL string) *sseStream {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/gosx/dev/events", nil)
	if err != nil {
		t.Fatalf("build SSE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open SSE stream: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("SSE stream status = %d, want 200", resp.StatusCode)
	}

	s := &sseStream{resp: resp, frames: make(chan frame)}
	reader := bufio.NewReader(resp.Body)
	go func() {
		for {
			f := readSSEFrame(reader)
			s.frames <- f // blocks until a consumer reads; no frame is dropped
			if f.err != nil {
				return
			}
		}
	}()
	return s
}

func (s *sseStream) Close() {
	if s.resp != nil {
		s.resp.Body.Close()
	}
}

// drainConnected consumes the initial "connected" frame so the client is
// registered before a change is triggered (the broadcast only reaches registered
// clients). handleSSE writes it synchronously on connect, so it arrives at once.
func (s *sseStream) drainConnected(t *testing.T) {
	t.Helper()
	s.awaitFrame(t, 3*time.Second, map[string]bool{"connected": true})
}

// awaitFrame consumes frames from the stream's reader goroutine until one whose
// name is in want arrives or the timeout elapses, tolerating heartbeats and
// unrelated frames. It returns the matched frame's name and data.
func (s *sseStream) awaitFrame(t *testing.T, timeout time.Duration, want map[string]bool) (name, data string) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case f := <-s.frames:
			if f.err != nil {
				t.Fatalf("read SSE frame: %v", f.err)
			}
			if want[f.name] {
				return f.name, f.data
			}
		case <-timer.C:
			t.Fatalf("timed out after %s waiting for one of %v SSE frames", timeout, keysOf(want))
			return "", ""
		}
	}
}

// readSSEFrame reads one event:/data: frame from an SSE reader, tolerating blank
// lines and heartbeat comments.
func readSSEFrame(reader *bufio.Reader) frame {
	var name, data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return frame{err: err}
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if name != "" || data != "" {
				return frame{name: name, data: data}
			}
		}
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s body: %v", url, err)
	}
	return string(body)
}

// writeAfterSettle pauses briefly before writing so the file watcher sees a
// modtime/size change distinct from the initial copy, then writes the new
// content. fsnotify keys on write events, but the polling fallback keys on
// modtime — a sub-second gap keeps both reliable.
func writeAfterSettle(t *testing.T, path string, data []byte) {
	t.Helper()
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

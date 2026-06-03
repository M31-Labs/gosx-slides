package slides

// dev.go is the real lane's hot-swap dev loop (Phase 1, Slice 3) — the spec's
// signature dev experience:
//
//   - Editing a deck component's <Name>.gsx hot-swaps the live island in the
//     browser WITHOUT a page reload: the count you already clicked up survives.
//   - Editing deck.md full-reloads with the new content.
//
// It reuses Phase-0 Track B (gosx's dev package) almost wholesale. The deck
// server (serve.go) is run as an in-process upstream on a free internal port and
// fronted by gosx/dev.Server, which proxies "/", injects the hot-swap client
// script, serves the staged /gosx/* runtime + island assets from <deck>/build,
// and watches the deck dir. On a .gsx change Track B recompiles JUST that island
// and broadcasts a "program" SSE event keyed by component name (the client fans
// out across window.__gosx.islands and hot-swaps in place — no reload). Any other
// change (e.g. deck.md) runs OnChange (re-stage assets) and broadcasts a full
// "reload"; the dev deck server (ServeOptions.Dev) re-loads the deck per request
// so the reload shows the new content.
//
// It does not touch the fallback presenter (server.go) or the production `serve`
// lane (serve.go's Serve): the dev deck server is the same App, only with
// ServeOptions.Dev set and fronted by the proxy.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"m31labs.dev/gosx/dev"
)

// DevOptions configures the real-lane hot-swap dev loop (DevDeck).
type DevOptions struct {
	// Addr is the PUBLIC listen address the dev proxy serves on (e.g.
	// "127.0.0.1:8080"). Defaults to "127.0.0.1:8080".
	Addr string

	// Title is the HTML document <title>; defaults to the deck's first heading,
	// then the deck dir name (see ServeOptions.Title).
	Title string

	// RebuildRuntime forces a fresh GOOS=js runtime.wasm even when a cached one
	// exists (see ServeOptions.RebuildRuntime). The wasm is existence-cached, so
	// this is how a gosx runtime change is picked up.
	RebuildRuntime bool

	// Logf, when set, receives dev-loop log lines (prefix-free); defaults to
	// stderr. Tests pass a capturing logger.
	Logf func(format string, args ...any)
}

// DevDeck runs the real-lane hot-swap dev loop for the deck at dir and blocks
// until interrupted (SIGINT/SIGTERM) or the server stops. It is the entry point
// the `slides dev` / `slides serve --watch` CLI calls.
func DevDeck(dir string, opts DevOptions) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dir, err)
	}

	logf := opts.Logf
	if logf == nil {
		logf = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "slides dev: "+format+"\n", args...)
		}
	}

	publicAddr := opts.Addr
	if publicAddr == "" {
		publicAddr = "127.0.0.1:8080"
	}

	// Stand the deck server up in-process on a free internal port, fronted by the
	// dev proxy. StartDevLoop owns the goroutine + readiness wait so the in-test
	// httptest variant and this blocking CLI variant share one wiring.
	loop, err := StartDevLoop(absDir, DevLoopConfig{
		Title:          opts.Title,
		RebuildRuntime: opts.RebuildRuntime,
		Logf:           logf,
	})
	if err != nil {
		return err
	}
	defer loop.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- loop.Server.ListenAndServe(publicAddr)
	}()

	logf("staged assets in %s", filepath.Join(absDir, "build"))
	logf("proxy http://%s -> %s", publicAddr, loop.InternalURL)
	logf("watching %s — edit a component .gsx to hot-swap, deck.md to reload", absDir)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-sigCh:
		logf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := loop.Server.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// DevLoopConfig configures StartDevLoop, the wiring shared by the blocking CLI
// loop (DevDeck) and integration tests.
type DevLoopConfig struct {
	Title          string
	RebuildRuntime bool
	Logf           func(format string, args ...any)
}

// DevLoop is a started dev loop: the in-process deck server (on an internal
// port) and the configured-but-not-yet-listening gosx/dev.Server proxy front.
// The caller starts the proxy (Server.ListenAndServe / httptest.NewServer with
// Server.Handler()) and must Close the loop when done to stop the internal
// server.
type DevLoop struct {
	// Server is the gosx dev proxy front. It is configured (Dir/BuildDir/
	// ProxyTarget/OnChange) but NOT listening — the caller starts it (and its
	// watcher) via ListenAndServe, or mounts Server.Handler() under httptest.
	Server *dev.Server

	// InternalURL is the in-process deck server's base URL (http://127.0.0.1:N).
	InternalURL string

	// BuildDir is <deck>/build, where the runtime + island assets are staged and
	// from which dev.Server serves /gosx/*.
	BuildDir string

	internalSrv *http.Server
	deckDir     string
	logf        func(format string, args ...any)

	mdWatcher *fsnotify.Watcher
	stopMD    chan struct{}
}

// StartDevLoop stages the deck's runtime + island assets, starts the dev deck
// server in-process on a free internal port (waiting until it is ready), and
// returns a DevLoop whose dev.Server proxy front is configured but not yet
// listening. The caller owns starting the front and Close-ing the loop.
//
// Who serves what:
//   - dev.Server serves /gosx/runtime.wasm, /gosx/wasm_exec.js, /gosx/bootstrap.js,
//     /gosx/patch.js from BuildDir, and /gosx/islands/<Name>.json from
//     BuildDir/islands. Those routes SHADOW the proxied deck server, so the
//     island programs must be staged on disk (StageIslandPrograms) — that is why
//     OnChange and the initial stage write build/islands/<Name>.json.
//   - The deck server (proxied at "/") renders the page and references each
//     island at /gosx/islands/<Name>.json; the proxy front serves that file.
//   - The "program" hot-swap SSE event carries the fresh bytecode INLINE, so the
//     live swap never re-fetches; the staged JSON only backs a hard refresh.
func StartDevLoop(deckDir string, cfg DevLoopConfig) (*DevLoop, error) {
	absDir, err := filepath.Abs(deckDir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", deckDir, err)
	}

	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	// 1. Stage the client runtime (wasm + bootstrap/patch JS) into <deck>/build
	// and the island programs into <deck>/build/islands so the dev proxy can
	// serve /gosx/* (it shadows the proxied deck server for those paths).
	buildDir := filepath.Join(absDir, "build")
	if _, err := StageRuntimeAssets(absDir, cfg.RebuildRuntime); err != nil {
		return nil, fmt.Errorf("stage runtime assets: %w", err)
	}
	if err := StageIslandPrograms(absDir); err != nil {
		return nil, fmt.Errorf("stage island programs: %w", err)
	}

	// 2. Build the dev-mode deck App (re-loads the deck per request so a deck.md
	// edit shows new content after a reload) and run it on a free internal port.
	deck, err := LoadIslandDeck(absDir)
	if err != nil {
		return nil, err
	}
	app, err := deck.NewServer(ServeOptions{
		Title: cfg.Title,
		Dev:   true,
		// Runtime is staged above and served by the dev proxy front from
		// BuildDir; the internal deck server does not need to serve /gosx/* (the
		// proxy shadows those paths), so leave StageRuntime off here.
	})
	if err != nil {
		return nil, err
	}

	internalPort, err := pickFreePort()
	if err != nil {
		return nil, fmt.Errorf("pick internal port: %w", err)
	}
	internalAddr := fmt.Sprintf("127.0.0.1:%d", internalPort)
	internalURL := "http://" + internalAddr

	internalSrv := &http.Server{
		Addr:              internalAddr,
		Handler:           app.Build(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := internalSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logf("internal deck server stopped: %v", err)
		}
	}()

	if err := waitForReady(internalURL, 20*time.Second); err != nil {
		_ = internalSrv.Close()
		return nil, fmt.Errorf("wait for deck server ready: %w", err)
	}

	// 3. Front the deck server with the gosx dev proxy. Watching absDir wires
	// Track B's hot-swap: a .gsx change recompiles just that island and
	// broadcasts a "program" event (no reload). A .gsx-only edit is the hot-swap
	// path; any change the gosx watcher classifies as non-island runs OnChange and
	// broadcasts a full "reload".
	devServer := &dev.Server{
		Dir:         absDir,
		BuildDir:    buildDir,
		ProxyTarget: internalURL,
		Logf:        logf,
		OnChange: func() error {
			// A reload-classified change reached us: re-stage the island programs
			// so a hard refresh after the reload fetches fresh bytecode. The deck
			// server itself re-loads deck.md per request (ServeOptions.Dev), so no
			// restart is needed. Runtime assets (wasm/bootstrap) don't change on a
			// deck edit, so they are not re-staged here — only the cheap island
			// JSON.
			if err := StageIslandPrograms(absDir); err != nil {
				return fmt.Errorf("re-stage island programs: %w", err)
			}
			return nil
		},
	}

	loop := &DevLoop{
		Server:      devServer,
		InternalURL: internalURL,
		BuildDir:    buildDir,
		internalSrv: internalSrv,
		deckDir:     absDir,
		logf:        logf,
	}

	// 4. The gosx dev watcher only watches .gsx/.go/.css/.js (see
	// dev.shouldWatchProjectFile) — it ignores deck.md (and other Markdown). The
	// deck's CONTENT lives in deck.md, so a deck.md edit must still full-reload.
	// Bridge it: a dedicated watcher on the deck dir's Markdown re-stages islands
	// and calls the dev server's public TriggerReload (gosx dev v0.24.2+), which
	// broadcasts "reload" over the one SSE the clients connect to.
	if err := loop.startMarkdownReloadBridge(); err != nil {
		_ = loop.Close()
		return nil, fmt.Errorf("start deck.md watcher: %w", err)
	}

	return loop, nil
}

// Close stops the in-process deck server and the Markdown reload bridge. It does
// not stop the dev.Server proxy front — the caller owns that (Server.Shutdown,
// or httptest ts.Close()).
func (l *DevLoop) Close() error {
	if l == nil {
		return nil
	}
	if l.stopMD != nil {
		select {
		case <-l.stopMD:
		default:
			close(l.stopMD)
		}
	}
	if l.mdWatcher != nil {
		_ = l.mdWatcher.Close()
	}
	if l.internalSrv != nil {
		return l.internalSrv.Close()
	}
	return nil
}

// startMarkdownReloadBridge starts a watcher on the deck dir that turns a deck.md
// (or any *.md) edit — which the gosx dev watcher ignores (it watches only
// .gsx/.go/.css/.js) — into a full reload via fireMarkdownReload (re-stage
// islands + dev.Server.TriggerReload). A .gsx edit is handled separately by the
// gosx watcher as an island hot-swap ("program" event, no reload); a deck.md
// edit changes the prose/layout the server renders, which needs a full reload.
func (l *DevLoop) startMarkdownReloadBridge() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(l.deckDir); err != nil {
		_ = watcher.Close()
		return err
	}
	l.mdWatcher = watcher
	l.stopMD = make(chan struct{})

	go l.markdownReloadLoop()
	return nil
}

func (l *DevLoop) markdownReloadLoop() {
	// Debounce: editors often emit several events per save; collapse them so we
	// re-stage and reload once per burst.
	var (
		timer  *time.Timer
		timerC <-chan time.Time
	)
	resetTimer := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.NewTimer(75 * time.Millisecond)
		timerC = timer.C
	}
	for {
		select {
		case <-l.stopMD:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-l.mdWatcher.Events:
			if !ok {
				return
			}
			if !isMarkdownWriteEvent(event) {
				continue
			}
			resetTimer()
		case err, ok := <-l.mdWatcher.Errors:
			if !ok {
				return
			}
			l.logf("deck.md watcher error: %v", err)
		case <-timerC:
			timer = nil
			timerC = nil
			l.fireMarkdownReload()
		}
	}
}

// fireMarkdownReload re-stages the island programs (so a hard refresh after the
// reload fetches fresh bytecode — a deck.md edit can add or drop a component
// reference) and drives a full reload via the dev server's public TriggerReload,
// which broadcasts over the one SSE the clients connect to. deck.md edits change
// the prose/layout the server renders (and the dev deck server re-loads the deck
// per request), so they need a full reload, not an island hot-swap.
func (l *DevLoop) fireMarkdownReload() {
	if err := StageIslandPrograms(l.deckDir); err != nil {
		l.logf("re-stage island programs after deck.md change: %v", err)
	}
	l.Server.TriggerReload("deck.md changed")
	l.logf("deck.md changed, triggering reload")
}

// isMarkdownWriteEvent reports whether a watch event is a create/write/rename on
// a .md file (the deck content the gosx watcher ignores).
func isMarkdownWriteEvent(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
		return false
	}
	return strings.EqualFold(filepath.Ext(event.Name), ".md")
}

// pickFreePort asks the OS for an unused TCP port on the loopback interface and
// returns it, so the in-process deck server can bind a non-colliding internal
// port. Mirrors gosx's cmd/gosx pickFreePort.
func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", ln.Addr())
	}
	return addr.Port, nil
}

// waitForReady polls baseURL until it answers below 500 (the deck server is up)
// or the timeout elapses. Mirrors gosx's cmd/gosx waitForAppReady: the deck
// server has no /readyz, so "/" is the readiness probe.
func waitForReady(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < http.StatusInternalServerError {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", baseURL)
}

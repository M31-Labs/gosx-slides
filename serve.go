package slides

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island/program"
	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

// serve.go is the deck server. It composes the lowering core (CompileComponent)
// with the node lowering into a runnable gosx server.App: a deck whose slides
// host LIVE GoSX islands, with the island programs served as JSON, the client
// WASM runtime staged so the page hydrates in a real browser, and the
// cross-device presenter endpoints (present_broker.go) mounted alongside.

// gosxModuleImportPath is the gosx module. gosx-slides depends on it as a public
// release (no replace), so `go list -m` resolves it to the module cache for asset
// staging. A scaffolded deck's generated go.mod requires the same module, so the
// same resolution works with cmd.Dir set to a deck dir outside this repo.
const gosxModuleImportPath = "m31labs.dev/gosx"

// ServeOptions configures the real-lane deck server.
type ServeOptions struct {
	// Addr is the listen address for Serve (e.g. "127.0.0.1:8080"). Ignored by
	// NewServer, which only builds the App.
	Addr string

	// Title is the HTML document <title>. Defaults to the deck's first heading,
	// then to the deck directory name.
	Title string

	// StageRuntime, when true, builds and stages the client WASM runtime
	// (gosx-runtime.wasm + wasm_exec.js + bootstrap/patch JS) into <deck>/build
	// and points the App's runtime root there, so /gosx/runtime.wasm and friends
	// serve and the island hydrates without `gosx dev`. Staging is cached: an
	// already-built runtime.wasm is reused (see RebuildRuntime).
	StageRuntime bool

	// RebuildRuntime forces the GOOS=js runtime.wasm to be rebuilt even when a
	// cached build/gosx-runtime.wasm already exists. The wasm build is
	// existence-cached (it is slow), so without this a gosx runtime change is NOT
	// picked up — `slides serve --rebuild` (or deleting build/) forces a fresh
	// build. No effect unless StageRuntime is also set.
	RebuildRuntime bool

	// Dev makes the deck server re-load the deck from disk on every GET / so
	// live edits to deck.md (and its components) take effect on the next request
	// without a manual restart. It is the in-process upstream behind the
	// `slides dev`/`slides serve --watch` hot-swap loop: the dev proxy front
	// (gosx/dev.Server) issues a full "reload" after a deck.md edit, and the
	// re-loaded deck then renders the new content.
	//
	// Re-parsing a deck and re-compiling its islands is milliseconds, so paying
	// it per request in dev is fine; the production `serve` lane leaves this
	// false and compiles once at startup. A re-load failure (e.g. a deck.md the
	// user is mid-edit) is non-fatal: the handler falls back to the deck the
	// server started with so the page never 500s on a transient bad parse.
	Dev bool
}

// NewServer builds (but does not start) a gosx server.App that serves the deck
// in the real lane. Each distinct referenced component is compiled to island
// bytecode exactly once and shared across slides; its JSON program is mounted at
// /gosx/islands/<Name>.json, and the page renders every slide with its live
// islands. When opts.StageRuntime is set, the client runtime is staged and
// served so the islands hydrate in the browser.
func (d *IslandDeck) NewServer(opts ServeOptions) (*server.App, error) {
	if d == nil {
		return nil, fmt.Errorf("NewServer: nil deck")
	}

	// Compile each distinct component once (CompileComponent recompiles on every
	// call — cache by name) and mount its JSON. The compiled cache is read-only
	// and is shared across every request; only the island.Renderer is per-request
	// (see the "/" handler below).
	//
	// A missing/uncompilable component is intentionally NOT fatal: it is left out
	// of the cache and renders as the inert data-gosx-unresolved placeholder, so a
	// typo'd or not-yet-created component degrades gracefully instead of 500-ing
	// the whole deck.
	compiled, failures := d.compileComponents()
	logCompileFailures(d.Dir, failures)

	app := server.New()
	app.SetPublicDir(d.Dir)

	for _, name := range sortedKeys(compiled) {
		cc := compiled[name]
		assetPath := "/gosx/islands/" + name + ".json"
		jsonBytes := cc.json
		app.Mount(assetPath, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			_, _ = w.Write(jsonBytes)
		}))
	}

	// Cross-device presenter: one broker per deck server relays {slide, step} from
	// the presenter window / phone remote to every audience over SSE. These mounts
	// are inert until a client connects, so they cost nothing for a plain audience
	// load (and are harmless on the export render path, which never hits them).
	broker := newPresenterBroker()
	app.Mount("/presenter/events", http.HandlerFunc(broker.handleEvents))
	app.Mount("/presenter/state", http.HandlerFunc(broker.handleState))
	app.Mount("/remote", http.HandlerFunc(handleRemote))

	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = d.title()
	}

	// The deck is an App.Page (NOT App.Route): the handler returns the page BODY,
	// and the App builds ONE document around it. This is the fix for the
	// nested-document hydration bug — App.Route would have wrapped a handler-built
	// server.HTMLDocument in the App's OWN document, nesting <html> inside <body>
	// so the island runtime never wired up. By routing through App.Page and
	// registering every island on ctx.Runtime() (the App's PageRuntime), the App
	// sees the islands, emits the correct document contract (runtime enabled), and
	// auto-adds the runtime's manifest + bootstrap to the single <head> (gosx
	// server.renderPageNode -> decoratePageContext). See examples/gosx-docs.
	//
	// Islands register on a fresh per-request renderer owned by ctx.Runtime()
	// (server.NewPageRuntime mints one per response), so there is no shared mutable
	// renderer to race or accumulate stale islands across requests.
	//
	// In dev mode the DECK itself is re-loaded per request (re-parse deck.md +
	// re-compile its components), so a deck.md edit shows new content after the
	// dev proxy's full reload. A re-load failure falls back to the startup deck +
	// cache so a mid-edit deck.md never 500s the page.
	app.Page("/", func(ctx *server.Context) gosx.Node {
		renderDeck, renderCompiled, renderFailures := d, compiled, failures
		if opts.Dev {
			if fresh, err := LoadIslandDeck(d.Dir); err == nil {
				if freshCompiled, freshFailures := fresh.compileComponents(); freshCompiled != nil {
					logCompileFailures(fresh.Dir, freshFailures)
					renderDeck, renderCompiled, renderFailures = fresh, freshCompiled, freshFailures
				}
			} else {
				log.Printf("slides: dev reload of deck %q failed; serving last good deck: %v", d.Dir, err)
			}
		}
		// Point each registered island at its served JSON program so the manifest
		// carries a fetchable programRef (ctx.Runtime()'s renderer has no program
		// dir by default, which would emit an empty programRef and break hydration).
		rt := ctx.Runtime()
		for _, name := range sortedKeys(renderCompiled) {
			rt.SetProgramAsset(name, "/gosx/islands/"+name+".json", "json", "")
		}
		ctx.SetMetadata(server.Metadata{Title: server.Title{Absolute: title}})
		return renderDeck.renderPageBody(ctx, renderCompiled, opts.Dev, renderFailures)
	})

	if opts.StageRuntime {
		root, err := StageRuntimeAssets(d.Dir, opts.RebuildRuntime)
		if err != nil {
			return nil, fmt.Errorf("stage runtime assets: %w", err)
		}
		app.SetRuntimeRoot(root)
	}

	return app, nil
}

// Serve builds the real-lane App for the deck and serves it on opts.Addr,
// staging the client runtime so islands hydrate. It blocks until the server
// stops.
func (d *IslandDeck) Serve(opts ServeOptions) error {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:8080"
	}
	opts.StageRuntime = true
	app, err := d.NewServer(opts)
	if err != nil {
		return err
	}
	return app.ListenAndServe(opts.Addr)
}

// ServeDeck loads the deck at dir and serves it in the real lane. It is the
// entry point the `slides serve` CLI command calls.
func ServeDeck(dir string, opts ServeOptions) error {
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		return err
	}
	return deck.Serve(opts)
}

// runtimeMounter adapts a *server.PageRuntime to the islandMounter interface the
// lowering lanes (render_program.go / render_island.go) consume. Islands rendered
// through it register on the App's PageRuntime, so the App's document contract
// reports the runtime as active and auto-emits the manifest + bootstrap into the
// single document <head> — the crux of the nested-document fix.
type runtimeMounter struct{ rt *server.PageRuntime }

// RenderIslandFromProgram registers the program as an island on the page runtime
// and returns its hydratable shell. It satisfies islandMounter (same signature as
// *island.Renderer.RenderIslandFromProgram).
func (m runtimeMounter) RenderIslandFromProgram(prog *program.Program, props any) gosx.Node {
	return m.rt.Island(prog, props)
}

// renderPageBody renders the whole deck as the page BODY for an App.Page handler
// and registers per-slide head assets on the context. It does NOT build a
// document — the App does that around the returned body, emitting exactly one
// <html>/<head>/<body> with the correct runtime contract.
//
// Slides render through the source-gen lane (renderProgramSlides): the deck is
// lowered to one GoSX source, compiled once, and each slide rendered via
// route.RenderProgramComponent — so inline {expr} EVALUATES server-side and
// inline <Component/> tags hydrate as real islands. Islands register on
// ctx.Runtime() (via runtimeMounter), so the App's auto-added runtime head sees
// them and ships the manifest + bootstrap. If the deck fails to compile, the flow
// falls back to the hand-built lane (renderIslandSlide) so a transient bad deck
// still serves (prose + islands; {expr} as raw text).
func (d *IslandDeck) renderPageBody(ctx *server.Context, compiled map[string]*compiledComponent, dev bool, failures map[string]error) gosx.Node {
	r := runtimeMounter{rt: ctx.Runtime()}
	cd, err := compileDeckProgram(d)
	if err != nil {
		// The deck failed to compile as one program: every slide will degrade to
		// the hand-built lane with inline {expr} rendered as raw text (the
		// documented safety net). That is a silent, deck-wide loss of live
		// expressions on an HTTP 200, so make it loud here — this is the single
		// most dangerous quiet failure in the real lane. A healthy deck never logs.
		log.Printf("slides: deck %q failed to compile; serving prose with inline {expr} as raw text: %v", d.Dir, err)
	}
	slideNodes := renderProgramSlides(r, d, cd, compiled)

	// Resolve the deck's theme from its `theme:` headmatter (themeName falls back
	// to the default for an absent/unknown value), so the served head always
	// carries one real theme and `<main>` a matching data-theme hook.
	theme := themeName(deckTheme(d))

	// Slide-visibility CSS + the selected THEME + viewport go in the document head
	// via the Context. The App composes ctx.Head() into the single <head>, after
	// which it appends the runtime's own head (manifest + bootstrap) — so these
	// never collide with the island bootstrap. The theme CSS is scoped under
	// main.deck[data-theme="<name>"] (themes.go) and the nav rule under the bare
	// main.deck, so they layer cleanly: themes never override slide visibility.
	ctx.AddHead(
		gosx.RawHTML(`<meta name="viewport" content="width=device-width, initial-scale=1">`),
		// Load ONLY the selected theme's designer webfonts (preconnect + one css2
		// stylesheet). The theme CSS keeps a system fallback at the end of every
		// --font-* stack, so an offline deck still looks intentional; this link makes
		// the designer faces actually render. fontLinks returns "" for a webfont-less
		// theme, in which case this emits nothing.
		gosx.RawHTML(fontLinks(theme)),
		// navStyle (one-slide visibility + overview grid) and presenterStyle (the
		// ?present chrome) go in one <style>. presenterStyle is inert until the
		// controller adds the deck-presenter class on a ?present load AND hides the
		// speaker-note asides below in BOTH views, so the audience page is unaffected.
		gosx.RawHTML("<style>"+navStyle()+"\n"+presenterStyle()+"\n"+baseContentStyle()+"</style>"),
		gosx.RawHTML("<style>"+themeCSS(theme)+"\n"+baseLayoutStyle()+"</style>"),
	)
	if dev {
		// Dev-only chrome CSS, injected solely in --watch so it never reaches a
		// production page or static export.
		ctx.AddHead(gosx.RawHTML("<style>" + devOverlayStyle() + "</style>"))
	}
	if deckHasDiagram(d) {
		// Inject diagram layout CSS only when the deck contains a diagram node —
		// non-diagram decks skip this style block entirely. Diagrams are rendered
		// server-side to inline SVG (fence.Render via renderSirenaDiagram), so there
		// is no script tag and no CDN dependency at all.
		ctx.AddHead(
			gosx.RawHTML("<style>" + baseDiagramStyle() + "</style>"),
		)
	}

	// Hidden per-slide speaker-note asides: one <aside class="slide-notes"
	// data-notes="N"> for every slide that HAS a note (extractSlideNotes reads the
	// <Notes>…</Notes> / trailing <!-- … --> forms out of the slide's mdpp subtree).
	// presenterStyle hides these in both views; the presenter chrome reads the
	// current slide's note out of them. A slide with no note emits nothing (the
	// presenter shows a graceful placeholder).
	noteNodes := d.noteAsides()

	return gosx.El("main",
		gosx.Attrs(
			gosx.Attr("class", "deck"),
			gosx.Attr("data-theme", theme),
			// data-dev gates the dev-only overflow badge; data-transition picks the
			// slide enter animation (fade | none) — both read by navScript.
			gosx.Attr("data-dev", boolAttr(dev)),
			gosx.Attr("data-transition", deckTransition(d)),
			// data-line-numbers="1" (deck headmatter `line-numbers: true`) turns on
			// the code-block line-number gutter (a CSS ::before; see baseContentStyle).
			gosx.Attr("data-line-numbers", boolAttr(deckLineNumbers(d))),
		),
		gosx.Fragment(slideNodes...),
		gosx.Fragment(noteNodes...),
		// Dev-only build-error overlay: a deck/island compile failure is otherwise
		// only a terminal log + a silently-degraded slide. In --watch, surface it
		// loudly in the page so the author sees it without leaving the browser.
		devErrorOverlay(dev, err, failures),
		// The slide-nav controller + presenter chrome controller run at the END of
		// the body, so the data-slide sections (and note asides) above already exist
		// when they wire up. presenterScript is emitted FIRST so it has defined
		// window.SlidesPresenter by the time navScript's IIFE runs its end-of-load
		// `if (present) SlidesPresenter.init(...)` call (each is a self-invoking IIFE,
		// so navScript would otherwise see an undefined SlidesPresenter). navScript
		// shows ONE slide at a time, handles keyboard + URL-hash nav, and (on a
		// ?present load) calls the presenter controller; both are self-contained (no
		// island-runtime dependency) and do not disturb the island bootstrap the App
		// adds to the head — hidden slides still hydrate.
		gosx.RawHTML("<script>"+presenterScript()+"\n"+navScript()+"\n"+codeCopyScript()+"</script>"),
	)
}

// devErrorOverlay builds a dismissible in-page build-error banner for the --watch
// loop. It returns an empty node unless dev is set AND there is a deck-compile
// error or a component-compile failure — so a healthy deck (and every production
// serve) renders nothing. Compiler text is HTML-escaped (opaque, untrusted).
func devErrorOverlay(dev bool, deckErr error, failures map[string]error) gosx.Node {
	if !dev || (deckErr == nil && len(failures) == 0) {
		return gosx.RawHTML("")
	}
	var b strings.Builder
	b.WriteString(`<div class="deck-dev-error" onclick="this.remove()" title="click to dismiss">`)
	b.WriteString(`<div class="deck-dev-error-head">⚠ gosx-slides build error <span>(dev only · click to dismiss · fix the source and save to reload)</span></div>`)
	if deckErr != nil {
		b.WriteString("<pre>deck did not compile — inline {expr} is degraded to raw text:\n")
		b.WriteString(html.EscapeString(deckErr.Error()))
		b.WriteString("</pre>")
	}
	for _, name := range sortedFailureNames(failures) {
		b.WriteString("<pre>")
		b.WriteString(html.EscapeString(name))
		b.WriteString(".gsx did not compile (rendered as an inert placeholder):\n")
		b.WriteString(html.EscapeString(failures[name].Error()))
		b.WriteString("</pre>")
	}
	b.WriteString(`</div>`)
	return gosx.RawHTML(b.String())
}

// devOverlayStyle is the CSS for the dev build-error banner and the loud
// unresolved-component placeholder. It is injected ONLY in dev mode (so the
// `[data-gosx-unresolved]` selector never appears in a production page), keeping
// authoring-error chrome out of shipped/exported output entirely.
func devOverlayStyle() string {
	return `main.deck .deck-dev-error { position: fixed; left: 0; right: 0; bottom: 0; z-index: 100; max-height: 60vh; overflow: auto; cursor: pointer; padding: 1rem 1.25rem; background: rgba(38,8,8,0.97); color: #ffd7d7; border-top: 3px solid #ff6b6b; font: 0.85rem/1.5 var(--font-mono, ui-monospace, monospace); }
main.deck .deck-dev-error-head { margin-bottom: 0.5rem; font-weight: 700; color: #ff8a8a; }
main.deck .deck-dev-error-head span { font-weight: 400; opacity: 0.7; }
main.deck .deck-dev-error pre { margin: 0.4rem 0; padding: 0.5rem 0.7rem; white-space: pre-wrap; word-break: break-word; background: rgba(0,0,0,0.28); border-radius: 6px; }
main.deck .gosx-unresolved { position: relative; outline: 2px dashed #ff6b6b; outline-offset: 3px; min-width: 12rem; min-height: 2rem; }
main.deck .gosx-unresolved::after { content: "\26A0 unresolved component (dev)"; position: absolute; top: 0; left: 0; padding: 0.15rem 0.45rem; font: 700 0.7rem var(--font-mono, ui-monospace, monospace); color: #ff6b6b; background: rgba(255,107,107,0.16); }`
}

func sortedFailureNames(failures map[string]error) []string {
	names := make([]string, 0, len(failures))
	for n := range failures {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// deckTheme reads the deck's raw `theme:` headmatter value as a string, returning
// "" when it is absent (or not a string). It reuses deckFrontmatterValues (the
// same headmatter the `{deck.theme}` expression sees) so the served theme and any
// in-prose reference to it stay in sync, and guards the type assertion so a deck
// without a theme key never panics — themeName then resolves "" to the default.
func deckTheme(d *IslandDeck) string {
	if v, ok := deckFrontmatterValues(d)["theme"].(string); ok {
		return v
	}
	return ""
}

// deckTransition reads the deck's `transition:` headmatter, normalized to the
// slide enter animation navStyle understands: "none" disables motion, anything
// else (including absent) is the default "fade".
func deckTransition(d *IslandDeck) string {
	if v, ok := deckFrontmatterValues(d)["transition"].(string); ok {
		if strings.EqualFold(strings.TrimSpace(v), "none") {
			return "none"
		}
	}
	return "fade"
}

// deckLineNumbers reports whether the deck's `line-numbers:` headmatter opts into
// the code-block line-number gutter (true / yes / on / 1).
func deckLineNumbers(d *IslandDeck) bool {
	v, _ := deckFrontmatterValues(d)["line-numbers"].(string)
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

// boolAttr renders a boolean as the "1"/"0" string used by the deck's data-* hooks.
func boolAttr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// compileComponents compiles every distinct component referenced anywhere in the
// deck exactly once, returning a name->compiled cache.
//
// A component whose <Name>.gsx is missing or fails to compile is SOFT-DEGRADED,
// not fatal: it is simply left out of the cache. At render time renderComponentRef
// emits the inert data-gosx-unresolved placeholder for any name absent from the
// cache, so a typo'd or not-yet-created component degrades to a visible placeholder
// instead of 500-ing the whole presentation. The error is returned via the second
// result (a name->error map) so callers can surface it (e.g. in `doctor`) without
// failing the server.
func (d *IslandDeck) compileComponents() (map[string]*compiledComponent, map[string]error) {
	compiled := map[string]*compiledComponent{}
	var failures map[string]error
	for _, slide := range d.Slides {
		for _, ref := range slide.Components {
			if _, ok := compiled[ref.Name]; ok {
				continue
			}
			if failures != nil {
				if _, failed := failures[ref.Name]; failed {
					continue
				}
			}
			prog, jsonBytes, err := d.CompileComponent(ref.Name)
			if err != nil {
				// Soft-degrade: record the failure and leave the component out of
				// the cache so it renders as the inert unresolved placeholder.
				if failures == nil {
					failures = map[string]error{}
				}
				failures[ref.Name] = err
				continue
			}
			compiled[ref.Name] = &compiledComponent{prog: prog, json: jsonBytes}
		}
	}
	return compiled, failures
}

// deckName is a stable bundle id for the island renderer, derived from the deck
// directory name.
func (d *IslandDeck) deckName() string {
	base := filepath.Base(strings.TrimRight(d.Dir, string(os.PathSeparator)))
	if base == "" || base == "." || base == string(os.PathSeparator) {
		return "deck"
	}
	return base
}

// title returns the deck's document title: its headmatter `title:` if set (the
// canonical title, also bound as {deck.title}), else its first heading's text,
// else the deck directory name. Preferring headmatter keeps the served <title>
// and the analysis tools agreeing with the author's declared title even when the
// first heading is an unevaluated `# {deck.title}` expression.
func (d *IslandDeck) title() string {
	if t, ok := deckFrontmatterValues(d)["title"].(string); ok {
		if t = strings.TrimSpace(t); t != "" {
			return t
		}
	}
	for _, slide := range d.Slides {
		if slide.Node == nil {
			continue
		}
		var found string
		slide.Node.Walk(func(n *mdpp.Node) bool {
			if found != "" {
				return false
			}
			if n.Level() >= 1 {
				found = strings.TrimSpace(n.Text())
				return false
			}
			return true
		})
		if found != "" {
			return found
		}
	}
	return d.deckName()
}

// StageRuntimeAssets builds and stages the client WASM runtime into <deckDir>/
// build so the gosx server can serve /gosx/runtime.wasm and /gosx/wasm_exec.js.
// It mirrors `gosx dev`'s prepareDevAssets layout exactly (see
// cmd/gosx/dev.go): runtime.wasm -> build/gosx-runtime.wasm, wasm_exec.js ->
// build/wasm_exec.js, plus the bootstrap/patch JS the browser loads for
// hydration — the directory layout server/runtime_assets.go's
// runtimeCompatSourcePath expects. It returns the runtime root to pass to
// App.SetRuntimeRoot (the deck dir).
//
// The runtime.wasm is EXISTENCE-CACHED: if build/gosx-runtime.wasm already exists
// it is NOT rebuilt, because the GOOS=js build is slow. This means a change to the
// gosx runtime is NOT picked up on a subsequent `slides serve` until the cache is
// invalidated — pass rebuild=true (the `slides serve --rebuild` flag) or delete
// the build/ directory to force a fresh build. wasm_exec.js and the bootstrap JS
// are cheap copies and are always refreshed.
func StageRuntimeAssets(deckDir string, rebuild bool) (string, error) {
	// Resolve to an absolute path: the `go build -o` output path below must be
	// absolute because we run the build with cmd.Dir set to the deck dir, and a
	// relative -o would then resolve against that dir (doubly-nesting it).
	absDeckDir, err := filepath.Abs(deckDir)
	if err != nil {
		return "", fmt.Errorf("resolve deck dir: %w", err)
	}
	deckDir = absDeckDir

	buildDir := filepath.Join(deckDir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("create build dir: %w", err)
	}

	gosxRoot, err := resolveGoSXRoot(deckDir)
	if err != nil {
		return "", err
	}

	// 1. runtime.wasm — the expensive artifact; build once and cache. rebuild
	// forces a fresh build even when a cached artifact exists (see I2: a gosx
	// runtime change is otherwise never picked up).
	wasmPath := filepath.Join(buildDir, "gosx-runtime.wasm")
	// The wasm is existence-cached, but a truncated or empty cached artifact (an
	// interrupted prior build) would otherwise be served silently and the islands
	// would never hydrate — a baffling failure mid-demo. Treat a sub-floor cached
	// file as a cache miss and rebuild. The real runtime is tens of MB; this floor
	// only catches corruption, never a legitimately small build.
	const minRuntimeWasmBytes = 1 << 20 // 1 MiB
	corruptCache := isRegularFile(wasmPath) && fileSizeBelow(wasmPath, minRuntimeWasmBytes)
	if rebuild || !isRegularFile(wasmPath) || corruptCache {
		// Remove any existing artifact first when forcing a rebuild or when the
		// cached file is corrupt: `go build -o` refuses to overwrite an output that
		// is not a Go object file (e.g. a stale or corrupt file), and removing it
		// also guarantees the rebuild can't be a silent reuse of the old bytes.
		if rebuild || corruptCache {
			if corruptCache {
				log.Printf("slides: cached runtime wasm at %s is truncated; rebuilding", wasmPath)
			}
			if err := os.Remove(wasmPath); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("remove stale runtime wasm: %w", err)
			}
		}
		cmd := exec.Command("go", "build", "-o", wasmPath, gosxModuleImportPath+"/client/wasm")
		cmd.Dir = deckDir
		// Neutralize an ambient GOFLAGS (e.g. an exported `GOFLAGS=-mod=vendor`)
		// that would otherwise skew the GOOS=js build, then set the wasm env
		// explicitly — mirroring gosx's execEnvWithoutGoFlags (cmd/gosx/dev.go).
		// GOFLAGS=-mod=mod (not empty) lets a freshly-scaffolded deck module
		// (go.mod present, go.sum not yet populated — see scaffold_real.go) have its
		// go.sum and indirect requires auto-filled during this build, so a portable
		// deck resolves gosx on its FIRST serve. For the in-repo case (cmd.Dir
		// inside gosx-slides, go.sum already complete) -mod=mod is a no-op.
		cmd.Env = append(execEnvWithoutGoFlags(),
			"GOOS=js", "GOARCH=wasm", "GOWORK=off", "GOFLAGS=-mod=mod",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("build runtime wasm (GOOS=js GOARCH=wasm go build %s/client/wasm): %w", gosxModuleImportPath, err)
		}
		// Guard against a silent no-op: the artifact must exist after a clean
		// build, or downstream serving would 404 with no explanation.
		if !isRegularFile(wasmPath) {
			return "", fmt.Errorf("build runtime wasm: %s/client/wasm produced no output at %s", gosxModuleImportPath, wasmPath)
		}
	}

	// 2. wasm_exec.js — straight from the Go toolchain.
	if err := copyFirstExisting(
		filepath.Join(buildDir, "wasm_exec.js"),
		filepath.Join(goroot(), "lib", "wasm", "wasm_exec.js"),
		filepath.Join(goroot(), "misc", "wasm", "wasm_exec.js"),
	); err != nil {
		return "", fmt.Errorf("stage wasm_exec.js: %w", err)
	}

	// 3. bootstrap + patch JS — small client glue the browser loads to drive
	// hydration. runtime_assets.go also resolves these from <root>/client/js as a
	// fallback, but staging them into build/ keeps the whole runtime under one
	// root that we own.
	for _, name := range []string{
		"bootstrap.js",
		"bootstrap-lite.js",
		"bootstrap-runtime.js",
		"bootstrap-feature-islands.js",
		"patch.js",
	} {
		src := filepath.Join(gosxRoot, "client", "js", name)
		if !isRegularFile(src) {
			continue
		}
		if err := copyFile(filepath.Join(buildDir, name), src); err != nil {
			return "", fmt.Errorf("stage %s: %w", name, err)
		}
	}

	return deckDir, nil
}

// StageIslandPrograms compiles every distinct component referenced by the deck
// at deckDir to its JSON wire program and writes it to <deckDir>/build/islands/
// <Name>.json. This is the on-disk asset the dev proxy front (gosx/dev.Server)
// serves at /gosx/islands/<Name>.json — it SHADOWS the proxied deck server's own
// island mounts, so the initial page load (and a hard refresh after a hot-swap)
// reads the bytecode from disk. The live hot-swap itself does not depend on this:
// the "program" SSE event carries the fresh bytecode inline.
//
// It mirrors `gosx dev`'s compileDevIslands layout (build/islands/<Name>.json),
// scoped to the components the deck actually references. A component that is
// missing or fails to compile is SOFT-DEGRADED (skipped), matching NewServer: it
// renders as the inert unresolved placeholder rather than failing the loop. Stale
// JSON for components no longer referenced is cleared first so a removed component
// does not leave a serveable orphan.
func StageIslandPrograms(deckDir string) error {
	absDeckDir, err := filepath.Abs(deckDir)
	if err != nil {
		return fmt.Errorf("resolve deck dir: %w", err)
	}

	deck, err := LoadIslandDeck(absDeckDir)
	if err != nil {
		return err
	}

	islandDir := filepath.Join(absDeckDir, "build", "islands")
	if err := os.MkdirAll(islandDir, 0o755); err != nil {
		return fmt.Errorf("create island build dir: %w", err)
	}

	// Clear any previously-staged island JSON so a renamed/removed component does
	// not leave a stale, serveable file behind.
	if entries, err := os.ReadDir(islandDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				_ = os.Remove(filepath.Join(islandDir, entry.Name()))
			}
		}
	}

	// compileComponents compiles each distinct referenced component once and
	// soft-degrades failures — exactly the cache NewServer mounts, so the staged
	// JSON is byte-identical to what the production lane serves in-process.
	compiled, failures := deck.compileComponents()
	logCompileFailures(deck.Dir, failures)
	for _, name := range sortedKeys(compiled) {
		path := filepath.Join(islandDir, name+".json")
		if err := os.WriteFile(path, compiled[name].json, 0o644); err != nil {
			return fmt.Errorf("write island %s: %w", path, err)
		}
	}
	return nil
}

// resolveGoSXRoot returns the gosx module directory (the local module cache entry,
// or the gosx-slides checkout's own resolution when projectDir is inside it) so
// its client/js assets can be staged. It runs `go list` with cmd.Dir = projectDir,
// so projectDir must be (or live inside) a Go module that requires gosx — for a
// scaffolded deck that is the generated go.mod (scaffold_real.go).
//
// Both the module-mode list and its fallback run with GOFLAGS=-mod=mod so a
// freshly-scaffolded deck (go.mod present, go.sum not yet populated) can resolve
// and download gosx on its first serve. For the in-repo case this is a no-op.
func resolveGoSXRoot(projectDir string) (string, error) {
	listEnv := append(execEnvWithoutGoFlags(), "GOFLAGS=-mod=mod")
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", gosxModuleImportPath)
	cmd.Dir = projectDir
	cmd.Env = listEnv
	out, err := cmd.Output()
	if err != nil {
		// Fall back to a non-module `go list` for older layouts.
		cmd2 := exec.Command("go", "list", "-f", "{{.Dir}}", gosxModuleImportPath)
		cmd2.Dir = projectDir
		cmd2.Env = listEnv
		out, err = cmd2.Output()
		if err != nil {
			return "", fmt.Errorf("resolve %s module root: %w", gosxModuleImportPath, err)
		}
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("resolve %s module root: empty result", gosxModuleImportPath)
	}
	return dir, nil
}

func goroot() string {
	if r := strings.TrimSpace(os.Getenv("GOROOT")); r != "" {
		return r
	}
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// fileSizeBelow reports whether path is smaller than min bytes. A stat error
// counts as "below" so the caller rebuilds rather than trusting an artifact it
// cannot measure.
func fileSizeBelow(path string, min int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return info.Size() < min
}

// logCompileFailures logs each component that failed to compile. compileComponents
// soft-degrades a broken/missing <Name>.gsx to an inert placeholder and returns the
// error via its second result; every call site otherwise discarded it, so a broken
// island was silent. Logging here makes it loud in the terminal (especially the
// `serve --watch` loop) without failing the server. A healthy deck logs nothing.
func logCompileFailures(dir string, failures map[string]error) {
	for name, err := range failures {
		log.Printf("slides: component %s.gsx in %q failed to compile; rendering inert placeholder: %v", name, dir, err)
	}
}

// execEnvWithoutGoFlags returns the process environment with any GOFLAGS entry
// removed, so callers can set an explicit GOFLAGS for a subprocess without an
// ambient `GOFLAGS=-mod=vendor` (or similar) leaking in. Mirrors gosx's
// cmd/gosx/moddeps.go helper of the same name (not importable from there).
func execEnvWithoutGoFlags() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyFirstExisting(dst string, candidates ...string) error {
	for _, c := range candidates {
		if isRegularFile(c) {
			return copyFile(dst, c)
		}
	}
	return fmt.Errorf("none of the candidate sources exist: %v", candidates)
}

func sortedKeys(m map[string]*compiledComponent) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

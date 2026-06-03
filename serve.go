package slides

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

// serve.go is the real lane's deck server (Phase 1, Slice 2). It composes the
// Slice-1 lowering core (CompileComponent) with the Slice-2 node lowering
// (renderIslandSlide) into a runnable gosx server.App: a deck whose slides host
// LIVE GoSX islands, with the island programs served as JSON and (optionally)
// the client WASM runtime staged so the page hydrates in a real browser.
//
// It is the real-lane counterpart to the fallback presenter (server.go) and does
// not touch it: the fallback `Serve`/`ServerOptions` stay exactly as they were.

// gosxModuleImportPath is the gosx module; gosx-slides `replace`s it to ../gosx,
// so `go list` resolves it to the local checkout for asset staging.
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
	compiled, _ := d.compileComponents()

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

	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = d.title()
	}

	// One island.Renderer PER REQUEST. RenderIslandFromProgram mutates renderer
	// state (manifest.AddIsland, counter++) unguarded, so a single shared renderer
	// both races under concurrent GETs and accumulates stale islands across
	// sequential ones (the manifest is never reset). This mirrors gosx's canonical
	// per-page pattern (server.NewPageRuntime -> fresh island.Renderer per
	// response; see gosx/server/runtime.go). The compiled cache above stays shared
	// — only the renderer is rebuilt here.
	//
	// In dev mode the DECK itself is re-loaded per request too (re-parse deck.md +
	// re-compile its components), so a deck.md edit shows new content after the
	// dev proxy's full reload. A re-load failure falls back to the startup deck +
	// cache so a mid-edit deck.md never 500s the page.
	app.Route("/", func(_ *http.Request) gosx.Node {
		renderDeck, renderCompiled := d, compiled
		if opts.Dev {
			if fresh, err := LoadIslandDeck(d.Dir); err == nil {
				if freshCompiled, _ := fresh.compileComponents(); freshCompiled != nil {
					renderDeck, renderCompiled = fresh, freshCompiled
				}
			}
		}
		r := island.NewRenderer(renderDeck.deckName())
		for _, name := range sortedKeys(renderCompiled) {
			r.SetProgramAsset(name, "/gosx/islands/"+name+".json", "json", "")
		}
		return renderDeck.renderPage(r, title, renderCompiled)
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

// renderPage renders the whole deck as one HTML document: every slide rendered
// in order (minimal multi-slide handling — no presenter chrome), with the island
// renderer's PageHead wiring the client bootstrap.
//
// Slice 4: slides render through the source-gen lane (renderProgramSlides) — the
// deck is lowered to one GoSX source, compiled once, and each slide rendered via
// route.RenderProgramComponent — so inline {expr} actually EVALUATES server-side
// and inline <Component/> tags hydrate as real islands (inlined through r, which
// registers them so PageHead below sees them). If the deck fails to compile, the
// flow falls back to the original hand-built lane (renderIslandSlide) so a
// transient bad deck still serves (prose + islands; {expr} as raw text).
func (d *IslandDeck) renderPage(r *island.Renderer, title string, compiled map[string]*compiledComponent) gosx.Node {
	cd, _ := compileDeckProgram(d)
	slideNodes := renderProgramSlides(r, d, cd, compiled)
	body := gosx.El("main",
		gosx.Attrs(gosx.Attr("class", "deck")),
		gosx.Fragment(slideNodes...),
		// Slice 6: the slide-nav controller runs at the END of the body, so the
		// data-slide sections above already exist in the DOM when it wires up.
		// It shows ONE slide at a time and handles keyboard + URL-hash nav; it is
		// self-contained (no island-runtime dependency) and does not disturb the
		// PageHead island bootstrap — hidden slides still hydrate their islands.
		gosx.RawHTML("<script>"+navScript()+"</script>"),
	)
	head := gosx.Fragment(
		gosx.RawHTML(`<meta name="viewport" content="width=device-width, initial-scale=1">`),
		// Slice 6: slide-visibility CSS so only the active slide shows (the real
		// lane otherwise stacks every slide). Scoped under main.deck with its own
		// active class so it never collides with the fallback lane's styling.
		gosx.RawHTML("<style>"+navStyle()+"</style>"),
		// PageHead is emitted AFTER island mounts have been rendered into the
		// body above, so its client-runtime plan sees the registered islands.
		r.PageHead(),
	)
	return server.HTMLDocument(title, head, body)
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

// title returns the deck's document title: its first heading's text, else the
// deck directory name.
func (d *IslandDeck) title() string {
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
	if rebuild || !isRegularFile(wasmPath) {
		// On a forced rebuild, remove any existing artifact first: `go build -o`
		// refuses to overwrite an output that is not a Go object file (e.g. a stale
		// or corrupt file), and removing it also guarantees the rebuild can't be a
		// silent reuse of the old bytes.
		if rebuild {
			if err := os.Remove(wasmPath); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("remove stale runtime wasm: %w", err)
			}
		}
		cmd := exec.Command("go", "build", "-o", wasmPath, gosxModuleImportPath+"/client/wasm")
		cmd.Dir = deckDir
		// Neutralize an ambient GOFLAGS (e.g. an exported `GOFLAGS=-mod=vendor`)
		// that would otherwise skew the GOOS=js build, then set the wasm env
		// explicitly — mirroring gosx's execEnvWithoutGoFlags (cmd/gosx/dev.go).
		cmd.Env = append(execEnvWithoutGoFlags(),
			"GOOS=js", "GOARCH=wasm", "GOWORK=off", "GOFLAGS=",
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
	compiled, _ := deck.compileComponents()
	for _, name := range sortedKeys(compiled) {
		path := filepath.Join(islandDir, name+".json")
		if err := os.WriteFile(path, compiled[name].json, 0o644); err != nil {
			return fmt.Errorf("write island %s: %w", path, err)
		}
	}
	return nil
}

// resolveGoSXRoot returns the local gosx module directory (../gosx via the
// replace) so its client/js assets can be staged.
func resolveGoSXRoot(projectDir string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", gosxModuleImportPath)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		// Fall back to a non-module `go list` for older layouts.
		cmd2 := exec.Command("go", "list", "-f", "{{.Dir}}", gosxModuleImportPath)
		cmd2.Dir = projectDir
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

package slides

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// scaffold_real.go scaffolds a runnable deck (the `slides init` command): a deck a
// user/agent can `slides serve` immediately and see the whole pipeline — live
// GoSX islands, server-evaluated {expr}, a syntax-highlighted code block, and a
// theme — with zero further setup. It writes a deck whose pieces are exactly what
// serve.go renders.
//
// What it creates under <name>/:
//   - deck.md   — `theme:` headmatter + three slides: a `layout: title` opener
//     with {expr} (proves server evaluation), a slide with a <Counter Initial={3}/>
//     island (proves hydration), and a slide with a fenced ```go block + inline
//     {expr} (proves the new code-block highlighting).
//   - Counter.gsx — the working counter island (props.Initial), copied verbatim
//     from examples/showcase so the scaffold is a known-good, hot-swappable island.
//   - go.mod — `module <name>`, `go 1.26`, `require m31labs.dev/gosx <version>`.
//     This is what makes the scaffolded deck PORTABLE: `slides serve` builds the
//     GOOS=js runtime.wasm and resolves the gosx module root with cmd.Dir set to
//     the deck dir, so the deck must itself be (or live inside) a Go module that
//     requires gosx. Without this go.mod a deck serves only from inside the
//     gosx-slides repo; with it, a deck serves from any directory. The pinned
//     <version> is sourced from the running binary's build info (gosxScaffoldVersion)
//     so the scaffold requires the SAME gosx the `slides` binary was built against.
//   - .gitignore — ignores build/ (the staged ~30MB GOOS=js wasm + island JSON)
//     and *.test, so a scaffolded deck is clean to commit.
//   - README — a one-liner pointing at `slides serve <name>`.
//
// Authoring invariants the template MUST honor (they are the real lane's
// contract, and getting them wrong silently degrades the deck):
//   - Per-slide frontmatter is a LEADING ` ```yaml ` fence, never a `---` block
//     (mdpp lifts only a slide's first ```yaml fence into its frontmatter —
//     finalizeSlide). The deck's own headmatter is the leading `---` block.
//   - Component props bind by EXACT name: `<Counter Initial={3}/>` matches
//     props.Initial (capital I) in Counter.gsx. A lowercase `initial` would not
//     seed the island.

// ScaffoldRealOptions configures a real-lane deck scaffold.
type ScaffoldRealOptions struct {
	// Theme is the deck's `theme:` headmatter value. Empty falls back to
	// defaultTheme (aurora). An unknown theme is rejected so the scaffold can
	// never produce a deck that silently renders with the default instead of the
	// requested look.
	Theme string
}

// ScaffoldRealLane creates a new real-lane deck directory at name. It writes
// <name>/deck.md, <name>/Counter.gsx, <name>/go.mod, <name>/.gitignore, and
// <name>/README, then returns nil. It refuses to overwrite an existing deck.md so
// re-running it never clobbers work.
//
// The returned deck is immediately runnable AND PORTABLE: the generated go.mod
// (module <name>, requiring m31labs.dev/gosx) lets `slides serve <name>` build the
// runtime.wasm and resolve the gosx module root from any directory, not just
// inside the gosx-slides repo. `slides serve <name>` compiles the Counter island,
// evaluates the slide exprs, highlights the code block, and serves the chosen
// theme. The caller (cmd/slides) prints the serve hint.
func ScaffoldRealLane(name string, opts ScaffoldRealOptions) error {
	if name == "" {
		return fmt.Errorf("deck name is required")
	}
	theme := opts.Theme
	if theme == "" {
		theme = defaultTheme
	}
	if !isRealLaneTheme(theme) {
		return fmt.Errorf("unknown theme %q (choose one of: %s)", theme, themesList())
	}

	if err := os.MkdirAll(name, 0o755); err != nil {
		return err
	}

	deckPath := filepath.Join(name, DeckFileName)
	if _, err := os.Stat(deckPath); err == nil {
		return fmt.Errorf("%s already exists", deckPath)
	}
	if err := os.WriteFile(deckPath, []byte(realLaneDeck(theme)), 0o644); err != nil {
		return err
	}

	// Counter.gsx is copied verbatim from the reference showcase island so the
	// scaffold always ships a known-good, hot-swappable component whose prop name
	// (Initial) matches the deck's <Counter Initial={3}/>.
	if err := os.WriteFile(filepath.Join(name, "Counter.gsx"), []byte(realLaneCounter), 0o644); err != nil {
		return err
	}

	// go.mod makes the deck a self-contained Go module that requires gosx, so
	// `slides serve` (which runs the GOOS=js build and `go list -m` with cmd.Dir =
	// deck dir) resolves gosx from ANY directory. The module path is derived from
	// the deck dir's base name (sanitized), and the gosx version is pinned to what
	// the running binary was built against.
	if err := os.WriteFile(filepath.Join(name, "go.mod"), []byte(realLaneGoMod(name)), 0o644); err != nil {
		return err
	}

	// .gitignore keeps the staged build artifacts (the ~30MB GOOS=js wasm + island
	// JSON under build/) and compiled test binaries out of version control.
	if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte(realLaneGitignore), 0o644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(name, "README"), []byte(realLaneReadme(name)), 0o644); err != nil {
		return err
	}
	return nil
}

// isRealLaneTheme reports whether theme is one of the registered themes
// (aurora/paper/neon/swiss). It reuses the themeRegistry (themes.go) so the
// scaffold's accepted set can never drift from what serve.go can actually render.
func isRealLaneTheme(theme string) bool {
	_, ok := themeRegistry[theme]
	return ok
}

// themesList is a human-readable, comma-separated list of the valid themes for
// error messages.
func themesList() string {
	names := Themes()
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// realLaneDeck is the scaffolded deck.md for theme: a showcase that exercises the
// framework's features — live islands, server-evaluated {expr}, stepped code with
// line numbers, the section/two-cols/quote layouts, a table, and speaker notes —
// so the first-run deck shows what gosx-slides can do, not just that it runs. Each
// slide ends with a trailing block (a <!-- speaker note -->), which both seeds the
// presenter view and forces clean slide splitting. Built from a line slice so the
// ```yaml / ```go fences (backticks) read literally without fighting Go raw strings.
func realLaneDeck(theme string) string {
	lines := []string{
		"---",
		"title: My Deck",
		"theme: " + theme,
		"line-numbers: true",
		"---",
		"",
		"```yaml",
		"layout: title",
		"```",
		"",
		`# {strings.ToUpper("my deck")}`,
		"",
		"Welcome to **{deck.title}** — a live GoSX presentation, compiled, not templated.",
		"",
		"Arrow keys (← / →) or Space to move · `o` overview · `p` presenter · `f` fullscreen.",
		"",
		"<!-- Open the presenter view with `p`: current + next slide, these notes, and a timer. -->",
		"",
		"---",
		"",
		"```yaml",
		"layout: section",
		"```",
		"",
		"# Part one — live islands",
		"",
		"<!-- A section divider. Drop a `layout: section` slide between parts of the talk. -->",
		"",
		"---",
		"",
		"# A live island",
		"",
		"The counter below is a real GoSX component, compiled to island bytecode and",
		"hydrated in your browser — not a screenshot:",
		"",
		"<Counter Initial={3}/>",
		"",
		"Click the buttons — the count is genuine reactive state. Run `slides serve --watch`",
		"and edit Counter.gsx to hot-swap it in place, state preserved.",
		"",
		"<!-- Demo beat: bump the counter, then edit Counter.gsx live to show the hot-swap. -->",
		"",
		"---",
		"",
		"# Code, stepped and evaluated",
		"",
		"Fenced code is highlighted server-side; the `{3|4}` meta reveals lines on click,",
		"and `line-numbers: true` headmatter shows the gutter:",
		"",
		"```go {3|4}",
		"package main",
		"",
		"func main() {",
		"\tprintln(\"hello, slides\")",
		"}",
		"```",
		"",
		"Inline `{expr}` is evaluated by the GoSX compiler — two plus three is {2 + 3},",
		"and this is slide {slide.index}.",
		"",
		"<!-- Press → to step through the highlighted lines one at a time. -->",
		"",
		"---",
		"",
		"```yaml",
		"layout: two-cols",
		"```",
		"",
		"# Two columns",
		"",
		"- mdpp prose substrate",
		"- live GoSX islands",
		"- server-evaluated `{expr}`",
		"- stepped, numbered code",
		"- images and tables",
		"- four built-in themes",
		"- quote / section / two-cols layouts",
		"- a cross-device presenter",
		"",
		"<!-- two-cols flows the body into two balanced columns; the heading spans both. -->",
		"",
		"---",
		"",
		"# At a glance",
		"",
		"| Feature    | How                      |",
		"| ---------- | ------------------------ |",
		"| Component  | `<Counter Initial={3}/>` |",
		"| Expression | `{6 * 7}` renders 42     |",
		"| Theme      | `theme:` headmatter      |",
		"| Layout     | `layout:` per slide      |",
		"",
		"<!-- GFM tables render, themed. -->",
		"",
		"---",
		"",
		"```yaml",
		"layout: quote",
		"```",
		"",
		"> Make the easy things easy, and the hard things possible.",
		"",
		"<!-- A closing pull-quote — layout: quote centers and enlarges it. -->",
	}
	return strings.Join(lines, "\n") + "\n"
}

// realLaneReadme is the generated README pointing at the serve command.
func realLaneReadme(name string) string {
	return fmt.Sprintf("Real-lane gosx-slides deck.\n\nRun it:\n\n    slides serve %s\n\nThen open the printed URL. Edit deck.md or Counter.gsx and use `slides serve --watch %s` for hot reload.\n\nThis deck is self-contained: its go.mod requires m31labs.dev/gosx, so it serves\nfrom any directory. The first `slides serve` fetches gosx and builds the GOOS=js\nruntime.wasm into build/ (cached, gitignored), which can take a few minutes.\n", name, name)
}

// fallbackGoSXVersion pins the gosx version the scaffold requires when the running
// binary carries no usable build info (e.g. `go run`/dev builds, where the dep
// version reads as "(devel)" or is absent). It tracks the gosx version this module
// is built against (see go.mod). gosxScaffoldVersion prefers the real build info.
const fallbackGoSXVersion = "v0.24.3"

// gosxScaffoldVersion returns the gosx module version to pin in a scaffolded
// deck's go.mod. It reads the RUNNING binary's build info and uses the version of
// the m31labs.dev/gosx dependency, so the scaffold requires the SAME gosx the
// `slides` binary was built against. It falls back to fallbackGoSXVersion when
// build info is unavailable or reports a non-release pseudo/"(devel)" version
// (which would not be a valid `require` target).
func gosxScaffoldVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallbackGoSXVersion
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != gosxModuleImportPath {
			continue
		}
		// A real dependency version starts with "v" (e.g. v0.24.3). "(devel)" and
		// the empty string are not valid require targets, so use the fallback.
		if strings.HasPrefix(dep.Version, "v") {
			return dep.Version
		}
		break
	}
	return fallbackGoSXVersion
}

// realLaneGoMod is the generated go.mod for a deck at deckPath. The module path is
// the deck dir's sanitized base name; the gosx require is pinned to
// gosxScaffoldVersion().
func realLaneGoMod(deckPath string) string {
	return fmt.Sprintf("module %s\n\ngo 1.26\n\nrequire %s %s\n",
		moduleNameFromDeck(deckPath), gosxModuleImportPath, gosxScaffoldVersion())
}

// moduleNameFromDeck derives a valid Go module path from a deck directory path. It
// uses the path's BASE name (so `slides init /tmp/foo` yields module `foo`, not
// `/tmp/foo`) and sanitizes it to characters safe in a module path, lowercasing
// and collapsing illegal runs to single hyphens. An empty or fully-stripped name
// falls back to "deck" so the generated go.mod is always valid.
func moduleNameFromDeck(deckPath string) string {
	base := filepath.Base(filepath.Clean(deckPath))
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(base) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || r == '.':
			// Collapse runs of separators (and any illegal rune below) to a single
			// hyphen so we never emit a leading/trailing/doubled separator.
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "deck"
	}
	return name
}

// realLaneGitignore keeps the staged runtime build (the ~30MB GOOS=js wasm and
// island JSON under build/) and compiled test binaries out of version control.
const realLaneGitignore = "build/\n*.test\n"

// realLaneCounter is the working counter island, copied verbatim from
// examples/showcase/Counter.gsx. It seeds its state from props.Initial, so the
// deck's <Counter Initial={3}/> starts at 3, and it is the hot-swap demo island.
const realLaneCounter = `package main

// Counter is a live GoSX island that SEEDS its state from a typed prop.
//
// <Counter Initial={3}/> in deck.md starts the count at 3 — the same gosx
// compiler that type-checks this embed evaluates the prop, so a wrong-typed
// Initial (e.g. Initial={"x"}) is a compile error, not a silent runtime bug.
//
// It is also the hot-swap demo: run ` + "`slides serve --watch`" + ` in this directory,
// bump the count, then edit below and save — the running island swaps in place
// (the count you clicked up stays put) without a page reload.
//
//gosx:island
func Counter(props any) Node {
	count := signal.New(props.Initial)
	increment := func() { count.Set(count.Get() + 1) }
	decrement := func() { count.Set(count.Get() - 1) }
	return <div class="counter">
		<button class="counter-btn" onClick={decrement}>-</button>
		<span class="counter-label">count is {count.Get()}</span>
		<button class="counter-btn" onClick={increment}>+</button>
	</div>
}
`

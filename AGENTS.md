# gosx-slides — agent reference

`gosx-slides` turns a directory of Markdown + GoSX components into a live, compiled
presentation. This file is the complete capability reference: every command, the
authoring model, themes, islands, the dev loop, and the non-obvious gotchas. Read
it and you can author and run a deck without reading source.

Module `m31labs.dev/gosx-slides`, package `slides`. The CLI is `cmd/slides`.
Build it once:

```bash
go build -o /tmp/slides ./cmd/slides
```

(Editor "not in go.mod" diagnostics for `.gsx` files are expected — the gosx
compiler reads them, not `go build`.)

---

## Two lanes

gosx-slides has two independent rendering lanes. Pick by what you need.

| | Real lane (flagship) | Fallback lane |
|---|---|---|
| Command | `slides serve` | `slides dev` / `slides present` |
| Deck shape | a **directory** with `deck.md` + `<Name>.gsx` files | a `deck.md` **file** (or directory with `slides/`) |
| `<Component/>` | compiled to **bytecode**, hydrated as a live GoSX island | a fixed built-in component registry, rendered to static HTML |
| `{expr}` | **evaluated server-side** by the GoSX compiler | not evaluated |
| Dependencies | local `m31labs.dev/gosx` + `m31labs.dev/mdpp`; stages a ~30 MB `runtime.wasm` | none (pure Go, zero toolchain) |
| Use it for | the real product: custom interactive islands, real expressions | a quick HTML presenter, static export (`build`/`export`), speaker tooling (`rehearse`, presenter view) |

The two lanes **share no rendering code**. They also parse decks differently (see
the [gotcha about cross-lane tooling](#gotchas--non-obvious)). When this doc says
"the deck", it means the real-lane deck unless it says otherwise.

---

## 30-second quickstart (real lane)

```bash
go build -o /tmp/slides ./cmd/slides
/tmp/slides serve examples/showcase --port 8080
# open http://127.0.0.1:8080
```

`examples/showcase` is a complete, copyable deck: themed title slide, server-evaluated
`{expr}`, and a live `<Counter/>` island. First run stages the client WASM runtime
into `examples/showcase/build/` (cached; gitignored).

For the hot-swap dev loop:

```bash
/tmp/slides serve examples/showcase --watch
# edit examples/showcase/Counter.gsx and save → the island hot-swaps in place,
# its state preserved, no page reload. Edit deck.md → full reload.
```

---

## CLI reference

Every command below exists in `cmd/slides/main.go`. Flags accept `--name value`,
`--name=value`, or `-name value`. A `[deck.md]` argument defaults to `deck.md` in
the current directory; a `[deck-dir]` argument defaults to `.`.

### Real lane

| Command | Purpose |
|---|---|
| `serve [deck-dir] [--port 8080] [--rebuild] [--watch]` | Serve a deck whose `<Component/>` are live hydrated islands and whose `{expr}` is evaluated by the GoSX compiler. |

- `--port N` — listen port (default `8080`); binds **`127.0.0.1`** only (localhost).
- `--watch` — turn `serve` into the **hot-swap dev loop**: a `.gsx` edit hot-swaps
  the live island in place (state preserved, no reload); a `deck.md` edit triggers
  a full reload with the new content.
- `--rebuild` — force a fresh `GOOS=js` `runtime.wasm` build. The wasm is
  existence-cached (the build is slow), so this is the only way a change to the
  gosx runtime is picked up. No effect on deck content — use it after upgrading
  gosx, not after editing your deck.

```bash
/tmp/slides serve examples/theme-neon --port 9000
/tmp/slides serve examples/showcase --watch
/tmp/slides serve examples/showcase --rebuild   # after a gosx upgrade
```

### Fallback lane — serving

| Command | Purpose |
|---|---|
| `dev [deck.md] [--port 8080]` | HTML presenter with keyboard nav + file-change reload. **Not** the real-lane hot-swap loop — for that use `serve --watch`. |
| `present [deck.md] [--port 8080]` | Audience + presenter + phone-remote views over SSE, with locked-follow, a timer, and a rehearsal recorder. |

### Fallback lane — authoring & analysis

| Command | Purpose | Example |
|---|---|---|
| `new <name> [--theme m31] [--template default\|gotreesitter]` | Scaffold a fallback `deck.md` (writes `<name>/deck.md` + `<name>/public/`). | `slides new my-talk --template gotreesitter` |
| `check [deck.md]` | Parse and report title, slide / click / notes counts, layout mix. | `slides check examples/showcase` |
| `fmt [deck.md]` | Normalize deck source spacing in place (then re-parses to confirm it still parses). | `slides fmt deck.md` |
| `inspect [deck.md] [--json]` | Layout mix, component counts, word count, estimated runtime, warnings. | `slides inspect deck.md --json` |
| `validate [deck.md] [--strict] [--profile standard\|conference\|demo\|lecture]` | Turn profile authoring rules into checks. `--strict` exits non-zero on failure (CI gate). | `slides validate deck.md --strict --profile conference` |
| `rehearse [deck.md]` | Print a speaker run sheet: per-slide timings, notes, component cues. | `slides rehearse deck.md` |
| `split <deck.md> --out <deck-dir>` | Break one `deck.md` into a `slides/*.md` directory. | `slides split talk.md --out talk/` |
| `merge <deck-dir> --out <deck.md>` | Flatten a deck directory back into one file. | `slides merge talk/ --out talk.md` |
| `components [--json]` | List the fallback built-in component registry, grouped by pack. | `slides components` |
| `themes [--json]` | List the **real-lane** themes selectable via headmatter `theme:`. | `slides themes` |
| `doctor [deck.md] [--json]` | Deck health + local export prerequisites (e.g. a Chrome binary for PDF/PNG). Exits non-zero on failures. | `slides doctor deck.md` |

### Fallback lane — export

| Command | Purpose | Example |
|---|---|---|
| `build [deck.md] [--out dist]` | Write a deterministic static SPA (alias for `export --format spa`). | `slides build deck.md --out dist` |
| `export [deck.md] --format spa\|single\|pdf\|png [--out dist]` | `spa` = static SPA (also writes `deck.json`, `notes.html`, `handout.html`); `single` = one portable `deck.html`; `pdf`/`png` = needs a local Chrome/Chromium. | `slides export deck.md --format single` |

### Misc

| Command | Purpose |
|---|---|
| `version` | Print the version. |
| `help`, `-h`, `--help` | Print usage. |

---

## Authoring a real-lane deck

A real-lane deck is a **directory** containing:

- `deck.md` — the deck content (the only required file).
- `<Name>.gsx` — one file per island component referenced in `deck.md`. A
  `<Counter/>` tag resolves to `Counter.gsx` in the same directory.

```text
my-deck/
  deck.md         # headmatter + slides
  Counter.gsx     # defines the Counter island
  build/          # auto-staged client runtime + island JSON (gitignored)
```

### Headmatter (deck-level)

The deck's leading `---` block is YAML headmatter. Every key is bound for
expressions as `{deck.<key>}`.

```md
---
title: My Talk
theme: neon
---
```

- `theme:` selects the visual theme (see [Themes](#themes)). Unknown/absent →
  the default theme (`aurora`).
- `title:` sets the document `<title>` (also available as `{deck.title}`). If
  absent, the title falls back to the deck's first heading, then the directory name.
- Any other key is just bound as `{deck.<key>}`.

### Slides

Slides are separated by a `---` **surrounded by blank lines** (a CommonMark
thematic break):

```md
# Slide one

content

---

# Slide two

more content
```

> ⚠️ A `---` directly between two text lines is **not** a slide break — CommonMark
> reads it as a setext heading (it turns the line above into an `<h2>`). Always
> leave a blank line above and below a slide separator.
>
> ```md
> line A
> ---          ← WRONG: makes "line A" an <h2>, does NOT split
> line B
> ```

### Per-slide frontmatter

> ⚠️ **Non-obvious:** per-slide frontmatter is a leading ` ```yaml ``` ` **fenced
> code block** at the top of the slide — **not** a `---` block. (A `---` block is
> only deck-level headmatter; mid-deck a `---` is a slide separator.)

````md
---

```yaml
layout: center
```

# A centered slide
````

The fence must be the slide's first content node, and its language must be `yaml`
or `yml`. Every key is bound as `{slide.<key>}`. `layout:` additionally drives the
slide's CSS layout class (see [Layouts](#layouts)).

`{slide.index}` is always available — the slide's **0-based** position (slide one
is `0`).

### Inline expressions `{expr}`

`{expr}` is evaluated **server-side** by the GoSX compiler and rendered into the
static HTML. What you can write:

| Form | Example | Renders |
|---|---|---|
| Pure arithmetic / string ops | `{2 + 3}`, `{6 * 7}`, `{"a" + "b"}` | `5`, `42`, `ab` |
| Deck headmatter | `{deck.title}` | the `title:` value |
| Slide frontmatter | `{slide.layout}`, `{slide.index}` | this slide's values |
| Bound `strings.*` functions | `{strings.ToUpper("hi")}` | `HI` |

The bound expression namespace is intentionally small (defined in
`exprFuncs()`, `render_program.go`). Only these are callable:

```
strings.ToUpper   strings.ToLower   strings.TrimSpace
strings.Title     strings.Repeat    strings.Join
```

An **unknown identifier renders empty** (fail-soft) — never an error. So a typo
like `{deck.titel}` produces nothing rather than breaking the deck.

### Components / islands

A `<Name .../>` tag resolves to a sibling `Name.gsx`. That file defines a GoSX
island:

```go
// Counter.gsx
package main

//gosx:island
func Counter(props any) Node {
	count := signal.New(props.Initial)
	inc := func() { count.Set(count.Get() + 1) }
	dec := func() { count.Set(count.Get() - 1) }
	return <div class="counter">
		<button class="counter-btn" onClick={dec}>-</button>
		<span class="counter-label">count is {count.Get()}</span>
		<button class="counter-btn" onClick={inc}>+</button>
	</div>
}
```

gosx-slides compiles it to island bytecode once, serves the program JSON at
`/gosx/islands/Counter.json`, and the staged client runtime hydrates it in the
browser. The count is real reactive state.

A component tag works both **on its own line** and **inline in prose**:

```md
The counter below is live:

<Counter Initial={3}/>

Or inline right here: <Counter Initial={3}/> — same thing.
```

> ⚠️ **Props bind by EXACT attribute name.** `<Counter Initial={3}/>` matches
> `props.Initial`. Lowercase `<Counter initial={3}/>` does **not** bind to
> `props.Initial` — that field falls through to its zero value. (Verified: a deck
> with `<Counter Initial={7}/>` renders `count is 7`; `<Counter initial={99}/>`
> renders `count is 0`.) Match the attribute name to the Go field name exactly,
> casing included.

Prop value forms the tag parser recognizes: `Initial={3}` (int), `delta={-2}`
(negative int), `live={true}` (bool), `live` (bare = `true`), `label="hi"` and
`title={"Q3"}` (string). Anything else is carried through as a raw string.

A component whose `.gsx` is **missing or fails to compile** does not break the
deck — it renders as an inert `data-gosx-unresolved` placeholder, and the rest of
the deck serves normally.

### A complete minimal deck

```text
my-deck/
  deck.md
  Counter.gsx
```

`deck.md`:

````md
---
title: My Deck
theme: aurora
---

```yaml
layout: title
```

# My Deck

Two plus three is {2 + 3}. Title: {deck.title}.

---

# A live island

<Counter Initial={5}/>

This is slide {slide.index}.
````

`Counter.gsx`: the island above. Then `slides serve my-deck`.

### Example decks

All under `examples/`:

| Deck | Demonstrates |
|---|---|
| `examples/showcase` | The full real lane: themed title slide, `{deck.title}`, `{strings.ToUpper}`, `{2+3}`, `{slide.index}`, and a `<Counter Initial={3}/>` island. Best starting point. |
| `examples/real-deck` | The minimum: no headmatter, one slide, a propless `<Counter/>`. |
| `examples/theme-neon`, `examples/theme-paper`, `examples/theme-swiss` | The same deck shape under the `neon`, `paper`, and `swiss` themes. |

(`examples/kitchen-sink`, `examples/gotreesitter-*` are **fallback-lane** decks for
the HTML presenter, not the real lane.)

---

## Themes

Themes are real-lane only, selected via deck headmatter `theme: <name>`. List them:

```bash
/tmp/slides themes
```

| Theme | Style |
|---|---|
| `aurora` | Dark elegance — near-black canvas, warm amber accent. **Default.** |
| `paper` | Editorial — warm ivory canvas, terracotta accent, serif headlines. |
| `neon` | Electric — deep indigo canvas, cyan + lime accents, uppercase display. |
| `swiss` | Swiss precision — white, black ink + one red accent, tight grid. |

An unknown or absent `theme:` resolves to `aurora`. The theme CSS is scoped under
`main.deck[data-theme="<name>"]` and injected into the document head.

**Adding a theme:** add one entry to `themeRegistry` in `themes.go` — a
`name -> CSS` pair, with the CSS scoped under `main.deck[data-theme="<your-name>"]`
(copy an existing theme's shape; CSS lives in `themes_css.go`). The name is then
returned by `Themes()`, accepted in headmatter, and listed by `slides themes`. No
other file needs changing.

> Note: `m31`, `noir`, `blueprint`, `ember` are **fallback-lane** themes (used by
> `slides new` and the HTML presenter). They are not real-lane themes — don't put
> them in a `serve` deck's headmatter.

## Layouts

A slide's `layout:` frontmatter (the ` ```yaml ``` ` fence) becomes a
`layout-<name>` class on its `<section>`, which every theme styles.

| Layout | Use |
|---|---|
| `default` | Standard slide (the fallback when `layout:` is absent or unknown). |
| `center` | Vertically/horizontally centered content. |
| `title` | Title-slide treatment (oversized display). |

---

## Navigation

The real lane shows one slide at a time with a self-contained controller (`nav.go`).

| Key | Action |
|---|---|
| `→` or `Space` | Next slide |
| `←` | Previous slide |
| `f` / `F` | Toggle fullscreen |

- **Deep-linking:** the URL hash is **1-based** — `#1` is the first slide, `#3` the
  third. It loads to that slide and stays in sync as you navigate
  (`history.replaceState`, so it doesn't pollute history).
- Keys are ignored while typing in an `input`/`textarea`/`select` (so island form
  fields work).
- Hidden slides still hydrate their islands on load; navigating only toggles
  visibility, so island state persists across slide changes.

---

## Hot-swap dev loop

```bash
/tmp/slides serve <deck-dir> --watch
```

This fronts the in-process deck server with the gosx dev proxy and watches the
deck directory:

- **Edit a component `.gsx`** → gosx recompiles just that island and ships the
  fresh bytecode over SSE; the running island **hot-swaps in place**. State is
  preserved and the page does **not** reload — bump a counter, edit its label, and
  the count stays put.
- **Edit `deck.md`** → a full reload with the new content (the deck server
  re-parses and re-compiles per request in dev mode, so a mid-edit bad parse
  falls back to the last good deck rather than 500-ing).

`Counter.gsx` in the examples documents concrete edits to try (text swap, handler
swap, attribute swap).

---

## Gotchas / non-obvious

- ⚠️ **Props bind by exact attribute name.** `Initial={3}` → `props.Initial`;
  lowercase `initial={3}` does not. Match the Go field name's casing.
- ⚠️ **Per-slide frontmatter is a ` ```yaml ``` ` fence, not a `---` block.** A
  `---` block is deck-level headmatter only; mid-deck a `---` separates slides.
- ⚠️ **Slide separators need blank lines.** A bare `---` between two text lines is
  a setext heading, not a slide break. Surround it with blank lines.
- ⚠️ **`{{include "..."}}` is fallback-lane only.** The real lane (`serve`) does
  **not** process include directives — it parses `deck.md` through mdpp. Includes
  work only in `dev`/`present`/`export`/etc.
- ⚠️ **`check` / `inspect` / `validate` / `doctor` use the FALLBACK parser**, even
  when pointed at a real-lane deck. So a real-lane deck reports oddly under them:
  `check examples/showcase` shows `layout default: 4` (it can't see the `yaml`-fence
  layouts) and `inspect` warns `unknown built-in theme aurora` (it knows only
  fallback themes). This is expected — these tools target fallback decks. To
  validate a real-lane deck, just `serve` it.
- ⚠️ **First-run wasm build is slow.** `serve` stages a ~30 MB `GOOS=js`
  `runtime.wasm` into `<deck>/build/` on first run. It's existence-cached, so
  subsequent runs are instant; a gosx upgrade is picked up only with `--rebuild`
  (or by deleting `build/`).
- ⚠️ **`serve` binds `127.0.0.1` only.** It is not reachable from other machines;
  use SSH port-forwarding to view remotely.
- `build/` is gitignored (`build/` and `examples/*/build/`). Don't commit it.
- `{slide.index}` is **0-based**, but the URL hash (`#N`) is **1-based**.
- An unknown `{identifier}` renders empty (fail-soft); an unresolvable
  `<Component/>` renders an inert placeholder. Neither breaks the deck.

---

## Architecture (for extending it)

The real-lane render flow:

1. **`bridge.go`** — `LoadIslandDeck(dir)` reads `deck.md`, parses it with mdpp,
   splits it into slides (`mdpp.SplitSlides`), and collects each slide's
   `<Component/>` references.
2. **`slidegen.go`** — lowers the whole deck to a single generated GoSX source:
   the referenced island definitions (read from each `<Name>.gsx`, merged) plus
   one `func Slide_N() Node { … }` per slide. Prose is emitted as Go
   string-literal *expressions* so it can never corrupt the generated program;
   `{expr}` is emitted verbatim so the compiler evaluates it.
3. **`render_program.go`** — `gosx.Compile` the generated source **once**, then
   render each slide via `route.RenderProgramComponent` with a per-slide
   expression env (`deck`, `slide`, and the `strings.*` funcs). This is what makes
   `{expr}` real.
4. **`serve.go`** — builds a gosx `server.App`: mounts each island program at
   `/gosx/islands/<Name>.json`, renders the deck as the page body (islands
   register on the App's page runtime so it hydrates), and (for `serve`) stages
   the client runtime via `StageRuntimeAssets`.
5. **`dev.go`** — the `--watch` loop: runs the deck server on an internal port
   behind the gosx dev proxy, which handles `.gsx` hot-swap; a Markdown watcher
   bridges `deck.md` edits to a full reload.

`render_island.go` is the **fallback render lane / compile-failure safety net**:
if the generated deck source fails to compile, slides render through a hand-built
mdpp→`gosx.Node` lowering instead (prose + islands still render; `{expr}` degrades
to raw text) so a transient bad deck never blanks the page.

Dependencies: local `m31labs.dev/gosx` and `m31labs.dev/mdpp` via `replace`
directives in `go.mod` (both point at sibling checkouts, `../gosx` and `../mdpp`).

The fallback lane (`deck.go`, `parse.go`, `render.go`, `server.go`, `export.go`,
`style.go`, …) is entirely separate and shares no code with the real lane.

---

## See also

- `README.md` — orientation and the two-lane overview.
- `examples/showcase` — the canonical real-lane deck.

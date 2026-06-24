# gosx-slides — agent reference

`gosx-slides` turns a directory of Markdown + GoSX components into a live,
compiled presentation. This file is the complete capability reference: every
command, the authoring model, themes, islands, the dev loop, and the
non-obvious gotchas. Read it and you can author and run a deck without reading
source.

Module `m31labs.dev/gosx-slides`, package `slides`. The CLI is `cmd/slides`.
Build it once:

```bash
go build -o /tmp/slides ./cmd/slides
```

(Editor "not in go.mod" diagnostics for `.gsx` files are expected — the gosx
compiler reads them, not `go build`.)

---

## One lane

There is one rendering lane. A deck is a **directory** containing:

- `deck.md` — headmatter + slides (the only required file).
- `<Name>.gsx` — one file per island component referenced in `deck.md`.
- `public/` — optional static assets served at `/public/`.
- `build/` — auto-staged client runtime + island JSON (gitignored).
- `go.mod` — optional but present in scaffolded decks; makes them portable
  (serve from any directory, not just inside the gosx-slides repo).

`deck.md` is parsed by mdpp. Each slide is lowered to GoSX source and compiled
to island bytecode via `gosx.Compile`. The deck is served by a gosx
`server.App` and hydrated in the browser. Inline `{expr}` is evaluated
server-side; `<Component/>` tags hydrate as live islands.

---

## 30-second quickstart

```bash
go build -o /tmp/slides ./cmd/slides
/tmp/slides serve examples/showcase --port 8080
# open http://127.0.0.1:8080
```

`examples/showcase` is a complete, copyable deck: themed title slide,
server-evaluated `{expr}`, and a live `<Counter/>` island. First run stages the
client WASM runtime into `examples/showcase/build/` (cached; gitignored).

For the hot-swap dev loop:

```bash
/tmp/slides serve examples/showcase --watch
# edit examples/showcase/Counter.gsx and save → the island hot-swaps in place,
# its state preserved, no page reload. Edit deck.md → full reload.
```

---

## CLI reference

Every command below exists in `cmd/slides/main.go`. Flags accept `--name
value`, `--name=value`, or `-name value`. All commands that read a deck accept
a `[deck-dir]` argument that defaults to `.`; passing a path to `deck.md`
directly also works (the parent directory is used).

| Command | Purpose |
|---|---|
| `init <name> [--theme aurora\|paper\|neon\|swiss]` | Scaffold a **portable** deck you can `serve` immediately: writes `<name>/{deck.md,Counter.gsx,go.mod,.gitignore,README}`. The generated `go.mod` pins the gosx version the running `slides` binary was built against, so the deck serves from any directory. |
| `serve [deck-dir] [--port 8080] [--rebuild] [--watch]` | Serve a deck with live hydrated islands and server-evaluated `{expr}`. |
| `build [deck-dir] [--out dist]` | Write a static SPA (alias for `export --format spa`): `index.html` + `gosx/` assets; islands stay live. |
| `export [deck-dir] --format spa\|single [--out dist]` | `spa` = hostable folder (islands hydrate); `single` = one self-contained snapshot HTML (theme + nav work, islands are static — the ~30 MB wasm cannot live in one file). |
| `check [deck-dir]` | Title, slide/click/notes counts, layout mix. |
| `inspect [deck-dir] [--json]` | Full authoring analysis: word count, estimated runtime, component usage, warnings. |
| `validate [deck-dir] [--strict] [--profile standard\|conference\|demo\|lecture]` | Authoring-rule checks by profile. `--strict` exits non-zero on failure (CI gate). |
| `rehearse [deck-dir]` | Print a speaker run sheet: per-slide timings and notes. |
| `components [deck-dir] [--json]` | The deck's own `.gsx` islands and their compile status. |
| `doctor [deck-dir] [--json]` | Deck health + `serve` prerequisites. Exits non-zero on failures. |
| `themes [--json]` | List the themes selectable via headmatter `theme:`. |
| `version` | Print the version. |
| `help`, `-h`, `--help` | Print usage. |

### `serve` flags

- `--port N` — listen port (default `8080`); binds **`127.0.0.1`** only.
- `--watch` — turn `serve` into the **hot-swap dev loop**: a `.gsx` edit
  hot-swaps the live island in place (state preserved, no reload); a `deck.md`
  edit triggers a full reload with the new content.
- `--rebuild` — force a fresh `GOOS=js` `runtime.wasm` build. The wasm is
  existence-cached (the build is slow); use this after upgrading gosx.

```bash
/tmp/slides serve examples/theme-neon --port 9000
/tmp/slides serve examples/showcase --watch
/tmp/slides serve examples/showcase --rebuild   # after a gosx upgrade
```

---

## Authoring a deck

### Directory shape

```text
my-deck/
  deck.md         # headmatter + slides (required)
  Counter.gsx     # defines the <Counter/> island
  public/         # optional: static assets → /public/
  build/          # auto-staged; gitignored
  go.mod          # scaffolded decks include this for portability
```

### Headmatter (deck-level)

The deck's leading `---` block is YAML headmatter. Every key is available as
`{deck.<key>}`.

```md
---
title: My Talk
theme: neon
---
```

- `theme:` selects the visual theme (see [Themes](#themes)). Unknown or absent
  → `aurora` (the default).
- `title:` sets the document `<title>` (also available as `{deck.title}`). If
  absent, the title falls back to the deck's first heading, then the directory
  name.
- Any other key is bound as `{deck.<key>}`.

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

> A `---` directly between two text lines is **not** a slide break — CommonMark
> reads it as a setext heading (it turns the line above into an `<h2>`). Always
> leave a blank line above and below a slide separator.
>
> ```md
> line A
> ---          <- WRONG: makes "line A" an <h2>, does NOT split
> line B
> ```

**Absorbed separator gotcha:** a slide that ends with several blocks (heading +
prose + code + prose) can cause mdpp to absorb the trailing `---` as text
rather than a split. End such a slide with a trailing block — an HTML comment
or speaker note — to force the split. gosx-slides warns when separators are
absorbed (`warnAbsorbedSeparators` in `bridge.go`).

### Per-slide frontmatter

> **Non-obvious:** per-slide frontmatter is a leading ` ```yaml ``` ` **fenced
> code block** at the top of the slide — **not** a `---` block. (A `---` block
> is deck-level headmatter only; mid-deck a `---` is a slide separator.)

````md
---

```yaml
layout: center
```

# A centered slide
````

The fence must be the slide's first content node and its language must be
`yaml` or `yml`. Every key is bound as `{slide.<key>}`. `layout:` additionally
drives the slide's CSS layout class.

`{slide.index}` is always available — the slide's **0-based** position (slide
one is `0`).

### Inline expressions `{expr}`

`{expr}` is evaluated **server-side** by the GoSX compiler and rendered into
the static HTML. What you can write:

| Form | Example | Renders |
|---|---|---|
| Pure arithmetic / string ops | `{2 + 3}`, `{6 * 7}`, `{"a" + "b"}` | `5`, `42`, `ab` |
| Deck headmatter | `{deck.title}` | the `title:` value |
| Slide frontmatter | `{slide.layout}`, `{slide.index}` | this slide's values |
| Bound `strings.*` functions | `{strings.ToUpper("hi")}` | `HI` |

The bound expression namespace is intentionally small (defined in
`exprFuncs()`, `render_program.go`). Only these `strings` functions are
callable:

```
strings.ToUpper   strings.ToLower   strings.TrimSpace
strings.Title     strings.Repeat    strings.Join
```

An **unknown identifier renders empty** (fail-soft) — never an error. So a
typo like `{deck.titel}` produces nothing rather than breaking the deck.

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

> **Props bind by EXACT attribute name.** `<Counter Initial={3}/>` matches
> `props.Initial`. Lowercase `<Counter initial={3}/>` does **not** bind to
> `props.Initial` — that field falls through to its zero value. Match the
> attribute name to the Go field name exactly, casing included.

Prop value forms the tag parser recognizes: `Initial={3}` (int), `delta={-2}`
(negative int), `live={true}` (bool), `live` (bare = `true`), `label="hi"` and
`title={"Q3"}` (string). Anything else is carried through as a raw string.

A component whose `.gsx` is **missing or fails to compile** does not break the
deck — it renders as an inert `data-gosx-unresolved` placeholder, and the rest
of the deck serves normally.

### Speaker notes

Notes are surfaced in the presenter view. Two forms work:

- `<Notes>…</Notes>` — an explicit notes block anywhere in the slide.
- A trailing `<!-- … -->` HTML comment — if the comment is the last node on the
  slide, it is treated as speaker notes.

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
| `examples/showcase` | Themed title slide, `{deck.title}`, `{strings.ToUpper}`, `{2+3}`, `{slide.index}`, and a `<Counter Initial={3}/>` island. Best starting point. |
| `examples/real-deck` | The minimum: no headmatter, one slide, a propless `<Counter/>`. |
| `examples/theme-neon`, `examples/theme-paper`, `examples/theme-swiss` | The same deck shape under the `neon`, `paper`, and `swiss` themes. |
| `examples/gotreesitter` | Real-lane example deck for a conference talk. |

---

## Themes

Selected via deck headmatter `theme: <name>`. List them:

```bash
/tmp/slides themes
```

| Theme | Style |
|---|---|
| `aurora` | Dark elegance — near-black canvas, warm amber accent. **Default.** |
| `paper` | Editorial — warm ivory canvas, terracotta accent, serif headlines. |
| `neon` | Electric — deep indigo canvas, cyan + lime accents, uppercase display. |
| `swiss` | Swiss precision — white, black ink + one red accent, tight grid. |

An unknown or absent `theme:` resolves to `aurora`. The theme CSS is scoped
under `main.deck[data-theme="<name>"]` and injected into the document head.

**Adding a theme:** add one entry to `themeRegistry` in `themes.go` — a
`name -> CSS` pair, with the CSS scoped under
`main.deck[data-theme="<your-name>"]` (copy an existing theme's shape; CSS
lives in `themes_css.go`). The name is then returned by `Themes()`, accepted in
headmatter, and listed by `slides themes`. No other file needs changing.

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

The deck shows one slide at a time with a self-contained controller (`nav.go`).

| Key | Action |
|---|---|
| `→` or `Space` | Next slide |
| `←` | Previous slide |
| `f` / `F` | Toggle fullscreen |
| `p` | Open presenter view |

- **Deep-linking:** the URL hash is **1-based** — `#1` is the first slide, `#3`
  the third. It loads to that slide and stays in sync as you navigate
  (`history.replaceState`, so it doesn't pollute history).
- Keys are ignored while typing in an `input`/`textarea`/`select`.
- Hidden slides still hydrate their islands on load; navigating only toggles
  visibility, so island state persists across slide changes.

---

## Presenter view

The presenter is built into `serve` — no separate command needed.

- **Open it:** append `?present` to the URL, or press `p` from any slide.
- **What you see:** current + next slide, speaker notes, a timer.
- **Phone remote:** browse to `/remote` on the serving machine.
- **Audience sync:** other machines follow the presenter in lockstep over
  Server-Sent Events. The server mounts `/presenter/events` (SSE stream) and
  `/presenter/state` (POST to advance). Same-machine windows also sync via the
  browser's BroadcastChannel.

The presenter broker lives in `present_broker.go`; the presenter page HTML in
`presenter.go`.

---

## Hot-swap dev loop

```bash
/tmp/slides serve <deck-dir> --watch
```

This fronts the in-process deck server with the gosx dev proxy and watches the
deck directory:

- **Edit a component `.gsx`** → gosx recompiles just that island and ships the
  fresh bytecode over SSE; the running island **hot-swaps in place**. State is
  preserved and the page does **not** reload — bump a counter, edit its label,
  and the count stays put.
- **Edit `deck.md`** → a full reload with the new content (the deck server
  re-parses and re-compiles per request in dev mode, so a mid-edit bad parse
  falls back to the last good deck rather than 500-ing).

`Counter.gsx` in the examples documents concrete edits to try (text swap,
handler swap, attribute swap).

---

## Gotchas / non-obvious

- **Props bind by exact attribute name.** `Initial={3}` → `props.Initial`;
  lowercase `initial={3}` does not. Match the Go field name's casing.
- **Per-slide frontmatter is a ` ```yaml ``` ` fence, not a `---` block.** A
  `---` block is deck-level headmatter only; mid-deck a `---` separates slides.
- **Slide separators need blank lines.** A bare `---` between two text lines is
  a setext heading, not a slide break. Surround it with blank lines.
- **Absorbed separators.** A slide with many trailing blocks (heading + prose +
  code + prose) can cause the following `---` to be absorbed as text. End such a
  slide with an HTML comment or `<Notes>` to force the split. gosx-slides warns
  you when this happens.
- **First-run wasm build is slow.** `serve` stages a ~30 MB `GOOS=js`
  `runtime.wasm` into `<deck>/build/` on first run. Subsequent runs are instant
  (existence-cached); a gosx upgrade is picked up only with `--rebuild` (or by
  deleting `build/`).
- **A `serve`-able deck must live in (or contain) a Go module that requires
  `m31labs.dev/gosx`.** `slides init` generates that `go.mod`
  (`module <name>`, `require m31labs.dev/gosx <version>`), so scaffolded decks
  are self-contained and serve from any directory. The in-repo `examples/*`
  decks resolve up to the gosx-slides `go.mod` instead. A hand-written deck
  (e.g. `examples/real-deck`) with no `go.mod` of its own only serves from
  inside the repo. The first `serve` auto-populates `go.sum`
  (`GOFLAGS=-mod=mod`).
- **`serve` binds `127.0.0.1` only.** It is not reachable from other machines;
  use SSH port-forwarding to view remotely.
- `build/` is gitignored. Don't commit it.
- `{slide.index}` is **0-based**, but the URL hash (`#N`) is **1-based**.
- An unknown `{identifier}` renders empty (fail-soft); an unresolvable
  `<Component/>` renders an inert placeholder. Neither breaks the deck.

---

## Architecture (for extending it)

The single render flow:

1. **`bridge.go`** — `LoadIslandDeck(dir)` reads `deck.md`, parses it with
   mdpp, splits it into slides (`mdpp.SplitSlides`), collects each slide's
   `<Component/>` references, and warns when separators are absorbed
   (`warnAbsorbedSeparators`).
2. **`slidegen.go`** — lowers the whole deck to a single generated GoSX source:
   the referenced island definitions (read from each `<Name>.gsx`, merged) plus
   one `func Slide_N() Node { … }` per slide. Prose is emitted as Go
   string-literal expressions so it can never corrupt the generated program;
   `{expr}` is emitted verbatim so the compiler evaluates it.
3. **`render_program.go`** — `gosx.Compile` the generated source **once**, then
   render each slide via `route.RenderProgramComponent` with a per-slide
   expression env (`deck`, `slide`, and the `strings.*` funcs). This is what
   makes `{expr}` real.
4. **`serve.go`** — builds a gosx `server.App`: mounts each island program at
   `/gosx/islands/<Name>.json`, mounts the presenter endpoints
   (`/presenter/events` SSE + `/presenter/state`), renders the deck as the page
   body, and stages the client runtime via `StageRuntimeAssets`.
5. **`dev.go`** — the `--watch` loop: runs the deck server on an internal port
   behind the gosx dev proxy, which handles `.gsx` hot-swap; a Markdown watcher
   bridges `deck.md` edits to a full reload.

**`render_island.go`** is the compile-failure safety net: if the generated deck
source fails to compile, slides render through a hand-built mdpp→`gosx.Node`
lowering instead (prose and islands still render; `{expr}` degrades to raw
text) so a transient bad deck never blanks the page.

**`export_island.go`** renders the deck through the same `server.App` as
`serve` — in-process via an `httptest` recorder — then writes the static bundle.

**`analysis_island.go`** implements `check` / `inspect` / `validate` /
`rehearse` / `doctor` / `components` against the `IslandDeck` (the mdpp-parsed
deck) so these tools report correct themes, layouts, and components.

**`present_broker.go`** is the in-process SSE broker that fans the presenter's
position out to all connected audience screens.

Dependencies: `m31labs.dev/gosx` and `m31labs.dev/mdpp` as **public releases**
in `go.mod` (no `replace`; the module builds standalone). `slides init` pins
the same gosx version it reads from the running binary's build info into a
scaffolded deck's `go.mod`.

---

## See also

- `README.md` — orientation.
- `examples/showcase` — the canonical starting deck.

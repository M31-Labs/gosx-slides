# gosx-slides

`gosx-slides` turns a directory of Markdown + GoSX components into a live,
compiled presentation. Your `<Component/>` tags are real, hydrated GoSX islands;
your `{expr}` is evaluated by the GoSX compiler — no JavaScript toolchain.

For the complete capability reference (every command and flag, the authoring
model, themes, islands, gotchas), see **[AGENTS.md](AGENTS.md)**.

## Quickstart

```bash
go build -o slides ./cmd/slides
./slides serve examples/showcase --port 8080
# open http://127.0.0.1:8080
```

`examples/showcase` is a complete deck: a themed title slide, server-evaluated
`{expr}`, and a live `<Counter/>` island. First run stages the client WASM
runtime into `examples/showcase/build/` (cached; gitignored).

Hot-swap dev loop — edit a component and watch it swap in place, state preserved:

```bash
./slides serve examples/showcase --watch
# edit examples/showcase/Counter.gsx → the island hot-swaps, no reload
# edit examples/showcase/deck.md     → full reload with new content
```

## A deck

A deck is a **directory** with `deck.md` plus one `<Name>.gsx` per island:

```text
my-deck/
  deck.md       # headmatter + slides
  Counter.gsx   # defines the <Counter/> island
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
````

What you get:

- **Live islands.** `<Counter Initial={5}/>` resolves to `Counter.gsx`
  (a `//gosx:island` component), compiles to bytecode, and hydrates in the
  browser. Props bind by **exact attribute name** (`Initial={5}` → `props.Initial`).
- **Real expressions.** `{2 + 3}`, `{strings.ToUpper("hi")}`, `{deck.title}`,
  `{slide.index}` are evaluated server-side. Unknown identifiers render empty.
- **Themes & layouts.** `theme:` in headmatter picks a theme; `layout:` in a
  slide's ` ```yaml ``` ` fence picks a layout. Run `./slides themes` for the
  four themes (`aurora`, `paper`, `neon`, `swiss`). Layouts: `default`, `center`,
  `title`, `quote`, `section`, `two-cols`, `full`.
- **Images & tables.** `![alt](src)` renders (height-capped to 58 vh; local
  assets in `public/`). GFM pipe tables render with a themed header row.
- **Per-slide overrides.** `background:` and `accent:` in a slide's
  ` ```yaml ``` ` fence set an inline background and `--accent` token override
  for that one slide.
- **Navigation.** `→` / `Space` next, `←` prev, `f` fullscreen, `o` overview
  grid, `p` presenter view; `#N` deep-links to slide N.
- **Audience chrome.** A themed progress bar and a slide counter (`3 / 11`)
  appear on every deck. Overflowing slides are auto-scaled to fit the viewport
  instead of clipping.
- **Presenter view.** Built into `serve`: open with `?present` in the URL or
  the `p` key — shows current + next slide, speaker notes (with basic markdown
  rendered), timer. Phone remote at `/remote`. Audience screens follow over SSE.
- **Code blocks.** Stepped highlights (` ```go {1-2|4-6} ``` `), a hover
  "copy" button, and optional line numbers (`line-numbers: true` in headmatter).
- **Transitions.** `transition: fade` (default) or `transition: none`; all
  motion respects `prefers-reduced-motion`.
- **Hot-swap dev loop** via `--watch`. Build errors surface as an in-page
  dismissible banner (dev only).

A few things bite if you don't know them: props bind by exact name, per-slide
frontmatter is a ` ```yaml ``` ` fence (not a `---` block), slide separators
need blank lines around them, and a slide with many trailing blocks can absorb
its separator. All of these are spelled out in
[AGENTS.md](AGENTS.md#gotchas--non-obvious).

### Example decks

| Deck | Demonstrates |
|---|---|
| `examples/showcase` | Full feature set — best starting point. |
| `examples/real-deck` | The minimum: one slide, a propless `<Counter/>`. |
| `examples/theme-{neon,paper,swiss}` | The same deck under each theme. |
| `examples/gotreesitter` | Real-lane example deck for a conference talk. |

## CLI

```text
slides init <name> [--theme aurora|paper|neon|swiss]          scaffold a portable deck (deck.md + Counter.gsx + go.mod) you can serve from anywhere
slides serve [deck-dir] [--port 8080] [--rebuild] [--watch]   serve live islands + evaluated {expr}; --watch = hot-swap loop;
                                                              presenter at ?present or 'p', phone remote at /remote, audience follows over SSE
slides build [deck-dir] [--out dist]                          static SPA: index.html + gosx/ assets; islands stay live
slides export [deck-dir] --format spa|single [--out dist]     spa = hostable folder; single = one self-contained snapshot html
slides check [deck-dir]                                       title / slide / click / notes / layout counts
slides inspect [deck-dir] [--json]                            full authoring analysis (words, estimate, components, warnings)
slides validate [deck-dir] [--strict] [--profile standard|conference|demo|lecture]
slides rehearse [deck-dir]                                    speaker run sheet with per-slide notes
slides components [deck-dir] [--json]                         the deck's own .gsx islands + compile status
slides doctor [deck-dir] [--json]                             deck health + serve prerequisites
slides themes [--json]                                        themes selectable via deck headmatter "theme: <name>"
slides version
```

See **[AGENTS.md](AGENTS.md)** for the full reference, including flags, the
authoring model, and the architecture.

## Architecture (brief)

`bridge.go LoadIslandDeck` reads `deck.md` through mdpp and splits it into
slides. `slidegen.go` lowers the whole deck to a single generated GoSX source —
the merged island definitions plus one `func Slide_N()` per slide. `render_program.go`
compiles it once with `gosx.Compile` and renders each slide via
`route.RenderProgramComponent` (which is what makes `{expr}` real). `serve.go`
builds the gosx `server.App`, mounts each island program at
`/gosx/islands/<Name>.json`, mounts the presenter SSE endpoints, and stages the
client runtime. `--watch` fronts it with the gosx dev proxy for hot-swap.
`render_island.go` is the compile-failure safety net (fail-soft fallback so a
bad deck never blanks the page).

Depends on `m31labs.dev/gosx` and `m31labs.dev/mdpp` as public releases (no
`replace`; builds standalone). `slides init` scaffolds self-contained decks with
their own `go.mod` that serve from any directory.

Full details in [AGENTS.md](AGENTS.md#architecture-for-extending-it).

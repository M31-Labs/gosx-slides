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

## The real lane (`slides serve`)

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
  slide's ` ```yaml ``` ` fence picks a layout. Run `./slides themes` for the four
  themes (`aurora`, `paper`, `neon`, `swiss`).
- **Navigation.** `→` / `Space` next, `←` prev, `f` fullscreen; `#N` deep-links to
  slide N.
- **Hot-swap dev loop** via `--watch`.

A few things bite if you don't know them — props bind by exact name, per-slide
frontmatter is a ` ```yaml ``` ` fence (not a `---` block), slide separators need
blank lines around them. All of these are spelled out in
[AGENTS.md](AGENTS.md#gotchas--non-obvious).

### Example decks

| Deck | Demonstrates |
|---|---|
| `examples/showcase` | The full real lane — best starting point. |
| `examples/real-deck` | The minimum: one slide, a propless `<Counter/>`. |
| `examples/theme-{neon,paper,swiss}` | The same deck under each theme. |

## The HTML presenter (fallback lane)

The secondary lane is a zero-dependency Markdown → HTML presenter. It does **not**
compile islands or evaluate `{expr}`; it renders a fixed registry of built-in
components to static HTML, and adds speaker tooling and static export. Use it for
a quick presenter, exports, or speaker prep — not for custom interactive islands.

```bash
./slides new my-talk
./slides dev my-talk/deck.md --port 8080
```

Fallback-lane decks are a `deck.md` file (or a directory with a `slides/` folder),
and use the `m31` / `noir` / `blueprint` / `ember` themes (distinct from the
real-lane themes above).

What this lane provides:

- `slides new` — scaffold a deck (`--template default|gotreesitter`).
- `slides dev` — presenter with keyboard nav and file-change reload.
- `slides present` — audience + presenter + phone-remote views over SSE, with
  locked-follow, a timer, presenter checkpoints, and a rehearsal recorder.
- `slides check` / `inspect` / `validate` / `rehearse` / `doctor` — deck analysis,
  CI-friendly validation profiles, speaker run sheets, health checks.
- `slides fmt` — normalize deck source spacing.
- `slides split` / `merge` — convert between one file and a `slides/*.md` directory.
- `slides components` — list the built-in component registry.
- `slides build` / `export` — static SPA, single-file `deck.html`, or PDF/PNG (via
  a local Chrome). SPA export also writes `deck.json`, `notes.html`, `handout.html`.

It supports deck/per-slide frontmatter, notes (`<Notes>…</Notes>` or HTML
comments), built-in layouts, step reveals, stepped code ranges, relative
`{{include "path.md"}}` fragments (this lane only), citations, and offline
navigation.

> Note: `slides check` / `inspect` / `validate` / `doctor` parse with the
> fallback parser even when pointed at a real-lane deck, so a real-lane deck
> reports oddly under them (see [AGENTS.md](AGENTS.md#gotchas--non-obvious)). To
> check a real-lane deck, just `serve` it.

## CLI

```text
slides init <name> [--theme aurora|paper|neon|swiss]          real lane: scaffold a portable deck (deck.md + Counter.gsx + go.mod) you can serve from anywhere
slides serve [deck-dir] [--port 8080] [--rebuild] [--watch]   real lane: live islands + evaluated {expr}; --watch = hot-swap loop
slides new <name> [--theme m31] [--template default|gotreesitter]   fallback HTML-presenter deck
slides check [deck.md]
slides fmt [deck.md]
slides inspect [deck.md] [--json]
slides validate [deck.md] [--strict] [--profile standard|conference|demo|lecture]
slides rehearse [deck.md]
slides split <deck.md> --out <deck-dir>
slides merge <deck-dir> --out <deck.md>
slides components [--json]
slides themes [--json]                                        real-lane themes (headmatter "theme: <name>")
slides doctor [deck.md] [--json]
slides dev [deck.md] [--port 8080]                            fallback HTML presenter
slides present [deck.md] [--port 8080]
slides build [deck.md] [--out dist]
slides export [deck.md] --format spa|single|pdf|png [--out dist]
slides version
```

See **[AGENTS.md](AGENTS.md)** for the full reference, including flags, the
authoring model, and the architecture.

## Architecture (brief)

The real lane lowers a deck to one generated GoSX source — the merged island
definitions plus one `func Slide_N()` per slide — compiles it once with
`gosx.Compile`, and renders each slide via `route.RenderProgramComponent` (which
is what makes `{expr}` real). `serve` stages the client runtime and serves each
island program at `/gosx/islands/<Name>.json`; `--watch` fronts it with the gosx
dev proxy for hot-swap. It depends on `m31labs.dev/gosx` and `m31labs.dev/mdpp` as
public releases (no `replace`; builds standalone), and `slides init` scaffolds
self-contained decks (with their own `go.mod` requiring gosx) that serve from any
directory. The fallback lane is entirely separate and shares no rendering code.
Full details in [AGENTS.md](AGENTS.md#architecture-for-extending-it).

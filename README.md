# gosx-slides

`gosx-slides` is a Go-native presentation runtime shaped by the
`hypha://m31labs/gosx-slides` v0.1 roadmap. This first implementation provides
the authoring and runtime surface locally while the upstream GoSX and mdpp seams
land separately.

## What works now

- `slides new` scaffolds a deck.
- `slides check` parses a deck and reports slide, click, layout, and notes data.
- `slides fmt` normalizes deck source spacing.
- `slides inspect` reports layout mix, components, warnings, and estimated run
  time.
- `slides validate --profile conference|demo|lecture --strict` turns
  profile-specific authoring rules into CI-friendly checks.
- `slides rehearse` prints a speaker run sheet with timings, notes, and
  component cues.
- `slides components` lists the built-in component registry and packs.
- `slides doctor` checks deck health and local export prerequisites.
- `slides dev` serves the deck with keyboard navigation and file-change reloads.
- `slides present` serves audience, presenter, and phone remote views with
  locked-follow state, timer controls, slide navigation, presenter checkpoints,
  and a client-side rehearsal recorder over server-sent events.
- `slides export --format spa` writes a deterministic static SPA.
- `slides export --format single` writes one portable `deck.html`.
- `slides export --format pdf|png` uses a local Chrome/Chromium binary when
  available.
- SPA export also writes `deck.json`, `notes.html`, and `handout.html`.

The renderer supports deck and per-slide frontmatter, notes, built-in layouts,
step reveals, stepped code ranges, static diagram placeholders, Scene3D and
Canvas demo surfaces, relative `{{include "path.md"}}` fragments, citations,
query demos, parser/runtime evidence components, and offline navigation.

## Quick start

```bash
go run ./cmd/slides new my-talk
go run ./cmd/slides dev my-talk/deck.md --port 8080
```

Then open `http://127.0.0.1:8080`.

## CLI

```text
slides new <name> [--theme m31]
slides check [deck.md]
slides fmt [deck.md]
slides inspect [deck.md] [--json]
slides validate [deck.md] [--strict] [--profile standard|conference|demo|lecture]
slides rehearse [deck.md]
slides split <deck.md> --out <deck-dir>
slides merge <deck-dir> --out <deck.md>
slides components [--json]
slides doctor [deck.md] [--json]
slides dev [deck.md] [--port 8080]
slides present [deck.md] [--port 8080]
slides build [deck.md] [--out dist]
slides export [deck.md] --format spa|single|pdf|png [--out dist]
```

## Authoring

Decks are Markdown++-shaped markdown files:

```md
---
title: Demo
theme: m31
---

# Demo

<Step n={1}>First reveal.</Step>

---
layout: two-cols
clicks: 2
---

Left column

::right::

Right column
```

Decks can also be directories:

```text
my-talk/
  deck.md              # deck-wide frontmatter
  slides/
    01-open.md         # slide frontmatter + content
    02-demo.md         # may contain one slide or `---` splits
  public/
    architecture.png
```

Every command accepts either `deck.md` or a deck directory. Use
`slides split talk.md --out talk/` to break one file into `slides/*.md`, and
`slides merge talk/ --out talk.md` to flatten it again.

Deck files can include reusable fragments:

```md
{{include "partials/architecture.md"}}
```

Includes are resolved relative to the file that contains the directive and are
tracked in `deck.json` as source files.

Built-in themes are `m31`, `noir`, `blueprint`, and `ember`.
Use `slides new my-talk --template gotreesitter --theme noir` for a technical
parser/runtime talk starter.

Code stepping uses fence metadata:

````md
```go {1-2|4|all}
func main() {
    println("hello")
}
```
````

Speaker notes can be written as `<Notes>...</Notes>` or HTML comments.

Built-in presentation components:

```md
<Metrics>
<Metric label="Runtime" value="Go" delta="no JS toolchain"/>
</Metrics>

<Callout tone="gold">A typed presentation surface.</Callout>

<Poll question="Best feature?" options="Presenter sync|Static export|Live embeds"/>

<Timeline items="Author|Render|Present|Export"/>

<Agenda/>

<Pipeline steps="Grammar blob: tables|Parser: LR plus GLR|Tree: normalized"/>

<ParseTree root="source_file" tree="package_clause>package,identifier;function_declaration>func,identifier,block"/>

<Benchmark title="Fork reduction" values="Before 5.95x|After 4.41x|C baseline 1.00x"/>

<Citation href="hypha://m31labs/gotreesitter/object/concept.glr-fork-reduction"/>

<QueryDemo lang="go" captures="@fn main">
```go
func main() {}
```
```query
(function_declaration name: (identifier) @fn)
```
</QueryDemo>

<ProfileBuckets buckets="scan 20|reduce 42|materialize 18"/>

<ParityMatrix rows="Go pass|JavaScript watch|Python pass"/>

<CorpusRun rows="Go stdlib 1.02x pass|JavaScript corpus 4.41x pass"/>

<GrammarBlob steps="parser.c|tables|blob|registry"/>

<Checkpoint id="demo-query" label="Live query demo"/>

<Takeaway>Fork count is the first-order GLR performance lever.</Takeaway>
```

`{$step}` renders as a live click-step value inside the active slide.

Run `slides components` for the full registry. Current packs are `core`,
`presenter`, `evidence`, `parser`, `product`, `architecture`, and `showcase`.

## Deep additions

- Presenter rehearsal recording in `/presenter`, downloadable as
  `rehearsal.json`.
- Source-backed citation labels for `hypha://...` references plus citation
  metadata in `deck.json`.
- Query, profiling, parity, corpus, and grammar artifact components for
  technical talks.
- Validation profiles for conference talks, demos, and lectures.
- Component packs exposed through `slides components`.
- Presenter checkpoints for demo jumps while audience mode remains locked.

## Upstream seams

The Hyphae roadmap still has upstream work that belongs in `m31labs.dev/gosx`
and `m31labs.dev/mdpp`: VM bytecode hot-swap, mdpp slide/component/expression
AST nodes, and true AST-to-GoSX island lowering. This repo currently implements
compatible authoring and runtime behavior with explicit adapters so those pieces
can replace the local markdown lane without changing the CLI shape.

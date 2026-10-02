# gosx-slides

GoSX Slides v0.7 coordinates element entrances, split text, diagram SVG,
Scene3D actors and cameras, code morphs, and shader time from one seekable
playhead. Edit and save motion timings in the browser, navigate named story
cues, and capture repeatable poses for images, video, PDF, or PowerPoint.

Download a binary for Linux, macOS, or Windows from the
[latest release](https://github.com/M31-Labs/gosx-slides/releases/latest),
or build the tagged source with Go 1.26 or newer:

```sh
git clone --depth 1 --branch v0.7.0 https://github.com/M31-Labs/gosx-slides.git
cd gosx-slides
go install ./cmd/slides
```

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

## Motion, authoring, and exports

Try `slides serve examples/storytelling-lab --edit` (or `examples/authoring-lab`).
**M** opens the motion studio. Edit presets, duration, delay, easing and replay;
drag timeline bars and resize their right edges. Undo/Redo restores preview edits,
and **Save to deck.md** persists them through source validation, revision checks
and recoverable saves. Stale source produces a conflict while keeping your draft.
The shared transport samples DOM entrances, split text, diagram SVG, native
Scene3D keyframes, cameras and shader time from one playhead. Pause/play, seek,
reverse and replay reconstruct absolute poses; revisiting a timestamp gives the
same declarative state. Completed text entrances remain seekable. Pause is
preserved across click steps and replay, including graphics that mount later.
Native animation defaults to a 10-second segment; slide `motion-duration: 1200`
sets its duration in milliseconds (1–600000). Keyframes use `durationMs` (0–600000)
and `easing` (`linear`, `ease`, `ease-in`, `ease-out`, `ease-in-out`), with a
600 ms default; explicit zero settles immediately. Stateful water, particle and
event-driven glTF simulations need their own replay state.

Save uses parsed directive ranges: existing spacing, quoting, other attributes
and body content stay intact. A fake directive inside a code fence cannot be
edited as motion. Source editing requires LF line endings.

Try the [request, failure and recovery demo](examples/request-recovery/README.md)
for coordinated actor, route, label, camera and code changes. `:::code-morph {duration=1200 easing=ease-in-out}` sets code
transition timing; supported easing names match graphic keyframes. Each step
samples its preceding authored code block into its selected block, regardless
of navigation history. Backward navigation settles the destination immediately;
seek and reverse replay that same authored pair. Captured stills
settle the final pose; video samples explicit timestamps and waits for commands
and paint before capturing.

Click-step
navigation keeps the studio and undo history open; entering another slide resets
the transport and closes the studio.
Timing tracks show the actual start and end for each click step, including `after`
dependencies, group staggering and split-text staggering. Forward references
resolve on first entry. Missing cues, cross-step dependencies and cycles appear
as track warnings; cycles use local delay/stagger without chaining through the
loop. Replay preserves native word/character splitting, including entrances
assigned to an explicit click step. Cued/grouped containers preserve child markup.

**Persistent source editing:** `slides serve my-deck --edit`, then press **E**
or click **Edit**. Save validates the deck and its components, stages the new
source, and reloads the preview at the current anchor. Stale or concurrent edits
report a conflict; **Reload source** loads the latest file. Each save retains the
displaced source in `.slides-history-*/deck.md`, including writes through an
already-open file. Publishing refuses to overwrite a file recreated by another
editor. The source path is briefly absent between capture and publication;
filesystems without hard-link support reject saves before moving the source.
Recovery files remain until you remove them after reconciling your edits.
The editor accepts up to 1 MiB
and refuses symlinked source files. Browser editing is available in plain serve;
use `--watch` separately for filesystem hot-swap.
Authoring accepts only `localhost`, `127.0.0.1`, or `[::1]` authorities, with
the same browser origin and a save token; custom hostnames cannot read or save
source through the editor endpoint.

**Annotations:** press **D** for pen or **L** for laser, or click **Draw**.
The toolbar provides a color picker, undo, clear, and done. Ink is per slide and
remains while navigating in the current tab; a reload clears it. Annotations
stay local to the presenter tab and are excluded from capture exports. Pointer
updates are batched, with bounded stroke and point storage.

**PowerPoint:** `slides export my-deck --format pptx --steps --out deck.pptx`.
Chrome captures the actual rendered graphics into 16:9 image slides; `--steps`
emits every click state. Speaker notes are editable text. Slide content is a
captured image by default. Add `--editable` for native text and supported SVG shapes
over captured graphics; see the detailed export limits below.
The PPTX writer streams image parts and needs no JavaScript or office toolchain.

**Browser measurements:** `slides bench my-deck --runs 3` emits JSON with server
setup time, fresh-profile browser readiness, resource transfer bytes, JS heap,
DOM nodes, and hydrated island count. Chrome is required. Server setup includes
cached runtime staging; browser timing starts at navigation and includes runtime
activation plus active graphic readiness. Transfer includes the shared WASM;
measure the same deck and browser when comparing releases. The command also reports p95/max requestAnimationFrame intervals from a 36-frame
replay sample on the opening slide; these are machine-dependent intervals, not GPU
frame-rate guarantees. `--budget scripts/performance-budget.json` exits nonzero
when startup, transfer, heap, DOM or frame-interval ceilings are exceeded. The
existing browser CI job enforces those budgets without an extra workflow.

Give a slide an ID and ordered cues in its YAML fence:

````md
```yaml
id: pipeline
cues: overview, request, worker, done
```

:::motion {preset=slide-up cue=request duration=450}
## Request arrives
:::

:::motion {preset=fade cue=worker duration=300 group=work stagger=100}
The worker processes it.
:::

:::motion {cue=worker after=worker duration=200}
Context appears after the worker's entrance.
:::
````

`#pipeline/worker` opens that cue directly. IDs and cue names use ASCII letters,
digits, `_`, or `-`, start with a letter, and allow at most 64 characters. Cues
share the existing code, list, and Scene3D click budget. `step=N` chooses a numeric
step; `after=cue` sequences entrances within that step, and `group` plus `stagger`
offsets matching group members. Replay defaults to every slide visit; use
`replay=once` or `replay=step` to choose otherwise. Cued groups preserve child
markup; use separate grouped entrances for rich content rather than text splitting.

Match `data-morph-id="idea"` across slides to animate an element's position and
size without replacing live widgets. Set slide `morph-duration: 600` for its
arrival timing. Native Sirena SVG nodes carry these identities too. Shared morphs
skip island and graphics-engine roots; put the identity on a surrounding element
when appropriate. Wrap successive code fences in `:::code-morph` to advance
versions with arrow keys: unchanged lines move and added lines fade in.

**R** opens a readability report for the current viewport, highlighting small
text and low contrast on solid backgrounds. Gradient and translucent backgrounds
need visual inspection. `layout: split`, `:::cards`, and `:::card` provide simple
responsive layout recipes. The [authoring lab](examples/authoring-lab/README.md)
is a copyable starting point.

Islands hydrate on the active slide and warm the next slide during idle time.
Visited instances retain state. `SlidesRuntime.stats()` reports the inventory;
set deck headmatter `hydration: eager` to hydrate all islands at startup. Deferred
hydration reduces initial widget work; the shared GoSX runtime still loads.

```sh
slides export my-deck --format single --capture --out snapshot
slides export my-deck --format pdf --steps --out walkthrough.pdf
slides export my-deck --format frames --steps --out frames
slides export my-deck --format video --steps --seconds 2 --fps 15 --out deck.webm
```

Captured formats need Chrome/Chromium (`SLIDES_CHROME` can point to its executable).
`--steps` captures every click state, including the initial pose. PNG frames and
self-contained HTML/PDF snapshots use the rendered scene at 1280×720; snapshots
include slide text for accessibility, while diagrams are captured as pixels.
Video additionally needs `ffmpeg` with the VP9 encoder and exports silent WebM.
`--seconds` is per slide or selected step, bounded to 0.1–60; FPS is 1–60.
GPU clocks advance during video sampling. Exports allow at most 10,000 states
and 18,000 video frames. Fonts and image assets must be available during capture.

Browser CI covers navigation, presenter sync, authoring, mobile sizing, reduced
motion, graphics clocks, and captured exports. Its optional developer tools use
`npm ci`, `npx playwright install chromium`, and `node scripts/browser-ci.cjs ./slides`;
normal CLI use requires no Node installation. Tagged releases build six platform
binaries and publish SHA-256 checksums automatically.

Select a native diagram layout with a fence such as `sirena class` or
`sirena diagram=timeline`. Available layouts: architecture, sequence, radial,
state, class, er, swimlane, and timeline. Field/method rows and timeline metadata
use Sirena's normal declarations; see `examples/diagram-lab` for all five new
families alongside semantic Scene3D steps. Mermaid ingestion supports flowcharts.

## A deck

### Native shaders and Scene3D

GoSX v0.57.2's native graphics engine is available directly in Markdown:

```md
<Shader Src="shaders/ink.sel" Shape="torus" Label="Shader illustration" />
<Scene3D Src="scenes/network.json" Shader="shaders/ink.sel" Targets="processor" />
```

`Shader` accepts Selena `.sel` source, optional `Material` selection, a `Uniforms`
JSON file, and `Shape="plane|sphere|box|torus"`. `Scene3D` accepts a GoSX SceneIR
document or full Scene3D props, including models, labels, particles, lights,
animation, and post effects. Optional `Shader` and comma-separated `Targets`
apply a Selena material to selected objects. All sources are relative to the deck.
Shader compilation produces both GLSL and WGSL before serving the page.

Set headmatter `scene: shaders/aurora.sel` or `scene: scenes/network.json` for a
background. A slide's YAML fence can replace it or set `scene: false`. Each
distinct background mounts once per page; GoSX pauses hidden surfaces. Defaults
cap native graphics at 30 FPS, 1.5 device pixel ratio, and two million pixels,
with adaptive quality. Scene JSON can override these budgets.

Try `slides serve examples/shader-lab`. This native graphics deck needs no WASM
island runtime. SPA exports retain live graphics. Add `--capture` to single-file or PDF exports
to render actual shader and Scene3D pixels; ordinary snapshots retain fallback labels. `slides doctor` reports invalid sources
and shader compilation failures; deck analysis includes a graphics inventory.

Sirena's `render --scene3d --shader material.sel --steps steps.json` output can
be used directly as `<Scene3D Src="request.scene.json" />`. Try
`slides serve examples/sirena-scene`: arrow keys focus the API, then the worker,
then restore the whole diagram before advancing to the next slide. Backward
navigation and direct seeks apply absolute frames, and hidden surfaces pause.
The click budget appears in `check`, `inspect`, and the presenter run sheet.

Custom Scene3D props can include `slideSteps: {"version":1,"frames":[...]}`,
where each frame has a `label` and an array of native GoSX `commands`. Frame zero
is the initial state. Author complete poses for each touched object in every
frame so reverse navigation restores the expected state. The transport accepts
up to 128 frames and 4 MiB of commands; it is supported on inline surfaces.
SPA exports keep the timeline live. Captured exports render each selected pose;
ordinary snapshots show the fallback label.

Navigation also includes a toolbar, touch swipes, Home/End,
PageUp/PageDown, and **B** to blank the screen. Changing slides pauses outgoing
media and emits `slides:change` with zero-based `index` and `step` values.
The toolbar hides after 2.2 seconds of inactivity and reappears on pointer
movement, touch, or local navigation. Keyboard focus keeps it visible.

### Search and navigation

Press **O** or **/** to open the slide picker. Search title and body text, enter
a slide number, or combine words to narrow the results. Search ignores case and
accents; speaker notes, scripts, and style content are excluded. Slide text is
indexed when the picker first opens. Use arrows to choose a card, Enter or Space
to jump, and Esc to return to your current step. **?** opens the shortcuts.

Link directly to a click step with **`#3/2`** (slide 3, step 2). Plain **`#3`**
starts slide 3 at step zero. These anchors restore code highlights, list reveals,
and absolute Scene3D frames; `replay=step` motion responds to the step change.
For example, `[Focus the worker](#3/2)` links within your deck. Navigation updates
the URL, so copy the browser address to share your current position. Reload and
back/forward navigation restore it, with steps clamped to the slide's budget.
Timed entrance animations still start on arrival; a click-step anchor does not
seek to a timestamp inside an animation.

The picker uses readable text cards and hides the original slides. It preserves
live island state, pauses playing media, and lets hidden native graphics sleep
instead of drawing every slide at once. It also works in the presenter window
and in SPA and single-file exports. Printing includes all slides and fragments.
Code/reveal styles shrink from about 32 KB of generated rules to under 1 KB.
Code highlights and list reveals support the entire authored click budget;
hidden fragment links and controls are excluded from the tab order until shown.
Focused inputs and interactive controls keep their normal keyboard behavior.

Try `slides serve examples/navigation-lab` for search, a persistent live counter,
code steps, list reveals, and repeatable motion in one deck.

### Element motion and slide timing

Wrap Markdown in `:::motion` to use GoSX's managed DOM motion, including
headings, lists, code, and native graphics inside the animated region:

```md
:::motion {preset=slide-up duration=450 delay=80 easing=ease-out distance=24}
## Arrive with intent

Your content stays readable before the bootstrap loads.
:::
```

Presets are `fade`, `slide-up`, `slide-down`, `slide-left`, `slide-right`, and
`zoom-in`. Durations and delays are milliseconds. `trigger=view` is the slide
default: start when visible; `trigger=load` starts on page load. Motion replays
on each slide entry by default (`replay=slide`). Set `replay=once` for a single
entrance, or `replay=step` to repeat on each presentation step as well. Use
`split=word`, `split=char`, or `split=line` with `stagger=60` for text-only
entrances (the native splitter replaces the region's markup with text units). Motion
respects reduced-motion preferences by default. This uses GoSX's shared
bootstrap and needs no WASM island runtime; the native `<Motion>` builtin is
also available inside your `.gsx` components.

Set `transition: fade` or `none`, `transition-duration: 450`,
`transition-delay: 80`, and `transition-easing: ease-out` in deck headmatter.
A slide's YAML fence overrides any timing independently. Times accept numeric
milliseconds, `ms`, or `s`; easing accepts CSS keywords, `cubic-bezier(...)`,
or `steps(...)`. Slide fades preserve viewport fitting and respect reduced
motion. SPA exports retain element motion; snapshots retain the content.
Try `slides serve examples/motion-lab` for a motion-only deck with independent
slide timing and staggered text.

### Upgrade and performance inventory

The current dependency baseline is GoSX **v0.57.2**, mdpp **v0.5.0**,
gotreesitter **v0.55.1**, and Sirena **v0.7.0**. Existing Sirena/Mermaid diagrams,
live islands, code walkthroughs, notes, phone remote, and SPA/PDF exports remain
available. The native graphics components add the current GoSX scene engine
without a separate renderer or frontend build system.

Production servers now compile the deck once at startup. A 20-slide server
benchmark improved from **11.06 ms to 0.29–1.00 ms per request**, with allocated bytes
falling from **11.36 MB to 1.36 MB**. These are local synthetic measurements;
graphics performance depends on the device and authored scene.

Runtime caches track the resolved dependency graph and selected Go toolchain,
publish WASM builds atomically, and stage every current bootstrap feature chunk.
Static snapshots skip runtime builds, and exported decks disable live SSE sync.
Images use lazy loading and asynchronous decoding. The dev watcher includes
shader and scene JSON sources in nested directories.
Watch mode stages the cached WASM bridge so islands added during an editing
session hydrate without restarting. Native-only pages still load only bootstrap JS.

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
- **Raw HTML, sanitized.** `<div class="grid">`, `<br>`, styled spans, and
  per-slide `<style>` blocks pass straight through (scripts, handlers, and
  `javascript:` URLs never survive), so free-form composition needs no island.
- **Per-deck CSS.** A `deck.css` next to `deck.md` (or headmatter
  `css: a.css, b.css`) loads after the theme and wins the cascade — restyle a
  deck without forking a theme.
- **Per-slide overrides.** `background:`, `accent:`, `class:`, `transition:`,
  and `reveal:` in a slide's ` ```yaml ``` ` fence: inline background,
  `--accent` token, extra section classes, a one-slide enter transition, and
  step-through list reveals.
- **Layers.** Headmatter `header:` / `footer:` render on every slide;
  per-slide `footer: false` hides, any other value replaces.
- **Scene layers.** `scene:` mounts an island full-bleed BEHIND a slide's
  content — built-in decorative presets (`parse-forest`) or your own `.gsx`
  (a live illustration with real state). Hidden under reduced-motion.
- **Snippet imports.** A fence body of `<<< ./file.go 10-20` shows real
  source read at render time (sandboxed to the deck dir), composing with
  ` {1-3|7} ` click-step highlights.
- **PDF handouts.** `slides export --format pdf` prints one slide per page
  through a system Chrome/Chromium.
- **Navigation.** `→` / `Space` next, `←` prev, `f` fullscreen, `o` / `/` searchable overview, `?` shortcuts, `p` presenter view; `#N` deep-links to slide N.
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

### Favicon

Every deck gets a default favicon (two stacked slide cards in amber). Set
`favicon:` in `deck.md` headmatter to give a deck its own. The icon is inlined
into the page, so it also works offline and in exports. `slides doctor` and
`slides build` fail with a clear message on a bad value.

```yaml
favicon: brand/icon.svg                      # your own .svg, .png or .ico, relative to deck.md
favicon: "🎤"                                # one emoji, centred in an SVG
favicon: {text: "GT", color: "#10b981"}      # 1-2 letter monogram; color defaults to the theme accent
```

### Example decks

| Deck | Demonstrates |
|---|---|
| `examples/showcase` | Full feature set — best starting point. |
| `examples/motion-lab` | Native element motion, text stagger, and slide timings. |
| `examples/shader-lab` | Selena materials, native shapes, diagrams, and backgrounds. |
| `examples/sirena-scene` | Native Sirena diagram with forward/backward click frames. |
| `examples/real-deck` | The minimum: one slide, a propless `<Counter/>`. |
| `examples/theme-{neon,paper,swiss}` | The same deck under each theme. |
| `examples/gotreesitter` | Real-lane example deck for a conference talk. |

## CLI

```text
slides init <name> [--theme aurora|paper|neon|swiss]          scaffold a portable deck (deck.md + Counter.gsx + go.mod) you can serve from anywhere
slides serve [deck-dir] [--port 8080] [--rebuild] [--watch]   serve live islands + evaluated {expr}; --watch = hot-swap loop;
                                                              presenter at ?present or 'p', phone remote at /remote, audience follows over SSE
slides build [deck-dir] [--out dist]                          static SPA: index.html + gosx/ assets; islands stay live
slides export [deck-dir] --format spa|single|pdf [--out dist] spa = hostable folder; single = one snapshot html; pdf = one-slide-per-page handout
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

### Diagram stories and editable PowerPoint

Try `slides serve examples/storytelling-lab --edit` for animated chart values,
stable diagram actors, radar/Sankey layouts and native Scene3D scatter tours.
Sixteen Sirena families are available through `sirena diagram=…` fences.

````md
:::diagram-morph {diagram=bar duration=1000 easing=ease-in-out}
```sirena
service adoption { value: 18 }
```

```sirena
service adoption { value: 84 }
service breadth { value: 56 }
```
:::
````

Each fence is an absolute pose. Navigation steps and cue anchors choose poses;
matching `data-morph-id` actors and edges interpolate compatible SVG geometry.
Added actors fade in; incompatible geometry takes the destination shape. Backward
steps, seek and reverse are deterministic. Reduced motion selects the exact pose
without animation. Storyboards reserve graph slots, bar rows and chart domains
through Sirena's bounded 2–32-state API. Flat architecture/state/class/ER/mindmap,
bar, line/scatter and radar stories are supported; radar axis order must match.
Use `duration` (0–10000 ms) and `easing` on the container. The shared playhead
scrubs SVG, DOM, native declarative animation and shader time together. Stateful
simulations require their own replay state.

```sh
slides export examples/storytelling-lab --format pptx --editable --steps --out dist
```

`--editable` adds native PowerPoint text and ordinary SVG rectangles, ellipses,
polylines and supported paths over a captured background. Converted content is
hidden during background capture to avoid duplicated text. SVG labels become
editable Arial text; other text uses its declared font and installed-font fallback.
SVG fill/stroke opacity is preserved. Shaders/3D, rotated/clipped content,
translucent groups and unsupported SVG geometry retain
their captured pixels. Arrow-marked paths remain captured to preserve arrowheads.
The default PPTX export keeps whole-slide images. Speaker notes remain editable.

For a repeatable Sirena/Mermaid comparison, install the pinned development
dependencies with `npm ci`, install Sirena v0.6+, then run:

```sh
node scripts/compare-diagrams.cjs > comparison.json
```

Both engines receive the same five flowchart/sequence/state/class/mindmap sources.
Two warm-ups precede seven measured iterations. Sirena reports Go parse/layout/SVG
time and allocations; Mermaid 12 reports its browser render pipeline. Process and
library startup, network and paint are excluded. Output bytes and SVG DOM counts
are reported separately. Execution backends and fonts differ, so the report does
not imply overall presentation performance or feature parity. See the
[Mermaid API](https://mermaid.js.org/config/usage.html) for its render contract.

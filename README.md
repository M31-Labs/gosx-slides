# gosx-slides

GoSX Slides coordinates element entrances, split text, diagram SVG,
Scene3D actors and cameras, code morphs, and shader time from one seekable
playhead. Edit and save motion timings in the browser, navigate named story
cues, and capture repeatable poses for images, video, PDF, or PowerPoint.

Download a binary for Linux, macOS, or Windows from the
[latest release](https://github.com/M31-Labs/gosx-slides/releases/latest),
or build current main with Go 1.26 or newer:

```sh
git clone --depth 1 --branch main https://github.com/M31-Labs/gosx-slides.git
cd gosx-slides
go install ./cmd/slides
```

Release archives include a verified `runtime/` directory beside the executable.
Keep both together: ordinary `serve` and live SPA exports then require neither Go
nor a dependency download. `SLIDES_RUNTIME_DIR` selects an explicit bundle.
Every asset is checked against its manifest before staging, and the GoSX version
must match the CLI. Missing or damaged explicit bundles fail with diagnostics.
Source installations build and cache the runtime with Go; `--watch` and `--rebuild`
remain Go development workflows. Build a portable bundle from a matching source
installation with `slides runtime pack . --out runtime`.

`gosx-slides` turns a directory of Markdown + GoSX components into a live,
compiled presentation. Your `<Component/>` tags are real, hydrated GoSX islands;
your `{expr}` is evaluated by the GoSX compiler — no JavaScript toolchain.

For the complete capability reference (every command and flag, the authoring
model, themes, islands, gotchas), see **[AGENTS.md](AGENTS.md)**.

## Quickstart

With a release archive, start from a curated deck and open project editing:

```sh
slides templates
slides init my-talk --template technical-talk
slides serve my-talk --edit
```

The starters cover architecture reviews, technical talks and teaching. They
ship pinned local branding, diagrams, story cues or a repeatable simulation.
See [the starter catalog](examples/starters/README.md). From a source checkout:

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

### Project editing and local agent tools

Press **E** in `serve --edit` to select original Markdown fragments, GoSX,
Sirena, manifests, CSS or shaders. Each tab retains its draft and undo history;
diagnostics and outline links point back to original files. Saves validate the
project, check file/dependency revisions and retain the displaced source for
recovery. Conflicts keep your draft. Unchanged errors elsewhere can remain while
you repair files individually.

`slides mcp my-talk` exposes eight bounded stdio tools for project discovery,
read/diagnose/validated write, scoped rename, static story assertions, address
resolution and actual snapshot/handout export. Sources and speaker notes are
author material. The [VS Code companion](editors/vscode/README.md) uses the same
API through its project view; install the release's `.vsix` with **Extensions:
Install from VSIX**. See [project authoring](docs/project-authoring.md) for
configuration, limits and recovery. Marketplace publication is separate.

### Markdown presentation migration

```sh
slides migrate talk.md --from slidev --out imported-talk --json
slides migrate talk.md --from marp --out imported-talk --json
slides migrate talk.qmd --from quarto --out imported-talk --json
```

Migration uses parsed Markdown structure to preserve supported slides, notes,
IDs, reveal lists, code highlights and approved local images. It never executes
Vue components, Quarto cells, expressions or package hooks. Unsupported layout,
motion and runtime behavior produce ranged fidelity diagnostics. Review
`migration/report.json` against the unchanged private `migration/source.md`
before presenting. Destinations must be fresh; each result stays within the
editor's 1MiB deck limit. Original source and reports remain outside publication.

## Motion, authoring, and exports

**Offline equations:** inline `$...$` and block `$$...$$` equations render on
the server as KaTeX HTML plus accessible MathML, with embedded fonts. No Node
installation, math browser runtime, or CDN is required. Invalid or unsupported
formulas retain a visible source diagnostic. Try `examples/math-lab`; equations
survive reading, snapshots, PDF, and live exports.

**Reusable content:** include whole fragments with
`<!-- slides:include sections/intro.md -->`, or select a named section with
`<!-- slides:include sections/library.md#closing -->`. References resolve from
the included file, with source maps back to the original file. Local authoring
packs pin themes, layouts and GoSX components through `packs: labs@1.0.0`.
`slides pack install path/to/pack my-deck` vendors a validated pack;
`slides packs my-deck --json` lists enabled pins. See
[the composition example](examples/composition-pack/README.md) for the manifest,
section markers, asset rules and precedence.

**Reading and handouts:** press **V** or append `?read` for a scrollable,
responsive view with a table of contents and all reveal content visible.
Sirena diagrams offer a transcript of rendered labels without publishing hidden
diagram source. `slides export my-deck --format handout --out dist` writes
`handout.html`; add `--notes` to explicitly include speaker notes. Snapshot
exports embed published local images and CSS font assets. Set `offline-required: true`
in headmatter to suppress remote theme fonts; external author URLs remain external.
PDF printing waits for fonts and images through Chrome's DevTools protocol.

Try `slides serve examples/storytelling-lab --edit` (or `examples/authoring-lab`).
**M** opens a docked motion studio beside a fitted live slide preview (a bottom
sheet on mobile). Opening it preserves the current pose. Playback and seek stay
visible while fields scroll; **Scene** and **Elements** tabs keep controls focused.
**Escape** closes the studio. Reopening keeps your draft and selected actor or element.
Edit presets, duration, delay, easing and replay;
drag timeline bars and resize their right edges. Undo/Redo restores preview edits,
and **Save to deck.md** persists them through source validation, revision checks
and recoverable saves. Stale source produces a conflict while keeping your draft.
For authored Sirena scenes, the studio also edits camera X/Y/Z and field of view,
actor X/Y/Z, scale and color, and each cue's duration and easing. Scene Undo/Redo
restores previews; **Save scene cues** persists the declared Steps JSON with
revision checks and a retained previous file. Labels, routes and arrowheads are
regenerated with their actors. Blank actor fields restore the original layout.
**Reset actor** clears position, scale and color overrides for the current cue.
Ctrl/Cmd+Z and Shift+Ctrl/Cmd+Z undo/redo within the focused scene or element
controls; text inputs keep their native undo. Save is enabled for unsaved, valid
edits after the preview settles. A single scene needs no redundant scene picker.
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
or click **Edit**. The editor shows parser/story diagnostics, a slide outline,
symbols and reference locations. Clicking a finding selects its exact source
range; outline previews stay local to the author tab. Rename changes a selected
symbol's scope in the unsaved draft, with collision checks and Undo/Redo.
Save validates the deck and its components, stages the new
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
text (including SVG), low contrast on solid backgrounds, and hidden, clipped or
truncated Scene3D labels. **Scan scene cues** samples scene endpoints and midpoints
without navigating the slide or broadcasting cue changes, then restores the
playhead and playback state. Scans stop at 64 poses and report that limit.
Gradient and translucent backgrounds
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
`npm ci`, `npx playwright install chromium`, and `node scripts/browser-ci.cjs ./slides`.
The suite also needs Chrome/Chromium for capture and ffmpeg/ffprobe for narrated
video; use `SLIDES_CHROME`, `SLIDES_FFMPEG`, and `SLIDES_FFPROBE` to name binaries
outside PATH. Normal CLI use requires no Node installation. Tagged releases
build six platform binaries and publish SHA-256 checksums automatically.

Select a native diagram layout with a fence such as `sirena class` or
`sirena diagram=timeline`. Available layouts: architecture, sequence, radial,
state, class, er, swimlane, and timeline. Field/method rows and timeline metadata
use Sirena's normal declarations; see `examples/diagram-lab` for all five new
families alongside semantic Scene3D steps. Mermaid ingestion supports flowcharts.

## A deck

### Native shaders and Scene3D

For a ready-made background, use a bundled Selena preset:

```yaml
scene: shader:aurora
shader-ink: "#08121f"
shader-glow: "#51d6b0"
shader-speed: 0.18
shader-strength: 0.38
shader-scale: 1
```

Put these keys in deck headmatter for a default or a slide's leading YAML fence
for an override. The six presets are **aurora, silk, contours, waves, grid, and
spotlight**. No shader files or islands are needed. Colors use `#rrggbb`; speed
is 0–2, strength 0–1, and scale 0.5–4. Speed zero creates a still background;
reduced motion uses the normal deck background.

Run `slides serve examples/background-gallery --edit` and select **Backgrounds**
in the toolbar for a three-step wizard: choose, tune, apply. It previews the
actual native shader, applies to the current slide or deck default, and offers
copyable Markdown and a `.sel` download with your settings. Deck defaults retain
per-slide overrides. Saves update the original included Markdown when needed,
validate the project, reject stale revisions, and retain a recovery copy.
Existing source drafts must be saved or reloaded before applying a background;
the wizard retains them. Structured saves require LF line endings and block YAML
with single-line values for the shader keys.

`slides backgrounds [--json]` lists templates. To own the shader, create a
`shaders/` directory, run `slides backgrounds source aurora > shaders/aurora.sel`,
then set `scene: shaders/aurora.sel`. The exported file's parameter defaults
control custom-file backgrounds; `shader-*` keys control bundled presets.

GoSX v0.57.4's native graphics engine is available directly in Markdown:

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

For editable camera and actor cues, use the authored source directly:

```md
<Scene3D Src="request.sir" View="Request" Steps="steps.json"
         Shader="material.sel" Material="Pearl" Targets="api" />
```

This compiles through Sirena's native Go adapter without a separate CLI process.
`View` selects an explicit view; otherwise the first declared view is used, or
all elements when no view exists. Sources are self-contained (workspace imports
require a precompiled scene). Serve with `--edit` and press **M**. Existing SceneIR
JSON stays supported; cue editing requires the `.sir` plus `Steps` form. Source
and cue files each have a 1 MiB authoring limit and at most 128 cues.

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

The current dependency baseline is GoSX **v0.57.4**, mdpp **v0.5.0**,
gotreesitter **v0.55.1**, and Sirena **v0.7.0**. Existing Sirena/Mermaid diagrams,
live islands, code walkthroughs, notes, phone remote, and SPA/PDF exports remain
available. The native graphics components add the current GoSX scene engine
without a separate renderer or frontend build system.

The v0.7.1 patch restores custom shader shapes beside retained Scene3D meshes
on WebGPU, including the API box in `examples/request-recovery`. Browser
regression checks verify visible shader pixels on both WebGPU and WebGL.

The v0.7.2 patch keeps Scene3D canvases and projected labels aligned when a
slide scales to fit. Canvases stay inside their mounts through cue changes and
desktop resizing. The request-recovery example uses closer authored camera
views so actors and their labels are easier to read.

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
| `examples/background-gallery` | Six bundled Selena backgrounds and a wizard to tune and apply them. |
| `examples/sirena-scene` | Native Sirena diagram with forward/backward click frames. |
| `examples/real-deck` | The minimum: one slide, a propless `<Counter/>`. |
| `examples/theme-{neon,paper,swiss}` | The same deck under each theme. |
| `examples/gotreesitter` | Real-lane example deck for a conference talk. |

## CLI

```text
slides init <name> [--theme aurora|paper|neon|swiss]          scaffold a portable deck (deck.md + Counter.gsx + go.mod) you can serve from anywhere
slides serve [deck-dir] [--edit] [--port 8080] [--rebuild] [--watch]   serve live islands + evaluated {expr}; --edit = browser authoring; --watch = hot-swap loop;
                                                              presenter at ?present or 'p', phone remote at /remote, audience follows over SSE
slides build [deck-dir] [--out dist]                          static SPA: index.html + gosx/ assets; islands stay live
slides export [deck-dir] --format spa|single|handout|pdf|frames|video|pptx [--capture] [--editable] [--steps] [--notes] [--seconds 2] [--fps 15] [--out dist]
slides bench [deck-dir] [--runs 3] [--budget file.json]         browser readiness, transfer, heap, DOM and frame intervals
slides check [deck-dir]                                       title / slide / click / notes / layout counts
slides inspect [deck-dir] [--json]                            full authoring analysis (words, estimate, components, warnings)
slides validate [deck-dir] [--strict] [--profile standard|conference|demo|lecture]
slides rehearse [deck-dir]                                    speaker run sheet with per-slide notes
slides components [deck-dir] [--json]                         the deck's own .gsx islands + compile status
slides doctor [deck-dir] [--json]                             deck health + serve prerequisites
slides themes [--json]                                        themes selectable via deck headmatter "theme: <name>"
slides packs [deck-dir] [--json]                              enabled version-pinned local packs
slides pack install <source-dir> [deck-dir]                   vendor a validated local authoring pack
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
The default PPTX export keeps whole-slide images. `--notes --editable` also
includes editable speaker notes.

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

### Authenticated shared presentations

Local serving defaults to `127.0.0.1`. A public listener requires room tokens.
Keep tokens in files containing 32–4096 bytes, and use distinct audience and
editor tokens. For HTTPS, run:

```sh
slides serve my-deck --edit --host 0.0.0.0 --port 8443 \
  --editor-token-file /private/editor-token \
  --audience-token-file /private/audience-token \
  --tls-cert /private/certificate.pem --tls-key /private/key.pem
```

Visitors join at `/_slides/session`. Audience sessions receive the deck and
presenter updates; editor sessions can use presenter controls and enabled
authoring tools. Audience responses omit speaker notes and editor metadata.
Tokens grant shared roles rather than verified personal identities.

GoSX stores encrypted, signed, HttpOnly, SameSite=Strict cookies for eight hours.
Unsafe requests also require its session CSRF token, alongside the existing
revision and authoring-token checks for saves. Each login has an absolute
eight-hour expiry. **Leave presentation room** clears the cookie and revokes all
editor sockets for that login immediately; changing roles or rejoining also
revokes the previous login. Copied old cookies remain rejected after restart.
A random session key invalidates sessions on restart;
`--session-secret-file` preserves active logins when the secret and room tokens
stay the same. Rotating either invalidates previous logins. Private
`.slides-sessions.json` stores at most 256 active login grants in the deck folder;
keep it and `.slides-session-*.tmp` out of version control and published assets.
Session changes require writable private storage. If revocation cannot be
persisted, room access fails closed; repair storage and restart with a new
session secret before accepting logins.
Plain HTTP requires the explicit `--session-http` flag for trusted local use.
The watch proxy stays local and cannot be combined with sessions or TLS.

`--collab` enables an editor-only shared draft and review panel, and implies
`--edit`. It uses GoSX CRDT text merging for concurrent edits, chosen display
names for presence, and comments anchored to slide IDs, cues or source quotes.
Publishing uses the normal validated, revision-checked source save. Stale bases,
external file conflicts and failed writes keep the local draft for explicit
reconciliation. Use one collaboration server per deck; `.gsx` files remain local
authoring files. Private `.slides-team.json` stores draft history and comments;
keep it out of version control and published assets.

Private file permissions use POSIX mode 0600 where supported. Windows files
inherit the destination directory's access controls; place private decks and
token files in a directory restricted to the serving account.

### Local recording and narrated exports

The Record button saves a shared tab or screen with optional microphone audio
and a camera inset. Downloads contain the video, navigation timing JSON, and
WebVTT when explicit slide `caption:` metadata is present. Recording stays in
browser memory until downloaded, and requires a supported screen-capture browser
on HTTPS or localhost. Keep the recording tab active for canvas composition;
the browser share picker and device permission prompts remain native.

For a reproducible video with existing narration and authored captions:

```sh
slides export my-deck --format video --seconds 5 --fps 15 \
  --narration public/narration.wav --captions public/captions.vtt --out talk.webm
```

Audio and caption paths stay inside the deck. Short narration pads with silence;
longer narration or captions require a longer video. These inputs are explicit
author material. Speaker notes never become inferred captions or speech.
Encoding requires Chrome and ffmpeg; audio also requires ffprobe.

### Office content and export dimensions

`--editable` PPTX retains native text, supported SVG geometry, plain tables,
and supported Sirena bar/pie charts with embedded editable workbooks. Unsupported
effects and graphics retain captured pixels. Use `--aspect 4:3`, another bounded
ratio, or paired `--width 900 --height 600` for consistent capture, PDF and PPTX
dimensions. `--template prior.pptx` reuses its Office theme; it does not copy
masters, slide layouts or animations.

```sh
slides export examples/office-interop --format pptx --editable --aspect 4:3 --out talk.pptx
slides import prior.pptx --out imported-talk --json
```

Import migrates supported text, notes, raster images, native tables and cached
chart data into a fresh Markdown deck. It refuses an existing destination and
reports unsupported layout, media, motion and other fidelity losses. Original
note recovery files stay private, outside `public/`.

Static exports omit private speaker notes by default. `--notes` explicitly
publishes them in SPA, handout or editable PPTX output. A default SPA export
refuses an output folder with an existing `notes.html`; use a fresh folder to
avoid carrying a previously published private sidecar forward.

### Semantic stories and architecture tours

Headmatter `story: story.yaml` binds one versioned local manifest to stable slide
IDs and named cues. Each beat declares an absolute pose: Sirena actor focus and
reveal, a relationship trace, a native camera, code lines, DOM visibility, an
authored caption and expected visibility/labels. The same playhead drives these
effects; direct links, scrubbing and backward navigation restore the addressed
state. The compiler preserves source ranges and leaves author files unchanged.

```sh
slides serve examples/semantic-story
slides story inspect examples/semantic-story --json
slides story assert examples/semantic-story --browser --json
slides tour before.sir after.sir --out new-tour
```

The browser assertion gate requires Chrome and checks actual geometry and
ancestor visibility, plus repeatable final/midpoint states in both directions.
It supports up to 100 beats in a two-minute browser run; canonical manifests
support up to 1000 beats. Explicit beat captions also drive local recording and
video VTT, including deliberately blank cues; slide `caption:` is the fallback
for other steps. `ArchitectureTour` compares parsed Sirena actors, boundaries
and relationships using stable `sid` identity, producing a Markdown diagram
morph and its story manifest in a fresh runnable deck directory.

`slides tour history examples/architecture-history/history.yaml --out my-tour`
turns 2–32 authored Sirena revisions into one runnable tour. Stable revision IDs
keep your explanations attached to the same transition on regeneration. Pass
`--curation my-tour/curation.json` to preserve edited captions, including an
intentionally empty caption. Removed explanations remain in the private report;
a changed predecessor requires an explicit curation update.

Snapshots may instead declare `revision: HEAD~2` with a repository-relative
`.sir` path; pass `--repo /path/to/local/repo`. Git input resolves to immutable
commit hashes, ignores working changes and replacement objects, rejects symlink
tree entries, and disables textconv and all transport protocols. Missing objects
fail locally. `tour.json` records actual commit/path and content hashes; it and
`curation.json` use owner-only permissions and are excluded from publication.
Each generated Markdown/manifest stays within the editor's 1MiB source budget.
See [the revision tour example](examples/architecture-history/README.md).

See [the semantic story example](examples/semantic-story/README.md) for the
manifest and indexing contract, and [the multi-surface example](examples/multi-surface-story/README.md)
for coordinated diagrams, charts and native scenes. A slide supports up to eight
named `:::story-surface {name=topology}` regions; each contains exactly one
diagram, morph or Sirena scene. Beat `surfaces:` poses apply independently, and
expectations use `topology/api` to qualify actors. Single unwrapped surfaces
retain the original manifest contract. Story poses and an explicit scene Steps
file remain exclusive. Browser metadata excludes private source paths/ranges;
local inspection retains them.

### Audience variants

Deck headmatter can declare `audiences: [engineers, leaders]`. Per-slide YAML
`audiences:` selects one or more of those names; untagged slides are shared.
Names are case sensitive. Slides retain their stable IDs/cues and source
origins while numeric navigation and compiled story/simulation poses are
reindexed. Links into omitted slides fail selection with source diagnostics.

```sh
slides audiences examples/audience-variants
slides serve examples/audience-variants --audience leaders
slides export examples/audience-variants --audience engineers --format handout
slides export examples/audience-variants --audience leaders --format pptx --editable
```

Selection works across the export formats. Use a fresh SPA destination when
switching variants: builds refuse stale island programs or notes sidecars before
changing the prior page. Live variants block cached programs from omitted
slides. Audience selection controls published slide content; `public/` assets
remain a shared asset library and require their own content policy. It does not
grant authenticated room roles. Filtered editing, collaboration and watch mode
are rejected until their source-preview mapping is supported.

### Repeatable simulations

Headmatter `simulation: simulation.yaml` and `:::simulation demo` mount a
seeded fixed-step particle example built on GoSX `sim.Simulation`. The manifest
logs inputs, checkpoint spacing and authored branches, then maps stable
slide/cue addresses to exact ticks. Scrubbing and branch changes use bounded
precompiled state frames; local viewing, offline exports and captured video
share the same states, without a wall-clock runner or remote hub.

```sh
slides serve examples/simulation-lab
slides export examples/simulation-lab --format single --out simulation-snapshot
slides export examples/simulation-lab --format video --steps --seconds 2 --out simulation.webm
```

The Go API `NewSimulationReplay` adapts other deterministic GoSX models using
explicit seeds, copied input logs, checkpoint restore and immutable branch
prefixes. A factory must return fresh state. Bounds are 1–120 ticks/second,
1–3600 ticks, 512 logged inputs of at most 4 KiB, 64 KiB per state/checkpoint and
16 MiB of compiled frames. The authoring example supports up to 8 models,
3 branches and 32 particles; it is a small model vocabulary, not a general
physics language. See [the replay example](examples/simulation-lab/README.md).

Browser developer checks use Node 24, `npm ci` and Playwright. CI runs the Go
race detector and vet on Linux, Windows and macOS, the complete Chromium export
suite, and shared navigation, WASM, reading, offline math, sessions and
collaboration regressions in Firefox and WebKit.

## Web page slides

Use the built-in `<WebPage/>` component to visit an HTTPS page during a talk.
It needs no `.gsx` file. List the exact hosts permitted to frame live content:

```md
---
title: A web demo
theme: paper
web-allow: [example.com]
---

# Visit the page

<WebPage src="https://example.com/" Title="Example page"/>
```

```sh
slides web refresh my-deck
slides serve my-deck
slides export my-deck --format pdf --out talk.pdf
slides build my-deck --refresh-web --out site
```

`web refresh` records every referenced page using Chrome (`SLIDES_CHROME` selects
the executable). Build/export captures missing snapshots and reuses existing
ones. `--refresh-web` explicitly replaces the recorded captures. Serving reads
the assets without fetching snapshots; refresh before serving, then restart or
reload with `--watch`/`--edit` to use updated metadata. Commit the PNGs and
`public/webpages/manifest.json` with your deck. Repeated exports reuse identical
pixels, dates, response policies and filenames.

The sandbox starts empty. Add `Scripts={true}` only when the page needs scripts;
add `SameOrigin={true}` only when it also needs its own origin's storage or APIs.
`Popups={true}` permits sandboxed popups explicitly. Top navigation, downloads,
form submission and popup escape remain blocked. No camera, microphone,
fullscreen, autoplay or other device permission is granted. Frames use
`referrerpolicy="no-referrer"` and `loading="lazy"`. Raw HTML iframes, scripts
and forms still go through the sanitizer and remain blocked. Third-party
headers are never stripped, and pages are never proxied into the deck origin.

`web-allow` accepts YAML string lists, including block lists. Hosts match exactly:
`example.com` covers its default HTTPS port, while `example.com:8443` permits
that explicit port. It does not permit subdomains or wildcards. Non-HTTPS URLs,
credentials and unknown component props fail deck loading. The HTTP CSP and
matching HTML meta policy set `frame-src` to the sorted allowlist; no allowlist
means `frame-src 'none'`. The deck's own host is excluded to keep page scripts
out of the deck origin. Redirect destinations must also be allowlisted.

The active audience slide mounts at most one frame. Other pages, speaker
previews, overview and reading mode show snapshots. Leaving the slide or hiding
the tab removes the frame. Reload, fit/100% zoom and scroll lock controls work
in the audience and speaker views; speaker commands use presenter synchronization
across browsers and devices. Scroll lock blocks pointer and keyboard interaction with the
frame; cross-origin rules prevent controlling its internal scroll position.
Open in new tab uses a separate tab without an opener or referrer.

| Output | WebPage content |
|---|---|
| Live `serve` | Snapshot while loading; live frame for an allowed, frameable page |
| SPA `build` / `export` | Local snapshot asset, no live frame; other islands stay live |
| Single HTML / handout | Embedded PNG and URL/date caption |
| PDF | Snapshot pixels and caption, including `--capture` / `--steps` |
| PPTX / editable PPTX | Captured snapshot pixels; supported surrounding content can remain editable |
| Frames / video | Snapshot pixels in every captured slide state |

`offline-required: true` disables frames and sets `frame-src 'none'` even while
serving. Its exports require recorded snapshots unless you explicitly request
`--refresh-web`. Author-tool single/handout exports also require recorded assets
and never launch Chrome. A disallowed host still supports a snapshot; the
allowlist controls live framing, not explicitly requested capture.

Each capture records its URL, final URL, UTC date, viewport, SHA-256, allowlist,
`X-Frame-Options` and enforced CSP. Restrictive `frame-ancestors` policies and
X-Frame-Options denial select the snapshot conservatively because the deck has
no fixed deployment origin. Policies can change after capture; refresh before
a talk. The strict host policy also blocks deck-origin frame previews such as
the background editor's iframe wizard when a deck enables WebPage; author its
scene settings directly instead. Browsers do not reliably expose cross-origin framing failures through
iframe load/error events. Cookie consent, login, browser defenses and dynamic
content can affect pixels; capture uses a fresh profile without your login.

`Width={1280}` and `Height={720}` set the captured viewport and live fit size.
Bounds are 320–1920 by 240–1080 pixels. `Scroll={true}` captures up to three
viewports of the page instead of one; it does not promise a full-page archive.
There are at most 64 pages/hosts per deck, 256 manifest entries, 16 MiB per PNG
and 96 MiB of referenced snapshot assets. Failed refreshes retain the previous
manifest. Older unreferenced images remain available for manual cleanup.

See [the runnable web page example](examples/webpage/README.md). CI retains
desktop, mobile, speaker and framing-denied screenshots in its browser artifacts.

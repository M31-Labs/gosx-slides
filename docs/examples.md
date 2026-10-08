# Presentations you can run, read and adapt

The gallery contains complete exported PDFs, not placeholder download links.
All were captured at 1280 × 720 with the real CLI and Chrome. Each click state
gets a page, including the initial state. PDFs preserve the rendered appearance
of shaders and native scenes; use the live sources to experience animation.

## Effects cookbook

[![Custom ribbons behind the cookbook title](examples/effects-cookbook.png)](examples/effects-cookbook.pdf)

**[Read the PDF](examples/effects-cookbook.pdf)** ·
[Browse the source](../examples/effects-cookbook) ·
[Follow the shader and motion guide](shaders-and-motion.md)

Five slides, seven captured states: a custom Selena background, word entrances,
named cues, chained motion, a shader sphere and a warm preset variation.

```sh
slides serve examples/effects-cookbook --edit
slides export examples/effects-cookbook --format pdf --capture --steps --out cookbook.pdf
```

## Selena background gallery

[![The native Aurora preset](examples/background-gallery.png)](examples/background-gallery.pdf)

**[Read the PDF](examples/background-gallery.pdf)** ·
[Browse the source](../examples/background-gallery)

Six slides show aurora, silk, contours, waves, grid and spotlight. Open
**Backgrounds** in edit mode to tune a preset and apply it to your own deck.

```sh
slides serve examples/background-gallery --edit
slides export examples/background-gallery --format pdf --capture --out backgrounds.pdf
```

## A request worth following

[![The request path with API actor and accompanying code](examples/request-recovery.png)](examples/request-recovery.pdf)

**[Read the PDF](examples/request-recovery.pdf)** ·
[Browse the source](../examples/request-recovery)

Two slides, five captured states: follow a request through acceptance, failure
and recovery. Camera, actor geometry, routes, labels and code change together.
The API has its own Selena material. Open `#request/accepted` and press **M**
to seek or edit the authored scene.

```sh
slides serve examples/request-recovery --edit
slides export examples/request-recovery --format pdf --capture --steps --out request.pdf
```

## Find the right example

Run these paths from a source checkout, using `slides serve <path> --edit`:

| Technique | Example |
|---|---|
| A ready-to-adapt technical talk, architecture review or lesson | [Curated starters](../examples/starters/README.md) |
| Animated charts, diagram states and code morphing | [Storytelling lab](../examples/storytelling-lab) |
| Timeline editing, live islands and shared-element motion | [Authoring lab](../examples/authoring-lab) |
| Multiple independently directed diagram surfaces | [Multi-surface story](../examples/multi-surface-story) |
| Reusable slide fragments, themes and component packs | [Composition pack](../examples/composition-pack) |
| Audience-specific versions of one deck | [Audience variants](../examples/audience-variants) |
| Local math rendering | [Math lab](../examples/math-lab) |
| Repeatable interactive simulation | [Simulation lab](../examples/simulation-lab) |
| Tables, editable charts and PowerPoint interchange | [Office interop](../examples/office-interop) |
| Live web pages with recorded export fallbacks | [Web page slides](../examples/webpage) |

## Export your own presentation

Install Chrome or Chromium, or set `SLIDES_CHROME` to its executable path.
No Node dependency is needed for the CLI export commands.

```sh
slides export my-talk --format pdf --capture --steps --out talk.pdf
slides export my-talk --format handout --out reading-copy
slides build my-talk --out dist
```

Choose `--capture` when native shader/3D pixels matter. Choose `--steps` when
the PDF must explain intermediate reveals; it implies capture. Without
`--steps`, captured output includes the initial state of each slide, so later
cue content can remain hidden. Ordinary PDF printing exposes static reveal
content but uses fallbacks for native graphics. A captured PDF is composed of
slide images; share the source or reading handout when selectable text and
reading flow matter.

Use a fresh output folder for web publication. `offline-required: true` avoids
remote theme fonts; keep authored images/fonts local for an offline deck. The
gallery examples omit private speaker notes from their PDFs.

## Maintaining the gallery

The deck sources are authoritative. Edit them, then regenerate the three PDFs
and their selected preview frames in one pass:

```sh
go build -o slides ./cmd/slides
node scripts/export-examples.cjs ./slides
node scripts/export-examples.cjs --check
```

The regeneration script needs Node and Chrome, but no npm packages. It checks
the expected state counts, captures at a fixed viewport and only publishes
after all captures succeed. `docs/examples/manifest.json` records the CLI
version and binary hash, source hashes, dimensions, selected frames, and output
hashes. `--check` detects stale sources or modified assets without a browser;
it does not regenerate anything. Review the actual pages and previews before
committing: hashes prove freshness, not legibility. PDFs can differ in browser
metadata between runs even when their source is unchanged.

The full browser/export CI remains the functional gate. This catalog is small
on purpose so its public artifacts stay easy to inspect and refresh. Add an
entry to `scripts/export-examples.cjs` when adding another published PDF; update
the README and this page with the source and download links.

[Back to the README](../README.md) · [Capability reference](reference.md)

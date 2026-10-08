# Shaders and motion: a practical cookbook

Start with the [runnable effects deck](../examples/effects-cookbook) or its
[exported PDF](examples/effects-cookbook.pdf). Commands below run from your
project's parent directory; `my-talk` contains `deck.md`.

## 1. Choose and tune a background

```sh
slides backgrounds
slides serve my-talk --edit
```

Select **Backgrounds** in the toolbar. Choose a preset, tune its real native
preview, then apply to this slide or the deck default. You can also copy the
Markdown or download a tuned `.sel` without saving the deck.

For source authoring, put this in the opening `---` headmatter block:

```yaml
scene: shader:aurora
shader-ink: "#08121f"
shader-glow: "#51d6b0"
shader-speed: 0.12
shader-strength: 0.3
shader-scale: 1
```

| Control | Meaning | Accepted values |
|---|---|---|
| `scene` | Pattern | `shader:aurora`, `silk`, `contours`, `waves`, `grid`, `spotlight` (each with `shader:` prefix) |
| `shader-ink` | Base color behind the text | Quoted `"#rrggbb"` |
| `shader-glow` | Highlight color | Quoted `"#rrggbb"` |
| `shader-speed` | Rate of pattern motion | 0–2; 0 makes a still |
| `shader-strength` | Amount of highlight mixed into the base | 0–1 |
| `shader-scale` | Scale of the pattern coordinates | 0.5–4; larger values repeat more tightly |

Keep strength low enough that small text remains legible. Try a quieter
spotlight for dense slides and a stronger aurora for a title. The
[background gallery](../examples/background-gallery) contains all six.

A slide's **leading YAML fence** overrides the deck default. It is separate
from the `---` separator between slides:

````md
---

```yaml
scene: shader:silk
shader-ink: "#17101c"
shader-glow: "#e8b68b"
shader-speed: 0
shader-strength: 0.25
```

# A quiet, warm slide
````

Use `scene: false` to return to the normal theme background on that slide.

## 2. Own the shader source

```sh
mkdir -p my-talk/shaders
slides backgrounds source aurora --glow '#79a8ed' --speed 0.1 \
  --strength 0.3 --scale 1.5 > my-talk/shaders/aurora.sel
```

The tuning flags bake the same validated controls as the wizard into the file.
Omitted flags keep that preset's defaults. Quote hex colors: an unquoted `#`
starts a shell comment. The command prints only Selena to standard output and
reports invalid settings as errors. Shell redirection can truncate an existing
file before a command fails; export to a new filename while experimenting.
On releases without tuning flags, use `slides backgrounds source aurora` and
edit the `param` defaults, or download your tuned source from the wizard.

Point the deck at the file:

```yaml
scene: shaders/aurora.sel
```

Custom paths resolve relative to the deck. **The file's `param` defaults own
its colors and timing. `shader-*` keys only tune bundled `shader:` presets.**
Serve with `--edit`, press **E**, select the `.sel` file and save to validate
and reload it. Alternatively edit on disk and restart ordinary serve, or use
the `--watch` development loop.

## 3. Write a background from scratch

Save this as `my-talk/shaders/ribbons.sel` and set
`scene: shaders/ribbons.sel`. This is the same shader used in the effects deck:

```selena
material Ribbons {
    param ink : color = rgb(0.031373, 0.070588, 0.121569)
    param glow : color = rgb(0.317647, 0.839216, 0.690196)
    param speed : float = 0.12
    param strength : float = 0.35
    param scale : float = 1.0
    context { time : float }
    surface(geo) -> color {
        let coord = geo.uv * scale
        let drift = time * speed
        let wave = sin(coord.x * 10.0 + sin(coord.y * 4.0 + drift) * 2.0 + drift) * 0.5 + 0.5
        let ribbon = pow(wave, 5.0)
        return mix(ink, glow, clamp(ribbon * strength, 0.0, 1.0))
    }
}
```

`surface` returns a color at each point. `geo.uv` supplies surface coordinates;
`time` is driven by the shared motion clock. `sin` creates the bands, `pow`
sharpens them, and `mix` blends base and highlight colors. `rgb` uses 0–1
channels, so a hex channel value becomes its numeric value divided by 255.

| Variation | Change in this shader | Result |
|---|---|---|
| Still pattern | `speed = 0.0` | No time-driven movement |
| Gentle glow | `strength = 0.18` | Less contrast behind text |
| Broad bands | `coord.x * 6.0` instead of `10.0` | Fewer horizontal repetitions |
| Thin ribbons | `pow(wave, 10.0)` | Narrower bright regions |
| Soft bands | `pow(wave, 2.0)` | Wider bright regions |
| Reverse drift | `let drift = -time * speed` | Opposite time direction |
| Warm palette | `glow = rgb(0.91, 0.71, 0.55)` | Warm highlights |

Keep the material valid for both native backends; GoSX compiles Selena to GLSL
and WGSL before serving. Check it with `slides doctor my-talk` and
`slides inspect my-talk --json`; the graphics inventory reports compilation
errors and source paths. A custom file can write arbitrary shader logic, so
the preset parameter ranges do not bound the cost of your own function.

## 4. Use the material on shapes and diagrams

```md
<Shader Src="shaders/ribbons.sel" Shape="sphere" Label="Mint ribbon sphere" />
<Shader Src="shaders/ribbons.sel" Shape="torus" Label="Mint ribbon torus" />
```

Shapes are `plane`, `sphere`, `box` and `torus`. `Material="Ribbons"` selects a
material when the source defines more than one. `Uniforms="palette.json"`
can supply a deck-relative JSON parameter map for an inline shader.

For example, save this as `my-talk/shaders/shape.json`:

```json
{"ink":[0.12,0.23,0.32],"glow":[0.45,1.0,0.82],"strength":0.9,"scale":2.5}
```

Then use `<Shader Src="shaders/ribbons.sel" Uniforms="shaders/shape.json"
Shape="sphere" Label="Bright ribbon sphere" />`. The inline shape gets a
brighter palette while the background keeps the shader's quiet defaults.

For a Sirena scene, apply a material to named actors while keeping other
elements neutral:

```md
<Scene3D Src="request.sir" View="Request" Steps="steps.json"
         Shader="material.sel" Material="Pearl" Targets="api" />
```

Use the complete files in [request recovery](../examples/request-recovery);
`Targets` names actor IDs from that diagram. **M → Scene** edits camera,
actor position/scale/color and cue timing when the source is `.sir` plus
Steps JSON. [Its README](../examples/request-recovery/README.md) explains the
four absolute poses and repeatable backward navigation.

## 5. Choose how elements arrive

```md
:::motion {preset=slide-up duration=500 delay=80 distance=24 easing=ease-out replay=slide}
## A thought, with room to arrive
:::
```

| Effect | Setting |
|---|---|
| Simple entrance | `preset=fade` |
| Directional entrance | `slide-up`, `slide-down`, `slide-left`, `slide-right` |
| Scale entrance | `preset=zoom-in` |
| Word rhythm | `split=word stagger=70` |
| Character or line rhythm | `split=char` or `split=line`, with `stagger` |
| Every slide visit | `replay=slide` (default) |
| Every presentation step | `replay=step` |
| Only the first entrance | `replay=once` |

Durations, delays and stagger are milliseconds. Splitting is for text-only
regions: it replaces their inner markup with text units. Keep links, emphasis,
diagrams and mixed markup in unsplit motion regions.

## 6. Sequence a story with named cues

````md
```yaml
id: sequence
cues: overview, idea, detail
```

# Let the idea lead

:::motion {cue=idea preset=slide-up duration=550 replay=step}
## First, the headline.
:::

:::motion {cue=detail preset=fade duration=400 replay=step}
Then the supporting detail.
:::

:::motion {cue=detail after=detail preset=slide-right duration=350 delay=100 replay=step}
Finally, the implication.
:::
````

The first cue names the initial state. Arrow keys advance through the other
two. `#sequence/detail` restores the third state; it does not encode an
arbitrary millisecond within an entrance. `after=detail` schedules the last
region after the first region on that cue. Open **M → Elements** to inspect
the resolved tracks, drag timing bars, undo and save. Cycles or missing
dependencies appear as track warnings.

Earlier cue entrances settle at their completed poses when you advance or
deep-link to a later cue. Seeking the active cue cannot hide an earlier reveal.
Going back hides future cues and restores the destination's own entrance.

For whole-slide fades, use `transition: fade`, `transition-duration: 350`,
`transition-delay: 0` and `transition-easing: ease-out` in headmatter or a
slide YAML fence. `transition: none` removes that fade. Matching
`data-morph-id` values carry an element between slides. Code and diagram
morphs use absolute authored states; see the [storytelling lab](../examples/storytelling-lab).

## 7. Publish the right form

```sh
slides build my-talk --out dist
slides export my-talk --format pdf --capture --steps --out talk.pdf
slides export my-talk --format video --steps --seconds 2 --fps 15 --out talk.webm
```

| Output | What survives |
|---|---|
| Live serve or SPA | Shaders, motion, islands and cue navigation |
| Captured PDF, PNG, single HTML | Rendered pixels at settled poses; no live animation |
| PDF with `--steps` | Every click state, including the initial state |
| Ordinary PDF or single HTML | Static content; native graphics can use fallback labels |
| Handout | Readable, scrollable content; prefer this when reading is the goal |
| Video | Sampled motion; requires Chrome and ffmpeg |

Chrome is required for capture (`SLIDES_CHROME` overrides discovery). PDFs
cannot demonstrate timing: share the runnable deck or a video alongside them.
SPA exports retain native graphics offline; set `offline-required: true` to
suppress remote theme fonts, and keep authored assets local.

Backgrounds pause when hidden, share equal preset settings, and respect reduced
motion by showing the normal theme background. Native graphics default to
30 FPS, 1.5 device pixel ratio and a two-million-pixel budget. Avoid piling
expensive surfaces behind text. Use `slides bench my-talk --runs 3` on the
actual target device; the CLI reports startup, heap and frame intervals.

[Back to the gallery](examples.md) · [Full capability reference](reference.md)

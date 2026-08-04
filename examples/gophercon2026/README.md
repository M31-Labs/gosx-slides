# GoTreeSitter: the structural layer for Go tools

GopherCon 2026 deck by Oscar Villavicencio of M31 Labs. The talk uses the GoTreeSitter trilogy as a deliberately lightweight 15-slide microcosm:

1. “Inside a Pure-Go Tree-sitter Runtime”
2. “Programmable Grammars Are Infrastructure”
3. “What GoTreeSitter Makes Possible”

The deck follows three cumulative acts: application-ready capabilities first; the proof methods used to build an ambitious compatible runtime second; then grammargen, ownership, downstream products, and a reusable project playbook. AI accelerated candidate code and tests, while an independent oracle, reduced witnesses, ratcheted gates, and real consumers decided what earned trust.

The deck source is native Markdown++ authoring, not raw-HTML layout scaffolding. mdpp container directives, admonitions, definition lists, code, notes, and the ParseTree component lower into compiled GoSX slide components. The same deck therefore demonstrates the integration it describes.

## Canonical materials

- `deck.md` — audience slides and presenter notes.
- `deck.md` speaker notes — canonical spoken track.
- `transcript.md` — archived long-form prior track; do not use for rehearsal.
- `rehearsal.md` — current timing, demo fallback, trim valves, and precision cues.
- `source-to-slide-beat-map.md` — editorial provenance for every slide.
- `deck.css` — deck-specific visual contract layered on the Aurora theme.
- `VISUAL_SYSTEM.md` — palette, typography, motion, and caption-safe rules.

Internal campaign and editorial notes are archived in Hyphae rather than shipped with the deck. The AI-methodology slide reflects the author’s implementation practice; the remaining technical claims trace to the article trilogy.

## Communication contract

By the end, Go developers should know which GoTreeSitter capabilities can be dropped into an application, understand grammargen’s delivery and ownership responsibilities, and know how to keep AI-assisted infrastructure evidence-led.

- Slot: 25:00.
- Nominal finish: 23:30, leaving a 1:30 hard-stop buffer.
- Hard stop: 25:00.
- Aspect ratio: 16:9 at 1600×900.
- Caption-safe lower band: 20 percent.
- Offline operation: required.
- Live interaction: none required; the Scene3D atmosphere degrades locally.
- Live visual layer: a bounded 30 fps full-deck Scene3D starfield, with the
zoomed, color-cycling closing galaxy isolated to the final slide and a static reduced-motion/print fallback.
- Projector contrast: every content region sits on a bounded near-black glass
  plate; unused canvas remains live.

## Build and authoring checks

From the repository root:

``` sh
GOWORK=off go build -o /tmp/gosx-slides ./cmd/slides
/tmp/gosx-slides check examples/gophercon2026
/tmp/gosx-slides inspect examples/gophercon2026 --json
/tmp/gosx-slides validate examples/gophercon2026 --strict --profile conference
/tmp/gosx-slides rehearse examples/gophercon2026
/tmp/gosx-slides components examples/gophercon2026 --json
/tmp/gosx-slides doctor examples/gophercon2026 --json
```

Run locally:

``` sh
/tmp/gosx-slides serve examples/gophercon2026 --port 8080
```

Navigation: `→` or Space advances, `←` goes back, `o` toggles overview, `p`
opens presenter view, and `f` toggles fullscreen.

## Durable release artifacts

The verified export target is outside the source example:

``` text
.tiller/artifacts/gophercon2026-trilogy/
├── spa/
├── gophercon2026-trilogy.pdf
├── screenshots-1600x900/
└── contact-sheet.png
```

Rebuild after any content or CSS change:

``` sh
/tmp/gosx-slides build examples/gophercon2026 \
  --out .tiller/artifacts/gophercon2026-trilogy/spa
/tmp/gosx-slides export examples/gophercon2026 --format pdf \
  --out .tiller/artifacts/gophercon2026-trilogy/gophercon2026-trilogy.pdf
```

## Stage fallback

If the slide-8 tree does not respond on the first click, stop interacting and
say, “The interaction is incidental; the structure is the artifact.” Point to
the visible `source_file`, `short_var_declaration`, and
`parenthesized_expression` labels, then continue. Do not refresh or troubleshoot
on stage.

# Pure-Go Tree-sitter — GopherCon 2026

Twenty-five-minute GopherCon 2026 talk by Oscar Villavicencio, founder of M31
Labs. The talk is Wednesday, August 5 at 4:15 PM PDT in Finneran Ballroom 2.

The 21-slide `aurora` deck follows one cumulative story:

1. Changing code needs useful structure before it becomes valid.
2. Pure Go removes a product boundary while preserving an honest performance
   tradeoff.
3. The original C runtime supplies an independent behavioral oracle.
4. grammargen turns the runtime into a pure-Go language toolchain.
5. M31 Labs uses that substrate across a family of working systems, including
   the deck itself.
6. Structural editing gives people and agents more testable intent than text
   patches alone.

The talk teaches first and acts as a credibility showcase for M31 Labs through
the artifacts on screen. The closing invitation points to the anchored Build
contact form without turning the stage into a sales pitch.

## Readiness materials

- [`transcript.md`](transcript.md) — editor-ready visible copy and spoken track.
- [`rehearsal.md`](rehearsal.md) — dated practice schedule, timing valves,
  question bank, and run scorecard.
- [`campaign.md`](campaign.md) — opportunity goals, publishing calendar, ready
  copy, conversation scripts, lead log, and follow-up path.

## Conference contract

`deck.md` encodes 16:9 output, a reserved 20-percent caption band, a 25-minute
slot, offline operation, and a static fallback for the interactive tree. Add
`caption-guide: true` temporarily while authoring to display the reserved band.
The guide is off during normal live and PDF output.

## Run it

```sh
GOWORK=off go build -o /tmp/slides ./cmd/slides
/tmp/slides serve examples/gophercon2026            # http://127.0.0.1:8080
/tmp/slides validate examples/gophercon2026 --profile conference --strict
/tmp/slides rehearse examples/gophercon2026         # speaker run sheet
/tmp/slides export examples/gophercon2026 --format spa --out /tmp/gophercon-offline
/tmp/slides export examples/gophercon2026 --format pdf --out /tmp/gophercon-2026.pdf
```

`→`/`Space` advances, `o` opens overview, `p` opens presenter view, and `f`
enters fullscreen. Presenter notes carry cumulative cues to a nominal 22:55
finish; the target finish stays 24:30, and the difference is planned slack.

Presenter-sync caveat: every open deck window drives every other one. Close
stray tabs before presenting.

## Components

| File | Role |
| --- | --- |
| `ParseTree.gsx` | Interactive incomplete-Go tree used in the live demo. |
| `Timeline.gsx` | Retained project chronology for alternate cuts. |
| `Benchmark.gsx` | Retained benchmark component; use only with current receipts. |
| `Citation.gsx` | Evidence chip for benchmark-bearing alternate cuts. |

The current talk intentionally omits unsealed benchmark numbers. Any numeric
performance claim added before August 5 must be single-sourced from the frozen
receipt and reconciled with `BENCH.md`.

## Final readiness gates

- Strict conference validation passes on the frozen deck.
- All custom components compile and hydrate.
- Live demo succeeds offline and has a rehearsed static fallback.
- Offline bundle and PDF backup are copied to the laptop and USB drive.
- Every spoken correctness and performance claim matches the frozen repo.
- QR opens `m31labs.dev/build#contact`; submission reaches the contact inbox.
- Notifications, screen saver, and automatic updates are disabled.
- Laptop, power supply, preferred adapter, and advancer are packed.

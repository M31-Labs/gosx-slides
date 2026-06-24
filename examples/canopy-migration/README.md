# canopy-migration — a live talk deck

A generic, reusable ~15–20 min talk: **how [canopy](https://github.com/odvcencio/canopy)
helps migrate a Java service to Go.** It's a real-lane gosx-slides deck — two of
its slides are *live GoSX islands*, not screenshots.

## Run it

```bash
go build -o /tmp/slides ../../cmd/slides
/tmp/slides serve . --port 8080
# open http://127.0.0.1:8080
```

Authoring loop (hot-swap an island in place, state preserved):

```bash
/tmp/slides serve . --watch
```

## What's here

| File | Role |
|---|---|
| `deck.md` | 11 slides. aurora theme. The narrative: Understand → Triage → Find the seams → Govern → Verify. |
| `HotspotExplorer.gsx` | **Live island** (slide 6). Click a Java method to see how risky it is to port — the `canopy analyze hotspot` story, made interactive. |
| `BoundaryGate.gsx` | **Live island** (slide 8). Toggle the tempting shortcut import and watch the `canopy analyze boundaries` CI gate flip PASS → FAIL. |
| `go.mod` | Makes this deck a self-contained module (`require m31labs.dev/gosx`), so it serves from **any** directory once you copy it out. |

## Honest framing (baked into the deck)

canopy is **structural code intelligence**, not a transpiler. It does **not**
translate Java to Go. It makes the migration *legible* (census, complexity,
hotspots, seams, dead code) and *enforceable* (architecture boundaries as a CI
gate). The deck is candid about the edges — the Java call graph is name-based and
noisy; cross-repo federation today is really just `graph services`.

## Make it yours

The data in the two islands is illustrative (a generic `OrderService`). Swap in
your own numbers from `canopy analyze complexity --json` and your real
`.canopyboundaries`, and rename the methods to your service's. Each slide ends
with a `<!-- speaker note -->` you can read in the presenter view (`p`).

> Note: every slide ends with a trailing block (the speaker-note comment). That's
> deliberate — it also forces clean slide splitting. If you add a slide whose
> content is several blocks (heading + prose + code + prose) and it doesn't split,
> end it with an HTML comment too.

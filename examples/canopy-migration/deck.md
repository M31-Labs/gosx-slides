---
title: Java to Go, with a map
theme: aurora
---

```yaml
layout: title
```

# {deck.title}

Migrating a service without flying blind — using **canopy** to *understand* and
*govern* the move from a Java service to Go.

A live, generic playbook. Pull it down, point it at your own service, keep talking.

<!-- Opener: this is a real gosx-slides deck — the two widgets later are live GoSX islands, not screenshots. Set the frame: we are NOT here to auto-translate Java to Go. We are here to make the migration legible and enforceable. ~30s. -->

---

# Rewrites die in the dark

The big-bang rewrite is a graveyard. Most don't fail at the keyboard — they fail
because **nobody holds the whole map**.

- You can't port what you don't understand — and the legacy service understands itself better than anyone left on the team.
- "We'll clean it up in Go" quietly recreates the same ball of mud, one package at a time.
- Without a seam, there's no safe order — so every change feels load-bearing.

We don't need a translator. We need a **map** of the old service and **guardrails**
on the new one.

<!-- Land the thesis hard: the risk in a rewrite is comprehension, not typing. Ask the room who has lived through a stalled rewrite. -->

---

# Meet canopy

Structural code intelligence: it parses **structure**, not vibes. Tree-sitter
under the hood, **206+ languages** — Java and Go both first-class.

- **Index** every symbol, import, and reference across the repo, in under a second.
- **Rank** complexity and churn-weighted hotspots — what's risky to touch.
- **Trace** call graphs, blast radius, and dead code.
- **Govern** architecture with enforceable import boundaries, as a CI gate.

One honest line up front: **canopy does not translate code.** No Java-to-Go
codegen. It makes the migration *legible and enforceable* — the two things that
actually decide whether a rewrite lands.

<!-- Say the honest line out loud and slowly. An internal eng crowd will trust you more for scoping it. canopy = understand + govern, never "magic port button". -->

---

```yaml
layout: center
```

# The migration loop

**Understand** → **Triage** → **Find the seams** → **Govern the target** → **Verify**

Canopy shows up at every step — as the map going in, and the guardrail coming out.

<!-- This is the spine of the talk. Each next slide is one of these five beats. Promise the two live demos: a hotspot triage explorer, and a CI boundary gate. -->

---

# 1 · Census — know what you have

Index the legacy Java service, then read its shape instead of guessing.

```bash
canopy index build .
canopy index stats
canopy index map src/main/java/com/acme/order/OrderService.java
```

`index stats` gives you the census — files, languages, symbol counts.
`index map` is a structural table of contents: every class and method, with full
signatures and line ranges. The "what is actually in this 1,800-line file" view,
in seconds.

<!-- Demo beat (optional live): run `canopy index stats` on any Java repo. The point: you now have ground truth, not tribal memory. -->

---

# 2 · Triage — what's scary to port

```bash
canopy analyze hotspot
canopy analyze complexity --sort cognitive --top 15
```

Hotspot fuses churn × complexity × centrality. This is your **porting backlog,
ranked by risk** — click a method to see how you'd approach it:

<HotspotExplorer/>

Port the green warm-ups first to build momentum; ring-fence the red ones with
characterization tests before you touch them.

<!-- LIVE DEMO: click each row. Top of the list = port last, behind tests. Bottom = port first, build momentum. The ranking turns "where do we even start" into a plan. -->

---

# 3 · Find the seams

The strangler-fig needs cut points. Canopy finds them structurally.

```bash
canopy graph impact OrderService.checkout    # reverse blast radius
canopy graph fanin                           # the API surface to preserve
canopy graph dead                            # what NOT to port
```

`graph dead` is **framework-aware** — it knows Spring controllers and JUnit tests
are roots, so it won't tell you to delete your endpoints.

> ⚠️ Honesty check: on Java the call graph is **name-based**, not type-aware. Treat
> its edges as *candidate calls* for orientation — not ground truth. Overloads and
> annotation-heavy code get noisy.

<!-- The seam is where you cut the monolith. fanin = the contract you must preserve. dead = scope you can drop. Be candid about the call-graph caveat — credibility. -->

---

# 4 · Govern the target

The Go rewrite only stays clean if something keeps it clean. Declare the intended
architecture and let canopy enforce it on every commit.

```bash
canopy analyze boundaries --format sarif    # exits non-zero on a violation
```

Toggle the tempting shortcut and watch the gate flip — this is the CI check that
stops the new service from rotting into the old one:

<BoundaryGate/>

<!-- LIVE DEMO: click the button. Green PASS -> red FAIL with exit 1. This is the difference between hoping the new architecture holds and proving it on every PR. SARIF -> inline GitHub annotations. -->

---

# 5 · Verify the port

```bash
canopy index diff  --before-cache java.idx --after-cache go.idx
canopy graph drift origin/main HEAD          # new imports, new cycles
```

Structural before/after: did the Go service preserve the surface and avoid new
dependency cycles?

> ⚠️ `analyze similarity` is a *same-language* clone detector — it will **not**
> prove your Go matches the Java. Verify behavior with tests; verify *structure*
> with canopy.

<!-- Verify is structural, not behavioral. Tests prove behavior; canopy proves the shape and the dependency hygiene. Don't oversell similarity. -->

---

# What canopy is — and isn't

Be precise on stage. It earns trust and it's the truth.

- **Is:** fast, accurate structural intelligence — census, complexity, hotspots, impact, dead code, architecture governance, CI gating. Strong on Java *and* Go.
- **Isn't:** a transpiler. Zero code translation.
- **Watch it:** Java call graph is name-based (noisy on overloads/annotations); cross-repo federation today is really just `graph services`.

The migration is still yours to write. Canopy makes sure you write it **with the
lights on**.

<!-- The honest-limits slide is a feature, not a hedge. Naming the edges is what makes the strong claims believable. -->

---

```yaml
layout: center
```

# Start today

```bash
canopy index build .
canopy analyze hotspot
```

Point it at the Java service this afternoon. The map draws itself — then go port
the easy wins first.

**Understand → Triage → Seam → Govern → Verify.**

<!-- Close on the loop. One concrete ask: run `canopy index build .` on the legacy service before the next standup. -->

---
title: A replayable simulation
theme: paper
simulation: simulation.yaml
offline-required: true
transition: none
---

```yaml
id: particles
cues: start, impulse, settled
```

# One seed, repeatable evidence

:::simulation demo
:::

The presentation playhead drives an exact fixed tick. Scrub backward or choose
the authored wind branch, then return to the baseline.

<!-- The model uses integer milli-pixels, an explicit seed and logged impulses. -->

---

```yaml
id: recap
```

# Explain what changed

- Seed **1729**, **30 ticks per second**, **180 ticks**.
- Each checkpoint restores positions, velocities and random state.
- The wind branch keeps the prefix through tick 60 and replaces future inputs.
- Offline exports carry the same authoritative state frames.

<!-- The GoSX sim.Simulation contract remains the model boundary. -->

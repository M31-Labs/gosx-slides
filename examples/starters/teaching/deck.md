---
title: Predict, observe, explain
theme: swiss
packs: studio@1.0.0
simulation: simulation.yaml
offline-required: true
transition: none
footer: LEARNING LAB · PREDICT → OBSERVE → EXPLAIN
---

```yaml
id: opening
layout: title
class: studio-cover
```

<p class="studio-kicker">A SMALL MODEL / A TESTABLE IDEA</p>

# Predict. Observe. Explain.

How does one impulse change a moving system?

<p class="studio-byline">A 10-minute classroom exploration</p>

<!-- Ask for a prediction before revealing the model. Learners should have time to form an explanation. -->

---

```yaml
id: prediction
reveal: true
```

<p class="studio-kicker">01 / PREDICT</p>

# What stays the same?

- The particles begin from the same seed.
- One input arrives at a known tick.
- A stronger impulse changes the future.

<p class="studio-note">Write a prediction. Explain the reason before pressing next.</p>

<!-- Reveal one condition at a time. Ask learners to distinguish initial conditions from later inputs. -->

---

```yaml
id: particles
cues: start, impulse, settled
footer: false
```

<p class="studio-kicker">02 / OBSERVE</p>

# Replay the same experiment.

:::simulation demo
:::

Compare the baseline and wind branches. Scrub backward to check your prediction.

<!-- Every frame is baked offline from the same seed. Advance named cues, then compare the wind branch and return to baseline. -->

---

```yaml
id: explanation
```

<p class="studio-kicker">03 / EXPLAIN</p>

# Separate cause from state.

:::cards
:::card "Seed"
The same initial conditions make a comparison meaningful.
:::
:::card "Input"
An authored event changes velocity at a specific tick.
:::
:::card "Replay"
Returning to a tick restores the state you observed.
:::
:::

<!-- Ask learners to point to an observation supporting each claim. -->

---

```yaml
id: practice
layout: center
```

<p class="studio-kicker">04 / TRY A NEW QUESTION</p>

# Change one variable.

Predict a later impulse. Edit its tick in `simulation.yaml`. Replay and explain the difference.

[Return to the experiment](#particles/start)

<!-- Preserve the seed while changing the input tick. Rebuild the deck, compare the outcome and record what surprised you. -->

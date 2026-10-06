---
title: Explain the mechanism
theme: paper
packs: studio@1.0.0
line-numbers: true
offline-required: true
transition: none
footer: TECHNICAL TALK · MECHANISM → EVIDENCE
---

```yaml
id: opening
layout: title
class: studio-cover
```

<p class="studio-kicker">A TECHNICAL TALK / 15 MINUTES</p>

# Explain the mechanism.

One question. A concrete model. Evidence you can inspect.

<p class="studio-byline">Your name · Your event</p>

<!-- State the problem in a sentence the audience already understands. Replace the title, byline and footer before presenting. -->

---

```yaml
id: constraint
layout: default
```

<p class="studio-kicker">01 / THE CONSTRAINT</p>

# What must survive?

:::columns
:::col "INPUT"
An edit can arrive halfway through a request.

The source may be incomplete. The last working result still matters.
:::
:::col "CONTRACT"
Keep the author’s source. Preserve stable identities. Publish a complete result.
:::
:::

<!-- Replace this constraint with a real failure your audience recognizes. -->

---

```yaml
id: mechanism
cues: read, validate, publish
```

<p class="studio-kicker">02 / THE MECHANISM</p>

# Make publication explicit.

```go {1-2|3-4}
source := readDraft()
candidate := compile(source)
validate(candidate)
publish(candidate)
```

The boundary is small enough to reason about and test.

<!-- Use the read, validate and publish cues. Code is illustrative: replace it with a snippet import from your implementation. -->

---

```yaml
id: model
footer: false
```

<p class="studio-kicker">03 / THE MODEL</p>

# Three responsibilities.

```sirena
artifact source { label: "Source" }
service compiler { label: "Compiler" }
artifact bundle { label: "Bundle" }
source -> compiler: flow "parse + validate"
compiler -> bundle: flow "publish"
```

Source remains editable. The published bundle remains reproducible.

<!-- Give each responsibility a name. Keep diagram actor IDs stable when changing labels. -->

---

```yaml
id: evidence
```

<p class="studio-kicker">04 / THE EVIDENCE</p>

# Measure the contract.

| Experiment | Observe |
| --- | --- |
| Replay the same source | Identical semantic output |
| Interrupt publication | Previous result remains usable |
| Navigate backward | The authored state is restored |

<p class="studio-note">Add measurements and source links from your own experiment.</p>

<!-- Do not invent performance figures. Explain the setup and show what the experiment actually established. -->

---

```yaml
id: close
layout: quote
```

<p class="studio-kicker">TAKE THIS WITH YOU</p>

> A useful abstraction makes its failure boundary visible.

[Revisit the mechanism](#mechanism/validate) · Replace with your repo or paper.

<!-- Leave a concrete next step and a stable link the audience can follow. -->

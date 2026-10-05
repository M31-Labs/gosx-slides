---
title: A boundary worth keeping
theme: aurora
packs: studio@1.0.0
story: story.yaml
offline-required: true
transition: none
footer: PLATFORM ENGINEERING · ARCHITECTURE REVIEW
---

```yaml
id: opening
layout: title
class: studio-cover
```

<p class="studio-kicker">ADR 024 / DECISION REVIEW</p>

# Make the boundary explicit.

Keep request handling fast. Make persistence observable.

<p class="studio-byline">Platform team · 12-minute review</p>

<!-- Replace the ADR number and team. Open with the decision we need, not a tour of the implementation. -->

---

```yaml
id: system
cues: overview, accepted, committed
footer: false
```

<p class="studio-kicker">01 / SYSTEM BOUNDARY</p>

# Follow one write.

:::columns
:::col
```sirena
client browser { label: "Browser" }
service api { label: "API" }
job worker { label: "Worker" }
database db { label: "Database" }
browser -> api: calls "request"
api -> worker: flow "dispatch"
worker -> db: writes "persist"
```
:::
:::col
```go
request := accept()
enqueue(request)
persist(request)
```

The API owns acceptance. The worker owns a durable result.
:::
:::

<!-- Advance the accepted and committed cues. story.yaml binds actors, code lines and captions to this same diagram. -->

---

```yaml
id: tradeoffs
```

<p class="studio-kicker">02 / THE TRADEOFF</p>

# Optimize for recovery.

:::cards
:::card "Fast acceptance"
The user gets an acknowledgment before persistence completes.
:::
:::card "Explicit ownership"
The queue marks the handoff. Every request keeps a stable identifier.
:::
:::card "Visible failure"
Retries and dead letters make incomplete work discoverable.
:::
:::

<!-- Name one cost: eventual consistency. Replace these claims with measured behavior from your own system. -->

---

```yaml
id: evidence
```

<p class="studio-kicker">03 / RELEASE GATES</p>

# Show the evidence.

| Question | Required evidence |
| --- | --- |
| Can we retry safely? | Duplicate requests converge to one durable result. |
| Can we recover a worker? | A restart resumes committed queue state. |
| Can operators find failure? | The trace links acceptance, retry and persistence. |

<p class="studio-note">Replace each gate with a reproducible test or runbook link.</p>

<!-- This starter deliberately supplies criteria, not fabricated measurements. Bring the actual test results to the review. -->

---

```yaml
id: decision
layout: quote
```

<p class="studio-kicker">04 / THE DECISION</p>

> Approve the boundary when recovery is demonstrated.

Assign an owner. Record the conditions. Set a review date.

<!-- End with the requested decision and the owner of each condition. -->

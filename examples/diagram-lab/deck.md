---
title: Diagrams that explain
theme: aurora
transition: none
offline-required: true
---

```yaml
id: state
```

# Lifecycle, one state at a time

```sirena diagram=state
service idle { label: "Idle" state: "initial" }
service running { label: "Running" }
service done { label: "Complete" state: "final" }
idle -> running: flow "start"
running -> idle: flow "retry"
running -> done: flow "finish"
```

States show the entry and final states explicitly.

<!-- state layout uses stable native identities. -->

---

```yaml
id: class
```

# Readable contracts

```sirena diagram=class
service order { label: "Order" fields: "id: UUID; total: Money" methods: "submit(); cancel()" }
service item { label: "OrderItem" fields: "sku: string; quantity: int" methods: "subtotal()" }
order -> item: flow "1 to many"
```

Fields and methods sit in measured compartments.

<!-- class layout uses stable native identities. -->

---

```yaml
id: er
```

# Data relationships

```sirena diagram=er
database customer { label: "Customer" fields: "PK id: UUID; email: string" }
database order { label: "Order" fields: "PK id: UUID; FK customer_id: UUID; total: decimal" }
customer -> order: flow "1 to many"
```

Entities keep their fields and relationship cardinality.

<!-- er layout uses stable native identities. -->

---

```yaml
id: swimlane
```

# Who does what

```sirena diagram=swimlane
client submit { label: "Submit request" lane: "Customer" }
service review { label: "Review request" lane: "Operations" }
service approve { label: "Approve" lane: "Operations" }
client notify { label: "Receive notification" lane: "Customer" }
submit -> review: flow "request"
review -> approve: flow "validated"
approve -> notify: flow "result"
```

Lanes make ownership explicit; row order shows progression.

<!-- swimlane layout uses stable native identities. -->

---

```yaml
id: timeline
```

# Time, in proportion

```sirena diagram=timeline
service design { label: "Design" start: 0 duration: 3 }
service build { label: "Build" start: 2 duration: 5 }
service test { label: "Validate" start: 6 duration: 2 }
service release { label: "Release" start: 8 duration: 1 }
design -> build: depends_on "handoff"
build -> test: depends_on "ready"
test -> release: depends_on "approved"
```

Bar lengths follow numeric duration; captions stay outside short bars.

<!-- timeline layout uses stable native identities. -->

---

```yaml
id: workflow
cues: overview, request, worker, done, restore
```

# Reveal the system, then follow the work

<Scene3D Src="workflow.scene.json" Label="Request path with focus, reveal, and traced relationships" />

[Focus the worker](#workflow/worker). Arrow keys restore each absolute state.

<!-- Scene3D native GPU motion sleeps when this slide is hidden. -->


---

```yaml
id: mindmap
```

# Build the story

```sirena diagram=mindmap
service story { label: "A compelling story" }
service context { label: "Set the context" }
service evidence { label: "Show the evidence" }
service change { label: "Make the change" }
service benchmark { label: "Measured performance" }
service demo { label: "Interactive demonstration" }
story -> context: flow
story -> evidence: flow
story -> change: flow
evidence -> benchmark: flow
evidence -> demo: flow
```

<!-- Native Sirena mindmap layout. -->

---

```yaml
id: bar
```

# See the gain

```sirena diagram=bar
service baseline { label: "Previous release" value: 35 }
service improvement { label: "New release" value: 72 }
service regression { label: "Removed overhead" value: -18 }
```

<!-- Native Sirena bar layout. -->

---

```yaml
id: pie
```

# Show the composition

```sirena diagram=pie
service authored { label: "Authoring" value: 45 }
service presenting { label: "Presenting" value: 35 }
service sharing { label: "Sharing" value: 20 }
```

<!-- Native Sirena pie layout. -->

---

```yaml
id: gantt
```

# Plan real calendar days

```sirena diagram=gantt
service design { label: "Design" start: "2026-10-01" end: "2026-10-04" }
service build { label: "Build" start: "2026-10-03" end: "2026-10-10" }
service validate { label: "Validation" start: "2026-10-10" end: "2026-10-13" }
service ship { label: "Ship" start: "2026-10-13" end: "2026-10-14" }
design -> build: depends_on "handoff"
build -> validate: depends_on "ready"
validate -> ship: depends_on "approved"
```

<!-- Native Sirena gantt layout. -->

---
title: A request worth following
theme: aurora
transition: none
offline-required: true
---

```yaml
id: opening
```

<p class="eyebrow">SIRENA × SELENA × GOSX</p>

# A request worth following.

One system. Four states. A timeline you can inspect.

Use the arrow keys to advance. Press **M** to pause, scrub or reverse.

---

```yaml
id: request
cues: overview, accepted, failure, recovery
motion-duration: 1200
```

<p class="eyebrow">01 / ACCEPT → FAIL → RECOVER</p>

# Keep every stage in view.

<Scene3D Src="request.scene.json" Label="Request path with animated focus, attached edges and camera" />

<p class="path">Browser → API → Queue → Worker → Database</p>

:::code-morph {duration=1200 easing=ease-in-out}
```go
// Follow durable work.
queue.Publish(request)
```

```go
// Enqueue, then acknowledge.
queue.Publish(request)
return Accepted
```

```go
// Keep the job on failure.
if worker.Unavailable() {
    queue.Retain(request)
}
```

```go
// Retry with an identity.
job := request.WithKey()
worker.Process(job)
queue.Acknowledge(job.ID)
```
:::

:::motion {cue=accepted preset=fade duration=800 replay=step}
**Accepted.** The API hands off durable work.
:::

:::motion {cue=failure preset=fade duration=800 replay=step}
**Unavailable.** The queue preserves the request.
:::

:::motion {cue=recovery preset=fade duration=800 replay=step}
**Recovered.** The worker resumes without losing the job.
:::

<!-- Seek 0, 600, 1200, then 600 again. Camera, labels and routes follow the same clock. -->

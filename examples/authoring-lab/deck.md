---
title: The motion workshop
theme: aurora
offline-required: true
transition: none
---

```yaml
id: opening
layout: title
```

# Ideas that move

Press **M** for the motion studio. Use **→** to advance a cue.

<div data-morph-id="message" style="padding:24px;border-radius:18px;background:#243d58;width:280px">One idea, continuous motion</div>

<!-- Opening: inspect without disturbing the audience’s navigation. -->

---

```yaml
id: pipeline
cues: overview, request, worker, done
layout: default
```

# A pipeline, one beat at a time

:::motion {preset=slide-up cue=request duration=600 group=pipeline}
## Request arrives
:::

:::motion {preset=slide-right cue=worker duration=700 group=pipeline}
## Worker processes it
:::

:::motion {preset=fade cue=worker after=worker duration=500 delay=80}
The worker’s context appears after its heading.
:::

:::motion {preset=zoom-in cue=done duration=450}
**Result delivered.** [Replay the worker](#pipeline/worker)
:::

<!-- Named cues survive rearranging the slide order. -->

---

```yaml
id: destination
layout: center
```

# The same idea, a new context

<div data-morph-id="message" style="padding:48px;border-radius:18px;background:#243d58;width:480px">One idea, continuous motion</div>

<Counter Initial={0}/>

<!-- Go back and forth: the counter remains the same live instance. -->

---

```yaml
id: later
```

# A later interactive slide

<Counter Initial={10}/>

<!-- Hydrated when this slide or its predecessor is visited. -->

---

```yaml
id: far-away
```

# Stay inexpensive

<Counter Initial={20}/>

<!-- Inspect SlidesRuntime.stats() to see the deferred islands. -->

---

```yaml
id: evolving-code
```

# Evolve the implementation

:::code-morph
```go
func greet() string {
    return "hello"
}
```

```go
func greet(name string) string {
    return "hello " + name
}
```

```go
func greet(name string) string {
    message := "hello " + name
    return message
}
```
:::

<!-- Three code states. Repeated lines move; new lines fade in. -->

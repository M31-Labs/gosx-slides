---
title: The queue buys us time
theme: neon
story: story.yaml
offline-required: true
---

```yaml
id: pressure
cues: baseline, burst, recover
```

# The queue buys us time

<p class="story-intro">One request. Three perspectives. One clock.</p>

:::columns
:::col
:::story-surface {name=map}
<p class="surface-label">Request map</p>

```sirena
client browser { label: "Browser" }
service api { label: "API" }
queue buffer { label: "Buffer" }
job worker { label: "Worker" }
browser -> api: calls "request"
api -> buffer: writes "enqueue"
buffer -> worker: flow "drain"
```
:::

:::
:::col
:::story-surface {name=runtime}
<p class="surface-label">Runtime view</p>

<Scene3D Src="runtime.sir" />
:::

```go
request := accept()
buffer.Enqueue(request)
worker.Drain(buffer)
```

<p data-story-id="takeaway">The buffer absorbs the burst; the worker drains it at a sustainable pace.</p>
:::
:::

:::story-surface {name=load}
<p class="surface-label">Pressure model</p>

:::diagram-morph {diagram=bar duration=1200}
```sirena
service api { label: "API pressure" value: 18 }
job worker { label: "Worker pressure" value: 18 }
```

```sirena
service api { label: "API pressure" value: 84 }
job worker { label: "Worker pressure" value: 32 }
```

```sirena
service api { label: "API pressure" value: 24 }
job worker { label: "Worker pressure" value: 24 }
```
:::
:::

<!-- Explicit captions describe the authored model. No measurements or speech are inferred. -->

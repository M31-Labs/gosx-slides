---
title: Stories that move
theme: aurora
transition: none
offline-required: true
---

```yaml
id: opening
```

# Stories that move

:::motion {preset=slide-up duration=800 delay=50 replay=slide}
## One clear idea at a time
:::

:::motion {preset=fade duration=600 delay=200 replay=slide}
Open **M** to edit the timeline. Drag a bar, undo a change, then save it to deck.md.
:::

---

```yaml
id: growth
cues: before, after
```

# Growth you can see

:::diagram-morph {diagram=bar duration=1000 easing=ease-in-out}
```sirena
service adoption { label: "Adoption" value: 18 }
service speed { label: "Speed" value: 42 }
```

```sirena
service adoption { label: "Adoption" value: 84 }
service speed { label: "Speed" value: 68 }
service breadth { label: "Breadth" value: 56 }
```
:::

The scale and rows stay fixed. Advance a step or open **#growth/after**.

---

```yaml
id: architecture
```

# The system evolves

:::diagram-morph {diagram=architecture duration=900}
```sirena
client browser { label: "Browser" }
service api { label: "API" }
browser -> api: calls "Request"
```

```sirena
client browser { label: "Browser" }
service api { label: "API" }
database store { label: "Store" }
browser -> api: calls "Request"
api -> store: writes "Persist"
```
:::

Existing actors keep their positions as the next relationship appears.

---

```yaml
id: trend
```

# A trend, two poses

:::diagram-morph {diagram=line duration=1100}
```sirena
service jan { x: 1 y: 12 series: "Adoption" }
service feb { x: 2 y: 28 series: "Adoption" }
service mar { x: 3 y: 32 series: "Adoption" }
```

```sirena
service jan { x: 1 y: 12 series: "Adoption" }
service feb { x: 2 y: 36 series: "Adoption" }
service mar { x: 3 y: 64 series: "Adoption" }
```
:::

Axes remain comparable across every pose.

---

```yaml
id: balance
```

# A broader view

```sirena diagram=radar
service speed { axis: "Speed" value: 8 }
service breadth { axis: "Breadth" value: 6 }
service clarity { axis: "Clarity" value: 9 }
service portability { axis: "Portability" value: 7 }
```

---

```yaml
id: flow
```

# Where the work flows

```sirena diagram=sankey
service visits { label: "Visits" }
service signup { label: "Sign-ups" }
service browse { label: "Browse" }
service active { label: "Active" }
visits -> signup: flow "120"
visits -> browse: flow "280"
signup -> active: flow "90"
```

Widths show how much moves through each path.

---

```yaml
id: native
```

# Take the data into 3D

<Scene3D Src="plot.scene.json" Label="A native scatter plot with numerical axes and point tours" />

Use the steps to focus each point. Native shaders and 3D remain live in the browser.

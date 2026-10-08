---
title: A little motion, a clear idea
theme: aurora
offline-required: true
scene: shaders/ribbons.sel
transition: fade
transition-duration: 350
transition-easing: ease-out
footer: GOSX SLIDES · EFFECTS COOKBOOK
---

```yaml
id: opening
layout: title
```

<p class="eyebrow">A SMALL, RUNNABLE COOKBOOK</p>

# A little motion.<br>A clear idea.

Own the background. Shape the entrance. Give each beat its moment.

<p class="recipe-note">Markdown + Selena · Use → to explore · M opens the motion studio</p>

---

```yaml
id: words
layout: center
scene: false
```

<p class="eyebrow">01 / TEXT THAT ARRIVES</p>

# Give the words room.

:::motion {preset=slide-up split=word duration=500 stagger=70 distance=18 replay=slide}
One clear thought. One word at a time.
:::

<p class="recipe-note">split=word · stagger=70 · replay=slide<br>Leave and return to see the entrance again.</p>

---

```yaml
id: sequence
cues: overview, idea, detail
scene: false
```

<p class="eyebrow">02 / A SEQUENCE YOU CONTROL</p>

# Let the idea lead.

:::motion {cue=idea preset=slide-up duration=550 replay=step}
## First, the headline.
:::

:::motion {cue=detail preset=fade duration=400 replay=step}
Then the supporting detail.
:::

:::motion {cue=detail after=detail preset=slide-right duration=350 delay=100 replay=step}
**Finally, the implication.** This follows the detail's entrance.
:::

<p class="recipe-note">Named cues + after=detail · Link directly to #sequence/detail</p>

---

```yaml
id: material
scene: false
class: material
```

<p class="eyebrow">03 / ONE MATERIAL, ANOTHER CANVAS</p>

# Put the pattern on a shape.

<Shader Src="shaders/ribbons.sel" Uniforms="shaders/shape.json" Shape="sphere" Label="A sphere with a brighter variation of the opening ribbon material" />

<p class="recipe-note">The same .sel file + brighter uniforms · Shape="sphere"<br>Also try plane, box or torus</p>

---

```yaml
id: variation
layout: center
scene: shader:silk
shader-ink: "#17101c"
shader-glow: "#e8b68b"
shader-speed: 0.1
shader-strength: 0.25
shader-scale: 1.5
```

<p class="eyebrow">04 / CHANGE THE ATMOSPHERE</p>

# Warm light.<br>The same clear story.

A slide-level preset overrides the custom deck background.

<p class="recipe-note">shader:silk · speed 0.1 · strength 0.25 · scale 1.5<br>Choose Backgrounds in edit mode to explore the controls.</p>

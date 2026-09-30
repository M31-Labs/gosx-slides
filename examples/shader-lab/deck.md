---
title: GoSX · Shader lab
theme: aurora
scene: shaders/aurora.sel
footer: GoSX SLIDES · SHADER LAB
---

```yaml
layout: title
```

<p class="eyebrow">LIGHT / SHAPE / IDEAS</p>

# Make ideas<br>feel alive.

Custom shaders. Native scenes. One slide deck.

<!-- Open with the shader background. It is shared across slides. -->

---

```yaml
scene: false
class: specimen
```

<p class="eyebrow">01 / MATERIALS</p>

# A shape with a point of view.

<Shader Src="shaders/pearl.sel" Shape="torus" Label="A rotating mint and violet striped torus" />

Selena compiles once to GLSL + WGSL. Drag to explore.

<!-- The torus uses the GoSX renderer and material compiler. -->

---

```yaml
scene: false
class: specimen
```

<p class="eyebrow">02 / DIAGRAMS</p>

# Show the transformation.

<Scene3D Src="scenes/network.json" Shader="shaders/pearl.sel" Targets="transform" Label="Source flows through a striped transformation into output" />

<div class="diagram-labels"><span>SOURCE</span><span>TRANSFORM</span><span>OUTPUT</span></div>

<!-- Only the transform node receives the custom shader; links stay neutral. -->

---

```yaml
layout: center
```

<p class="eyebrow">03 / COMPOSITION</p>

# Your renderer.<br>Your language.

Shapes, GLB models, particles, lights, animation, and post effects use the GoSX Scene3D contract.

<!-- Bring native GoSX scene JSON into a deck. No CDN or alternate renderer. -->

---

```yaml
layout: center
scene: false
```

<p class="eyebrow">BUILD SOMETHING BEAUTIFUL</p>

# Small syntax.<br>Big canvas.

```gosx
<Shader Src="shaders/pearl.sel" Shape="sphere" />
<Scene3D Src="scenes/network.json" />
```

Use ← →, swipe, or the controls below. Press B to dim the room.

<!-- Finish on the authoring syntax. -->

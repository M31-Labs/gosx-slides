---
title: GoSX · Motion lab
theme: aurora
transition-duration: 500
transition-easing: ease-out
footer: GoSX SLIDES · MOTION LAB
---

```yaml
layout: center
```

# Move with purpose.

:::motion {preset=slide-up duration=650 delay=100 distance=30}
## Give the idea room to arrive.

Native motion. Readable content. Your timing.
:::

<!-- Arrow right shows a second entrance with its own slide timing. -->

---

```yaml
layout: center
transition-duration: 250ms
transition-delay: 50ms
transition-easing: linear
reveal: true
```

# One word at a time.

:::motion {preset=fade split=word stagger=90 duration=400 replay=step}
Let each word carry its weight.
:::

- First beat
- Second beat

:::motion {preset=fade duration=400 replay=once}
This entrance plays once.
:::

<!-- GoSX handles repeated entry, step motion and reduced-motion preferences. -->

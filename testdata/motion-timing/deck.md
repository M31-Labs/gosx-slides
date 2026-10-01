---
title: Timing accuracy
transition: none
---

# One dependable beat

:::motion {cue=beat after=beat delay=20 duration=200}
The follower is authored before its lead.
:::

:::motion {cue=beat delay=30 duration=100}
The lead finishes first.
:::

:::motion {group=pair stagger=40 delay=10 duration=80}
First group member.
:::

:::motion {group=pair stagger=40 delay=10 duration=80}
Second group member.
:::

:::motion {cue=cycle_a step=0 after=cycle_b delay=5 duration=50}
Cycle A.
:::

:::motion {cue=cycle_b step=0 after=cycle_a delay=15 duration=60}
Cycle B.
:::

:::motion {step=0 after=cycle_a delay=10 duration=40}
Downstream of the cycle.
:::

:::motion {step=0 after=absent delay=7 duration=40}
Missing cue.
:::

:::motion {cue=other step=2 duration=80}
A different click step.
:::

:::motion {step=0 after=other delay=9 duration=40}
Cross-step dependency.
:::

<div data-slides-motion-replay="slide" data-slides-motion-step="0" data-gosx-motion-preset="slide-up" data-gosx-motion-distance="0" data-gosx-motion-duration="10.6" data-gosx-motion-delay="2.6">Zero distance and fractional timing.</div>

:::motion {split=word stagger=40 duration=120 delay=10}
One clear idea
:::

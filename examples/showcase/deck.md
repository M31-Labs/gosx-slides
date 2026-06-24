---
title: gosx-slides
theme: aurora
---

```yaml
layout: title
```

# gosx-slides

Beautiful, live presentations — compiled, not templated.

<!-- Title slide. Press the right arrow to begin; press p for the presenter view. -->

---

# The real lane

This is the **real lane** — the prose you are reading is static server HTML.

Deck title via an expression: {deck.title}. Uppercased: {strings.ToUpper("live gosx")}.

Use the arrow keys (← / →) or Space to move between slides. Press `f` for fullscreen, `o` for an overview of every slide, or `p` to open the presenter view (current + next previews, speaker notes, an elapsed timer, and a slide counter — synced peer-to-peer with this window).

> Themes and layouts are selected entirely from deck frontmatter.

<!-- Open the presenter view with `p` (or load this page with ?present). The two windows stay in lockstep over a BroadcastChannel — no server. -->

---

# A live GoSX island

The counter below is a genuine GoSX component, compiled to island bytecode and
hydrated in your browser — not a screenshot:

<Counter Initial={3}/>

Click the buttons — the count is real reactive state.

<Notes>
Demo beat: click the counter live so the audience sees real reactive state, not a screenshot. Then mention it is the same island bytecode `gosx dev` hot-swaps.
</Notes>

---

# Evaluated expressions

Inline `{expr}` is evaluated server-side by the GoSX compiler:

- two plus three is {2 + 3}
- six times seven is {6 * 7}
- this is slide number {slide.index}

```go {2-3|6}
// Fenced code blocks are syntax-highlighted, per theme.
func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
```

The same compiler that type-checks the island evaluates these expressions. A
fence can spotlight specific lines with `{2-3|6}` meta — the rest dim.

<Notes>
Press the right arrow on this slide: the spotlight steps through the fence meta, the same step engine as the reveal on the next slide.
</Notes>

---

```yaml
reveal: true
```

# Incremental reveal

Add `reveal: true` to a slide's frontmatter and its bullets arrive one press at a
time — the first is on screen as you land, each `→` brings the next:

- First, the problem lands on its own.
- Then the consequence, once it has had a moment to sit.
- Finally the fix, with the room already leaning in.

`←` walks them back; overview (`o`) and print show every point at once.

<!-- Demo: advance one bullet at a time so the room stays with you instead of reading ahead. Reduced-motion users get the reveal without the fade. -->

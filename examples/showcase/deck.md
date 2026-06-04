---
title: gosx-slides
theme: aurora
---

```yaml
layout: title
```

# gosx-slides

Beautiful, live presentations — compiled, not templated.

---

# The real lane

This is the **real lane** — the prose you are reading is static server HTML.

Deck title via an expression: {deck.title}. Uppercased: {strings.ToUpper("live gosx")}.

Use the arrow keys (← / →) or Space to move between slides. Press `f` for fullscreen, or `o` for an overview of every slide.

> Themes and layouts are selected entirely from deck frontmatter.

---

# A live GoSX island

The counter below is a genuine GoSX component, compiled to island bytecode and
hydrated in your browser — not a screenshot:

<Counter Initial={3}/>

Click the buttons — the count is real reactive state.

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

---
title: gosx-slides
---

# gosx-slides

This is the **real lane** — the prose you are reading is static server HTML.

Deck title via an expression: {deck.title}. Uppercased: {strings.ToUpper("live gosx")}.

Use the arrow keys (← / →) or Space to move between slides. Press `f` for fullscreen.

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

The same compiler that type-checks the island above evaluates these.

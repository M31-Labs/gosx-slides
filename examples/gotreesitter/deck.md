---
title: GoTreeSitter Without CGo
theme: neon
---

```yaml
layout: title
```

# GoTreeSitter Without CGo

Pure-Go tree-sitter parsing, GLR pressure, and the work it takes to make parity boring.

Pure Go · cross-compilable · byte-for-byte C parity

<!-- Press → / Space to advance, o for an overview of every slide, p for the
presenter view. The code slides step through one reveal at a time, and the two
charts are live GoSX islands — click them. -->

---

# The constraint

tree-sitter gives editors and agents a fast, incremental, multi-language parse. The catch is **CGo**: a C compiler on every build, on every target.

**gotreesitter** keeps tree-sitter's semantics and drops the C dependency:

- A pure-Go LR + GLR runtime — `GOOS=js`, `GOOS=windows`, anything Go targets
- The grammar's own tables, carried forward — not a reimplementation
- The bar is **byte-for-byte parity** with tree-sitter C, not "close enough"

<Notes>
Frame the talk around the engineering constraint, not novelty: tree-sitter
behavior without the C toolchain. The goal is to make parser parity boring —
so everything downstream can just depend on it.
</Notes>

---

# The pipeline

1. **Grammar blob** — `parser.c` tables + scanner hooks, extracted to a Go registry
2. **Pure-Go parser** — the LR + GLR runtime, no C on the critical path
3. **Forest** — ambiguity under control, forks tracked
4. **Tree** — compatibility normalization to the tree-sitter shape
5. **Queries** — highlights, tags, captures
6. **Consumers** — editors and agents

> gotreesitter doesn't replace the grammar ecosystem. It carries the tables and semantics into a pure-Go runtime.

<Notes>
The hard part is preserving tree-sitter behavior while changing the runtime
beneath it. Nothing here invents a new grammar format.
</Notes>

---

# What the parser produces

A parser talk needs a tree on screen early. This is the tree-sitter node shape for `func main(){ fmt.Println("hello") }` — **click a node to expand it**:

<ParseTree/>

```go
package main

func main() {
    fmt.Println("hello")
}
```

<Notes>
Keep it concrete. This is a live island — the same node names a query would
capture, expandable in the browser. Click call_expression to open the
fmt.Println shape.
</Notes>

---

# Queries, not just trees

The tree is the substrate; **queries** are what editors and agents consume. A capture pattern, matched against the tree above:

```go
package main

func main() {
    fmt.Println("hello")
}
```

```scheme
(function_declaration
  name: (identifier) @fn)

(call_expression
  function: (selector_expression
    field: (field_identifier) @call))
```

`@fn` captures `main`; `@call` captures `Println`. Highlights, tags, and agent context all fall out of captures like these.

<Notes>
This is where the runtime becomes useful. The parser is plumbing; captures are
the product surface.
</Notes>

---

```yaml
layout: center
```

# The GLR lesson

Fork count is the **first-order** performance lever. Collapsing forks that C resolves deterministically removes work at **zero parity risk** — C itself proves the action is correct. Click **Collapse forks**:

<Benchmark Before={5.95} After={4.41} Baseline={1.00}/>

JavaScript, Go-vs-C wall ratio — `5.95x → 4.41x` by collapsing forks (PR #90).

<Notes>
The money slide. Each live GLR stack reruns the per-token action machinery, so
surviving forks multiply work. This attacks fork COUNT; per-fork cost is a
separate axis. Click the button live.
</Notes>

---

# The methodology

A repeatable workflow, not a one-off. Step through it:

```go {1-2|4-6|8-9|11}
// 1. find the hot ambiguities, sorted by fork count
//    run_parse_gap_report.sh --langs js --hot-shapes 30

// 2. decode the dominant state's actions:
//    a reduce  <X>_repeat1  vs a repetition shift (rep=true)
//    measure fork CONCENTRATION first — stop at the knee

// 3. VERIFY against tree-sitter C's parser.c — trace the goto chain

// 4. extend only verified (state, lookahead) entries, shape-guarded
//    so other grammars' identically-numbered states are untouched
```

<Notes>
Concentration first: a handful of states usually own most forks. The dispatch
is language-gated and fires only on {repetition-shift, reduce}, so it can't
bleed into other grammars.
</Notes>

---

# The safety criterion

The naive rule is **wrong**. The real one is subtle — reveal it:

```go {2-4|6-9|11-12}
func choose(state int, lookahead Symbol) Action {
    // naive: "resolve to the shift when C does a count=1 SHIFT."
    // WRONG — there are ZERO pure count=1 SHIFT_REPEAT cells in JS parser.c.

    // correct: shift only when the reduce is a ZERO-PROGRESS DEAD-END —
    // taking it gotos back into the SAME conflict on the SAME unconsumed
    // lookahead (an infinite loop), so the shift is the only action that
    // actually consumes a token. Verify by tracing parser.c's goto chain.
    if reduceGotoChain(state).loopsBackTo(state, lookahead) {
        return shift // exactly what C's runtime does on a continuation token
    }
    return reduce
}
```

State `1421` (`object_pattern_repeat1`) *looked* entangled with the declared `[object, object_pattern]` conflict — a goto-chain trace proved the `,` is a strictly internal separator.

<Notes>
This is why the resolution matches C at runtime: C always shifts on a
continuation token because its reduce makes no progress. The whole method
hinges on verifying that dead-end, not pattern-matching the cell.
</Notes>

---

# The transfer limit

The shape is seductive. It does **not** blindly transfer:

```go {1|3-5|7-8}
// Python: states 72 (module_repeat1) and 2309 (_collection_elements_repeat1)

// matched the JS shape textbook-perfectly. So we added
//   pythonRepetitionShiftConflictChoice
// ...and turned a clean parse of python3.8_grammar.py into root=ERROR.

// Python has no ASI. NEWLINE/INDENT externals drive its boundaries, so the
// repeat-reduce is GENUINELY required — not a zero-progress dead-end.
```

> Always verify per state against `parser.c`. Never assume the shape transfers.

<Notes>
The failure is the lesson. The method is "verify the dead-end against C,"
not "find states that look like the JS ones." Python is the counter-example
that proves it.
</Notes>

---

# Why profiling got weird

Ordinary wall-time profiles weren't enough — they name functions, not parser **phases**. Attribution had to grow to ~80 buckets:

- Constructed vs final node volume
- Arena / checkpoint / reduction-transient storage
- Final tree materialization
- Normalization timing
- GLR collapse behavior

> Optimization needed attribution to phases, not function names. The profiling surface grew because the hot path was otherwise opaque.

<Notes>
Hyphae: parse-profile-attribution. You can't optimize fork shape if your
profiler can only see runtime.mallocgc.
</Notes>

---

# GLR materialization v2

The architecture pivot: stop allocating final nodes early for forks that may die.

1. **Compact full leaves** — known-final subtrees, materialized now
2. **Pending parents** — ambiguous structure, held back
3. **Forest collapse** — defer materialization until the fork resolves
4. **Post-pass** — compatibility normalization, once

> v2 separates known-final leaves from pending parents and delays normalization until after collapse.

<Notes>
Hyphae: glr-materialization-v2. This attacks per-fork COST — the complement of
fork-count reduction. Both axes, not one.
</Notes>

---

```yaml
layout: center
```

# Parity is a matrix

Not a single boolean — these surfaces move together, green at once or it isn't parity:

- **Go grammar** — pass
- **JavaScript GLR** — `4.41x` · pass
- **Python** external scanner — `1.12x` · pass
- **Markdown** injections — stable
- **Query captures** — pass

<Notes>
Keep the room honest. Parity is language coverage, external scanners,
injections, captures, and corpus behavior — all green at once, or it isn't
parity.
</Notes>

---

```yaml
layout: center
```

> The right abstraction is not "a faster parser." It is a parser runtime that can explain its own shape.

<Notes>
The thesis slide before the close. The profiling surface and the GLR work are
the same idea: a runtime that is legible to the people optimizing it.
</Notes>

---

```yaml
layout: title
```

# Make parity boring

When parity is boring, language tooling becomes **portable infrastructure**:

Portable parsing · editor features without CGo · agent-visible syntax trees · deterministic query tooling · safer grammar evolution

<Notes>
Close on portability and discipline. The interesting product is everything
that gets to depend on a pure-Go parser substrate — and stop thinking about it.
</Notes>

---
title: GoTreeSitter Without CGo
theme: noir
transition: slide
---

# GoTreeSitter Without CGo

Pure-Go parsing, GLR pressure, and the work it takes to make parity boring.

<Metrics>
<Metric label="Runtime" value="pure Go" delta="cross-compilable"/>
<Metric label="Target" value="C parity" delta="byte-for-byte trees"/>
<Metric label="Hot path" value="GLR" delta="fork count first"/>
</Metrics>

<Checkpoint id="opening-frame" label="Opening frame"/>

<Notes>
Frame the talk around the engineering constraint: tree-sitter semantics without
the C dependency. The goal is not novelty; it is making parser parity boring.
</Notes>

---
layout: center
---

## Run of Show

<Agenda/>

<Notes>
Use the generated agenda to set expectations: architecture first, then the
performance story, then what this unlocks.
</Notes>

---
layout: two-cols
clicks: 2
---

## The Pipeline

<Pipeline steps="Grammar blob: extracted tables|Pure-Go parser: LR + GLR runtime|Forest: ambiguity control|Tree: compatibility normalization|Queries: highlights, tags, captures|Consumers: editors and agents"/>

::right::

<GrammarBlob steps="parser.c tables|scanner hooks|grammar blob|Go registry"/>

<Step n={1}>No C compiler on the critical path.</Step>

<Step n={2}>The hard part is preserving tree-sitter behavior while changing the runtime beneath it.</Step>

<Notes>
Explain that gotreesitter does not replace the grammar ecosystem. It carries
the tables and semantics forward into a pure-Go runtime.
</Notes>

---
layout: image-right
---

## What the Parser Produces

<ParseTree root="source_file" tree="package_clause>package,package_identifier;function_declaration>func,identifier,parameter_list,block;call_expression>selector_expression,argument_list"/>

::image::

```go {1|3|4|all}
package main

func main() {
    fmt.Println("hello")
}
```

<Notes>
Keep this concrete. A parser talk needs a tree on screen early so the rest of
the deck has an object to refer back to.
</Notes>

---
layout: default
---

## Query Behavior, Not Just Trees

<QueryDemo lang="go" title="Capture a function declaration" captures="@fn main|@call Println">
```go
package main

func main() {
    fmt.Println("hello")
}
```
```query
(function_declaration
  name: (identifier) @fn)

(call_expression
  function: (selector_expression
    field: (field_identifier) @call))
```
</QueryDemo>

<Checkpoint id="query-demo" label="Query capture demo"/>

<Notes>
This is where the runtime becomes useful to users. The parser is only the
substrate; queries, captures, highlights, and tags are what editors and agents
actually consume.
</Notes>

---
layout: center
---

## The GLR Lesson

<Takeaway>Fork count is the first-order performance lever; per-fork cost comes after the fork shape is under control.</Takeaway>

<Benchmark title="JavaScript fork reduction" values="Before 5.95x|After 4.41x|C baseline 1.00x"/>

<Citation label="Hyphae: glr-fork-reduction" href="hypha://m31labs/gotreesitter/object/concept.glr-fork-reduction"/>

<Notes>
This is the core performance story from Hyphae: JS moved from 5.95x to 4.41x
relative to C by collapsing forks that C proves deterministic.
</Notes>

---
layout: two-cols
---

## Why Profiling Had to Get Weird

- Constructed vs final node volume
- Arena and checkpoint storage
- Reduction/transient storage
- Final tree materialization
- Normalization timing
- GLR collapse behavior

::right::

<ProfileBuckets title="Parse profile attribution" buckets="construct nodes 39|arena storage 22|reductions 17|materialization 14|normalization 8"/>

<Callout tone="gold">The profiling surface grew to roughly 80 buckets because the parser hot path was otherwise opaque.</Callout>

<Citation label="Hyphae: parse-profile-attribution" href="hypha://m31labs/gotreesitter/object/concept.parse-profile-attribution"/>

<Notes>
Explain why ordinary wall-time profiles were not enough. Optimization needed
attribution to parser phases, not just function names.
</Notes>

---
layout: center
---

## GLR Materialization v2

<Pipeline steps="Compact full leaves: known-final subtrees|Pending parents: ambiguous structure|Forest collapse: defer materialization|Post-pass: compatibility normalization"/>

<GrammarBlob steps="known-final leaves|pending parents|collapse forest|normalize once"/>

<Takeaway>v2 stops allocating final nodes early for forks that may die.</Takeaway>

<Citation label="Hyphae: glr-materialization-v2" href="hypha://m31labs/gotreesitter/object/concept.glr-materialization-v2"/>

<Notes>
Use this as the architecture pivot. v2 separates known-final leaves from pending
parents and delays normalization until after collapse.
</Notes>

---
layout: two-cols
---

## Parity Is a Matrix

<ParityMatrix rows="Go grammar pass|JavaScript GLR watch|Python external scanner pass|Haskell conflicts watch|Markdown injections pass|Query captures pass"/>

::right::

<CorpusRun title="Corpus smoke run" rows="JavaScript corpus 4.41x pass|Go stdlib 1.02x pass|Python samples 1.12x pass|Injection fixtures stable pass"/>

<Citation href="hypha://m31labs/gotreesitter/object/concept.glr-fork-reduction"/>

<Notes>
Keep the room honest that parity is not a single boolean. It is language
coverage, external scanners, injections, query captures, and corpus behavior
moving together.
</Notes>

---
layout: quote
---

> The right abstraction is not "a faster parser." It is a parser runtime that can explain its own shape.

<Notes>
This is the thesis slide before the close.
</Notes>

---
layout: center
---

## What This Unlocks

<Timeline items="Portable parsing|Editor features without CGo|Agent-visible syntax trees|Deterministic query tooling|Safer grammar evolution"/>

<Poll question="Which gotreesitter surface should we demo live?" options="Query captures|Incremental parse|GLR profile|Grammar generation"/>

<Notes>
Use the poll if the room is interactive. Otherwise, treat it as a prompt for
the next demo segment.
</Notes>

---
layout: cover
---

# Make Parity Boring

<Takeaway>Once parity is boring, language tooling becomes portable infrastructure.</Takeaway>

<Notes>
Close on portability and discipline: the interesting product is everything that
gets to depend on a pure-Go parser substrate.
</Notes>

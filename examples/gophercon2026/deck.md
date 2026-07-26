---
title: "Pure-Go Tree-sitter"
theme: aurora
aspect-ratio: 16:9
caption-safe-bottom: 20%
duration-minutes: 25
offline-required: true
scene: m31-starfield
---

```yaml
layout: title
class: copy-tight
```

# Pure-Go Tree-sitter

Rebuilding the runtime, proving behavioral parity, and making grammars in Go.

Oscar Villavicencio · M31 Labs · GopherCon 2026

<Notes>
0:00. Most parser demonstrations begin with valid code. That is a little
dishonest. The moment our tooling earns its keep is when the program is half
typed, half wrong, and still moving.
Today I want to make three claims. First, a Tree-sitter runtime can be worth
rebuilding in pure Go even when a full parse costs more. Second, a
reimplementation does not have to ask you to trust the reimplementer. It can
be checked against an independent executable answer. And third, once grammar
generation is in Go too, this stops being merely a port. It becomes a language
toolchain that can carry existing grammars and create entirely new ones.
This is the story of gotreesitter, grammargen, and what happened after making
a parser became inexpensive enough to do more than once.
</Notes>

---

```yaml
class: m31-intro
```

# I build tools that see structure

<style>
.m31-layout { display: grid !important; grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.05fr); gap: 2.2rem; align-items: center; }
.m31-copy { display: flex; flex-direction: column; gap: 0.9rem; }
.m31-copy p { margin: 0; }
.m31-art { min-width: 0; }
.m31-og { width: 100%; max-height: 44vh; object-fit: cover; border-radius: 0.75rem; box-shadow: 0 1.2rem 3rem rgba(0,0,0,0.35); }
.m31-og-static { display: none !important; }
@media print {
  .m31-og-live { display: none !important; }
  .m31-og-static { display: block !important; }
}
@media (prefers-reduced-motion: reduce) {
  .m31-og-live { display: none !important; }
  .m31-og-static { display: block !important; }
}
</style>

<div class="m31-layout">
  <div class="m31-copy">
    <p>Oscar Villavicencio · founder, <strong>M31 Labs</strong></p>
    <p><strong>People, tools, and agents should be able to work from the same structure.</strong></p>
    <p>M31 Labs is where I build that thesis. gotreesitter made it concrete.</p>
  </div>
  <div class="m31-art">
    <img class="m31-og m31-og-live" src="/public/m31labs-og.gif" alt="Animated M31 Labs particle galaxy" />
    <img class="m31-og m31-og-static" src="/public/m31labs-og.png" alt="M31 Labs particle galaxy" />
  </div>
</div>

<Notes>
~0:50. I am Oscar. I founded M31 Labs to build tools that understand the
structure of software, so people, conventional tools, and software agents can
work from the same source of truth. That is the M31 Labs thesis. gotreesitter
made it concrete.
Before any tool can explain, navigate, rewrite, or reason about code, it first
has to keep understanding that code while we are still changing it.
</Notes>

---

# Code does not wait until it is valid

```go
func Handle(req *http.Request) {
    result :=
```

The editor still has to know:

**What is this?** · **What am I inside?** · **What belongs here?** · **What changed?**

<Notes>
~1:25. Consider this exact keystroke. A batch compiler is allowed to reject
this program. An editor is not allowed to become useless.
Syntax highlighting still has to color the function. Navigation still has to
know that I am inside Handle. Completion needs to understand what may follow
the assignment. A refactoring tool needs to avoid damaging the rest of the
file. And after the next keystroke, the editor should not have to rediscover
the whole program from scratch.
Those are the conditions under which Tree-sitter becomes interesting.
</Notes>

---

```yaml
class: copy-tight
```

# Tree-sitter keeps broken code useful

Tree-sitter is a parser generator and runtime for tools that read changing code.

- Concrete trees keep every source detail.
- Recovery keeps incomplete code queryable.
- Incremental parsing reuses what did not change.
- Queries name the structures tools care about.

**Its contract is usefulness before validity.**

<Notes>
~2:10. Tree-sitter produces a concrete syntax tree. Unlike a compiler AST,
whose job is often to discard punctuation and normalize several forms into one
semantic representation, this tree stays close to the source. It keeps
punctuation, byte ranges, delimiters, and the distinctions editing tools need.
When the input is incomplete, the parser records uncertainty instead of
abandoning the tree. When an edit arrives, it can reuse portions of the
previous tree. And its query system gives tools a way to say: find function
names, imported modules, or the language embedded inside this string.
Its most important promise is not, “This program is valid.” It is, “Here is
the structure I can still defend.”
</Notes>

---

```yaml
fallback: static
class: copy-tight
```

# Even broken code has a useful shape

The tree remains queryable while the program is incomplete. **Select a node:**

<div class="tree-legend"><code>ERROR</code> = source the parser could not place · <code>MISSING</code> = expected syntax that is absent</div>

<ParseTree/>

<Notes>
~3:10. Let us make that concrete.
[Select the function declaration.] The body is incomplete, but the outer
function_declaration still exists. It has a type, an exact byte range, and
children we can inspect. The parser has not reduced the whole file to “syntax
error.”
[Select the incomplete expression and its recovery node.] Here the uncertainty
becomes visible. An ERROR node contains source the parser could not fit into
the expected structure. A MISSING node is different: it is a zero-width record
that expected syntax is absent. A tool can react differently to malformed
source and syntax that simply has not been typed yet.
[Complete the expression, then make one small edit.] The recovery node
disappears and a more specific expression takes its place. Incremental parsing
combines the edit, the old tree, and the new source, reconsidering the affected
path without treating the rest of the file as a new discovery.
Syntax highlighting is only hello world for this tree. Navigation, indexing,
selection, rewriting, refactoring, and language injection all depend on the
precise observable shape of the result. That exact shape is the behavior a
reimplementation has to preserve.
</Notes>

---

```yaml
class: copy-tight
```

# Why rebuild a runtime that already works?

- **One binary** wherever Go runs.
- **Cross-compile normally** without target-specific C toolchains.
- **Profile, fuzz, cover, and race-check** the parser in Go.
- **Keep grammars in process** as Go data.

The point is not to escape C. It is to remove a product boundary.

<Notes>
~5:40. CGo is not the villain in this talk. It is a perfectly sensible way to
use Tree-sitter, and for many products it is the correct choice.
But it creates a system boundary. Every command-line tool, server image,
plugin host, cross-build, and WASM target inherits some combination of a C
compiler, target libraries, ABI assumptions, and ownership rules.
I wanted Go's ordinary build story all the way down. I wanted Go's profiler,
fuzzer, coverage tools, and race detector to see the parser itself—not only the
wrapper around it. And I wanted grammars to be ordinary in-process Go data.
The goal was not ideological purity. It was to make the parser part of the Go
program instead of a dependency sitting immediately beneath it.
</Notes>

---

# The cost does not disappear. It moves.

Pure Go may spend more time on a full parse.

CGo moves complexity to the build, ABI, ownership, and deployment boundaries.

**Benchmark the product you need to ship, not only the parser call.**

<Notes>
~6:40. There is no free lunch hiding behind the word “pure.” I do not have a
slide where every performance bar points in my direction. That would be
marketing, not engineering.
Some full parses cost more wall-clock time in pure Go. That is real, and it
belongs in the decision. But CGo has a bill too. It arrives in cross-toolchains,
deployment matrices, memory ownership, debugging boundaries, and targets where
the foreign runtime is difficult or impossible to carry. Incremental workloads
may also have a different shape from cold full parses.
The useful question is not which function call wins in isolation. It is: what
is the cost of the product workload, on every target where this product must
run? A fast parser that cannot comfortably reach the deployment target is not
necessarily the faster system.
</Notes>

---

```yaml
class: copy-tight
```

# 206 grammar packages now ship like Go

**One runtime · One Go API · No CGo in the product**

GLR · recovery · queries · highlights · tags · injections · incremental edits

```go
lang := grammars.GoLanguage()
tree, err := gotreesitter.NewParser(lang).Parse(source)
```

*Coverage is not the same as identical maturity.*

<Notes>
~7:35. The current result is one Go runtime serving 206 grammar packages.
Those are not 206 handwritten parser ports. The grammar-specific portion is
generated data, and the same runtime executes it.
A consumer imports a package, gets a language value, and parses through one Go
API. The runtime supports generalized parsing, recovery, queries, highlights,
tags, injections, and incremental edits.
That number is an ecosystem-coverage claim, not a claim that every grammar has
identical maturity or an equally deep test corpus. Getting a grammar to load
is not the same thing as knowing its trees are correct. That led to the harder
question: how could I know the new runtime was producing the right answer?
</Notes>

---

```yaml
layout: section
```

# The original runtime became an oracle

**Same question. Independent execution. Comparable answer.**

<Notes>
~8:25. Reimplementations are often checked against a written specification. I
had something more concrete: the original C runtime could execute the exact
same grammar against the exact same source and answer the exact same structural
question. That made it a behavioral oracle.
I use the word “prove” operationally here, not in the theorem-prover sense. For
a pinned grammar revision, a specific byte sequence, and a specific edit
history, two separate runtime implementations must agree on the observable
result.
That does not prove agreement over every input anyone could construct. It gives
us repeatable, falsifiable evidence over a growing corpus—and when the
implementations disagree, a concrete counterexample. The C runtime is the
compatibility target. Its most valuable property is that it can disagree with
me.
</Notes>

---

```yaml
class: copy-tight
```

# CGo stayed in the laboratory

```text
                same grammar · same source · same edit
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
             C Tree-sitter       Pure-Go runtime
                    └─────────┬─────────┘
                              ▼
                      structural comparison
```

**Product:** pure Go · **Reference tests:** CGo, isolated in a separate module

<Notes>
~9:25. The reference tests live in a separate test module. Both runtimes
receive the same grammar revision, the same source bytes, and—when testing
incremental behavior—the same edit sequence. Then their answers are compared.
The product does not call into C. The laboratory does, deliberately. The
deployment constraint and the verification strategy do not have to be
identical. CGo is removed from the product boundary while remaining available
where it provides the strongest independent evidence. I did not want to throw
away the C implementation. I wanted to quarantine it somewhere it could be
maximally useful.
</Notes>

---

```yaml
class: copy-tight
```

# Parity hides in the details

For every node: symbol · byte range · named / missing / error · ordered children

Then queries, highlights, tags, injections, and incremental results.

```text
real source → structural difference → minimal witness → permanent test
```

**A plausible tree can still be wrong. Every fixed bug leaves a witness.**

<Notes>
~10:15. A root node named program is not parity. The two implementations walk
the tree in lockstep: symbol, byte range, named, missing, and error state, and
the order of children. Those are byte ranges, not rune counts. A single UTF-8
boundary mistake is a real behavioral difference. We also compare query-facing
behavior: captures, highlights, tags, injections, and the result after
incremental edits. When a real file exposes a difference, the first job is to
preserve it. Then we shrink the input until the reason becomes legible. Pinned
open-source files keep us honest on programs nobody designed to flatter the
parser. Deliberately invalid files exercise recovery, because malformed input
is not an edge case in an editor. Every fixed failure becomes a permanent
witness—a small museum of parser misunderstandings we never have to
rediscover.
</Notes>

---

# AI raises throughput. The reference supplies evidence.

**No change supplies its own evidence.**

Every human or agent change passes the same independent comparison.

<Notes>
~11:25. Once that proof loop existed, AI could safely increase the rate of
change. It became possible to produce, inspect, and revise a large amount of
parser code very quickly.
But faster code production is not stronger evidence. It can also be a way to
become wrong at impressive speed. The model is not the judge, and I am not the
judge. A model reviewing code it helped produce is not an independent
verification strategy. Human confidence is not one either.
The evidence comes from outside the process that proposed the change: the
reference runtime, the locked corpus, and the permanent witnesses. AI expands
the search. The parity gate decides which results survive. Every change—human
or agent—enters through exactly the same door.
</Notes>

---

```yaml
class: copy-tight
```

# First prove it. Then measure it.

**Does the structure match?**

C-reference parity · focused witnesses · one grammar at a time

**What does that structure cost?**

full parse · single-byte incremental edit · no-edit incremental parse

`GOMAXPROCS=1` · repeated samples · allocations · peak RSS (resident set size)

<Notes>
~12:15. Correctness and performance are different experiments. A benchmark of
the wrong tree is just a fast bug.
First, the implementation has to pass the structural gate against the C
reference, one grammar at a time. Different languages stress different lexer
states, conflicts, recovery paths, and parse-table shapes. Only then do we
measure cost.
A full parse measures the cold path. A single-byte edit measures an editor-like
workload: update the source, edit the old tree, and parse with reuse. A no-edit
incremental parse measures fixed orchestration cost when reuse has its greatest
opportunity. The settings stay stable: one logical processor, repeated samples,
allocation data, and process-level peak memory—not only the Go heap.
This separation has paid for itself. At one point an optimization preserved
the expected JavaScript output and turned Python into a root ERROR. A blended
throughput number could have hidden that. The correctness gate did exactly what
it was designed to do.
</Notes>

---

```yaml
layout: section
```

# Then the parser learned to make parsers

Running generated grammars was only half a toolchain.

<Notes>
~13:35. At this stage gotreesitter could execute grammar tables, but it could
not create them. The runtime was pure Go, yet production of those tables still
depended on an external generator.
That was a workable bootstrap, but it left an important boundary in place. A
runtime can consume a language. A toolchain can define one. grammargen is where
the project crossed that line.
</Notes>

---

```yaml
class: copy-tight
```

# Two roads produce one runtime grammar

```text
upstream parser.c ── ts2go ─────┐
                                ├─ grammar blob ─ one runtime
grammar source ──── grammargen ─┘
```

**Bootstrap road:** carry existing grammars into Go

**Native road:** create grammars without a C ancestor

Different producers. Same blob. Same runtime. Same parity gate.

<Notes>
~14:20. There are two roads into the same runtime representation. ts2go starts
with an upstream generated parser.c. It extracts the grammar tables and carries
them into Go. That brought the existing Tree-sitter ecosystem across without
first recreating the entire generator.
grammargen starts earlier. It consumes the grammar description and constructs
the tables in Go. Both roads emit the same grammar blob, and the runtime does
not need to know which producer created it.
This gives the project a controlled migration path. A grammar can begin on the
bootstrap road through ts2go. Its native grammargen output can then be compared
against the reference behavior. It moves to the native road only after parity
agrees. The old ecosystem remains available while the new toolchain proves
itself grammar by grammar.
</Notes>

---

# grammargen turns syntax into tables

```text
grammar DSL · grammar.json · .grammar
                    │
                    ▼
        normalize → NFA → DFA → LALR(1)
                    │        + LR(1) splitting / GLR
                    ▼
              runtime grammar blob
```

**High-level syntax in. Executable parser data out.**

<Notes>
~15:25. This slide contains enough acronyms for a small compiler course, so
here is the practical version.
A grammar author begins with sequences, alternatives, repetition, precedence,
associativity, and declared conflicts. Normalization removes that surface
shorthand and converts it into smaller, explicit rules the generator can
analyze.
For lexing, an NFA represents the paths by which characters may form tokens. A
DFA determinizes those possibilities into efficient transitions. For parsing,
the generator builds states that answer: given the current state and the next
token, should the parser shift, reduce, accept, or report a conflict?
LALR state merging keeps the tables compact. When merging loses context that a
grammar genuinely needs, LR(1) splitting restores more precise states. When
ambiguity is intentional, GLR preserves multiple legal paths until there is
enough evidence to choose.
The compiler emits executable grammar data. At runtime there is no grammar DSL
to interpret and no C generator to call. There is one blob and one engine that
knows how to execute it.
</Notes>

---

```yaml
class: copy-tight copy-tighter
```

# Inexpensive grammars made new languages practical

M31 Labs now uses the same mechanism across several domain-specific languages (DSLs).

**Authoring and interface**

GoSX · GoSX Native · Markdown++ · Sirena

**Policy and workflow**

Arbiter · Danmuji · Horizon · blockchain-lang

**Systems and compute**

Selena · Eos · Fyx/Fyrox · Ferrous Wheel

**Different maturity levels. One grammar mechanism.**

<Notes>
~16:50. Once grammar creation became an in-process, testable operation, M31
Labs kept finding places to use it. Almost every one of these began the same
way: as a language hiding inside our own tools—in strings, templates, and
conventions—before it had a grammar. Please do not try to memorize this
slide. The point is the spread.
Some of these languages describe interfaces and documents. Some encode policy
or workflow. Some explore systems and compute. They are not all equally mature,
and this is not a claim that every name represents a finished production
language.
The important result is that a new language no longer has to begin with an ad
hoc parser and a promise to build tooling later. It can begin on the same
structural substrate: recovery, incremental parsing, queries, and a testable
tree contract. “Inexpensive” does not mean free. It means the marginal cost is
low enough that domain-specific syntax becomes a practical design option rather
than a research project.
</Notes>

---

```yaml
class: gosx-inline
```

# One file. One tree. Go and markup together.

**`card.gsx`**

```gosx
type CardProps struct {
    Title string
    Saved bool
}

func actionLabel(saved bool) string {
    if saved { return "Saved" }
    return "Save"
}

func Card(props CardProps) Node {
    return <article class="card">
        <h2>{props.Title}</h2>
        <button disabled={props.Saved}>
            {actionLabel(props.Saved)}
        </button>
    </article>
}
```

**The markup is not a string. The Go is not glue. One parser sees both.**

<Notes>
~17:55. The filename matters. This is one card.gsx file. CardProps and
actionLabel are ordinary Go declarations. The component invokes that Go
function directly inside its markup.
In many template systems, the host language constructs some data, a second
parser interprets a string or template file, and tooling has to reconstruct the
seam between them. GoSX composes the markup into the Go grammar. The markup is
represented by real nodes, the surrounding Go is represented by real nodes,
and one parse tree spans the boundary.
A structural tool can understand where a Go expression enters markup instead
of treating the transition as opaque interpolation. The component then lowers
back into ordinary Go. This is not a separate template language wearing a
Go-shaped API. Notice what disappears when the language boundary disappears:
less glue, fewer coordinate systems, and fewer places where one tool's
understanding stops exactly where another language begins.
GoSX uses the same idea at a larger scale: syntax marks where work runs—server,
action, island, engine, hub—so the toolchain can see deployment boundaries,
not only expression boundaries.
</Notes>

---

```yaml
class: deck-loop copy-tight
```

# This deck is running the stack it describes

<div class="deck-proof">
  <div class="deck-proof-source">
    <span>one authored source</span>
    <code>deck.md</code>
  </div>
  <div class="deck-proof-pipeline">
    <span><strong>Markdown++</strong> parses structure</span>
    <span><strong>GoSX</strong> compiles components</span>
    <span><strong>gosx-slides</strong> runs the room</span>
  </div>
  <div class="deck-proof-outputs">
    <span><strong>LIVE</strong> interactive tree + Scene3D</span>
    <span><strong>OFFLINE</strong> self-contained bundle</span>
    <span><strong>BACKUP</strong> 16:9 PDF</span>
  </div>
</div>

<div class="deck-proof-verdict">You are looking at the live build.</div>

<Notes>
~19:20. The presentation is inside the presentation.
Markdown++ parses deck.md. GoSX compiles the components and the interactive
tree. gosx-slides is running the room, and Scene3D is moving behind every slide.
The same authored source can become the offline bundle and the PDF backup.
So the M31 Labs stack I am describing is also producing the talk you are
watching. [Pause.]
That does not prove every grammar is perfect or every architectural claim is
universally correct. It proves something narrower and useful: these pieces
compose well enough to carry their own demonstration through authoring,
parsing, rendering, bundling, and export.
Dogfooding is not a replacement for differential testing. It is a high-pressure
integration test where a failure would be extremely visible. Especially to me.
</Notes>

---

```yaml
class: agent-contract
```

# What if agents edited structure, not text?

<div class="agent-cycle">
  <span>human or agent</span>
  <span>query</span>
  <span>transform</span>
  <span>parse again</span>
  <span>verify</span>
</div>

<div class="agent-verdict">Structure makes intent reviewable and testable.</div>

<Notes>
~20:30. Highlighting was only the first consumer of these trees. Most software
agents still operate primarily through text patches. A text diff can tell us
which bytes moved, but often says little about which program construct the
change intended to target.
A grammar gives us a selection language. A query can identify a function
declaration, import, call expression, or argument with a particular structural
relationship. A transformation can operate on captured nodes. The next parse
can verify that the expected structure still exists.
That does not make an agent correct. Reparsing cannot prove the business logic
is right. It can prove more than “the patch applied”: it can constrain the
target, expose malformed output, and make structural postconditions executable.
Humans receive the same benefit in codemods, refactoring tools, migrations, and
code review. A text edit says what changed physically. A structural edit can
also say what the change meant to touch.
</Notes>

---

```yaml
layout: center
class: final-invitation
```

# What will your tools understand next?

Somewhere in your work is a language your tools still see as text.

**If one came to mind, I would love to hear about it.**

`github.com/odvcencio/gotreesitter`

<div class="closing-galaxy" aria-hidden="true">
  <img class="closing-galaxy-still" src="/public/m31labs-og.png" alt="" />
</div>

<div class="closing-contact">
  <img src="/public/contact-qr.png" alt="QR code for the M31 Labs Get in Touch form" />
  <span>Continue the conversation</span>
  <code>m31labs.dev/build</code>
</div>

<Notes>
~21:40. We began with `result :=`: a program that was not valid yet, but
still had useful structure. That is the thread through the whole talk.
Pure Go makes that structure easier to ship, inspect, profile, fuzz, and carry
wherever the rest of a Go product needs to run. The C reference makes the
reimplementation accountable to an independent behavioral answer. Self-hosted
grammar generation makes new structured languages inexpensive enough to
explore. And M31 Labs is where those ideas are being turned into working
systems—not someday, but in the presentation you just watched.
Somewhere in your work there is probably a language hiding in strings, regular
expressions, YAML conventions, comments, filenames, or tribal knowledge. Your
team already sees the structure. Your tools still see text.
What would change if they could see it too? [Pause.]
If one came to mind, I would love to hear about it after the talk. The
conference does not want stage questions, so hold on the repository and QR
code. Nominal finish near 22:55. Hard finish by 24:30.
</Notes>

---
title: "GoTreeSitter: building an ambitious parser substrate with AI"
theme: aurora
aspect-ratio: 16:9
caption-safe-bottom: 20%
duration-minutes: 25
offline-required: true
scene: m31-starfield
---

``` yaml
layout: title
class: copy-tight
```

# GoTreeSitter

How we built an ambitious parser substrate with AI—and kept it honest

Oscar Villavicencio · M31 Labs · GopherCon 2026

<!--
[TIME 0:00–0:50]

GoTreeSitter is a pure-Go Tree-sitter runtime and grammar ecosystem. It can
detect languages, build full or incremental trees, run queries, produce
highlights and tags, parse injections, and carry structural rewrites into the
next parse.

Every existing Go option wraps the C library through CGo. That is fine until
you cross-compile, target WebAssembly, or want to ship one clean static binary.
So we rebuilt the runtime in pure Go—and then had to learn how to trust it.

But this talk is mostly about how we built something that large without
confusing AI-generated progress with correctness. I will establish the product
surface first, then spend the rest of our time on the methods that made it
possible: decomposition, oracles, reduction, ratchets, and owned boundaries.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime.”
- “Programmable Grammars Are Infrastructure.”
- “GoTreeSitter: The Product Starts One Layer Above the Parser.”
-->

---

``` yaml
class: copy-tight mdpp-four
```

# It ships the layer between parsing and product

:::columns
:::col "FIND"

Detect a language; fully or incrementally parse it.
:::

:::col "READ"

Run queries; return tags and typed captures.
:::

:::col "PRESENT"

Produce highlight ranges and injected child trees.
:::

:::col "CHANGE"

Apply atomic rewrites; emit the next `InputEdit` records.
:::
:::

> [!IMPORTANT]
> Application-ready Go APIs: 206 embedded grammars, 119 hand-written Go scanners, 156 highlight and 69 tags query packs—not a checklist to rebuild.

<!--
[TIME 0:50–2:15]

A parser that prints a tree has completed the tutorial. Applications need the
loop around it. They start with a file, retain an edited tree, ask bounded
structural questions, render classified ranges, parse embedded languages, and
often change the source again.

GoTreeSitter packages those transitions as one coordinate-preserving layer.
An editor, document system, compiler, code browser, or refactoring tool can
drop in the capabilities it needs and spend its complexity on product meaning.

The registry ships 206 grammars today, and all 206 parse their smoke samples
without errors. 119 of them need context-sensitive tokens, so 119 external
scanners are hand-written Go. Coverage is a receipt, not a quality stamp: every
entry carries a quality classification you can inspect before you promise a
feature on top of it.

That is the table stakes. The next slide shows how little application code is
required to cross that boundary.

[Sources]
- “GoTreeSitter: The Product Starts One Layer Above the Parser,” capability pipeline.
- GoTreeSitter README, detection, queries, highlighting, tags, injections, and rewriting.
-->

---

``` yaml
class: copy-tight query-operations mdpp-two
```

# Start with a file; ask for the capability

:::columns
:::col "APPLICATION CODE"

``` go
entry := grammars.DetectLanguage("handler.ts")
if entry == nil {
    return ErrUnsupported
}

tree, _ := gotreesitter.NewParser(
    entry.Language(),
).Parse(source)
```
:::

:::col "WHAT THE ENTRY CAN CARRY"

Grammar
: portable parser tables for the runtime

Feature queries
: highlights and tags when the grammar ships them

Runtime facts
: extensions, scanner support, and quality classification
:::
:::

> [!NOTE]
> GoTreeSitter is more than a parser—and deliberately less than a language server.

<!--
[TIME 2:15–3:35]

The application begins with the file, not a hard-coded parser constructor.
Detection returns a capability entry: the grammar plus the optional highlight
and tags packs and runtime facts the application can inspect before promising a
feature.

The boundary is intentionally narrow. GoTreeSitter supplies syntax, exact
ranges, and reusable structural operations. Scope, resolution, diagnostics,
workspace meaning, and product policy still belong above it.

Now that we know what consumers receive, we can talk about why building that
surface was an ambitious systems project rather than a parser port.

[Sources]
- “GoTreeSitter: The Product Starts One Layer Above the Parser,” registry and responsibility boundaries.
- GoTreeSitter README, `grammars.DetectLanguage` and `LangEntry`.
-->

---

``` yaml
class: copy-tight mdpp-four
```

# The bet was much larger than “rewrite C in Go”

:::columns
:::col "REPLACE A RUNTIME"

Own lexing, LR/GLR parsing, recovery, scanners, queries, and incremental reuse in Go.
:::

:::col "KEEP THE ECOSYSTEM"

Load mature Tree-sitter grammars instead of asking every language to start over.
:::

:::col "PRESERVE BEHAVIOR"

Match observable trees, fields, ranges, errors, and scanner-dependent results.
:::

:::col "SHIP THE NEXT LAYER"

Expose the application capabilities those structures make possible.
:::
:::

> [!IMPORTANT]
> The project was too large to “vibe-check.” Every boundary needed a witness.

<!--
[TIME 3:35–5:05]

The naive framing is a C-to-Go rewrite. The real scope was a chain of ownership
claims: the lexer recognized the correct token, the parser attached the right
children and fields, recovery localized damage, a scanner restored its exact
state, and incremental parsing reused only structure it still owned.

Any mistake in that chain can produce a plausible-looking tree. That made
ordinary code review insufficient and made unrestricted AI generation actively
dangerous. We needed a way for a fast implementation loop to collide with
independent evidence on every change.

The project became tractable when we stopped asking “is the parser done?” and
started asking “which observable contract can we prove next?”

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” runtime boundary and constrained decisions.
- “Programmable Grammars Are Infrastructure,” maintained tree contract.
-->

---

``` yaml
class: copy-tight mdpp-two method-slide
```

# We ported observable behavior—not source code

:::columns
:::col "REFERENCE LANE"

Same source

Same grammar version

Tree-sitter C runtime

Normalized structural result
:::

:::col "CANDIDATE LANE"

Same source

Same grammar version

Pure-Go runtime

Normalized structural result
:::
:::

> [!IMPORTANT] The oracle was the unlock
> Compare node types, child shape, fields, byte ranges, missing nodes, and error placement.

<!--
[TIME 5:05–6:45]

A parser runtime has a rare advantage: there is a reference implementation.
We could run the same source and grammar through both systems, normalize their
observable results, and compare them structurally.

The production boundary stayed pure Go. The C runtime remained an independent
validation lane. That separation mattered: we did not ask the implementation
to certify itself, and we did not require consumers to carry the oracle.

This reframed every large unknown as a measurable difference. Instead of
arguing whether a tree “looked right,” we could name the first type, field,
range, error, or child-shape divergence.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” evidence loop.
- “Part 2 — Oracles and Bench Gates,” oracle workflow.
-->

---

``` yaml
layout: center
class: copy-tight mdpp-four method-slide
```

# AI proposed; evidence decided

:::columns
:::col "PROPOSE"

AI explores candidate code, tests, and explanations.
:::

:::col "COMPARE"

The oracle and invariants reject plausible lies.
:::

:::col "REDUCE"

One mismatch becomes the smallest source that still fails.
:::

:::col "KEEP"

The witness becomes a permanent regression test.
:::
:::

> [!IMPORTANT]
> The model accelerated the search. It was never the evidence.

<!--
[TIME 6:45–8:25]

AI was useful because it could search a wide solution space quickly. It could
trace unfamiliar mechanisms, propose an implementation, generate test
variations, and challenge an assumption. None of those outputs earned trust
by themselves.

The loop was propose, compare, reduce, keep. A candidate crossed the oracle and
invariant gates. A failure was reduced to a minimal witness. The fix was
accepted only when that witness passed without regressing the corpus, and the
witness stayed in the suite.

That is the transferable method: give AI high freedom inside a boundary whose
acceptance criteria it does not control.

[Sources]
- “Part 2 — Oracles and Bench Gates,” AI-assisted proof loop.
- GoTreeSitter parity and regression-test methodology.
-->

---

``` yaml
class: copy-tight mdpp-five method-slide
```

# Every mismatch became a smaller problem

:::columns
:::col "CORPUS"

Find a real file where behavior diverges.
:::

:::col "DIFF"

Name the first structural disagreement.
:::

:::col "REDUCE"

Delete everything that does not preserve it.
:::

:::col "INVARIANT"

State the rule the implementation violated.
:::

:::col "RATCHET"

Keep the minimal case and move the floor forward.
:::
:::

> [!TIP]
> Ask AI to solve a falsifiable boundary—not “make the whole parser correct.”

<!--
[TIME 8:25–9:50]

Large failures create vague prompts and vague patches. Reduction changed the
unit of work. We started from a real corpus divergence, located the first
observable mismatch, minimized the source, and named the violated invariant.

That gave the model a bounded problem and gave the reviewer a bounded claim.
It also produced durable project memory. The reduced witness explained more
than a comment because it could still fail the implementation years later.

This is applicable beyond parsers. Minimize a database history, protocol
exchange, rendering state, or compiler input until one contract is under test.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” focused witnesses and parity.
- “Part 2 — Oracles and Bench Gates,” compare-and-ratchet workflow.
-->

---

``` yaml
class: copy-tight mdpp-three method-slide
```

# Build vertical proof slices

:::columns
:::col "FULL PARSE"

Grammar → tables → blob → loader → tree → oracle comparison
:::

:::col "PRODUCT CAPABILITY"

Query pack → captures → highlight, tag, or injection result → fixture
:::

:::col "EDIT LOOP"

Old tree → edit → incremental candidate → fresh parse comparison → admit or fall back
:::
:::

> [!IMPORTANT]
> A component is not done when its unit test passes; it is done when one real path crosses the system.

<!--
[TIME 9:50–11:25]

Horizontal implementation plans are seductive: finish the lexer, then the
parser, then the loader, then queries. They delay integration evidence until
the most assumptions have accumulated.

We used vertical proof slices instead. A small grammar crossed generation,
serialization, loading, parsing, and comparison. A small query crossed compile,
execution, capture conversion, and a product-shaped result. One edit crossed
coordinate maintenance, reuse admission, and a fresh-parse comparison.

Each slice exposed bad interfaces early and created an executable path the next
slice could reuse. Massive projects become manageable when every milestone
ends in evidence at the boundary users will actually cross.

[Sources]
- “Programmable Grammars Are Infrastructure,” transport chain and consumer contract.
- GoTreeSitter runtime, query, and incremental test lanes.
-->

---

``` yaml
class: copy-tight mdpp-four method-slide
```

# Treat every optimization as an admission protocol

:::columns
:::col "CANDIDATE"

An old subtree, scanner checkpoint, or fast path might be reusable.
:::

:::col "PROOF"

Ranges, parser state, fragility, and external state must still agree.
:::

:::col "FALLBACK"

Uncertain work returns to the slower production path.
:::

:::col "MEASURE"

Bench gates verify the safe path is still useful.
:::
:::

> [!WARNING]
> A slower honest result is better than a fast structural lie.

<!--
[TIME 11:25–12:55]

Incremental parsing taught the most general systems lesson in the project.
Reuse is not an entitlement. It is a candidate that must prove it still belongs
to the new document. Unchanged bytes help, but parser and scanner history can
still make the old structure invalid.

The same pattern applies to caches, memoization, indexes, replicated state, and
AI-generated patches: name the conditions under which the shortcut is valid,
test those conditions independently, and fail closed when they are uncertain.

Correct fallback is a feature. Performance only counts after the optimized
result has earned admission.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” incremental admission and scanner checkpoints.
- “Part 2 — Oracles and Bench Gates,” performance gates.
-->

---

``` yaml
class: copy-tight mdpp-four
```

# grammargen turned language work into a reproducible pipeline

:::columns
:::col "AUTHOR"

Import resolved `grammar.json` or maintain a reviewable Go grammar DSL.
:::

:::col "NORMALIZE"

Converge both inputs on one grammar intermediate representation.
:::

:::col "GENERATE"

Build lexer tables, parser tables, fields, conflicts, and scanner metadata.
:::

:::col "SHIP"

Embed a portable grammar blob beside optional query packs.
:::
:::

> [!IMPORTANT]
> Applications load the artifact. They do not carry the generator or its toolchain.

<!--
[TIME 12:55–14:30]

The runtime became reusable when grammar work became reproducible. grammargen
can import a resolved upstream grammar or accept a grammar authored as Go
values. Both paths converge on one IR, generate the tables and metadata the
runtime needs, and serialize a portable blob.

That pipeline separated build-time complexity from the consumer. A product can
embed or load the artifact without Node, a C compiler, or the generator. This
is the self-hosting arc in the talk title: ts2go bootstraps the 206-grammar
breadth from upstream tables, while grammargen is a real pure-Go grammar
compiler that already compiles our whole in-house language family with no C
ancestor, replacing bootstrap blobs grammar by grammar behind the parity
ratchet. The
same pipeline also made every failure locatable: grammar source, normalized IR,
generated tables, blob, loader, runtime, or consumer.

For an ambitious project, this is the artifact lesson: turn expensive knowledge
into a versioned output that the next layer can consume cheaply.

[Sources]
- “Programmable Grammars Are Infrastructure,” grammar IR and transport chain.
- GoTreeSitter `grammargen` authoring and generation guides.
-->

---

``` yaml
class: copy-tight mdpp-three responsibility-slide
```

# Explicit ownership kept the layers honest

:::columns
:::col "RUNTIME OWNS"

Parser execution, recovery, ranges, queries, bounded work, and safe reuse.
:::

:::col "GRAMMAR OWNS"

Tree vocabulary, fields, conflicts, scanner behavior, feature queries, and corpus compatibility.
:::

:::col "PRODUCT OWNS"

Scope, resolution, semantics, policy, user experience, and migration intent.
:::
:::

> [!NOTE]
> A clear boundary lets people—and AI agents—change one layer without pretending to own the others.

<!--
[TIME 14:30–16:00]

The runtime cannot infer what a grammar does not encode. The grammar cannot
turn a syntax capture into workspace meaning. The product should not quietly
rebuild parser safety or coordinate logic in every feature.

Writing those responsibilities down prevented magical thinking. It also made
parallel work safer. A runtime change had parity and invariant gates. A grammar
change had tree-shape and query fixtures. A product change had semantic and
user-facing acceptance tests.

Boundaries are not bureaucracy here. They are the mechanism that lets a large
project move quickly without every change reopening the entire system.

[Sources]
- “Programmable Grammars Are Infrastructure,” maintained tree contract.
- “GoTreeSitter: The Product Starts One Layer Above the Parser,” syntax/semantics boundary.
-->

---

``` yaml
class: copy-tight mdpp-four method-slide
```

# The harness had to ratchet, not merely test

:::columns
:::col "PARITY GATE"

Compare observable structure against the reference runtime.
:::

:::col "CORPUS GATE"

Keep real language families and scanner paths in the loop.
:::

:::col "BENCH GATE"

Catch fake wins. We withdrew our own headline when the harness caught one.
:::

:::col "RELEASE RECEIPT"

Pin the grammar, corpus, capability, and version behind each claim.
:::
:::

> [!IMPORTANT]
> After a bug is fixed, the allowed regression surface should only shrink.

<!--
[TIME 16:00–17:35]

A test suite can stay green while the project quietly changes its definition
of success. A ratchet prevents that. Every reduced mismatch becomes a fixture;
every supported corpus keeps a floor; every accepted capability is attached to
a named grammar and version; benchmark gates prevent fake wins that simply do
less work.

The bench gate is not decoration. An audit found an old full-parse headline was
timing a path that skipped tree materialization, so the project withdrew it in
public. The current sealed four-file receipt (gotreesitter main at `492cd600`,
2026-08-02) reports **4.815× C** for production full parses and **3.986× C**
for the compact route. Those are locked, human-authored Go fixtures—not a
universal speed claim. We trade raw full-parse speed for portability.

On the separate pinned 19,294-byte, 500-function straight-LR control, a
materialized full parse is 10.907 ms, a one-byte edit is 1.98 µs, and a no-edit
reparse is 9.9 ns; both incremental lanes allocate zero. Those absolute timings
are host- and fixture-specific, so we publish no incremental Go-versus-C
headline. That is the ratchet doing its job on our own marketing.

The harness also improved the AI workflow. Agents could explore aggressively
because the acceptance surface was executable and cumulative. The repository
remembered the project better than any prompt could.

When you build something ambitious, invest early in machinery that makes it
hard to redefine “done” after a regression.

[Sources]
- “Part 2 — Oracles and Bench Gates,” parity, corpus, and benchmark gates.
- GoTreeSitter performance receipt (v9, 2026-08-02) and incremental parsing receipt.
- GoTreeSitter release and certification methodology.
-->

---

``` yaml
class: copy-tight products-slide
```

# The substrate paid back across very different products

GoSX
: composes maintained Go syntax with native markup, then compiles the combined tree

Markdown++
: gives parsing, formatting, linting, LSP, rendering, and these slides one document model

qml-language-server
: built by someone outside this project—QML and Qt workspace meaning above GoTreeSitter trees and ranges

Canopy and Graft
: build structural code intelligence and entity-aware version control above shared syntax

> [!IMPORTANT] This deck is recursive dogfooding
> Markdown++ authoring → compiled GoSX slide components → a live Go application.

<!--
[TIME 17:35–19:15]

The proof of a substrate is not another substrate demo. It is the different
products that can begin above it. GoSX composes a language and builds a compiler.
Markdown++ shares one tree across editing, diagnostics, formatting, rendering,
and this deck. qml-language-server is the one I did not build: someone else
picked up the runtime and added real QML and Qt workspace semantics on top of
it. That is the strongest evidence the boundary is in the right place—an
outside developer could start at trees and ranges instead of at a parser.
Canopy and Graft add code-intelligence and version-control rules.

None receives semantics for free. They share grammar artifacts, trees, ranges,
queries, and edits, then deliberately diverge at the product boundary.

Dogfooding made that boundary concrete. When a downstream product hurt, it
exposed missing substrate behavior more honestly than another synthetic test.

[Sources]
- “Programmable Grammars Are Infrastructure,” downstream product boundaries.
- “GoTreeSitter: The Product Starts One Layer Above the Parser,” downstream consumers.
- This deck’s mdpp → GoSX rendering path.
-->

---

``` yaml
class: copy-tight mdpp-five lessons-slide
```

# A playbook for your massive project

:::columns
:::col "1 · SUBSTRATE"

Choose the narrow foundation that several outcomes can share.
:::

:::col "2 · CONTRACT"

Name observable behavior and ownership before implementation spreads.
:::

:::col "3 · WITNESS"

Build an independent oracle, simulator, fixture, or invariant.
:::

:::col "4 · SEARCH"

Let AI explore freely inside that falsifiable boundary.
:::

:::col "5 · COMPOUND"

Reduce failures, ratchet the harness, and dogfood the result.
:::
:::

> [!IMPORTANT]
> AI amplifies the system you give it. Build the evidence system before chasing velocity.

<!--
[TIME 19:15–22:15]

Here is the method I would carry into another massive project.

First, choose a substrate narrow enough to own but valuable enough to support
several outcomes. Second, write its observable contracts and responsibility
boundaries. Third, create an independent witness: a reference implementation,
simulator, replay log, model checker, invariant sweep, or carefully curated
fixture. Fourth, let AI search hard inside that falsifiable boundary. Finally,
turn every discovery into a reduced regression and dogfood the substrate in a
real downstream product.

The goal is not to make AI cautious. The goal is to make ambitious exploration
cheap and incorrect acceptance expensive. Velocity compounds when the harness,
artifacts, and consumers all remember what the team has learned.

[Sources]
- The complete GoTreeSitter article trilogy.
- GoTreeSitter oracle, reduction, ratchet, and dogfooding methodology.
-->

---

``` yaml
class: final-invitation copy-tight mdpp-two
```

# Build one layer lower—so every product can start higher

Name the contract. Create the witness. Let AI search. Keep the proof.

**Then spend your ambition on what only your product can mean.**

:::columns
:::col "REPOSITORY"

`github.com/odvcencio/gotreesitter`
:::

:::col "BUILD WITH M31 LABS"

![QR code for M31 Labs contact](/public/contact-qr.png)

`m31labs.dev/build`
:::
:::

<!--
[TIME 22:15–23:30]

GoTreeSitter is useful because consumers can start with language detection,
trees, queries, highlights, tags, injections, and safe rewrite coordinates
instead of rebuilding them.

The larger lesson is how it got there. Name the observable contract. Build an
independent witness. Give AI freedom to search. Reduce every failure. Ratchet
the proof. Turn expensive knowledge into a reusable artifact. Dogfood it in
the products that depend on the boundary.

That is how an ambitious project stops being one enormous leap and becomes a
sequence of claims the system can actually earn.
-->

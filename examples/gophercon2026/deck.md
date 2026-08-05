---
title: "GoTreeSitter: building an ambitious parser foundation with AI"
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

How we built an ambitious parser foundation with AI—and kept it honest

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
layout: center
class: copy-tight
```

# Hi, I’m Oscar

I build developer tools at M31 Labs. GoTreeSitter, GoSX, Markdown++, Canopy, and Graft all ship from one bet: **own the syntax layer once, in pure Go, and let every product start above it.**

This deck is one of those products. Let’s look at the layer underneath it.

<!--
[TIME 0:50–1:15]

Twenty-five seconds, no life story. I make developer tools at M31 Labs, and
the products share one syntax foundation. The deck itself runs on that stack,
which the audience will see proven on the demo slide.

Land the bolded bet sentence slowly; it is the thesis the whole talk keeps
returning to. Then preview the route.
-->

---

``` yaml
class: copy-tight mdpp-three
```

# The next twenty-five minutes

:::columns
:::col "I · WHAT IT IS"

An application-ready parsing layer in pure Go—and the C reference implementation it answers to.
:::

:::col "II · HOW WE BUILT IT"

An AI-assisted loop where evidence, not the model, decides what merges.
:::

:::col "III · HOW IT’S USED"

Products above the boundary, running live on this deck—and a playbook you can steal.
:::
:::

<!--
[TIME 1:15–1:45]

One breath per act. What it is: the capability layer and the oracle that
keeps it honest. How we built it: the method—decomposition, reduction,
ratchets—with AI proposing and evidence deciding. How it's used: real
products, a live demo of this deck, and the playbook to take home.

Do not preview any numbers here; the receipts land later where they can be
defended. Move.
-->

---

``` yaml
class: copy-tight mdpp-three
```

# First, the reference: Tree-sitter

:::columns
:::col "THE C RUNTIME"

Over a decade of parser engineering: incremental, error-tolerant, and hardened inside the editors millions of developers type into daily.
:::

:::col "THE GRAMMAR ECOSYSTEM"

Hundreds of community grammars—accumulated years of real-world edge cases nobody wants to rediscover.
:::

:::col "THE GO PROBLEM"

Every Go binding wraps the C library through CGo—painful to cross-compile, ship static, or run in WebAssembly.
:::
:::

> [!IMPORTANT]
> GoTreeSitter reimplements the runtime in pure Go, keeps the grammar ecosystem, and the C original becomes the reference implementation we test against.

<!--
[TIME 1:45–2:25]

Forty seconds of shared vocabulary, and the scale matters. Tree-sitter is not
a weekend parser: it is over a decade of engineering on incremental,
error-tolerant parsing, running inside the editors and code tools millions of
developers use every day. Around it sit hundreds of community grammars, and
each one is years of accumulated edge cases—string quirks, ambiguities,
recovery behavior—that nobody sane wants to rediscover from scratch.

So the bet was never "replace Tree-sitter." Throwing that heritage away would
have been the most expensive possible mistake. The catch for Go teams is only
the CGo boundary. We wanted the runtime itself in Go while keeping every
grammar—and the C implementation becomes the most valuable thing an ambitious
rewrite can have: a source of truth to compare against. Hold that thought; it
becomes the oracle in a few minutes.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” motivation and reference lanes.
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
[TIME 2:25–3:45]

A parser that prints a tree has completed the tutorial. Applications need the
loop around it. They start with a file, retain an edited tree, ask bounded
structural questions, render classified ranges, parse embedded languages, and
often change the source again.

GoTreeSitter packages those transitions as one layer that keeps every byte
position accurate across edits.
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
[TIME 3:45–5:00]

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
class: copy-tight query-operations mdpp-two
```

# Queries and rewrites are ordinary Go

:::columns
:::col "ASK A STRUCTURAL QUESTION"

``` go
q, _ := gotreesitter.NewQuery(
    `(function_declaration
        name: (identifier) @fn)`, lang)
cur := q.Exec(tree.RootNode(), lang, src)
for {
    m, ok := cur.NextMatch()
    if !ok { break }
    fmt.Println(m.Captures[0].Node.Text(src))
}
```
:::

:::col "REWRITE, THEN REPARSE INCREMENTALLY"

``` go
rw := gotreesitter.NewRewriter(src)
rw.Replace(fnName, []byte("newName"))
rw.Delete(unusedNode)

newSrc, _ := rw.ApplyToTree(tree)
newTree, _ := parser.ParseIncremental(
    newSrc, tree)
```

A no-edit reparse returns in single-digit nanoseconds, with zero allocations.
:::
:::

<!--
[TIME 5:00–6:25]

Both halves of the loop are plain Go. A query is the full Tree-sitter
S-expression pattern language—quantifiers, alternation, field constraints,
predicates—and each capture keeps its node and exact source range. The
`tsquery` generator can turn a query’s captures into typed Go structs, so a
misspelled capture name becomes a compile error instead of a runtime surprise.

The rewriter closes the loop. It collects replacements, insertions, and
deletions, rejects overlaps, applies the accepted set atomically, and emits
the `InputEdit` records that let the next parse reuse everything the edit did
not touch.

That is what GoTreeSitter is: parse, ask, change, reparse—as ordinary Go.
The next question is why anyone should trust those trees. The answer is that
we never asked you to take our word for it.

[Sources]
- GoTreeSitter README, query execution, `tsquery` codegen, and source rewriting.
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

> [!IMPORTANT] The oracle was the breakthrough
> Compare node types, child shape, fields, byte ranges, missing nodes, and error placement.

<!--
[TIME 6:25–7:55]

A parser runtime has a rare advantage: there is a reference implementation.
We could run the same source and grammar through both systems, normalize their
observable results, and compare them structurally.

The production boundary stayed pure Go. The C runtime remained an independent
validation lane. That separation mattered: we did not ask the implementation
to certify itself, and we did not require consumers to carry the oracle.

This reframed every large unknown as a measurable difference. Instead of
arguing whether a tree “looked right,” we could name the first type, field,
range, error, or child-shape divergence.

That is what GoTreeSitter is, and the reference it answers to. Act two is how
we actually built it.

[Sources]
- “Inside a Pure-Go Tree-sitter Runtime,” evidence loop.
- “Part 2 — Oracles and Bench Gates,” oracle workflow.
-->

---

``` yaml
class: copy-tight mdpp-four
```

# The bet was much larger than “rewrite C in Go”

:::columns
:::col "REPLACE A RUNTIME"

Own lexing, parsing, error recovery, scanners, queries, and incremental reuse in Go.
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
[TIME 7:55–9:10]

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
[TIME 9:10–10:25]

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
> Give AI one small, checkable problem—not “make the whole parser correct.”

<!--
[TIME 10:25–11:40]

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
[TIME 11:40–12:50]

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

# Every optimization had to earn its way in

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
[TIME 12:50–14:05]

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
class: copy-tight query-operations mdpp-two
```

# grammargen made language work reproducible

:::columns
:::col "A GRAMMAR IS REVIEWABLE GO"

``` go
g := NewGrammar("mini_expr")
g.Define("expression", Choice(
    PrecLeft(1, Seq(
        Field("left", Sym("expression")),
        Field("operator", Str("+")),
        Field("right", Sym("expression")))),
    Sym("number")))
g.Test("precedence", "1 + 2 * 3", "")
```
:::

:::col "ONE PIPELINE, ONE ARTIFACT"

Author
: Go DSL or resolved upstream `grammar.json`

Normalize
: both inputs merge into one internal grammar format

Generate
: lexer tables, parser tables, fields, conflicts, scanner metadata

Ship
: one portable grammar blob beside optional query packs

**Applications load the artifact—never the generator or its toolchain.**
:::
:::

<!--
[TIME 14:05–15:30]

The runtime became reusable when grammar work became reproducible. grammargen
can import a resolved upstream grammar or accept a grammar authored as Go
values—the snippet on screen is a real grammargen grammar, and that `g.Test`
line is an embedded corpus test that locks the parse. Both paths converge on
one internal representation, generate the tables and metadata the runtime
needs, and serialize a portable blob.

That pipeline separated build-time complexity from the consumer. A product can
embed or load the artifact without Node, a C compiler, or the generator. This
is the project’s self-hosting arc: ts2go bootstraps the 206-grammar breadth
from upstream tables, while grammargen is a real pure-Go grammar compiler that
already compiles our whole in-house language family with no C ancestor,
replacing bootstrap blobs grammar by grammar behind the parity ratchet. The
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
[TIME 15:30–16:40]

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
> Once a bug is fixed, it can never quietly come back—the bar only moves up.

<!--
[TIME 16:40–18:15]

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

# The foundation paid back across very different products

GoSX
: composes maintained Go syntax with native markup, then compiles the combined tree

Markdown++
: gives parsing, formatting, linting, LSP, rendering, and these slides one document model

qml-language-server
: built by someone outside this project—QML and Qt workspace meaning above GoTreeSitter trees and ranges

Canopy and Graft
: build structural code intelligence and entity-aware version control above shared syntax

> [!IMPORTANT] One of these was built outside the project
> qml-language-server starts at trees and ranges—the strongest evidence the boundary is in the right place.

<!--
[TIME 18:15–19:10]

The proof of a foundation is not another foundation demo. It is the different
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
exposed missing foundation behavior more honestly than another synthetic test.

[Sources]
- “Programmable Grammars Are Infrastructure,” downstream product boundaries.
- “GoTreeSitter: The Product Starts One Layer Above the Parser,” downstream consumers.
- This deck’s mdpp → GoSX rendering path.
-->

---

``` yaml
class: copy-tight query-operations mdpp-two
fallback: "If the tree does not respond, point at the rendered node names and continue; the static structure carries the beat."
```

# This deck is the demo

:::columns
:::col "THE ISLAND SOURCE (GoSX, TRIMMED)"

``` go
type ParseTreeProps struct {
    Expr string
}

//gosx:island
func ParseTree(props ParseTreeProps) Node {
    open := signal.New(true)
    toggle := func() { open.Set(!open.Get()) }
    return <button onClick={toggle}>
        {open.Get() ? "▾" : "▸"}
        function_declaration
    </button>
}
```
:::

:::col "THE SAME ISLAND, RUNNING HERE"

<ParseTree/>
:::
:::

<!--
[TIME 19:10–20:00]

Nothing on this screen is a screenshot. These slides are Markdown++, parsed by
GoTreeSitter grammars, lowered into compiled GoSX components, and running as a
Go application in front of you.

The tree on the right is the concrete syntax tree for `total := price *
(count + 1)`—a faithful rendering of the verified GoTreeSitter parse, and the
collapse toggles are Go signals compiled to WebAssembly.

[Click `function_declaration` to collapse and restore it. If the click does
not respond, point at the node names and move on. Do not troubleshoot.]

The code on the left is that island’s source, trimmed to the mechanism: props
are a typed Go struct—a misspelled prop is a compile error—state is a Go
signal, and markup literals are ordinary Go, compiled by the GoSX compiler
that starts from a GoTreeSitter tree. The stack on stage is the stack in the
talk.

[Sources]
- This deck’s `ParseTree.gsx` island.
- This deck’s mdpp → GoSX rendering path.
-->

---

``` yaml
class: copy-tight query-operations code-dense mdpp-two
```

# Danmuji: tests you can read aloud

:::columns
:::col "THE DSL (.dmj)"

``` danmuji
package cart_test

import "testing"

unit "ShoppingCart.Add" {
    given "an empty cart" {
        cart := NewCart()
        when "adding an item" {
            cart.Add("widget")
            then "count increases" {
                expect cart.Count() == 1
            }
        }
    }
}
```
:::

:::col "THE GENERATED `go test` (TRIMMED)"

``` go
func TestShoppingCartAdd(t *testing.T) {
    t.Parallel()
    t.Run("an empty cart", func(t *testing.T) {
        cart := NewCart()
        t.Run("adding an item", func(t *testing.T) {
            cart.Add("widget")
            t.Run("count increases", func(t *testing.T) {
                assert.EqualValues(t, 1, cart.Count())
            })
        })
    })
}
```
:::
:::

<!--
[TIME 20:00–20:40]

The left column is highlighted by Danmuji’s own grammar blob, loaded by this
deck at render time—the registry mechanism from Act I, live on stage. The
right column is the transpiler’s real output, trimmed of `//line` directives.

Danmuji extends Go with behavior-driven test structure and emits normal Go
tests: those line directives keep failures located in the original source,
and the result runs under plain `go test` with no Danmuji runtime. The
language disappears into the host toolchain.

[Sources]
- Danmuji README, unit/given/when/then example; `danmuji build` output.
- This deck’s `grammars/danmuji.bin` highlight lane.
-->

---

``` yaml
class: copy-tight query-operations code-dense mdpp-two
```

# Ferrous Wheel: rusty in, boring Go out

:::columns
:::col "THE DSL (.fw)"

``` ferrous
package main

import "os"

enum Status { Active, Suspended(string) }

derive Equal for Status

func loadUser(path string) (string, error) {
    let data = os.ReadFile(path)?
    let mut name = string(data)
    if name == "" {
        name = "anonymous"
    }
    return name, nil
}
```
:::

:::col "THE EMITTED GO (TRIMMED)"

``` go
type Status struct {
    tag        int
    suspended0 string
}

func (x Status) Equal(other Status) bool {
    return x == other
}

func loadUser(path string) (string, error) {
    data, _fwTryErr0 := os.ReadFile(path)
    if _fwTryErr0 != nil {
        return *new(string), _fwTryErr0
    }
    name := string(data)
    // … unchanged from the source
}
```
:::
:::

<!--
[TIME 20:40–21:20]

Again the left column is highlighted by Ferrous Wheel’s own grammar blob.
The right column is the transpiler’s real `emit` output, trimmed of the
generated header, `//line` directives, and the constructor block.

Deliberately Rust-shaped in: a payload enum, a derive, `let mut`, and the `?`
operator. Deliberately boring Go out: a tag struct, an `Equal` method, and
the classic explicit error return. Same grammar-composition mechanism as
Danmuji; opposite product choice—Danmuji disappears into `go test`, Ferrous
Wheel keeps its own surface and emits Go you could have written by hand.

That choice belongs to the product. The foundation just hands both a tree
they can trust.

[Sources]
- Ferrous Wheel README; `ferrous-wheel emit` output for this exact source.
- This deck’s `grammars/ferrous.bin` highlight lane.
-->

---

``` yaml
class: copy-tight mdpp-five lessons-slide
```

# A playbook for your massive project

:::columns
:::col "1 · FOUNDATION"

Choose the narrow foundation that several outcomes can share.
:::

:::col "2 · CONTRACT"

Name observable behavior and ownership before implementation spreads.
:::

:::col "3 · WITNESS"

Build an independent oracle, simulator, fixture, or invariant.
:::

:::col "4 · SEARCH"

Let AI explore freely where wrong answers get caught.
:::

:::col "5 · COMPOUND"

Reduce failures, ratchet the harness, and dogfood the result.
:::
:::

> [!IMPORTANT]
> AI amplifies the system you give it. Build the evidence system before chasing velocity.

<!--
[TIME 21:20–23:20]

Here is the method I would carry into another massive project.

First, choose a foundation narrow enough to own but valuable enough to support
several outcomes. Second, write its observable contracts and responsibility
boundaries. Third, create an independent witness: a reference implementation,
simulator, replay log, model checker, invariant sweep, or carefully curated
fixture. Fourth, let AI search hard inside that boundary, where wrong answers
get caught. Finally,
turn every discovery into a reduced regression and dogfood the foundation in a
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
[TIME 23:20–24:10]

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

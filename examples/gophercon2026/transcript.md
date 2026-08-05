# GoTreeSitter: building an ambitious parser foundation with AI

## Editor brief

**Speaker:** Oscar Villavicencio, M31 Labs
**Venue:** GopherCon 2026
**Slot:** 25 minutes
**Nominal finish:** 24:10
**Hard stop:** 25:00
**Audience:** Go developers of all levels who may know Tree-sitter by
reputation without knowing its runtime, grammar-generation, or product
boundaries.

The talk runs in three acts, previewed on the agenda slide: **I. What it is
and the reference implementation (slides 1–8)**, **II. How we built it
(slides 9–16)**, **III. How it's used, use cases, and demos (slides 17–22)**.
By the end, the audience should understand that GoTreeSitter is a bounded
pure-Go structural foundation they can safely build above, and should leave
with an evidence-gated method for using AI on correctness-sensitive work:
contract → witness → AI search → reduction → ratchet → dogfood.

This is the canonical spoken track for the 22-slide deck. Section titles and
time ranges match the slide headlines and the `[TIME]` blocks in `deck.md`.
Bracketed text is a stage direction, not spoken copy.

---

## Act I — What it is and the reference implementation

## 1. GoTreeSitter — 0:00–0:50

GoTreeSitter is a pure-Go Tree-sitter runtime and grammar ecosystem. It can
detect languages, build full or incremental trees, run queries, produce
highlights and tags, parse injections, and carry structural rewrites into the
next parse.

Every existing Go option wraps the C library through CGo. That is fine until
you cross-compile, target WebAssembly, or want to ship one clean static
binary. So we rebuilt the runtime in pure Go—and then had to learn how to
trust it.

But this talk is mostly about how we built something that large without
confusing AI-generated progress with correctness. I will establish the product
surface first, then spend the rest of our time on the methods that made it
possible: decomposition, oracles, reduction, ratchets, and owned boundaries.

## 2. Hi, I'm Oscar — 0:50–1:15

I build developer tools at M31 Labs. GoTreeSitter, GoSX, Markdown++, Canopy,
and Graft all ship from one bet: own the syntax layer once, in pure Go, and
let every product start above it.

This deck is one of those products. Let's look at the layer underneath it.

[Land the bet sentence slowly; it is the thesis the talk keeps returning to.]

## 3. The next twenty-five minutes — 1:15–1:45

Three acts. First, what it is: an application-ready parsing layer in pure Go,
and the C reference implementation it answers to. Second, how we built it: an
AI-assisted loop where evidence, not the model, decides what merges. Third,
how it's used: products above the boundary, running live on this deck, and a
playbook you can steal.

[One breath per act. No numbers here; the receipts land where they can be
defended.]

## 4. First, the reference: Tree-sitter — 1:45–2:25

Forty seconds of shared vocabulary, and the scale matters. Tree-sitter is
not a weekend parser: it is over a decade of engineering on incremental,
error-tolerant parsing, running inside the editors and code tools millions
of developers use every day. Around it sit hundreds of community grammars,
each one carrying years of accumulated edge cases—string quirks,
ambiguities, recovery behavior—that nobody sane wants to rediscover.

So the bet was never "replace Tree-sitter." Throwing that heritage away
would have been the most expensive possible mistake. The catch for Go teams
is only the CGo boundary. We wanted the runtime itself in Go while keeping
every grammar—and the C implementation becomes the most valuable thing an
ambitious rewrite can have: a source of truth to compare against. Hold that
thought; it returns as the oracle in a few minutes.

## 5. It ships the layer between parsing and product — 2:25–3:45

A parser that prints a tree has completed the tutorial. Applications need the
loop around it. They start with a file, retain an edited tree, ask bounded
structural questions, render classified ranges, parse embedded languages, and
often change the source again.

GoTreeSitter packages those transitions as one layer that keeps every byte
position accurate across edits. An editor, document system, compiler, code
browser, or refactoring tool can drop in the capabilities it needs and spend
its complexity on product meaning.

The registry ships 206 grammars today, and all 206 parse their smoke samples
without errors. 119 of them need context-sensitive tokens, so 119 external
scanners are hand-written Go. Coverage is a receipt, not a quality stamp:
every entry carries a quality classification you can inspect before you
promise a feature on top of it.

That is the table stakes. The next slide shows how little application code is
required to cross that boundary.

## 6. Start with a file; ask for the capability — 3:45–5:00

The application begins with the file, not a hard-coded parser constructor.
Detection returns a capability entry: the grammar plus the optional highlight
and tags packs and runtime facts the application can inspect before promising
a feature.

The boundary is intentionally narrow. GoTreeSitter supplies syntax, exact
ranges, and reusable structural operations. Scope, resolution, diagnostics,
workspace meaning, and product policy still belong above it.

And once you have a tree, the rest of the loop is just as plain.

## 7. Queries and rewrites are ordinary Go — 5:00–6:25

Both halves of the loop are plain Go. A query is the full Tree-sitter
S-expression pattern language—quantifiers, alternation, field constraints,
predicates—and each capture keeps its node and exact source range. The
`tsquery` generator can turn a query's captures into typed Go structs, so a
misspelled capture name becomes a compile error instead of a runtime surprise.

The rewriter closes the loop. It collects replacements, insertions, and
deletions, rejects overlaps, applies the accepted set atomically, and emits
the `InputEdit` records that let the next parse reuse everything the edit did
not touch. When nothing changed at all, a reparse returns in single-digit
nanoseconds, with zero allocations.

That is what GoTreeSitter is: parse, ask, change, reparse—as ordinary Go.
The next question is why anyone should trust those trees. The answer is that
we never asked you to take our word for it.

## 8. We ported observable behavior—not source code — 6:25–7:55

A parser runtime has a rare advantage: there is a reference implementation.
We could run the same source and grammar through both systems, normalize their
observable results, and compare them structurally.

The production boundary stayed pure Go. The C runtime remained an independent
validation lane. That separation mattered: we did not ask the implementation
to certify itself, and we did not require consumers to carry the oracle.

This reframed every large unknown as a measurable difference. Instead of
arguing whether a tree "looked right," we could name the first type, field,
range, error, or child-shape divergence.

That is what GoTreeSitter is, and the reference it answers to. Act two is how
we actually built it.

## Act II — How we built it

## 9. The bet was much larger than "rewrite C in Go" — 7:55–9:10

The naive framing is a C-to-Go rewrite. The real scope was a chain of
ownership claims: the lexer recognized the correct token, the parser attached
the right children and fields, recovery localized damage, a scanner restored
its exact state, and incremental parsing reused only structure it still owned.

Any mistake in that chain can produce a plausible-looking tree. That made
ordinary code review insufficient and made unrestricted AI generation actively
dangerous. We needed a way for a fast implementation loop to collide with
independent evidence on every change.

The project became tractable when we stopped asking "is the parser done?" and
started asking "which observable contract can we prove next?"

## 10. AI proposed; evidence decided — 9:10–10:25

AI was useful because it could search a wide solution space quickly. It could
trace unfamiliar mechanisms, propose an implementation, generate test
variations, and challenge an assumption. None of those outputs earned trust by
themselves.

The loop was propose, compare, reduce, keep. A candidate crossed the oracle
and invariant gates. A failure was reduced to a minimal witness. The fix was
accepted only when that witness passed without regressing the corpus, and the
witness stayed in the suite.

That is the transferable method: give AI high freedom inside a boundary whose
acceptance criteria it does not control.

## 11. Every mismatch became a smaller problem — 10:25–11:40

Large failures create vague prompts and vague patches. Reduction changed the
unit of work. We started from a real corpus divergence, located the first
observable mismatch, minimized the source, and named the violated invariant.

That gave the model a bounded problem and gave the reviewer a bounded claim.
It also produced durable project memory. The reduced witness explained more
than a comment because it could still fail the implementation years later.

This is applicable beyond parsers. Minimize a database history, protocol
exchange, rendering state, or compiler input until one contract is under test.

## 12. Build vertical proof slices — 11:40–12:50

Horizontal implementation plans are seductive: finish the lexer, then the
parser, then the loader, then queries. They delay integration evidence until
the most assumptions have accumulated.

We used vertical proof slices instead. A small grammar crossed generation,
serialization, loading, parsing, and comparison. A small query crossed
compile, execution, capture conversion, and a product-shaped result. One edit
crossed coordinate maintenance, reuse admission, and a fresh-parse comparison.

Each slice exposed bad interfaces early and created an executable path the
next slice could reuse. Massive projects become manageable when every
milestone ends in evidence at the boundary users will actually cross.

## 13. Every optimization had to earn its way in — 12:50–14:05

Incremental parsing taught the most general systems lesson in the project.
Reuse is not an entitlement. It is a candidate that must prove it still
belongs to the new document. Unchanged bytes help, but parser and scanner
history can still make the old structure invalid.

The same pattern applies to caches, memoization, indexes, replicated state,
and AI-generated patches: name the conditions under which the shortcut is
valid, test those conditions independently, and fail closed when they are
uncertain.

Correct fallback is a feature. Performance only counts after the optimized
result has earned admission.

## 14. grammargen made language work reproducible — 14:05–15:30

The runtime became reusable when grammar work became reproducible. grammargen
can import a resolved upstream grammar or accept a grammar authored as Go
values—the snippet on screen is a real grammargen grammar, and that `g.Test`
line is an embedded corpus test that locks the parse. Both paths converge on
one internal representation, generate the tables and metadata the runtime
needs, and serialize a portable blob.

That pipeline separated build-time complexity from the consumer. A product can
embed or load the artifact without Node, a C compiler, or the generator. This
is the project's self-hosting arc: ts2go bootstraps the 206-grammar breadth
from upstream tables, while grammargen is a real pure-Go grammar compiler that
already compiles our whole in-house language family with no C ancestor,
replacing bootstrap blobs grammar by grammar behind the parity ratchet. The
same pipeline also made every failure locatable: grammar source, normalized
representation, generated tables, blob, loader, runtime, or consumer.

For an ambitious project, this is the artifact lesson: turn expensive
knowledge into a versioned output that the next layer can consume cheaply.

## 15. Explicit ownership kept the layers honest — 15:30–16:40

The runtime cannot infer what a grammar does not encode. The grammar cannot
turn a syntax capture into workspace meaning. The product should not quietly
rebuild parser safety or coordinate logic in every feature.

Writing those responsibilities down prevented magical thinking. It also made
parallel work safer. A runtime change had parity and invariant gates. A
grammar change had tree-shape and query fixtures. A product change had
semantic and user-facing acceptance tests.

Boundaries are not bureaucracy here. They are the mechanism that lets a large
project move quickly without every change reopening the entire system.

## 16. The harness had to ratchet, not merely test — 16:40–18:15

A test suite can stay green while the project quietly changes its definition
of success. A ratchet prevents that. Every reduced mismatch becomes a fixture;
every supported corpus keeps a floor; every accepted capability is attached to
a named grammar and version; benchmark gates prevent fake wins that simply do
less work.

The bench gate is not decoration. An audit found an old full-parse headline
was timing a path that skipped tree materialization, so the project withdrew
it in public. The current sealed four-file receipt reports 4.815 times C for
production full parses and 3.986 times C for the compact route. Those are
locked, human-authored Go fixtures—not a universal speed claim. We trade raw
full-parse speed for portability.

On the separate pinned control—a 19,294-byte file with 500 functions—a
materialized full parse is 10.9 milliseconds, a one-byte edit is about two
microseconds, and a no-edit reparse is under ten nanoseconds; both incremental
lanes allocate zero. Those absolute timings are host- and fixture-specific, so
we publish no incremental Go-versus-C headline. That is the ratchet doing its
job on our own marketing.

The harness also improved the AI workflow. Agents could explore aggressively
because the acceptance surface was executable and cumulative. The repository
remembered the project better than any prompt could.

When you build something ambitious, invest early in machinery that makes it
hard to redefine "done" after a regression.

## Act III — How it's used, use cases, and demos

## 17. The foundation paid back across very different products — 18:15–19:10

The proof of a foundation is not another foundation demo. It is the different
products that can begin above it. GoSX composes a language and builds a
compiler. Markdown++ shares one tree across editing, diagnostics, formatting,
rendering, and this deck. qml-language-server is the one I did not build:
someone else picked up the runtime and added real QML and Qt workspace
semantics on top of it. That is the strongest evidence the boundary is in the
right place—an outside developer could start at trees and ranges instead of at
a parser. Canopy and Graft add code-intelligence and version-control rules.

None receives semantics for free. They share grammar artifacts, trees, ranges,
queries, and edits, then deliberately diverge at the product boundary. And the
nearest dogfood is on this screen.

## 18. This deck is the demo — 19:10–20:00

Nothing on this screen is a screenshot. These slides are Markdown++, parsed by
GoTreeSitter grammars, lowered into compiled GoSX components, and running as a
Go application in front of you.

The tree on the right is the concrete syntax tree for `total := price *
(count + 1)`—a faithful rendering of the verified GoTreeSitter parse, and the
collapse toggles are Go signals compiled to WebAssembly.

[Click `function_declaration` to collapse and restore it. If the click does
not respond, point at the node names and move on. Do not troubleshoot.]

The code on the left is that island's source, trimmed to the mechanism: props
are a typed Go struct—a misspelled prop is a compile error—state is a Go
signal, and markup literals are ordinary Go, compiled by the GoSX compiler
that starts from a GoTreeSitter tree. The stack on stage is the stack in the
talk.

## 19. Danmuji: tests you can read aloud — 20:00–20:40

The left column is highlighted by Danmuji's own grammar blob, loaded by this
deck at render time—the registry mechanism from Act I, live on stage. The
right column is the transpiler's real output, trimmed of line directives.

Danmuji extends Go with behavior-driven test structure and emits normal Go
tests: those line directives keep failures located in the original source,
and the result runs under plain `go test` with no Danmuji runtime. The
language disappears into the host toolchain.

## 20. Ferrous Wheel: rusty in, boring Go out — 20:40–21:20

Again the left column is highlighted by Ferrous Wheel's own grammar blob, and
the right column is the transpiler's real emit output, trimmed of the
generated header and constructor block.

Deliberately Rust-shaped in: a payload enum, a derive, let mut, and the
question-mark operator. Deliberately boring Go out: a tag struct, an Equal
method, and the classic explicit error return. Same grammar-composition
mechanism as Danmuji; opposite product choice—Danmuji disappears into go
test, Ferrous Wheel keeps its own surface and emits Go you could have
written by hand.

That choice belongs to the product. The foundation just hands both a tree
they can trust.

## 21. A playbook for your massive project — 21:20–23:20

[Slow down. This is the protected teaching slide.]

Here is the method I would carry into another massive project.

First, choose a foundation narrow enough to own but valuable enough to support
several outcomes. Second, write its observable contracts and responsibility
boundaries. Third, create an independent witness: a reference implementation,
simulator, replay log, model checker, invariant sweep, or carefully curated
fixture. Fourth, let AI search hard inside that boundary, where wrong answers
get caught. Finally, turn every discovery into a reduced regression and
dogfood the foundation in a real downstream product.

The goal is not to make AI cautious. The goal is to make ambitious exploration
cheap and incorrect acceptance expensive. Velocity compounds when the harness,
artifacts, and consumers all remember what the team has learned.

## 22. Build one layer lower—so every product can start higher — 23:20–24:10

GoTreeSitter is useful because consumers can start with language detection,
trees, queries, highlights, tags, injections, and safe rewrite coordinates
instead of rebuilding them.

The larger lesson is how it got there. Name the observable contract. Build an
independent witness. Give AI freedom to search. Reduce every failure. Ratchet
the proof. Turn expensive knowledge into a reusable artifact. Dogfood it in
the products that depend on the boundary.

That is how an ambitious project stops being one enormous leap and becomes a
sequence of claims the system can actually earn.

Thank you.

[Hold the final slide through applause. Do not add an improvised recap.]

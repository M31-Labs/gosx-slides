# GoTreeSitter: from parser runtime to product substrate

## Editor brief

**Speaker:** Oscar Villavicencio, M31 Labs
**Venue:** GopherCon 2026  
**Slot:** 25 minutes
**Nominal finish:** 24:10
**Hard stop:** 24:50
**Audience:** Go developers who may know Tree-sitter by reputation without
knowing its runtime, grammar-generation, or product boundaries.

The communication job is precise: by the end, the audience should understand
that GoTreeSitter is a bounded pure-Go structural substrate they can safely
build above because runtime correctness, grammar artifacts, and product
capabilities each carry explicit proof boundaries.

This is the canonical spoken track. Bracketed text is a stage direction, not
spoken copy. The exact inter-slide transitions also appear in `deck.md`.

---

## 1. GoTreeSitter — 0:00–0:40

Parser demos end when source becomes a tree. Products begin there.

Editors and agents need structure while code is incomplete; compilers and tools
need exact nodes and ranges they can trust. I rebuilt Tree-sitter's runtime in
Go because CGo pushed deployment and observability across a foreign boundary.

This talk follows the work that earns the substrate: defensible trees, portable
grammar artifacts, and bounded product operations.

One question remains: what can the next layer safely believe?

The answer begins with a rule that sounds obvious and turns out to be
demanding.

## 2. Every layer inherits the proof burden below it — 0:40–1:10

This is an accumulating contract backed by operational evidence.

The runtime earns a reusable tree. The grammar pipeline makes that tree
contract portable. Products then depend on stable nodes, fields, and ranges.
If a lower layer can lie, the product merely amplifies the lie.

So let us begin at the moment text stops being enough.

## 3. Incomplete code still has to power the tool — 1:10–1:55

[Pause for the room to read the incomplete function.]

This program is incomplete, but the editor cannot become useless. An agent,
analyzer, or compiler service cannot treat every mid-edit document as opaque
text either.

Text search can find the spelling `Handle`; it cannot distinguish a call from a
declaration, comment, string, or unrelated identifier.

A parser can preserve the outer function, damaged expression, and exact ranges
while the user is still typing. That is the first product requirement:
usefulness before validity.

To see how the runtime keeps that promise, follow six verbs.

## 4. Follow six verbs through the runtime — 1:55–2:40

Recognize a token. Shift it onto the parser stack. Reduce completed grammar
pieces into a parent. Fork when one interpretation cannot safely win. Recover
when no ordinary action accepts the input. Reuse only when an old subtree still
belongs in the new document.

Each new verb appears because the simpler mechanism can no longer preserve
correctness.

Those verbs are not improvised at runtime; the grammar compiles most of the
decisions in advance.

## 5. A grammar becomes executable decisions — 2:40–3:35

A Tree-sitter grammar describes tokens, rules, precedence, fields, conflicts,
and external tokens. Generation turns that knowledge into decision tables.

A lex table maps lexical state and an input character to another lexical
state. A parse table maps parser state and lookahead symbol to an action.

The grammar contains the language knowledge. The runtime executes decisions
that were already encoded.

But even the first arrow in this picture hides a negotiation.

## 6. The lexer and parser negotiate — 3:35–4:25

The obvious design tokenizes the whole file first. Real grammars break that
design because the same characters can represent different tokens in different
parser states.

Parser state constrains which token rules are valid. The lexer returns
lookahead. The selected parse action changes parser state and therefore the
next lexical context.

This feedback matters again during incremental parsing: unchanged bytes alone
cannot restore the context that owned them.

With a token in hand, one parser stack handles the easy path.

## 7. One stack works—until the grammar disagrees — 4:25–5:30

LR handles the ordinary case. Shift consumes lookahead. Reduce replaces
completed grammar pieces with their parent.

When a state and lookahead admit several actions, GLR keeps competing stacks
alive, shares their common history, and later kills or merges them.

That preserves ambiguity, but only within limits. Iteration, stack-depth,
stack-count, and node-work budgets keep one pathological grammar or document
from holding an editor forever.

A successful stack is still not the public object a tool needs.

## 8. Parser states must become a useful source tree — 5:30–6:35

A successful parser stack is not yet a useful result. A concrete syntax tree
keeps grammar concepts, punctuation, fields, and exact ranges. Those details
are why the tree can support highlighting, selection, refactoring, and reuse.

[Click `function_declaration` once to collapse it, then again to restore it.
Click `parenthesized_expression` to reveal the grouped addition.]

The interaction is incidental; the structure is the artifact.

[If the first click does not respond, point to `source_file`,
`short_var_declaration`, and `parenthesized_expression` in the rendered tree. Do not
troubleshoot.]

The tree becomes most valuable exactly where clean input ends.

## 9. Recovery can be approximate; scanner checkpoints cannot — 6:35–7:30

When no ordinary action accepts lookahead, recovery keeps nearby structure and
localizes damage.

External scanners are stricter. HTML scanner state includes the complete stack
of open tags; names and depth can exceed the 4,096-byte checkpoint capacity.
Truncating that state would invent the wrong history, so serialization is all
or nothing.

The certification witness is a 137-kibibyte stateful document that must safely
reuse at least 25 percent of its bytes.

That same fail-closed posture governs the whole incremental path.

## 10. Reuse is admission control before it is optimization — 7:30–8:50

Incremental parsing is an admission decision, not “the bytes look unchanged,
so keep the subtree.”

An invariant sweep compared single-byte incremental edits with fresh parses of
identical bytes. One duplicated opening parenthesis returned quickly and
without an error, yet the incremental program root had 20 extra children.

A hidden reduction still had to occur before the next visible sibling could
attach. The fix replays that deterministic reduction chain until the live stack
reaches the subtree’s ownership frontier. Otherwise reuse is rejected.

Byte equality is evidence, not proof.

That bug suggests the discipline for the entire runtime.

## 11. Pure Go moves the boundary; evidence keeps it honest — 8:50–9:55

Removing CGo simplifies the product boundary: ordinary Go builds,
cross-compilation, WebAssembly, profiling, fuzzing, and race detection can all
reach the runtime.

But a pure-Go tree can still lie. Fixtures, corpora, invariant sweeps, and
C-reference lanes compare observable shape, fields, ranges, missing nodes,
error placement, scanners, and focused witnesses.

The product remains pure Go; the oracle may use C because validation and
deployment have different jobs. Performance then climbs behind a correctness
ratchet.

A dependable runtime solves execution. It creates a new question: how does a
language reach that runtime?

## 12. A dependable runtime creates a new problem — 9:55–10:15

The runtime can defend the tree it returns. A product still needs a repeatable
path from grammar rules to that runtime, with consumers surviving grammar
change. That is Act II.

The first move is to separate generation from execution.

## 13. A grammar becomes a portable runtime artifact — 10:15–11:10

Go is the control case: its established grammar must survive import, generation,
serialization, loading, and parsing with the expected tree intact.

GoTreeSitter can import resolved `grammar.json` or accept reviewable grammar
source written with a Go DSL. Both converge on one grammar intermediate
representation.

Pure-Go generation produces lexer and parser tables, and a portable blob
carries those tables plus metadata into the product. The product embeds the
artifact, not the generator.

That separation changes how a product owns syntax.

## 14. A grammar ships like data—but behaves like a dependency — 11:10–12:00

The product can embed a grammar blob and load it without shipping the generator.
But portable does not mean ownerless.

A renamed node can break a lowerer. A hidden node can break an editor. A
shifted range can break a formatter even when the parser still accepts the
source.

External scanners also remain language-specific code with their own tests.
Once consumers depend on tree shape, the grammar behaves like a public package
and deserves the same review.

Owning that contract also lets us change a language without copying the whole
thing.

## 15. Extend a grammar instead of forking it — 12:00–13:05

Grammar composition means extending a base without copying its entire
definition. GoSX starts from the maintained Go grammar and adds native markup
rules.

Its public tree keeps `jsx_*` node names because lowering and formatting depend
on them. A renamed node can break consumers even when source syntax does not
change.

Conflicts, precedence, and external scanners still need explicit policy and
tests. Composition exposes the choice; it does not choose the intended parse.

GoSX turns that mechanism into a visible product boundary.

## 16. Go becomes GoSX — 13:05–14:05

GoSX starts with ordinary Go and adds native markup.

The parser must know when the less-than sign begins markup rather than
comparison, preserve Go expressions inside braces, and distinguish text,
attributes, components, and raw script or style bodies.

The tree then feeds a compiler: server components become HTML; island
components become compact browser programs. The grammar is not merely
highlighting syntax. It defines a compiler boundary.

The same composition machinery can preserve the host toolchain or erase richer
syntax back into it.

## 17. Composition can preserve—or disappear into—the Go toolchain — 14:05–15:20

Danmuji extends Go with behavior-driven test structure, then emits normal Go
tests. Line directives keep failures located in the original source, and the
result runs through `go test` without a Danmuji runtime.

Ferrous Wheel adds Rust-inspired enums, matches, derives, `Result`, and
`Option`, then walks the tree and emits standard Go. Support helpers are
injected only when the tree shows they are needed and the source has not
defined those names itself.

Same composition mechanism; opposite product choices.

A grammar contract does not have to begin with Go at all.

## 18. The same contract can govern policy or documents — 15:20–16:20

Arbiter authors a decision language directly in the Go grammar DSL. Its tree
lowers to checked IR and fixed-width bytecode, while the runtime can emit a
structured explanation of why an outcome was eligible.

Markdown++ uses block and inline grammars so formatting, linting, navigation,
rendering, and PDF export share one document model.

These products want very different semantics. They share the same
infrastructure obligation: node kinds, fields, and ranges must be owned and
tested.

At this point syntax is durable—but a durable tree is still not a product.

## 19. A dependable grammar is still not a product — 16:20–16:40

We now have a runtime and maintained grammar artifact. Can one Go process detect
the language, keep an edited tree, ask structural questions, parse embedded
regions, and produce the next edits without pretending syntax is semantics?

The product path starts before anyone constructs a parser.

## 20. Start with the file, not the parser constructor — 16:40–17:50

A multi-language tool first asks what it is looking at.

At the trilogy’s pinned revision, the registry contains 206 grammars and maps
filenames to languages plus capability metadata. The shipped tests compile 156
highlight queries and 69 tags queries.

Those are precise coverage receipts, not a quality stamp. Registered means the
runtime can locate a grammar. Parse-, highlight-, and tag-capable are separate
claims.

A caller can inspect the difference instead of flattening it into a misleading
supported-or-unsupported bit.

Once a tree exists, queries let consumers ask bounded questions.

## 21. Parse once; ask bounded questions — 17:50–18:50

Walking children is enough for a prototype. A query makes a repeated structural
question readable and shareable.

Each capture retains a node and source range; streamed execution avoids
materializing another tree-shaped model. The same mechanism drives highlights,
tags, and application-specific captures.

But the boundary matters: a query can identify a function declaration locally.
It cannot resolve calls across scopes, imports, packages, generated files, and
build configurations. The consumer still owns that meaning.

Two extensions make those bounded questions easier to carry into real
products.

## 22. Captures become APIs; embedded languages become child trees — 18:50–20:05

String capture names are pleasant until several packages depend on `@name` and
`@body`.

The `tsquery` generator keeps the query readable while turning its expected
captures into typed Go results. That moves many spelling and shape failures
into normal compilation.

Embedded documents need another boundary. An injection query selects included
ranges in a parent tree and chooses child grammars for HTML script, style, or
Markdown fence regions. The parent still owns the document; each child parser
sees only its range.

If the query misses a region, the runtime refuses to guess the author’s intent.

Structural questions become a loop when they can produce safe,
coordinate-aware edits.

## 23. Rewrites close the loop; ordinary Go changes deployment — 20:05–21:20

The rewriter collects replacements, insertions, and deletions against source
ranges. It rejects overlaps, applies the accepted set atomically, and emits the
`InputEdit` records that prepare the tree for the next parse.

That closes the mechanical loop. It does not turn a text replacement into a
safe rename; scope, workspace resolution, comments, formatting, and policy
still belong above it.

Because the runtime, queries, and edit machinery are Go, the same substrate can
ship in a CLI, beside server handlers, in Go or TinyGo WebAssembly, or on
`wasip1`.

Zero CGo is a deployment claim, not a speed claim.

The strongest proof of that boundary is a consumer that starts above it.

## 24. Downstream tools prove the boundary by starting higher — 21:20–22:35

An independent consumer is better evidence than another parser demo.

`qml-language-server` embeds a QML grammar, reparses changed documents, and
builds symbols, references, completion, diagnostics, semantic tokens, and
rename above the syntax layer. Its workspace index supplies QML and Qt meaning;
GoTreeSitter supplies trees and ranges.

Canopy adds symbol search, call graphs, impact analysis, and architecture
checks. Graft uses structure for entity-level diff and merge.

Each product owns the semantics it adds. The shared substrate lets each one
begin from structure instead of reconstructing it.

That gives us the boundary I wanted the whole trilogy to earn.

## 25. The substrate is useful because it stops — 22:35–24:10

GoTreeSitter can identify languages, produce full and incrementally reusable
trees, execute queries, drive highlights and tags, parse injected regions,
generate typed query consumers, apply structural edits, and deploy as ordinary
Go.

It cannot make every grammar equally complete. It cannot certify scanner state
a scanner does not expose. It cannot turn a capture into a resolved symbol or a
rewrite into a safe refactor.

[Pause.]

That is not an apology. It is what makes the substrate composable.

The runtime defends structure. The grammar owns a durable tree contract. The
product owns the meaning above it. Correctness witnesses form the ratchet;
measurements can then improve cost without making the claim broader than the
evidence.

If your tools are rebuilding structure at every boundary—or if a language is
hiding inside conventions, strings, or files—I would love to compare notes.
The repository and contact link are here.

Thank you.

[Hold the final slide through applause. Do not add an improvised recap.]

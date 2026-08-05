# Source-to-slide beat map

## Communication job

By the end, Go developers of all levels should recognize GoTreeSitter as a
package of application-ready structural capabilities; know the guarantees that
make those APIs trustworthy; recognize grammargen as the portable
language-artifact path; and see how products add semantics above that layer.
They should also leave with an evidence-gated method for using AI in
correctness-sensitive systems.

Because this is a Go conference, every act carries real code: the consumer
API in Act I, the grammar DSL in Act II, and the deck's own compiled GoSX
island source beside its live rendering in Act III.

## Editorial and authoring method

The three finalized GoTreeSitter articles support the runtime, grammar, and
product claims. The AI-method slides reflect the author's implementation
practice: AI accelerated candidates and tests, while the independent
reference, minimized witnesses, and regression tests decided correctness.

The deck is deliberately authored as Markdown++ rather than raw-HTML-shaped
Markdown. Container directives, admonitions, definition lists, code fences,
and the `ParseTree` island are parsed by mdpp and lowered into compiled GoSX
slide components. The island takes a typed Go struct as props, and the demo
slide shows that source beside the running component. All Go snippets are taken from the shipped
GoTreeSitter and grammargen documentation or this deck's own island source,
not written for the slides.

## Mapping

| Slides | Narrative job | Evidence preserved | Reusable lesson |
| ---: | --- | --- | --- |
| 1–3 | Open: the title beat, who is talking, and the three-act route. | The bet sentence; the agenda columns. | Say where you are going before you go. |
| 4–7 | Establish the product surface in ordinary Go, then the oracle that earns trust. | Detection, parsing, queries, highlights, tags, injections, rewrites; 206 grammars with inspectable quality classifications; the detect-parse and query-rewrite snippets; the C reference lane. | Ship the capability a consumer can call, and find an independent witness before trusting generated code. |
| 8–12 | Teach the AI operating loop. | The ownership chain from lexer to incremental reuse; propose → compare → reduce → keep; vertical proof slices; reuse as an earned admission. | Give AI freedom inside a boundary that catches wrong answers. |
| 13–15 | Show the delivery and ratchet machinery. | The grammargen Go DSL with its embedded corpus test; the runtime/grammar/product ownership table; parity, corpus, and bench gates; the withdrawn headline. | Turn expensive knowledge into artifacts and make "done" hard to redefine. |
| 16–19 | Prove the payoff, demo the stack, and close. | Downstream products, including one built outside the project; the deck's own island source rendering live; the five-step playbook. | Trust the tree, own the artifact, build the meaning. |

## Act integrity

### Act I — what it is and the reference implementation, slides 1–7

The title, a twenty-five-second hello, and the agenda put the route up front.
Then the audience learns what GoTreeSitter lets an application call—detection,
trees, queries, highlights, tags, injections, and rewrites—and reads the
actual Go for detect-parse and query-rewrite. The act closes on the C
reference lane: the project is defined by observable behavior, checked
against an independent implementation.

### Act II — how we built it, slides 8–15

The real scope was a chain of ownership claims too large to vibe-check. The
propose–compare–reduce–keep loop, vertical proof slices, earned reuse, the
grammar DSL, explicit ownership, and the ratcheting harness explain how AI
moved fast without deciding correctness—including against the project's own
marketing.

### Act III — how it's used, use cases, and demos, slides 16–19

Downstream products prove the boundary: a compiler, a document system, an
outside language server, code intelligence, and version control. The demo is
the deck itself—island source on the left, the compiled result running on the
right—and the playbook makes the method stealable.

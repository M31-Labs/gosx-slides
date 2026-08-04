# Source-to-slide beat map

## Communication job

By the end, Go developers should recognize GoTreeSitter as a package of
application-ready structural capabilities; know the guarantees that make those
APIs trustworthy; recognize grammargen as the portable language-artifact path;
and see how products add semantics above that layer. They should also leave
with an evidence-gated method for using AI in correctness-sensitive systems.

## Editorial and authoring method

The three finalized GoTreeSitter articles support the runtime, grammar, and
product claims. The AI-method slide reflects the author’s implementation
practice: AI accelerated candidates and tests, while the independent reference,
minimized witnesses, and regression tests decided correctness.

The deck is deliberately authored as Markdown++ rather than raw-HTML-shaped
Markdown. Container directives, admonitions, definition lists, code fences,
notes, and the ParseTree component are parsed by mdpp and lowered into compiled
GoSX slide components.

## Mapping

| Slides | Narrative job | Evidence preserved | Reusable lesson |
| ---: | --- | --- | --- |
| 1–2 | Define the application-facing capability layer. | Detection, parsing, queries, highlights, tags, injections, rewrites. | Ship the feature a consumer can call, not only its primitive. |
| 3–7 | Make each ready-made capability concrete. | Registry contracts, highlight ranges, tags, typed queries, injections, atomic rewrites. | Let applications start above parser plumbing. |
| 8–10 | Explain the shared CST, runtime guarantees, and AI method. | CST, exact ranges, partial trees, safe reuse, oracle, permanent witnesses. | Let AI propose; use independent behavior to decide. |
| 11–13 | Explain grammargen, ownership, and product possibilities. | Rules → tables → blob; feature queries are an API; downstream consumers; mdpp → GoSX. | Ship language artifacts and maintain their consumer contract. |
| 14–15 | Synthesize and close. | Four lessons and the bounded substrate. | Trust the tree, own the artifact, build the meaning. |

## Act integrity

### Act I — ready-made capabilities, slides 1–7

The audience learns what GoTreeSitter lets an application call: detection,
highlighting, tags, typed queries, injections, and rewrites. Parsing is the
shared substrate, not the delayed payoff.

### Act II — trust and delivery, slides 8–12

The CST explains how the capabilities compose. Runtime guarantees and the
evidence-gated AI loop explain why they can be trusted. grammargen turns
language support into a portable asset, and grammar maintainers own its public
feature contract.

### Act III — possibilities and lessons, slides 13–15

The final act shows products that begin above the parser, uses the deck’s own
Markdown++ → GoSX path as proof, and closes on reusable engineering lessons.

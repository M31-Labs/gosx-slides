# Editorial review — Pure-Go Tree-sitter (GopherCon 2026)

Reviewed: `deck.md` (23 slides), `transcript.md`, `campaign.md`, `README.md`,
`rehearsal.md`. This is an editorial pass, not a rewrite. No changes were made
to `deck.md`.

## Overall verdict

The talk is in strong shape. The argument is honest, the qualifications are
disciplined, and the arc (thesis → justification → proof → turn → payoff →
close) holds. Two structural problems remain, and both are fixable without
touching the voice:

1. **The evidence act shows no evidence.** Slides 9–14 argue that independent
   evidence beats assertion, yet the act displays zero artifacts: no witness,
   no diff, no count, no number. For this specific talk, that is a
   self-inflicted wound. One real minimal witness on screen fixes it.
2. **The plan has zero slack.** The notes' timing cues sum to exactly 24:30.
   No applause buffer, no demo variance, no transition cost. The rehearsal
   valves are reactive; the deck needs structural slack. Merging slides 11+12
   and cutting slide 20 reclaims about 1:45 and also fixes the two pacing
   problems below.

Secondary finding: slide 20's spoken notes say "…across Studio, Admin, Native,
and **Slides**." That word leaks the slide-21 reveal one slide early. Even if
nothing else changes, remove it.

---

## Question 1 — Do slides 11–14 sag after the 206 applause line?

**Quantified from the notes' own markers.** The verification act (slides 9–14)
runs 8:25 → 14:30 = **6:05**, which is **24.8%** of the 24:30 talk. Slides
11–14 alone run 10:15 → 14:30 = **4:15**. Per-slide budgets: 11 = 1:10,
12 = 0:55, 13 = 0:50, 14 = 1:20.

**Verdict: it is not a content sag, but it is a visual and energy sag.** Each
slide has a distinct job — what parity compares (11), how failures persist
(12), who judges AI output (13), proof before measurement (14). None is
redundant. But they are four consecutive static text slides with no code, no
demo, no artifact, immediately after the talk's biggest number (slide 8) and
its 0:50 budget, which includes no applause pause.

**Compression: yes, to three slides, with no credibility loss.**

- Merge 11 and 12. Both describe the comparison machinery: what we compare and
  what happens when comparison fails. Combined budget today is 2:05; the
  merged slide needs about 1:10. Saves ~55 seconds. See Edit 1.
- Keep 13 as its own slide. `rehearsal.md` marks it never-cut, correctly. It is
  the AI turn and carries a memorize-verbatim line.
- Keep 14. The Python-root-ERROR anecdote is the act's payoff and the only
  receipt-shaped moment in the act.

The merged slide is also the right home for a real evidence artifact (Edit 2),
which converts the act from described methodology into displayed proof.

---

## Question 2 — Does the close earn the QR ask?

**Mostly yes. The ask itself is soft and well judged.** Slide 23 asks for a
conversation, not a purchase, and the notes give the audience an action
("hold on the repository and QR code") that matches the no-questions format.

**The "hidden language" motif is seeded, but faintly.** The seeds that exist:

- Slide 4 notes: queries can find "the language embedded inside this string."
- Slide 8 on-slide: "injections" appears in the feature list.
- Slide 11 notes: the string-node hypothetical breaks "an injection query."

All three frame injection as a *parser feature*. None frames it as *the
audience's own hidden languages*. So when slide 23 says "a language hiding in
strings, regular expressions, YAML conventions, comments, filenames, or tribal
knowledge," the enumeration is new material arriving in the final 75 seconds.
It does not land cold — slide 22's pivot ("Humans receive the same benefit…")
builds a ramp — but it lands as a fresh idea when it should land as a callback.

**Fix: one sentence at slide 18.** The DSL portfolio is literally a list of
languages M31 Labs found hiding in its own tools. Say so (Edit 4). Then slide
23 pays off a planted idea instead of introducing one.

One more close note: slide 23's budget is 1:15 for ~160 spoken words plus a
mandatory pause — about 135–140 wpm. That is the one slide `rehearsal.md`
forbids accelerating. It needs the slack that Edits 1 and 3 reclaim.

---

## Question 3 — Is slide 21 positioned to land?

**The position in the sequence is right.** Proof (21) before vision (22) before
invitation (23) is the correct order. Do not move it. Three things around it
limit the payoff:

1. **The reveal leaks one slide early.** Slide 20 notes: "The same model
   reaches HTML, WASM islands, realtime hubs, native surfaces, and Scene3D
   across Studio, Admin, Native, and **Slides**." An attentive listener now
   expects slide 21. Delete the product-surface list, or cut slide 20 (Edit 3).
2. **Slide 20 is the weakest slide in the deck and it sits directly before the
   strongest.** It is the most brand-forward, least evidenced 65 seconds of the
   talk — five framework categories plus a product-surface roll call, with an
   unbacked cost claim ("A static page pays for the server-rendered portion").
   `campaign.md` says the talk must teach first; slide 20 is the closest the
   deck comes to a product pitch. Slide 19 (real code, real tree) into slide 21
   (live proof) is a stronger runway than 19 → 20 → 21.
3. **The punchline is on-screen before it is spoken.** "You are looking at the
   live build" renders the moment the slide appears, before the speaker says
   "The presentation is inside the presentation." If gosx-slides supports a
   staged reveal, delay the verdict line one click. If not, consider removing
   the on-slide verdict and delivering it only in voice — the room, the moving
   Scene3D background, and the pause do the work.

Also extend the `rehearsal.md` rule "do not accelerate the final two slides" to
cover slide 21. Under time pressure at the 17:45 checkpoint, 21 is currently
unprotected.

---

## Question 4 — Is 24:30 realistic?

**The notes' cues sum to exactly 24:30.** Verified by addition of all 23
inter-slide deltas (0:50 + 0:35 + 0:45 + 1:00 + 2:30 + 1:00 + 0:55 + 0:50 +
1:00 + 0:50 + 1:10 + 0:55 + 0:50 + 1:20 + 0:45 + 1:05 + 1:25 + 0:55 + 1:10 +
1:05 + 1:10 + 1:10 + 1:15 = 24:30). The rehearsal checkpoints match these cues
exactly.

**So the target is reachable only if nothing varies, which nothing ever does.**
There is no budget for applause after slide 8, laughter after "Especially to
me," demo latency, or the fallback decision (rehearsal allows 10 seconds for
it). The valves in `rehearsal.md` are good recovery tools, but a plan that
needs its valves on a nominal run is over-committed.

Highest overrun risk, in order:

| Slide | Budget | Risk |
| --- | ---: | --- |
| 5 (live tree) | 2:30 | ~215 spoken words plus three live interactions leaves ~20s per interaction. Any hesitation or fallback burns the whole margin. Largest variance in the talk. |
| 23 (close) | 1:15 | ~160 words + required pause ≈ 135–140 wpm. Cannot be accelerated by rule. Needs upstream slack. |
| 1 (open) | 0:50 | ~130 words ≈ 156 wpm. Openings run fast under adrenaline, but this is the densest pace in the deck. Consider trimming the third claim's restatement. |
| 19 (card.gsx) | 1:10 | ~145 words at ~125 wpm while the audience reads a 19-line code block. Reading time competes with speaking time. |
| 17 (grammargen) | 1:25 | ~165 words ≈ 115 wpm — fine on paper, but this is the densest diagram; the valve (group NFA/DFA and LALR/GLR) should be pre-committed, not reactive. |

**Recommendation:** apply Edits 1 and 3. They reclaim ~1:45, bringing the
nominal plan to ~22:45. Spend it as: +0:15 applause buffer after slide 8,
+0:15 on slide 21, and hold ~1:15 as true slack. That makes 24:30 realistic on
a normal run and 24:45 (the hard stop) safe on a bad one.

---

## Question 5 — Unbacked claims

The deck is unusually disciplined about qualifications ("coverage is not
maturity," "prove operationally," "not all equally mature"). The gaps are
places where a *number or artifact exists* and is simply not shown:

1. **Slides 9–14 show zero artifacts** (see Q1). The act about evidence
   contains only descriptions of evidence. Fix with Edit 2: one real reduced
   witness plus its S-expression difference, and the corpus counts.
2. **Slide 12/13: "locked corpus," "permanent witnesses" — no counts.** The
   counts exist in the repo. "N witnesses across M grammars" is a correctness
   receipt, not a benchmark, so it does not violate the README's
   no-unsealed-benchmark rule. Placeholders must be filled from the frozen repo
   tag, per the July 25 precision run.
3. **Slide 14 describes a benchmark harness in detail and shows no result.**
   `GOMAXPROCS=1`, allocations, max RSS — and not one number. An attentive
   listener will notice the asymmetry with "First prove it. Then measure it."
   Two honest options: (a) seal one number by the July 31 freeze
   (single-sourced from `BENCH.md` per `README.md`) — even a single full-parse
   ratio for one grammar, stated with its qualification; or (b) shorten the
   measurement half to two sentences and let the Python-root-ERROR anecdote
   carry the slide. Option (a) is stronger; option (b) is safer. Doing neither
   is the worst outcome.
4. **Slide 16: "It moves to the native road only after parity agrees" — how
   many grammars have crossed?** The hallway question bank already anticipates
   "How complete is grammargen?" and the talk never answers. State the honest
   number in the notes, even if the answer is "the native road currently
   carries the M31 DSLs; upstream grammars remain on the bootstrap road." That
   honesty is on-brand and cheaper than being asked in the hallway.
5. **Slide 13: "a large amount of parser code very quickly" — vague quantity.**
   Either quantify or soften. Softening is acceptable here because the slide's
   own point is that throughput is not evidence.
6. **Slide 6: "Profile, fuzz, cover, and race-check."** As written it is a
   capability claim (Go's tools *can* see the parser), which is true by
   construction. If fuzzing actually runs in CI, one sentence in the notes with
   a corpus size upgrades it to a practice claim. If it does not run, leave the
   wording as capability and do not embellish aloud.
7. **Slide 20: "A static page pays for the server-rendered portion."** Unbacked
   cost claim; it dies with the slide cut (Edit 3).
8. **Freeze-time verification (residual risk, from org memory): gts#111.** The
   known JavaScript block/assignment collapse on minified input (`{a}b=c`) was
   open as of the last record. The deck's qualifications already cover it —
   nothing claims all 206 grammars pass parity on all real-world input. But the
   July 25 precision run must confirm the bug's status so no spoken sentence or
   hallway answer drifts into an overclaim, and so question 6 in the question
   bank ("Are all 206 grammars equally mature?") has a truthful, rehearsed
   answer.
9. **"206" itself:** repo-checkable and properly qualified. Confirm the frozen
   tag still counts 206 on August 5 so the number on the slide matches the
   number in the repo the audience will open that evening.

---

## Prioritized edits

### Edit 1 (structure) — Merge slides 11 and 12

**Current:** two slides, 2:05 combined. Slide 11 lists the node comparison;
slide 12 shows the witness pipeline.

**Proposed:** one slide, ~1:10.

On-slide:

```
# Parity hides in the details

For every node: symbol · byte range · named / missing / error · ordered children
Then queries, highlights, tags, injections, and incremental results.

real source → structural difference → minimal witness → permanent test

**A plausible tree can still be wrong. Every fixed bug leaves a witness.**
```

Notes (~135 words, ~60–65s):

> A root node named program is not parity. The two implementations walk the
> tree in lockstep: symbol, byte range, named, missing, and error state, and
> the order of children. Those are byte ranges, not rune counts. A single
> UTF-8 boundary mistake is a real behavioral difference. We also compare
> query-facing behavior: captures, highlights, tags, injections, and the
> result after incremental edits. When a real file exposes a difference, the
> first job is to preserve it. Then we shrink the input until the reason
> becomes legible. Pinned open-source files keep us honest on programs nobody
> designed to flatter the parser. Deliberately invalid files exercise
> recovery, because malformed input is not an edge case in an editor. Every
> fixed failure becomes a permanent witness — a small museum of parser
> misunderstandings we never have to rediscover.

**Reason:** removes the four-in-a-row text-slide run after the applause line,
saves ~55 seconds, loses no distinct claim. The string-node hypothetical
(already the rehearsal valve) moves to hallway material. The "museum" line
survives because it is the act's best image.

### Edit 2 (evidence) — Put one real witness on screen

**Current:** slides 9–14 describe evidence; none is shown.

**Proposed:** on the merged 11/12 slide (or on 14, tied to the Python
anecdote), show one actual reduced witness and its structural difference:

```
witness <ID> · <grammar> · reduced from <N> lines to <n>
- (expression_statement (assignment_expression …))
+ (ERROR …)
```

All values must come from the frozen repo tag — do not invent them. If the
Python-root-ERROR case from slide 14 has a banked witness, use that one, so
the anecdote and the artifact are the same object. Also fill the corpus counts
in the notes: "N witnesses across M grammars."

**Reason:** the talk's refrain is evidence over assertion. Displaying one real
counterexample is worth more than four slides describing the process that
produces them. This is the highest-leverage single edit in the review.

### Edit 3 (structure) — Cut slide 20; fold one sentence into slide 19

**Current:** slide 20 (execution map), 1:05, five categories plus a
product-surface roll call, ending "…across Studio, Admin, Native, and Slides."

**Proposed:** delete the slide. Append to slide 19 notes:

> GoSX uses the same idea at a larger scale: syntax marks where work runs —
> server, action, island, engine, hub — so the toolchain can see deployment
> boundaries, not only expression boundaries.

**Reason:** slide 20 is the least evidenced, most pitch-adjacent slide, it
carries an unbacked cost claim, and its notes leak the slide-21 reveal. Cutting
it saves ~50 seconds net and gives the deck's strongest moment a clean runway.

**Minimum fallback if the owner keeps slide 20:** delete "across Studio,
Admin, Native, and Slides" from the notes and drop the static-page cost
sentence. The reveal leak is the non-negotiable part.

### Edit 4 (seeding) — Plant the closing motif at slide 18

**Current notes:** "Once grammar creation became an in-process, testable
operation, M31 Labs kept finding places to use it."

**Proposed addition (one sentence, after that line):**

> Almost every one of these began the same way: as a language hiding inside
> our own tools — in strings, templates, and conventions — before it had a
> grammar.

**Reason:** slide 23's enumeration ("strings, regular expressions, YAML
conventions, comments, filenames, tribal knowledge") currently arrives as new
material in the final 75 seconds. This sentence makes it a callback. Cost:
~8 seconds, absorbed by Edit 1's savings.

### Edit 5 (evidence) — Answer "how complete is grammargen" on slide 16

**Current notes end:** "The old ecosystem remains available while the new
toolchain proves itself grammar by grammar."

**Proposed addition:** one factual sentence with the real number, for example:

> Today, <K> grammars run from native grammargen output; the rest remain on
> the bootstrap road while parity is established.

(Fill `<K>` from the frozen repo. If the honest answer is "the native road
currently carries the M31 DSLs," say exactly that.)

**Reason:** the question bank predicts this exact question. Answering it from
the stage, with a number, converts a potential weakness into a credibility
moment.

### Edit 6 (copy) — Slide 7 on-slide

**Current:** "CGo spends complexity at the build, ABI, ownership, and
deployment boundaries."

**Proposed:** "CGo moves complexity to the build, ABI, ownership, and
deployment boundaries."

**Reason:** "spends complexity" is a metaphor; "moves" is concrete and echoes
the slide title ("It moves."). Also consider expanding ABI once —
"ABI (application binary interface)" — in the spoken notes, not on the slide.

### Edit 7 (copy) — Slide 13 on-slide bold

**Current:** "**No change grades its own homework.**"

**Proposed:** "**No change supplies its own evidence.**"

**Reason:** the current line is an idiom (STE flag) and, more importantly, the
proposed line echoes the slide title verb ("The reference **supplies**
evidence"), which makes the pair read as one thought. If the owner prefers the
homework line for its warmth, keep it as a conscious exception — it is the
kind of line rooms remember. This is a judgment call; the STE-clean version
happens to also be the tighter one.

### Edit 8 (copy) — Slide 5 legend

**Current:** "`ERROR` source the parser could not place · `MISSING` expected
syntax that is absent"

**Proposed:** "`ERROR` = source the parser could not place · `MISSING` =
expected syntax that is absent"

**Reason:** the current telegraphic form drops the copula and reads as a noun
pile on first glance. One character per term fixes it.

### Edit 9 (STE, abbreviations) — Define at first use, selectively

- Slide 14: "maximum RSS" → "peak RSS (resident set size)" in the on-slide
  line or the notes. RSS is the least universally known abbreviation on any
  slide.
- Slide 18: "several kinds of DSL" → "several domain-specific languages
  (DSLs)". First on-slide use of the abbreviation.
- Slide 17's NFA / DFA / LALR / LR(1) / GLR and slide 8's GLR: leave the
  acronyms on-slide (the audience is developers and the notes explain each one
  functionally), but treat this as a recorded conscious exception rather than
  an oversight.

### Edit 10 (copy) — Slide 23 on-slide

**Current:** "**If one came to mind, I’d love to hear about it.**"

**Proposed:** "**If one came to mind, I would love to hear about it.**"

**Reason:** minimal STE fix (contraction) that keeps the warmth. Do not
formalize further; the invitation must sound like a person. Note the on-slide
line answers a question the speaker has not asked yet when the slide first
appears — acceptable here because the slide stays up through the whole close.

### Edit 11 (timing) — Slide 1 pace

**Current:** ~130 words in 0:50 (~156 wpm), the fastest budget in the deck, at
the moment of maximum adrenaline.

**Proposed:** trim the third claim's second sentence. "And third, once grammar
generation is in Go too, this stops being merely a port — it becomes a
toolchain that can carry existing grammars and create new ones." (One sentence
instead of two; saves ~5 seconds and one breath.)

**Reason:** the opening is memorize-verbatim; make it speakable at 140 wpm.

### Edit 12 (logistics, not deck) — Room name inconsistency

`rehearsal.md` says "Arrive at the Main Theatre by 3:15 PM." `README.md` and
`campaign.md` both say Finneran Ballroom 2 (SCC Summit, Level 5). One of these
is wrong. Fix `rehearsal.md` before August 5 — arriving at the wrong room at
3:15 PM is the cheapest catastrophe to prevent in this entire review.

### Edit 13 (rehearsal, not deck) — Protect slide 21

Extend the `rehearsal.md` rule "Do not accelerate the final two slides" to the
final three. Slide 21 is the peak; it must not absorb lateness from act 4.

---

## Cut list if the talk runs long

In order, with savings, honoring the never-cut list (2, 9–10, 13, 19, 21, 23):

1. **Slide 20 entirely** — saves ~0:50 net after the slide-19 fold-in. Already
   recommended unconditionally (Edit 3).
2. **String-node hypothetical in slide 11 notes** — saves ~0:15. Already the
   designated valve; pre-commit it if Edit 1 lands, since the merged slide
   absorbs it anyway.
3. **Slide 17, third block of notes** (LALR merging / LR(1) splitting / GLR as
   three separate explanations → one grouped sentence, per the existing
   valve) — saves ~0:20.
4. **Slide 12 "museum" closer** (only if Edit 1 is rejected) — saves ~0:08.
5. **Slide 3 navigation and refactoring examples** (existing valve) — saves
   ~0:15. Use last; the slide is doing thesis setup.

Total recoverable without touching protected slides: ~1:45–2:00.

Do not cut: slide 5's three demo beats (the demo is the talk's only live
interaction before 21), slide 14's Python anecdote (the act's one receipt),
or any part of slides 21–23.

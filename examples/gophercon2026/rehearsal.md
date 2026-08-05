# GopherCon 2026 rehearsal plan

## Stage contract

- Slot: **25:00**; nominal finish: **23:20**; hard stop: **25:00**.
- Deck: **19 slides**, no required live demo (slide 17's interactive tree has a
  static fallback).
- Act structure, previewed on slide 3: **I. What it is and the reference
  implementation (1–7)** · **II. How we built it (8–15)** · **III. How it's
  used, use cases, and demos (16–19)**.
- Table-stakes outcome: attendees can identify the GoTreeSitter capabilities
  they can drop into an application and the boundary where product meaning begins.
- Methodology outcome: attendees can apply the same operating loop to ambitious
  work: **contract → witness → AI search → reduction → ratchet → dogfood**.
- Green-room sentence: **AI proposed; evidence decided; every mismatch became a
  smaller permanent test.**

## Checkpoint spine

Framing done — hello and the three-act route
: **1:45**

Act I done — surface, real code, reference lane
: **7:15**

AI proof loop taught
: **9:45**

Reduction and vertical slices
: **12:10**

Admission lesson
: **13:25**

grammargen and ownership
: **16:00**

Act II done — ratchet lands
: **17:40**

Products and live-demo proof
: **19:50**

Playbook delivered
: **22:20**

Nominal finish
: **23:20**

## Run sheet

1. **0:50 — Promise a talk about the project and the method.**
   Trim: keep the second paragraph only.
2. **1:15 — Say hello; land the bet sentence.**
   Twenty-five seconds. Do not add credentials or history.
3. **1:45 — Preview the three acts.**
   One breath per act; no numbers.
4. **3:05 — Establish the application-ready capability surface.**
   Trim: read FIND / READ / PRESENT / CHANGE.
5. **4:20 — Show the consumer boundary in ordinary Go.**
   Trim: read the code and the note.
6. **5:45 — Walk the query and rewrite snippets.**
   Trim: read the query pattern and the two rewrite lines; keep the
   nanoseconds line.
7. **7:15 — Establish the C reference lane as the oracle; close Act I.**
   Trim: keep the “oracle was the breakthrough” box.
8. **8:30 — Open Act II: make the project’s real scope feel ambitious.**
   Trim: read the four column titles.
9. **9:45 — Teach propose → compare → reduce → keep.**
   Trim: read one sentence per verb.
10. **11:00 — Make reduction the unit of AI-assisted work.**
    Trim: read the five-step chain.
11. **12:10 — Explain vertical proof slices.**
    Trim: use full parse and edit loop only.
12. **13:25 — Generalize safe reuse: optimizations must earn their way in.**
    Trim: keep the warning and fallback lesson.
13. **14:50 — Show the grammar DSL; explain the artifact pipeline.**
    Trim: read the `g.Define` block, then author → normalize → generate → ship.
14. **16:00 — Separate runtime, grammar, and product ownership.**
    Trim: read the three owners and the note.
15. **17:40 — Explain why the harness ratchets; close Act II.**
    Trim: keep parity, corpus, and benchmark gates.
16. **18:50 — Open Act III: prove the foundation through downstream products.**
    Trim: name GoSX and qml-language-server only.
17. **19:50 — Land the live demo: this deck is the stack.**
    Click the tree once; if it does not respond, point and continue.
18. **22:20 — Give the audience the reusable massive-project playbook.**
    This is the protected teaching slide; do not trim.
19. **23:20 — Resolve the method over the live galaxy.**
    Trim: use the final two paragraphs of notes.

## Visual fallback

The active starfield and closing galaxy are atmosphere, not a demo dependency.
If WebGL is unavailable, continue without comment: the local CSS starfield and
closing image preserve contrast and composition. On slide 17, the parse tree
renders statically even if clicks fail—point at the node names and keep
moving. Do not troubleshoot on stage.

## Precision rules

- Say **“CGo”** as “see-go”; zero CGo is a deployment claim, not a speed claim.
- Say **“syntax-aware”**, not “semantically safe,” for structural edits.
- Do not imply an AI-generated result is validated merely because it compiles.
Reference behavior, a minimized witness, and a retained regression test are the evidence.
- Do not imply a query resolves symbols, a grammar guarantees consumer
  compatibility without tests, or unchanged bytes alone permit incremental
  reuse.
- On slide 17, say the code on the left is the island source **trimmed to the
  mechanism**; do not present trimmed code as complete.
- On slide 18, slow down. The project story exists to make that playbook
  credible and stealable.

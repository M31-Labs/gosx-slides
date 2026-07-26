# GopherCon 2026 rehearsal system

The objective is not to memorize 25 minutes of prose. Memorize the opening,
the close, and the bridges. Know the technical evidence well enough to speak
naturally between them.

## Non-negotiable timing

- Slot: 25 minutes.
- Target finish: **24:30**.
- Hard stop: **24:45**.
- No audience questions from the stage. Invite conversations afterward.
- Arrive at the Main Theatre by **3:15 PM** on Wednesday, August 5.

## Checkpoints

| Checkpoint | Target |
| --- | ---: |
| Live tree begins | 3:10 |
| Why pure Go begins | 5:40 |
| Oracle act begins | 8:25 |
| grammargen act begins | 14:30 |
| Language portfolio begins | 17:45 |
| Deck-runs-itself proof begins | 20:55 |
| Closing question begins | 23:15 |
| Finish | 24:30 |

If a checkpoint is more than 30 seconds late, use the valve for that act. Do
not accelerate the final two slides.

## Timing valves

- Slides 3–4: remove the navigation and refactoring examples; preserve “useful
  structure before validity.”
- Slide 5: perform only the three demo beats—outer structure, honest
  uncertainty, completed expression.
- Slides 6–7: keep the product-boundary tradeoff; cut the second deployment
  example.
- Slides 11–12: keep byte ranges and permanent witnesses; cut the string-node
  hypothetical.
- Slide 14: name the three benchmarks without explaining each one.
- Slide 17: explain NFA/DFA and LALR/GLR as two grouped ideas.
- Slide 18: say “do not memorize this” and name one example per group.
- Slide 20: name the five boundaries without describing every product surface.

Never cut slides 2, 9–10, 13, 19, 21, or 23. They carry the M31 Labs thesis,
independent-evidence story, live product proof, and invitation.

## Rehearsal calendar

### July 21 — table read

Read the transcript aloud without slides. Mark any sentence that cannot be said
comfortably in one breath. Goal: conversational language, not speed.

### July 23 — first timed run

Run the full deck with the live tree. Do not stop for mistakes. Record total
time and every checkpoint. Goal: learn the real shape of the talk.

### July 25 — technical precision run

Pause after every technical claim and answer: What artifact proves this? What
qualification does it require? Reconcile wording with the frozen repo and the
GopherCon freeze checklist.

### July 27 — camera run

Record from audience eye level. Stay behind a podium-sized boundary. Review at
1.5× speed for filler, repeated setup, downward gaze, and sentences that sound
read rather than spoken.

### July 29 — demo failure run

Disable or skip the live interaction and deliver the fallback cleanly. Then
restore it and practice the successful path. Goal: no visible panic and less
than ten seconds lost to the fallback decision.

### July 31 — freeze run

Use the frozen deck, repo tag, offline bundle, and backup PDF. No copy edits
after this run unless they fix a factual error, timing failure, or delivery
problem observed twice.

### August 2 — invited audience run

Use two or three technically opinionated listeners. Ask them to write down:

1. the central claim;
2. what M31 Labs does;
3. the least credible sentence;
4. the moment they became most curious; and
5. the question they would ask afterward.

Do not defend the talk during feedback. Look for repeated confusion.

### August 4 — travel-room run

One calm run at speaking volume with the actual laptop, adapter, power supply,
advancer, offline bundle, and PDF. Stop changing the talk afterward.

### August 5 — stage check

Use the available rehearsal window or tech check for the first slide, last
slide, audio, confidence monitor, countdown clock, live tree interaction, and
video playback if any. Do not spend the ten-minute stage window delivering the
middle of the talk.

## Three-beat live demo

1. **Useful outer structure:** select the incomplete function declaration and
   name its type, range, and children.
2. **Honest uncertainty:** distinguish `ERROR` from zero-width `MISSING`.
3. **Incremental change:** complete the expression, make one small edit, and
   show that the rest of the tree remains useful.

Fallback decision: if the first interaction does not respond immediately,
advance to the static state and say, “The interaction is optional; the tree is
the artifact.” Continue without troubleshooting on stage.

## What to memorize verbatim

Opening:

> Most parser demonstrations begin with valid code. That is a little
> dishonest. The moment our tooling earns its keep is when the program is half
> typed, half wrong, and still moving.

Oracle turn:

> The C runtime is the compatibility target. Its most valuable property is that
> it can disagree with me.

AI turn:

> AI expands the search. The parity gate decides which results survive.

Deck reveal:

> The presentation is inside the presentation.

Final turn:

> Your team already sees the structure. Your tools still see text. What would
> change if they could see it too?

## Hallway question bank

Practice answers of 20–40 seconds, then ask a question back.

1. Why not keep using CGo?
2. Is pure Go faster?
3. What exactly does parity compare?
4. Why call the C runtime an oracle if it can have bugs?
5. How complete is grammargen?
6. Are all 206 grammars equally mature?
7. Where did AI help, and where was it unsafe?
8. How does GoSX combine Go and markup in one file?
9. What would structural editing let an agent verify?
10. What does M31 Labs take on for clients?

For question 10:

> We work on language systems, code intelligence, reliable agent
> infrastructure, and difficult Go product boundaries. The best fit is a team
> with a concrete structural problem and no desire to spend a year inventing a
> language-tooling practice before solving it.

Then ask: “What are you trying to make your tools understand?”

## Run scorecard

Score each dimension from 1 to 5. A run is green only when timing and factual
precision are both at least 4.

| Date | Total | Opening | Narrative | Precision | Demo | M31 clarity | Delivery | Close | Notes |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
|  |  |  |  |  |  |  |  |  |  |

Definitions:

- **Opening:** earns attention within 30 seconds.
- **Narrative:** every act creates the need for the next.
- **Precision:** claims match frozen artifacts and qualifications.
- **Demo:** three beats land without explanation debt.
- **M31 clarity:** a listener can explain what M31 Labs does without hearing a
  sales pitch.
- **Delivery:** pace, pauses, eye line, and voice remain controlled.
- **Close:** the room is left with a question and a natural next step.

## After every run

1. Record total time and checkpoint deltas.
2. Write the three moments that felt least natural.
3. Fix only the smallest cause: copy, transition, visual, or delivery.
4. Repeat the affected two-slide transition once.
5. Do not immediately run the entire talk again.

The final week is for stability. Confidence comes from repeated recovery, not
from pretending nothing can fail.

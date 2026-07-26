# GopherCon 2026 opportunity campaign

Talk: **Pure-Go Tree-sitter**  
Speaker: Oscar Villavicencio, founder of M31 Labs  
Stage: Wednesday, August 5, 2026 at 4:15 PM PDT  
Room: SCC Summit, Level 5, Finneran Ballroom 2

## Business outcome

The talk must teach first. The campaign makes it easy for the right people to
recognize a fit and continue the conversation.

Working targets for August:

- 8 qualified conversations with people who have a real language-tooling,
  developer-infrastructure, agent-reliability, or structural-tooling problem.
- 3 follow-up calls booked by August 14.
- 1 paid discovery, consulting engagement, partnership, or serious hiring
  process created by August 31.

A conversation is qualified when there is a concrete problem, an owner, a
credible timeline, and a plausible budget or role. Stars, impressions, and QR
scans are signals, not outcomes.

## Positioning

M31 Labs builds systems that understand software as structure rather than
undifferentiated text.

The talk supplies three kinds of evidence:

1. **Engineering depth:** a pure-Go Tree-sitter runtime and grammar compiler.
2. **Verification discipline:** an independent C oracle, permanent witnesses,
   and separate correctness and performance gates.
3. **Product range:** GoSX, Markdown++, Sirena, and this live deck all use the
   same structural foundation.

The commercial translation is simple:

> If your team has a language, toolchain, or agent workflow that still depends
> on strings, regexes, or fragile conventions, M31 Labs can help turn that
> hidden structure into a working system.

## Who the campaign is for

- Go engineering leaders shipping CLIs, developer platforms, editors, build
  systems, or cross-platform products.
- AI engineering leaders who need agents to make reviewable, testable changes.
- Developer-tool founders who need parser, compiler, DSL, or code-intelligence
  expertise without building a language team first.
- Hiring managers seeking principal-level Go, language-tooling, or applied-AI
  engineering leadership.

Do not flatten these audiences into one generic pitch. Name the problem that
is relevant to the person in front of you.

## Conversion path

```text
useful public artifact
        ↓
talk or technical conversation
        ↓
specific problem recognized
        ↓
m31labs.dev/build#contact
        ↓
reply within one business day during conference week
        ↓
25-minute fit call
        ↓
paid discovery, engagement, partnership, or hiring process
```

Before campaigning, verify that the QR opens the anchored form on iOS and
Android, the form submits successfully, the Postgres inbox records the message,
and a human notification makes a one-business-day reply realistic. Add a
durable `gophercon-2026` source field before changing the QR if attribution
matters; do not infer attribution from message text.

## Publishing calendar

### Tuesday, July 21 — establish the thesis

Publish the announcement below. Pin it wherever possible. Update personal and
M31 Labs profiles so the first line explains the work in the same language as
the talk.

### Friday, July 24 — teach one useful idea

Show the incomplete Go example and explain why useful structure before validity
matters. End with the talk details, not a commercial ask.

### Monday, July 27 — show the verification discipline

Publish the oracle diagram. Explain that AI increased throughput while the
independent reference supplied evidence. This is the strongest credibility
post for engineering leaders.

### Thursday, July 30 — reveal the toolchain

Show the two roads into one grammar blob and one real GoSX source file. The
message is that gotreesitter became a language toolchain, not that M31 Labs has
a long list of unrelated projects.

### Monday, August 3 — make meeting easy

Post that Oscar will be at GopherCon and name the problems he would enjoy
comparing notes on. Invite direct messages and brief hallway conversations. Do
not publish an artificial calendar-scarcity claim.

### Wednesday, August 5 — talk day

- Morning: one short reminder with the time and room.
- One hour before: stop campaigning and switch entirely to stage preparation.
- After the talk: publish the repository, contact link, and one photograph or
  short clip from the room if available.

### August 6–7 — follow up while memory is fresh

Send a personal note to every qualified conversation. Mention the specific
problem discussed and propose a short next step. Do not send a generic blast.

### August 10–14 — turn interest into decisions

Run fit calls. For consulting prospects, end with a concrete next artifact: a
paid discovery outline, a scoped technical assessment, or a clear no-fit. For
hiring conversations, establish role, authority, compensation range, and
process before doing unpaid project work.

## Ready-to-publish copy

### Announcement

Most parser demos begin with valid code. The useful moment is when the program
is half typed, half wrong, and still moving.

At GopherCon 2026, I’ll show why I rebuilt the Tree-sitter runtime in pure Go,
how the original C runtime became an independent behavioral oracle, and how a
self-hosted grammar compiler turned one project into a language-tooling stack.

The talk is also a little unusual: the deck is running the M31 Labs stack it
describes—Markdown++, GoSX, Scene3D, offline bundling, and a PDF backup.

Wednesday, August 5 · 4:15 PM · Finneran Ballroom 2

If your tools still see a language as text, I suspect we’ll have something to
talk about.

### Broken-code post

```go
func Handle(req *http.Request) {
    result :=
```

A compiler can reject this. An editor still has to answer: What am I inside?
What belongs here? What changed?

That is the Tree-sitter contract I care about: useful structure before valid
code. I’ll make the tree interactive during my GopherCon talk and show exactly
what survives while the program is incomplete.

### Oracle post

AI can help produce parser code quickly. It cannot be the evidence that the
parser is correct.

gotreesitter asks the original C runtime the same structural question, compares
the answers node by node, and keeps every fixed disagreement as a permanent
witness.

AI raises throughput. The independent reference supplies evidence.

That distinction is one of the three ideas I’m bringing to GopherCon.

### Toolchain post

I set out to run Tree-sitter grammars without CGo. Then the parser learned to
make parsers.

ts2go carries existing generated grammars into Go. grammargen creates new
grammars without a C ancestor. Both produce the same runtime representation.

That mechanism now powers GoSX, Markdown++, Sirena, and the GopherCon deck
itself. The language-tooling story became much larger than the original port.

### Conference availability post

I’ll be at GopherCon this week and would especially enjoy meeting people who
are working on:

- parsers, compilers, DSLs, or code intelligence;
- reliable agent-driven code changes;
- Go products that need to cross difficult deployment boundaries; or
- developer tools whose real structure is still trapped in strings and
  conventions.

No pitch required. I’m happy to compare notes. If there is a genuine fit for
deeper work, we can discover that naturally.

### Post-talk note

Thank you to everyone who spent 25 minutes thinking about broken code, pure Go,
behavioral oracles, and inexpensive grammars with me.

The repository is `github.com/odvcencio/gotreesitter`.

If the talk made you think of a language or structural problem your tools still
cannot see, tell me about it at `m31labs.dev/build#contact`.

## Conversation scripts

### Ten-second introduction

> I’m Oscar. I run M31 Labs, where I build language tooling and structural
> systems in Go. My GopherCon talk is about the pure-Go Tree-sitter toolchain
> underneath that work.

### Twenty-second commercial bridge

> The open-source work is also how M31 Labs proves the kind of engineering we
> take on: parsers and DSLs, code intelligence, reliable agent infrastructure,
> and difficult Go product boundaries. If your team has a concrete version of
> that problem, I’d be glad to compare notes and see whether there is a fit.

### Graceful non-fit

> That is adjacent to our work, but I do not want to pretend it is already in
> our wheelhouse. I can still point you toward the parts of the project that
> may help.

## Lead log

Record only professional context willingly shared in the conversation.

| Person | Organization | Problem | Opportunity type | Urgency | Next step | Due |
| --- | --- | --- | --- | --- | --- | --- |
|  |  |  | consulting / hiring / partnership / OSS |  |  |  |

## Weekly scoreboard

| Measure | Jul 21–26 | Jul 27–Aug 2 | Aug 3–9 | Aug 10–16 |
| --- | ---: | ---: | ---: | ---: |
| Useful posts published |  |  |  |  |
| Qualified conversations |  |  |  |  |
| Follow-up calls booked |  |  |  |  |
| Scoped opportunities |  |  |  |  |
| Paid or hiring processes opened |  |  |  |  |

The campaign succeeds when the right people understand the work and know how
to continue. Attention without a next conversation is not the goal.

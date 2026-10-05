# Semantic request story

Run `slides serve examples/semantic-story` from the repository root. Open
`#request/accepted` or advance with the arrow keys. The shared motion studio
can pause, seek, and reverse the authored beat.

`deck.md` owns stable slide and cue names. `request.sir` owns actors and
relationships. `story.yaml` expresses each narrative pose once: focus, reveal,
relationship path, camera, code lines, DOM visibility, and an authored caption.
The compiler creates native scene steps in memory and leaves all source files
unchanged. `inspect --json` includes the compiled story and its original source
ranges.

A missing effect restores its baseline at that cue. Omitted `reveal` shows all
actors; `reveal: []` hides every actor. Each `trace` is a directed path through
declared relationships. Code blocks are zero based and code lines are one based.
DOM targets use `id` or `data-story-id`; ambiguous identities are rejected.
Camera poses need a Sirena `Scene3D` surface. Existing unwrapped slides have one
story surface, either a native scene or SVG diagram (including sequential states
inside `diagram-morph`). Named `:::story-surface {name=map}` containers allow up
to eight independent surfaces under one playhead; see
[the multi-surface example](../multi-surface-story/README.md). Story poses and an
authored `Steps` file remain exclusive for each native scene.

The YAML `expect` clauses test canonical visibility and labels. The Go API
`AssertStory` also validates named Markdown links and local links/snippet sources.
`AssertStoryBrowser` tests rendered expectations and repeatable forward,
backward, and midpoint poses through local Chrome. It checks at most 100 beats
with a two-minute browser deadline. External URLs are not fetched by assertions.

`ArchitectureTour(before, after)` accepts parsed Sirena snapshots and returns
a ready Markdown diagram morph plus its story YAML. Stable `sid` identities
let it explain actor changes, boundary moves, and dependency changes without
treating source-position changes as architectural changes.

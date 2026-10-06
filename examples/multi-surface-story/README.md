# Three perspectives, one authored clock

Run `slides serve examples/multi-surface-story`, then open `#pressure/burst`.
The request map, changing pressure chart and native 3D view share named cues,
captions, code highlights and the motion studio's absolute playhead. Pressure
values are an explicitly authored model, not observed measurements.

Each `:::story-surface {name=map}` container holds exactly one Sirena SVG,
`diagram-morph`, or `<Scene3D Src="runtime.sir" />` surface. Names are unique
within a slide, start with an ASCII letter and contain at most 64 letters,
digits, underscores or hyphens. Nested surfaces and unwrapped Sirena diagrams
on a named-surface slide are rejected. At most eight surfaces share a slide.
Compiled story metadata is bounded to 16 MiB.

The existing version-1 story manifest adds an optional `surfaces` map:

```yaml
surfaces:
  map:
    focus: [api, buffer]
    reveal: [browser, api, buffer]
    trace: [browser, api, buffer]
  runtime:
    focus: [buffer]
    camera: {x: 0, y: 0, z: 14, fov: 45}
expect:
  visible: [map/buffer, runtime/buffer]
  hidden: [map/worker, runtime/worker]
```

Actor IDs inside a pose are local to its named surface. Actor expectations use
`surface/actor`, so two surfaces may reuse `api` while carrying different labels,
source ranges and visibility. Unknown surfaces, unqualified actor expectations
and cameras on SVG surfaces fail compilation. Shared DOM targets, code blocks,
captions and duration remain beat-level fields. Existing one-surface decks keep
their unqualified `focus`, `reveal`, `trace` and `camera` fields unchanged.
Browser metadata carries semantic names, IDs and labels; source paths/ranges
remain in CLI inspection rather than audience pages. Notes never become captions.

Omitted surface poses restore that surface's baseline; omitted `reveal` shows
all of its actors, while `reveal: []` hides them all. Each directed trace must
exist in that surface's parsed graph. Native surfaces using the same `.sir`
file still receive independent step programs. Authored native `Steps` and
generated story poses remain mutually exclusive for each scene.

Named SVG morphs always reconstruct the previous authored pose for their
transition, including direct-link startup and backward navigation. Reverse
seek and repeated midpoint samples therefore use the same geometry. Reduced
motion settles the addressed pose immediately. A diagram-morph still uses its
authored container duration/easing; match that duration to the narrative beat
when both should finish together.

Run `slides story assert examples/multi-surface-story --browser --json` for
rendered assertions. Developer regression coverage is
`node scripts/multi-surface-story-browser.cjs ./slides`; it also exercises two
native scenes sharing one source, direct-link reload and reduced motion.

## Revision tours and explicit explanations

`ArchitectureTourHistory` accepts 2–32 ordered `ArchitectureSnapshot` values
with stable `ID`, explicit `Sirena` bytes and optional `Label`/`Source` metadata.
It returns a runnable Markdown/story pair, stable `revision-<id>/before|after`
addresses, structural transition diffs and SHA-256 provenance. It never fetches
files or URLs. Source metadata remains in the report rather than public captions.

`ArchitectureTourOptions.Curation` supplies a version-1 explanation map keyed
by the incoming revision ID. `BeforeCaption` and `AfterCaption` are string
pointers: omitted values use generated explanations; explicit empty strings stay
blank. Optional `FromID` binds curation to its predecessor, failing if history
is reordered. Regeneration copies explicit explanations unchanged, retains
orphaned entries and reports them for reconciliation. It preserves this explicit
curation contract, not arbitrary edits to generated Markdown.

Each snapshot is at most 1 MiB; total source is at most 4 MiB. Revision IDs are
unique ASCII identifiers of at most 48 characters. Curation holds at most 64
explanations with captions of at most 4096 bytes. Stable Sirena `sid` metadata
preserves actor identity through renames; the API performs no code inference.
Generated Markdown and story files each stay within the 1 MiB authoring limit;
large histories fail rather than produce a deck the source editor cannot edit.

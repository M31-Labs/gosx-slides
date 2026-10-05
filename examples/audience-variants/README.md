Untagged slides are shared. Per-slide `audiences:` accepts a comma list or a
YAML sequence of strings. Deck-level `audiences:` optionally declares the valid
names; otherwise they are discovered from tagged slides. Names are case sensitive
and use the stable slide ID syntax: a letter, then letters, digits, `_` or `-`,
up to 64 characters total.

This source has five slides. `engineers` selects introduction, technical, and
conclusion; `leaders` selects introduction, business, and conclusion; `workshop`
also includes the exercise. The technical slide comes from an included file.

```go
deck, err := slides.LoadIslandDeckAudience("examples/audience-variants", "engineers")
// Or select from a complete, already loaded deck:
complete, err := slides.LoadIslandDeck("examples/audience-variants")
variant, err := slides.SelectAudience(complete, "leaders")
```

Use `SelectAudience` on the complete deck, loaded with `LoadIslandDeck`; an
already selected variant must be reloaded before choosing another audience.
`DeckAudiences` lists valid names. Empty, unknown, malformed, and slide-free
selections fail. Stable slide IDs and cues survive filtering. Numeric authored
slide links remap to selected numbering. Links into omitted slides fail selection
with an `AudienceSelectionError` containing original file/range diagnostics.

Selection preserves author source and included-file coordinates, compiles the
selected components, and filters presenter notes and semantic story timelines.
It adapts rendered slide content; it is not an asset access-control mechanism.
Assets in `public/` retain their ordinary serve/export behavior.

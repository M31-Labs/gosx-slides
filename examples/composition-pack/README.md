# Reusable content and local packs

Run from the repository:

```sh
go run ./cmd/slides serve examples/composition-pack
go run ./cmd/slides doctor examples/composition-pack
```

A block HTML comment includes a complete Markdown fragment, with optional named
section selection:

```md
<!-- slides:include sections/workflow.md -->
<!-- slides:include sections/library.md#closing -->
```

Name reusable sections with `<!-- slides:section closing -->`. A section ends at
the next section marker or the end of its file. Included files contain ordinary
Markdown and slide YAML fences, without deck headmatter. Nested includes resolve
relative to their author file, remain inside the deck, and reject cycles.

Relative images, HTML assets, source snippets and graphics source props resolve
from the included file's directory. A fragment's `<Component/>` resolves a sibling
`Component.gsx`; a deck-level file overrides it. Two conflicting fragment
definitions fail with a diagnostic. Asset URLs are registered individually for
serving and copied into static bundles. Source mappings retain the original
filename and byte range. Browser text saves retain the root include comments;
included motion directives are not offered as deck.md range edits.

The headmatter pin `packs: labs@1.0.0` loads `packs/labs/pack.json`. Its schema
version, name and exact semantic version must match. The manifest explicitly
lists CSS, custom layouts and GoSX components. `theme: labs` selects the pack's
built-in `baseTheme`; pack CSS then overrides its tokens, and deck CSS overrides
pack CSS. Components in the deck or a fragment take precedence over pack defaults.

To reuse the pack, vendor the complete `packs/labs` directory into another deck,
or call `slides.InstallDeckPack(deckDir, sourceDir)`. The installer validates the
manifest and CSS assets, rejects symlinks, refuses existing destinations, and
executes no hooks. Add the printed exact pin to that deck's headmatter. Pack CSS
must be self-contained; relative `url(...)` images/fonts are carried into exports,
while `@import` is rejected. Author GoSX component asset URLs explicitly using
`/public/`, or place visual asset references in pack CSS.

Copy the entire example directory inside a Go module requiring GoSX to retain
live hydration. Scaffolded portable decks already contain the needed go.mod.

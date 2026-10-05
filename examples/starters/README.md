# Curated starters

Use `slides templates` to browse the catalog, then create a fresh portable deck:

```sh
slides init my-review --template architecture-review
slides init my-talk --template technical-talk
slides init my-lesson --template teaching
```

The same authored examples run directly from this checkout:

- `slides serve examples/starters/architecture-review` — review a boundary, trace a request and record a decision.
- `slides serve examples/starters/technical-talk` — explain a mechanism with stepped code and inspectable evidence.
- `slides serve examples/starters/teaching` — predict, replay a seeded experiment and compare a branch.

Each ships a pinned local `studio` pack. Edit `packs/studio/brand.css` for colors and `studio.css` for typography and spacing. All resources are local. Replace example claims, presenter notes and attribution before presenting; the evidence slides contain criteria rather than invented measurements.

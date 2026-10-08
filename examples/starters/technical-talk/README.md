# technical-talk

Run `slides serve . --edit` in this directory, then open the printed URL. Replace the example content and speaker notes before presenting.

Edit `packs/studio/brand.css` for your brand colors and `packs/studio/studio.css` for typography and spacing. Keep stable slide IDs and cue names when sharing deep links.

The architecture starter binds its diagram and code with `story.yaml`; the teaching starter bakes a seeded model from `simulation.yaml`. These files are local and exports run offline. Code on slides is displayed; it is not executed.

## Edit, present and share

Press **E** to edit source, **M** for the motion studio, **P** for presenter view,
and **?** for shortcuts. Save drafts before reloading the browser. Keep the
server terminal open while presenting; **Ctrl+C** stops it.

Run these commands from another terminal in this deck directory:

```sh
slides doctor .
slides validate .
slides build . --out dist
slides export . --format pdf --capture --steps --out talk.pdf
```

Publish the whole `dist/` folder through HTTP for live interaction. PDF capture
needs Chrome or Chromium; `SLIDES_CHROME` selects its executable. `--steps`
includes every click state as a still. Notes are private unless explicitly
included in a supported export with `--notes`.

[First-deck walkthrough](https://github.com/M31-Labs/gosx-slides/blob/main/docs/getting-started.md) ·
[Troubleshooting and export choices](https://github.com/M31-Labs/gosx-slides/blob/main/docs/troubleshooting.md) ·
[Shader and motion cookbook](https://github.com/M31-Labs/gosx-slides/blob/main/docs/shaders-and-motion.md)

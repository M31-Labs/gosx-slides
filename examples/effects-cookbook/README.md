# Effects cookbook

[Read the exported PDF](../../docs/examples/effects-cookbook.pdf) or run this
deck from the repository root:

```sh
slides serve examples/effects-cookbook --edit
```

Five slides cover a custom background, word entrances, cue sequencing, a shader
sphere and a tuned preset. The sequence has three states (`overview`, `idea`,
`detail`); open `#sequence/detail` to jump to the last. **M** opens the motion
studio, **E** edits source, and **Backgrounds** changes the background.

The complete [shader and motion guide](../../docs/shaders-and-motion.md) explains
each technique. The opening and sphere share `shaders/ribbons.sel`; change its
parameters to customize both. `shader-*` metadata only tunes bundled presets,
so the final slide's silk controls do not change that custom file.
The sphere overrides colors, strength and scale through `shaders/shape.json`
so a quiet background can become a more visible foreground material.

```sh
slides export examples/effects-cookbook --format pdf --capture --steps --out cookbook.pdf
slides build examples/effects-cookbook --out dist/cookbook
```

The PDF has seven still pages. The web export retains live effects. Decorative
backgrounds disappear under reduced motion, leaving the ordinary deck color;
entrances settle immediately. No island component or remote asset is required.

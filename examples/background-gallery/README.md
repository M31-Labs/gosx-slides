# Selena backgrounds

Run `slides serve examples/background-gallery --edit` and open **Backgrounds**
in the presentation toolbar. Choose a template, tune colors/motion, and apply it
to the current slide or deck default. Previewing never writes source. Applying
uses validated, revision-checked project saves and retains a recovery copy.

No shader files or GoSX islands are needed for bundled presets. Copy an editable
shader with `slides backgrounds source aurora > shaders/aurora.sel` (create the
`shaders/` directory first), or use **Download .sel** in the wizard to retain
your custom settings. Point `scene:` at that file when using a custom shader.

Set `shader-speed: 0` for a still. Reduced motion hides decorative backgrounds,
using the normal deck theme. Capture export is needed to bake native pixels
into single-file/PDF output; SPA builds retain live backgrounds.

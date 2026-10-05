# Office interoperability example

This deck declares `aspect-ratio: 4:3`. Captured exports and PowerPoint use a
960 × 720 viewport unless explicit export dimensions or an aspect override are
provided. Tables and Sirena bar/pie charts are intentionally simple enough to
become native Office objects in editable PowerPoint export.

`ExportOptions{Format: "pptx", Editable: true}` creates editable table cells,
chart series, and an embedded Excel workbook. Chart data comes from the values
visible in Sirena's accessible SVG labels; PowerPoint applies its own layout,
so editable charts do not promise pixel fidelity. Unsupported content retains
captured pixels. Use ordinary captured PPTX for the original appearance.

`ExportOptions.PPTXTemplate` reuses the first slide master's self-contained
theme (palette, fonts, formatting). It does not copy the template's slides,
masters, positions, layouts, embedded content, or background artwork.

`ImportPPTX(source, destination)` writes a new portable deck directory containing
`deck.md`, `go.mod`, `public/` assets, and `import-report.json`. It recovers common
slide text, native tables, cached chart data, PNG/JPEG images and speaker
notes in the presentation's relationship order. Imported content requires
layout review. Unsupported shapes, SmartArt, external media, cropping, motions
and macros are reported or omitted; original Office layout is not reconstructed.
The destination must be absent and its parent directory must exist. Existing
author work is never overwritten. External links are never fetched.
GIF and animated PNG images become first-frame PNG snapshots with a report entry.

# The motion workshop

Run `slides serve examples/authoring-lab` from the repository root. Press **M**
to preview timing and **R** to check readability at your current viewport.
Arrow keys advance named cues and code versions; `#pipeline/worker` is a stable
link to the worker beat. Backward navigation restores each absolute step.

The opening and destination share a morph identity. Visit the destination,
change its counter, and return: the same island keeps its state. The later
counters hydrate when active or next in idle time. Inspect `SlidesRuntime.stats()`
to see which instances remain deferred.

Motion studio changes are temporary. Copy the directive back into `deck.md`
to keep them. Shared element morph duration comes from slide YAML
`morph-duration: 600`; add `replay=once` or `replay=step` to choose entrance behavior.

Export all beats with `slides export examples/authoring-lab --format pdf --steps
--out workshop.pdf` (one command line). Captures need Chrome/Chromium; silent
WebM video also needs ffmpeg. See the root README for complete export commands.

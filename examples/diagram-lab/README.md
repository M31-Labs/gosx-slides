# Diagrams that explain

Run `slides serve examples/diagram-lab`. The first five slides demonstrate native
Sirena state, class, ER, swimlane, and numeric timeline layouts. Use
`sirena diagram=class` (or `sirena class`) as the fence language to select a layout.
The same declarations retain their IDs and relationships across layouts.

The last slide imports Sirena Scene3D props with focus, reveal, and trace actions.
Arrow keys navigate absolute poses; `#workflow/worker` restores the worker cue
on arrival. The scene uses a compiled Selena material shared by API and worker.

Regenerate its scene from the Sirena repository using:

```sh
sirena render --scene3d --shader examples/scene3d/material.sel \
  --targets api,worker --steps examples/scene3d/choreography.json \
  -o workflow.scene.json examples/scene3d/request.sir
```

Press M to pause/play active graphics. Export per-step PDF with `--steps` or PNG
frames with `--format frames --steps`; captured formats preserve rendered pixels.

This deck uses a Sirena diagram exported to native GoSX Scene3D props.

Run `slides serve examples/sirena-scene`. Advance through the diagram's three
click steps before moving to the second slide. Reverse navigation restores the
same absolute frame, including labels. The API and worker use Selena materials;
the remaining nodes retain their typed shapes and standard materials.

Regenerate `request.scene.json` from Sirena's `examples/scene3d` sources:

```sh
sirena render --scene3d --shader examples/scene3d/material.sel \
  --targets api,worker --steps examples/scene3d/steps.json \
  -o request.scene.json examples/scene3d/request.sir
```

The scene uses stable diagram identities, bounded rendering, and shared shaders.
Labels follow focus transforms; relationship routes retain their original layout.

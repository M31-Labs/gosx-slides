# Request, failure and recovery

Run `slides serve examples/request-recovery --edit` and open `#request/accepted`.
Arrow keys advance the system and its code together. **M** opens the shared
transport: pause, seek 0/600/1200 ms, reverse, or replay. Seeking 600 ms again
reconstructs the same pose, independent of your navigation history.

The Scene3D material is compiled from Selena. Authored camera and actor changes
use absolute Sirena keyframes; labels, routes and arrowheads follow their actors.
The four click states share stable cue addresses, including `#request/failure`.
Each state authors its camera field of view in `steps.json`, keeping the full
path legible through fitted desktop and mobile layouts. Change `fov` to control
the framing; changes interpolate with the same playhead as the actors and code.

Regenerate the scene with Sirena v0.7.0 or newer:

```sh
cd examples/request-recovery
sirena render --scene3d --shader material.sel --material Pearl --targets api \
  --steps steps.json -o request.scene.json request.sir
```

The transport controls declarative animation and shader time. Stateful particle,
water and event-driven glTF simulations require their own state replay.

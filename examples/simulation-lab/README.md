# Repeatable particles

Run `slides serve examples/simulation-lab` from the repository root. Open the
motion studio with **M**, pause and scrub, or use the labeled branch/tick
controls. Named links `#particles/start`, `#particles/impulse` and
`#particles/settled` restore exact state ranges.

`simulation.yaml` owns the seed, tick rate, checkpoints, input log and authored
branch. Integer particle state includes positions, velocities and random state.
The wind branch preserves the baseline prefix through tick 60 and changes its
future input. Tick hashes and SVG geometry remain repeatable when seeking
forward, backward, switching branches and reloading.

The compiler precomputes bounded authoritative frames. Single/handout snapshots
keep this simulation interactive without WASM or network access; captured video
samples the same presentation playhead. This particle vocabulary demonstrates
the general `NewSimulationReplay` adapter over GoSX `sim.Simulation`; custom
models use that Go API and must provide fresh deterministic state.

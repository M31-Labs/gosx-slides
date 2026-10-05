# Architecture history tour

Generate, check and run a story about three versions of a service:

```sh
slides tour history examples/architecture-history/history.yaml --out my-tour
slides story assert my-tour --browser
slides serve my-tour
```

Every transition uses stable actor `sid` identities and explicit before/after
cues. Edit `my-tour/curation.json` to change captions, then regenerate into a
fresh folder:

```sh
slides tour history examples/architecture-history/history.yaml \
  --curation my-tour/curation.json --out revised-tour
```

Revision IDs survive appending snapshots. A changed predecessor is rejected
until its curated explanation is updated; orphaned explanations are retained in
the private report. Blank captions remain blank. Reports and curation stay out
of public exports.

To use real local repository history, replace snapshot entries with:

```yaml
snapshots:
  - id: initial
    revision: HEAD~2
    path: architecture/service.sir
  - id: queued
    revision: HEAD~1
    path: architecture/service.sir
  - id: resilient
    revision: HEAD
    path: architecture/service.sir
```

Then add `--repo /path/to/repository`. Commit hashes and content hashes are
recorded in `tour.json`. Git never fetches missing objects or runs textconv;
working changes do not affect committed snapshots. This reads authored Sirena
models, so code-to-model extraction remains your own upstream step.

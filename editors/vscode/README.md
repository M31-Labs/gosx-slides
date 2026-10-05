# GoSX Slides companion

Open a trusted local or Remote WSL workspace containing `deck.md`. Configure
`gosxSlides.binary` with a slides binary supporting `slides mcp`, then run
**GoSX Slides: Open Project**. The Explorer view lists original author files.

Files open through the `gosxslides:` filesystem. Ctrl/Cmd+S uses the same
validated, revision-checked project API as browser editing; source remains in
the original file and its prior revision is retained. A conflict keeps the
editor dirty. **Diagnose Project** reports project findings; typing shows
diagnostics for the current file with UTF-8 ranges converted for VS Code.
**Assert Story** runs static story assertions. **Export Handout** creates a new
snapshot without private notes or external capture tools.

The companion does not intercept saves to ordinary `file:` tabs. Open source
from the GoSX Slides view to use validated saves. Saving one tab checks its
draft alongside the other saved files. Cross-file transactions, completions,
drag-to-reorder, embedded live preview, HTTP MCP and web-extension support are
not implemented. This companion has no telemetry or installation hooks.

Build a VSIX with the official VS Code packager from this directory:

```sh
npx @vscode/vsce package --no-dependencies
code --install-extension gosx-slides-0.1.0.vsix
```

The repository publishes source for packaging; no Marketplace release is
claimed. See [project authoring](../../docs/project-authoring.md) for limits,
private path policy, MCP setup and reconciliation.

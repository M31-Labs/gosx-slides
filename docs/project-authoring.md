# Project authoring and local agent tools

`slides serve my-deck --edit` enables the project file picker in the existing
source editor. Edit original Markdown fragments, GoSX components, Sirena
diagrams, YAML/JSON story and simulation manifests, CSS and Selena shaders.
Project findings retain original filenames and UTF-8 ranges. Project outline
entries jump to original files; preview buttons stay local to the author tab.
Non-Markdown diagnostics use their native parsers. Parser/compiler failures
whose APIs supply no range point to the file's first line.

Open file tabs preserve independent drafts and undo histories. Saves validate
one draft with all other **saved** project files. A successful save preserves
other dirty tabs; otherwise the preview reloads. Closing the dialog preserves
drafts in that tab; reloading the browser discards them. **Reload source**
explicitly discards the current file draft and reads fresh revisions.

Every save requires both a SHA-256 of that file and a context SHA-256 covering
validation inputs and directory membership. Changes to an include, component,
manifest or asset cause a conflict even when the target file is unchanged.
Conflicts preserve the draft. Reconcile against current source and reload its
revisions before retrying. A known successful save rebases other open tabs
from the same previous context; it never masks unrelated external changes.

Validation runs in a private temporary project snapshot, without installation
hooks, compiler shell commands or author-file writes. It parses and compiles
the composed deck, components, graphics and declared story/simulation
manifests. New syntax/compiler errors block saves; an edited file must have no
remaining errors. Identical existing errors elsewhere may remain so files can
be repaired one at a time. Warnings remain visible. Saved
files use LF line endings. The writer retains the displaced inode under a
sibling `.slides-history-*/<filename>` and publishes through a no-overwrite
hard link, so writes through already-open descriptors remain recoverable.
As with deck.md editing, publication briefly removes the source path;
filesystems without hard-link support reject before moving the original.
Keep recovery directories until you have reconciled external edits.

Bounds are 512 validation files, 64 MiB total, 1 MiB per editable source and
16 MiB per supporting asset. The browser retains at most eight file drafts
and a shared 16 MiB undo budget. Discovery ignores symlinks, hidden files,
`build/`, `dist/`, `node_modules/`, `private/`, `secrets/` and key/certificate
files. Public assets and pinned packs may be copied for validation but are
not editable project sources. Store credentials outside the deck or in an
excluded private location, rather than in ordinary author JSON/YAML files.

The HTTP endpoint `/_slides/project` exists only with editing enabled and
uses the same editor role, authoring token, same-origin and GoSX session-CSRF
policy as deck.md editing. Audience and static servers cannot read project
source. `GET` lists files (and the authoring token), or reads `?file=…`;
`POST` diagnoses `{file,source}`; `PUT` saves
`{file,source,revision,contextRevision}`. Neither selection nor diagnosis
broadcasts presenter navigation. Project editing is unavailable for filtered
audience variants, which retain the existing edit/watch restriction.

## Stdio MCP

Start a local project tool server with `slides mcp my-deck`. Configure an
MCP client with an installed binary and an explicit project directory:

```json
{
  "mcpServers": {
    "gosx-slides": {
      "command": "/absolute/path/to/slides",
      "args": ["mcp", "/absolute/path/to/my-deck"]
    }
  }
}
```

The server speaks newline-delimited UTF-8 JSON-RPC and negotiates MCP
2024-11-05, 2025-03-26, 2025-06-18 or 2025-11-25. Initialize and send
`notifications/initialized` before calling tools. stdout contains protocol
messages only; diagnostics go to stderr. Messages are limited to 6 MiB plus
16 KiB, requests execute sequentially, and notifications never execute tools.
There is no HTTP transport, async progress, cancellation, arbitrary command
execution, browser driving, file creation/deletion or multi-file transaction.
The local client has author permissions and may read source and speaker notes;
MCP is not an audience distribution endpoint.

| Tool | Result |
| --- | --- |
| `slides_project_list` | Editable original files and context revision. |
| `slides_project_read` | Original source, file revision and context revision. |
| `slides_project_diagnose` | Original-file findings and composed slide outline; optional single-file draft. |
| `slides_project_write` | Validated save and recovery path; both read revisions required. |
| `slides_project_rename` | Unsaved mdpp scoped rename in deck.md; included/cross-file renames refused. |
| `slides_story_assert` | Static compiled story assertions; browser assertions remain a CLI operation. |
| `slides_resolve_address` | Validated stable slide/cue anchor; no navigation broadcast. |
| `slides_export_snapshot` | Actual `single` or `handout` export in a fresh private `.slides-export-*` folder, with notes excluded. |

Snapshot exports execute no external process. Chrome capture, live SPA,
recording, PDF, editable Office and narrated video remain available through
their existing CLI/browser workflows. Generated snapshot folders stay outside
project discovery and public assets.

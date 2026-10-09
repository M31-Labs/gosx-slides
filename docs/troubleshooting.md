# Troubleshooting and export choices

Start with `slides version` and `slides doctor my-talk`, replacing `my-talk`
with your deck's directory. `doctor` checks the deck and serving prerequisites;
it does not certify Chrome capture or every exported format. Keep the first
reported error and the command that produced it.

## The shell cannot find `slides`

Run the executable from inside the extracted release folder: `./slides version`
on macOS/Linux or `.\slides.exe version` in PowerShell. If that works, add that
folder to PATH using the [installation walkthrough](getting-started.md#1-install-the-whole-release-folder).
PowerShell needs the `./` or `.\` prefix for a program in the current directory.

For source installs, run `go env GOBIN GOPATH`: a nonempty `GOBIN` wins; otherwise
use `GOPATH`'s `bin` directory. To find a stale executable earlier on PATH, run
`command -v slides` on macOS/Linux or `Get-Command slides` in PowerShell.

## A release binary asks for Go or cannot load its runtime

Keep the archive's `runtime/` beside the executable. The executable and bundle
must come from the same release; upgrade them together. You can point
`SLIDES_RUNTIME_DIR` at that release's runtime directory when storing it
elsewhere. Check `slides doctor my-talk` again after restoring the bundle.

Ordinary `serve`, `serve --edit` and SPA `build` use the portable bundle.
`--watch` and `--rebuild` explicitly use Go. Source installations also use Go to
build the browser runtime; let the first build finish and keep the resulting
`build/` cache. A missing bundle is not fixed by installing Node.

## Chrome is not found

Install Chrome or Chromium. If the executable is not on PATH, set
`SLIDES_CHROME` to the executable file, then rerun the export. Do not include
command-line flags in this variable.

macOS, with Chrome in its usual Applications location:

```sh
export SLIDES_CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

Linux, when `chromium` is installed on PATH:

```sh
export SLIDES_CHROME="$(command -v chromium)"
```

Windows PowerShell, with Chrome in its usual machine-wide location:

```powershell
$env:SLIDES_CHROME = "$env:ProgramFiles\Google\Chrome\Application\chrome.exe"
Test-Path $env:SLIDES_CHROME
```

Use the actual installed path if yours differs. If Chrome cannot start its
capture endpoint, first check that the chosen executable runs with `--version`,
and that its platform/architecture matches your machine. A Windows executable
path is not a Linux executable path when working inside WSL; use a Linux browser
for the Linux CLI. For video, also install `ffmpeg`; narration needs `ffprobe`.

## An edit will not save

Browser source editing requires `slides serve my-talk --edit`. **Edit** is not
available in published static exports. `--watch` serves a different purpose:
watching files changed by an external editor.

Fix the error shown next to the affected source before saving again. A conflict
means a file or another validation input changed since the draft was opened.
Copy your draft somewhere safe, reconcile it against the current files, then
use **Reload source** to obtain current revisions. Reload source discards that
file's current draft; refreshing the browser discards all unsaved drafts in it.

Successful saves retain displaced source in `.slides-history-*` directories.
Keep those recovery copies until you have reconciled your edits. The
[project-authoring guide](project-authoring.md) explains multi-file drafts,
validation, recovery and the VS Code companion.

## Motion, backgrounds or cues look different than expected

| Symptom | Check |
|---|---|
| A reveal is missing after opening a slide | Open its named cue, such as `#idea/evidence`, or advance through its steps. The initial state may deliberately hide it. |
| An entrance only plays once | Remove `replay=once`, use `replay=slide` for every slide entry, or `replay=step` for click-step replays. |
| Decorative motion or a shader background is absent | Check your system/browser reduced-motion preference. GoSX Slides respects it; keep the slide readable without decoration. |
| A preset setting does not change a custom shader | `shader-*` controls tune `scene: shader:<preset>`. A file-backed `.sel` owns its parameters; edit that file or its explicit uniforms. |
| A named cue link fails | Use the slide's `id`, followed by one of its declared `cues`. Check both spellings and `slides validate my-talk`. |
| Two slides became one | Put blank lines around `---`. Per-slide settings belong in a leading `yaml` fence; deck headmatter belongs only at the top. |
| Controls stay on screen | Move the pointer away and allow the idle delay. Keyboard focus intentionally keeps the toolbar visible. |

Use **M** to inspect the current motion segment, **R** to check rendered
readability, and the [cookbook](shaders-and-motion.md) for complete examples.

## Choose an export format

| Goal | Command after `slides` | What survives |
|---|---|---|
| Interactive website | `build my-talk --out talk-web` | Live islands, motion, shader/3D rendering and cue navigation; publish the entire folder. |
| A readable document | `export my-talk --format handout --out talk-handout` | Static reading flow and text; native graphics use available fallbacks. |
| PDF with selectable text | `export my-talk --format pdf --out talk-text.pdf` | Browser print layout and static reveal content; native graphics use fallbacks. Needs Chrome. |
| PDF showing every rendered beat | `export my-talk --format pdf --capture --steps --out talk.pdf` | Image-based pages of shader/3D and cue states. Needs Chrome; no live motion or selectable slide text. |
| Clickable PDF walkthrough | `export my-talk --format pdf --steps --pdf-navigation --out talk.pdf` | Every rendered state plus internal Previous/Next links; slide pixels fit above a navigation strip. |
| Editable PowerPoint | `export my-talk --format pptx --editable --steps --out talk.pptx` | Supported native text, tables, SVG and charts; other graphics use captured images. Needs Chrome. |
| A movie | `export my-talk --format video --steps --seconds 2 --fps 30 --out talk.webm` | Sampled presentation motion in WebM, without live interaction. Needs Chrome and ffmpeg. |

Use a fresh web/handout output directory for publication. Speaker notes remain
private by default; `--notes` includes them in supported exports. Editable PPTX
is a supported subset, so inspect the result in your target Office application.
For recording, narration, captions and dimensions, see the
[capability reference](reference.md#local-recording-and-narrated-exports).

### My captured PDF is missing later reveals

Add `--steps`. A capture without it takes the initial state of each slide,
which may intentionally hide cue content. `--steps` includes initial and later
states and implies capture. Ordinary printing exposes static reveal content,
but does not preserve native graphics in the same way. Neither PDF route
retains animation; choose the live web output or video when movement matters.

### Can a PDF play transitions?

`--steps --pdf-navigation` gives readers a clickable sequence of still states,
including intermediate reveals. It does not embed GoSX animations, shaders or
interactive islands. Some readers support their own page-transition effects;
[Acrobat documents those for full-screen presentations](https://helpx.adobe.com/acrobat/using/setting-pdfs-presentation.html).
Those effects depend on the reader and are not authored by this exporter.
Use the live web deck to preserve the exact motion and interactions.

### The exported web deck works locally but not under a URL subdirectory

Publish all of `index.html`, `gosx/`, and any generated `public/` assets together
through HTTP, preserving their relative paths. Use the directory URL with a
trailing slash. Do not upload only `index.html` or open it through `file://`.
Inspect the first failed request in the browser's Network panel. Re-export with
the current release if using an older bundle; text-layout assets are resolved
relative to the bundle in current releases. External URLs authored into a deck
still require connectivity. `offline-required: true` suppresses remote theme
fonts; it does not download arbitrary external media.

## Report a reproducible problem

Include the output of `slides version` and `slides doctor my-talk`, your OS and
browser, the exact command, the first error, and the smallest deck that still
fails. For cue problems, include the `#slide/cue` address and navigation sequence.
Remove private notes, credentials and confidential content from shared examples.
Use the [issue tracker](https://github.com/M31-Labs/gosx-slides/issues).

[First-deck walkthrough](getting-started.md) · [Example gallery](examples.md) ·
[README](../README.md)

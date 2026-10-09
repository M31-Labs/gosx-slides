# From download to your first presentation

This walkthrough creates a portable deck, adds a timed reveal, and exports a
copy you can share. Start with the release archive; Go and Node are not needed
for ordinary authoring. PDF and PowerPoint capture also need Chrome or Chromium.

## 1. Install the whole release folder

Download and extract an archive from the
[latest release](https://github.com/M31-Labs/gosx-slides/releases/latest).
Choose the operating system and processor that match your computer:

| Computer | Archive name ends in |
|---|---|
| Mac with Apple silicon | `darwin-arm64.tar.gz` |
| Mac with Intel processor | `darwin-amd64.tar.gz` |
| Windows on Intel/AMD | `windows-amd64.zip` |
| Windows on ARM | `windows-arm64.zip` |
| Linux on Intel/AMD | `linux-amd64.tar.gz` |
| Linux on ARM64 | `linux-arm64.tar.gz` |

Keep `slides` (or `slides.exe`) and `runtime/` together. Put this tool folder
somewhere permanent, separate from the presentations you create. The runtime
contains the matching browser assets; copying only the executable loses the
portable setup. Release downloads include `SHA256SUMS.txt` for archive checksums.

Open a terminal **inside the extracted tool folder**. On macOS or Linux:

```sh
./slides version
export PATH="$PWD:$PATH"
```

On Windows, in PowerShell:

```powershell
.\slides.exe version
$env:Path = "$PWD;$env:Path"
```

These PATH changes last for the current terminal. For future terminals, add the
tool folder to your shell profile or Windows user **Path** setting. Confirm
`slides version` works before continuing. When upgrading, replace the tool
folder with the new archive's executable **and** runtime; keep your deck folders.

Building from source instead? See [source installation](#source-installation).

## 2. Create and open a deck

Change to the parent directory where you keep presentations. Use a new folder
name; starters refuse an existing destination.

```sh
slides templates
slides init my-talk --template technical-talk
slides serve my-talk --edit
```

Open the URL printed by `serve`, usually `http://127.0.0.1:8080`. Leave that
terminal running. Use **Ctrl+C** there when you want to stop the server. If the
port is busy, add `--port 8081` and use the new printed URL.

Choose `architecture-review` for a system decision, `teaching` for a lesson, or
omit `--template` for the basic component demo. Starters are included in the
binary and create their own `go.mod`; you can move the deck independently of
the GoSX Slides source checkout.

## 3. Make it yours

Press **E** or choose **Edit**. The file picker opens the deck's source files.
Change the title in `deck.md`, then save. Saved changes are validated and shown
in the presentation. Closing the editor retains an unsaved draft in that browser
tab; reloading the page discards it. For file conflicts and recovery, see
[troubleshooting](troubleshooting.md#an-edit-will-not-save).

| File or folder | What to put there |
|---|---|
| `deck.md` | Slide text, metadata, cues and speaker notes |
| `deck.css` | Optional styling for this deck |
| `public/` | Images, fonts and other assets intended for your audience |
| `shaders/` | Custom `.sel` shaders and their uniform JSON files |
| `packs/studio/` | The curated starter's colors, typography and spacing |
| `go.mod` | The dependency pin for source builds; keep it with the deck |
| `build/` | Generated browser assets; keep this out of version control |

`public/` and `shaders/` are optional; create them when needed. The browser
editor edits discovered files, so create new files and folders with your local
editor or file manager. The technical-talk starter includes a branding pack;
the basic component demo instead includes `Counter.gsx`.

For a small first exercise, replace the new deck's `deck.md` with this:

````md
---
title: My first talk
theme: aurora
offline-required: true
scene: shader:aurora
shader-speed: 0.12
shader-strength: 0.25
---

```yaml
id: opening
layout: title
```

# One idea worth sharing.

Give your audience a reason to care.

---

```yaml
id: idea
cues: overview, point, evidence
```

# Let the idea build.

:::motion {cue=point preset=slide-up duration=600 replay=step}
First, make one clear point.
:::

:::motion {cue=evidence preset=fade duration=400 replay=step}
Then, show the evidence behind it.
:::
````

There are two slides. On the second, the first right arrow reveals the point;
the next reveals the evidence. Open `#idea/evidence` to jump directly to that
state. Slide separators are `---` **surrounded by blank lines**. Only the first
`---` block holds deck metadata; each slide's settings use a leading `yaml` fence.

Use **M** to open the motion studio and preview timing. With `--edit`, you can
save supported timing edits back to the source. Use **Backgrounds** to tune a
preset. Follow the [shader and motion cookbook](shaders-and-motion.md) to create
your own material, put it on a shape, or vary entrances and replay rules.

## 4. Present, then share

| Key | Action |
|---|---|
| **→ / ←** | Advance or go back through cues and slides |
| **P** | Open presenter view with speaker notes |
| **O** | Find a slide in overview/search |
| **V** | Open reading view |
| **?** | Show the complete shortcut list |

Presenter Next/Previous moves through every cue before changing slides. You can
also use `/remote` on the serving machine for the same cue controls from a phone.
The current preview keeps its animations and live scene as cues change.

Shortcuts pause while you type in an editor or input. The toolbar fades after
inactivity; keyboard focus keeps it visible. Move the pointer to bring it back.

In a second terminal where `slides` is on PATH, run from the deck's parent:

```sh
slides check my-talk
slides validate my-talk
slides build my-talk --out my-talk-web
slides export my-talk --format pdf --steps --pdf-navigation --out my-talk.pdf
```

The web folder preserves live interaction. Serve the **whole folder** through
an HTTP static host; opening `index.html` as a `file://` URL is not a deployment.
The PDF captures every click state as a still: this exercise produces four
pages. `--pdf-navigation` adds clickable Previous/Next links; omit that flag
for full-frame pages without a navigation strip. Configure [Chrome for exports](troubleshooting.md#chrome-is-not-found)
if capture cannot find it. Use the [format comparison](troubleshooting.md#choose-an-export-format)
for reading copies, editable PowerPoint or recorded motion. Exporting does not
require the live server to stay running.

Before sharing, replace starter claims and speaker notes, visit each cue, and
inspect the exported file. Speaker notes are excluded from exports by default;
`--notes` deliberately includes them in supported formats.

## Source installation

Install Go 1.26 or newer, then clone the source:

```sh
git clone https://github.com/M31-Labs/gosx-slides.git
cd gosx-slides
go install ./cmd/slides
go env GOBIN GOPATH
```

Go installs the executable into `GOBIN` when set, otherwise the `bin` directory
inside `GOPATH`. Add that directory to PATH. `slides version` confirms which
binary is running; source builds may report a development version. For a
particular release's source, check out its tag before `go install`.

Source installs build and cache the runtime using Go, which can take longer on
first use and may download dependencies. `--watch` is the Go-backed development
loop for changes in an external editor; it is separate from browser editing
with `--edit`. `--rebuild` also requires Go. A release archive avoids those
requirements for ordinary serving and web export.

[README](../README.md) · [Troubleshooting and export choices](troubleshooting.md) ·
[Runnable examples and PDFs](examples.md)

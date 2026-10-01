# Navigation lab

From the repository root, run `slides serve examples/navigation-lab`.

Open the overview with **O** or **/**. Search matches title and body words,
ignores case and accents, and treats a number as a slide number. Speaker notes
are excluded. Use arrows to choose a result, Enter to jump, and Esc to return.
**?** shows the keyboard shortcuts.

Increment the counter before opening the overview: its state survives. The
picker uses text cards while live slides stay hidden, so opening a large deck
avoids rendering every shader, diagram, or widget at once.

Code highlights and list reveals support the full authored step count. Hidden
list items leave the tab order until revealed. The overview and these controls
also work in SPA and single-file exports; single-file snapshots retain static
widget content.

Link to an exact click step with `#3/4`. The URL tracks your current step, so you
can copy it, reload, or follow a Markdown link to restore that position. A plain
`#3` starts at step zero. This addresses reveal/diagram steps; timed entrance
animations start on arrival.

Optional browser regressions use Playwright and its Chromium installation:

```sh
npm install --no-save playwright
npx playwright install chromium
node scripts/navigation-browser.cjs http://127.0.0.1:8080/
```

Run these commands from the repository root while this example is serving.
`SLIDES_PLAYWRIGHT_MODULE` and `SLIDES_BROWSER` can point to existing installations.
The checks cover widget state, search privacy, step links, focus restoration,
and idle toolbar behavior. Run against an otherwise idle server because its
presenter sync intentionally connects all audience windows.

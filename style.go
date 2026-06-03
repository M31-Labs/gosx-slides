package slides

func baseCSS() string {
	return `
:root {
  --font-display: "Source Serif 4", "Literata", Georgia, serif;
  --font-body: "Plus Jakarta Sans", "Work Sans", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "IBM Plex Mono", ui-monospace, monospace;

  --type-xs: 0.8125rem;
  --type-sm: 0.9375rem;
  --type-md: 1.0625rem;
  --type-lg: 1.25rem;
  --type-xl: 1.625rem;
  --type-2xl: 2.125rem;
  --type-3xl: 3rem;
  --type-4xl: 4.5rem;
  --type-hero: 6.5rem;

  --space-2xs: 0.25rem;
  --space-xs: 0.5rem;
  --space-sm: 0.75rem;
  --space-md: 1rem;
  --space-lg: 1.5rem;
  --space-xl: 2rem;
  --space-2xl: 3rem;
  --space-3xl: 4.5rem;

  --color-canvas: #f7f4ed;
  --color-surface: #fffdf8;
  --color-elevated: #f0eadf;
  --color-ink: #16211f;
  --color-secondary-text: #3f4d49;
  --color-muted: #66706c;
  --color-line: #d7d1c4;
  --color-accent: #0b756f;
  --color-coral: #c94f3d;
  --color-gold: #c79a2b;
  --color-blue: #2f6f9f;
  --color-code: #17211f;
  --color-code-line: #253532;
  --color-code-text: #ecf1ee;
  --color-code-muted: #78908a;
  --color-code-keyword: #79d4ca;
  --color-canvas-rgb: 247, 244, 237;
  --color-surface-rgb: 255, 253, 248;
  --color-ink-rgb: 22, 33, 31;
  --color-accent-rgb: 11, 117, 111;
  --color-coral-rgb: 201, 79, 61;
  --color-gold-rgb: 199, 154, 43;

  --shadow-raised: 0 1.25rem 4.5rem rgba(var(--color-ink-rgb), 0.16);
  --shadow-control: 0 0.5rem 2rem rgba(var(--color-ink-rgb), 0.14);
  --duration-fast: 150ms;
  --duration-medium: 240ms;
  --duration-slow: 520ms;
  --ease-out-expo: cubic-bezier(0.16, 1, 0.3, 1);
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
  --ease-in-out-quart: cubic-bezier(0.76, 0, 0.24, 1);
  font-family: var(--font-body);
}
.theme-noir {
  --color-canvas: #0f1413;
  --color-surface: #161f1d;
  --color-elevated: #22302d;
  --color-ink: #f1eee5;
  --color-secondary-text: #c9d0c8;
  --color-muted: #93a39d;
  --color-line: #32443f;
  --color-accent: #79d4ca;
  --color-coral: #ff8a6f;
  --color-gold: #e6c15b;
  --color-blue: #8ab6df;
  --color-code: #090d0c;
  --color-code-line: #273632;
  --color-code-text: #f5f1e8;
  --color-code-muted: #91a29d;
  --color-code-keyword: #97ece1;
  --color-canvas-rgb: 15, 20, 19;
  --color-surface-rgb: 22, 31, 29;
  --color-ink-rgb: 241, 238, 229;
  --color-accent-rgb: 121, 212, 202;
  --color-coral-rgb: 255, 138, 111;
  --color-gold-rgb: 230, 193, 91;
}
.theme-blueprint {
  --font-display: "Space Grotesk", "Plus Jakarta Sans", system-ui, sans-serif;
  --color-canvas: #eef5f7;
  --color-surface: #fbfeff;
  --color-elevated: #dcebf0;
  --color-ink: #12212f;
  --color-secondary-text: #31495c;
  --color-muted: #607587;
  --color-line: #b6cbd5;
  --color-accent: #1e6fb7;
  --color-coral: #c74e66;
  --color-gold: #b98620;
  --color-blue: #1e6fb7;
  --color-code: #10202e;
  --color-code-line: #22394d;
  --color-code-text: #e8f3f9;
  --color-code-muted: #8da8ba;
  --color-code-keyword: #80cff6;
  --color-canvas-rgb: 238, 245, 247;
  --color-surface-rgb: 251, 254, 255;
  --color-ink-rgb: 18, 33, 47;
  --color-accent-rgb: 30, 111, 183;
  --color-coral-rgb: 199, 78, 102;
  --color-gold-rgb: 185, 134, 32;
}
.theme-ember {
  --font-display: "DM Serif Display", "Source Serif 4", Georgia, serif;
  --color-canvas: #fbf0e6;
  --color-surface: #fffaf4;
  --color-elevated: #f2dac4;
  --color-ink: #241813;
  --color-secondary-text: #60493d;
  --color-muted: #806c61;
  --color-line: #dfbfa7;
  --color-accent: #8c3f2b;
  --color-coral: #c94f3d;
  --color-gold: #b98022;
  --color-blue: #397285;
  --color-code: #27150f;
  --color-code-line: #4a2d22;
  --color-code-text: #fff2e7;
  --color-code-muted: #c39d8a;
  --color-code-keyword: #ffb178;
  --color-canvas-rgb: 251, 240, 230;
  --color-surface-rgb: 255, 250, 244;
  --color-ink-rgb: 36, 24, 19;
  --color-accent-rgb: 140, 63, 43;
  --color-coral-rgb: 201, 79, 61;
  --color-gold-rgb: 185, 128, 34;
}
* { box-sizing: border-box; }
html, body { width: 100%; height: 100%; margin: 0; }
body {
  background: var(--color-canvas);
  color: var(--color-ink);
  overflow: hidden;
}
button, input { font: inherit; }
button:focus-visible, a:focus-visible, input:focus-visible {
  outline: 0.1875rem solid rgba(var(--color-accent-rgb), 0.45);
  outline-offset: var(--space-2xs);
}
.deck-shell {
  width: 100vw;
  height: 100vh;
  position: relative;
}
.slide {
  position: absolute;
  inset: 0;
  display: none;
  padding: min(5vw, var(--space-3xl));
  background:
    linear-gradient(90deg, rgba(var(--color-accent-rgb), 0.09), transparent 34%),
    radial-gradient(circle at 78% 15%, rgba(var(--color-coral-rgb), 0.12), transparent 28%),
    var(--color-canvas);
}
.slide.is-active {
  display: grid;
  place-items: stretch;
}
.slide-inner {
  width: min(74rem, 100%);
  height: min(49rem, 100%);
  margin: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: var(--space-lg);
}
.slide h1, .slide h2, .slide h3 {
  margin: 0;
  font-family: var(--font-display);
  line-height: 1.02;
  letter-spacing: 0;
}
.slide h1 { font-size: var(--type-hero); max-width: 12ch; }
.slide h2 { font-size: var(--type-4xl); }
.slide h3 { font-size: var(--type-3xl); }
.slide p, .slide li, .slide blockquote {
  font-size: var(--type-2xl);
  line-height: 1.22;
  max-width: 34ch;
}
.slide p { margin: 0; }
.slide ul {
  margin: 0;
  padding-left: 1.2em;
  display: grid;
  gap: var(--space-sm);
}
.slide a {
  color: var(--color-accent);
  text-decoration-thickness: 0.12em;
  text-underline-offset: 0.15em;
}
.slide code {
  font-family: var(--font-mono);
  font-size: 0.78em;
  background: rgba(var(--color-accent-rgb), 0.12);
  padding: 0.08em 0.22em;
  border-radius: var(--space-2xs);
}
.slide img {
  max-width: 100%;
  max-height: 62vh;
  object-fit: contain;
  border-radius: var(--space-xs);
}
.layout-cover .slide-inner, .layout-section .slide-inner, .layout-center .slide-inner {
  align-items: center;
  text-align: center;
}
.layout-cover h1 { max-width: 11ch; }
.layout-cover p { color: var(--color-muted); }
.layout-quote blockquote, blockquote {
  border-left: 0.625rem solid var(--color-coral);
  margin: 0;
  padding-left: var(--space-lg);
  font-family: var(--font-display);
  font-weight: 750;
  max-width: 28ch;
}
.layout-full { padding: 0; }
.layout-full .slide-inner { width: 100%; height: 100%; }
.cols, .image-right {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: min(6vw, var(--space-3xl));
  align-items: center;
  height: 100%;
}
.step {
  opacity: 0;
  transform: translateY(var(--space-sm));
  transition: opacity var(--duration-fast) var(--ease-out-expo), transform var(--duration-fast) var(--ease-out-expo);
}
.step.is-visible { opacity: 1; transform: translateY(0); }
.bind-step {
  display: inline-grid;
  place-items: center;
  min-width: 1.8em;
  padding: 0 0.35em;
  border: 1px solid var(--color-line);
  border-radius: var(--space-2xs);
  color: var(--color-accent);
  background: var(--color-surface);
  font-variant-numeric: tabular-nums;
}
.steps-list { display: grid; gap: var(--space-md); }
.code-frame {
  width: min(61rem, 100%);
  margin: 0;
  background: var(--color-code);
  color: var(--color-code-text);
  border: 1px solid var(--color-code-line);
  border-radius: var(--space-sm);
  box-shadow: var(--shadow-raised);
  overflow: hidden;
}
.code-frame figcaption {
  color: var(--color-code-muted);
  padding: var(--space-sm) var(--space-lg);
  border-bottom: 1px solid var(--color-code-line);
  font-size: var(--type-xs);
  text-transform: uppercase;
}
.code-frame pre { margin: 0; padding: var(--space-md) 0; overflow: auto; }
.code-frame code {
  display: block;
  background: transparent;
  padding: 0;
  border-radius: 0;
  font-size: var(--type-lg);
}
.code-line {
  display: grid;
  grid-template-columns: 3.375rem minmax(0, 1fr);
  gap: var(--space-md);
  padding: var(--space-2xs) var(--space-lg) var(--space-2xs) 0;
  white-space: pre;
  opacity: 0.64;
}
.code-line.is-focus, .code-frame.no-step .code-line {
  background: rgba(var(--color-gold-rgb), 0.22);
  opacity: 1;
}
.line-no { color: var(--color-code-muted); text-align: right; user-select: none; }
.kw { color: var(--color-code-keyword); font-weight: 700; }
.diagram, .scene3d, .canvas-board, .poll, .timeline, .metric-grid, .callout, .pipeline, .parse-tree, .benchmark, .citation, .takeaway, .query-demo, .profile-buckets, .parity-matrix, .corpus-run, .grammar-blob, .checkpoint {
  width: min(58rem, 100%);
  margin: 0;
}
.agenda {
  width: min(58rem, 100%);
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-sm);
  list-style: none;
}
.agenda a {
  display: grid;
  grid-template-columns: 3rem minmax(0, 1fr) auto;
  gap: var(--space-md);
  align-items: center;
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  color: var(--color-ink);
  padding: var(--space-sm) var(--space-md);
  text-decoration: none;
  box-shadow: var(--shadow-control);
}
.agenda a:hover { border-color: var(--color-accent); }
.agenda span {
  display: grid;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  border-radius: 50%;
  background: var(--color-accent);
  color: var(--color-surface);
  font-weight: 800;
  font-size: var(--type-sm);
}
.agenda strong { font-size: var(--type-lg); }
.agenda em {
  color: var(--color-muted);
  font-style: normal;
  font-size: var(--type-xs);
  text-transform: uppercase;
}
.agenda .is-current a { border-color: var(--color-coral); }
.diagram svg, .scene3d canvas, .canvas-board canvas {
  width: 100%;
  aspect-ratio: 16 / 9;
  display: block;
  background: var(--color-surface);
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  box-shadow: var(--shadow-raised);
}
.diagram figcaption {
  color: var(--color-muted);
  font-size: var(--type-sm);
  margin-top: var(--space-sm);
}
.diagram-bg { fill: var(--color-surface); stroke: var(--color-line); stroke-width: 2; }
.diagram-line { fill: none; stroke: var(--color-accent); stroke-width: 8; stroke-linecap: round; opacity: 0.55; }
.diagram-node circle { fill: var(--color-elevated); stroke: var(--color-coral); stroke-width: 4; }
.diagram-node text { fill: var(--color-ink); font-size: 1.625rem; font-weight: 700; }
.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: var(--space-md);
}
.metric {
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: rgba(var(--color-ink-rgb), 0.035);
  padding: var(--space-lg);
}
.metric strong {
  display: block;
  font-family: var(--font-display);
  font-size: var(--type-3xl);
}
.metric span { display: block; color: var(--color-secondary-text); font-size: var(--type-sm); }
.metric em { display: block; color: var(--color-accent); font-style: normal; font-size: var(--type-md); margin-top: var(--space-xs); }
.callout {
  border: 1px solid var(--color-line);
  border-left: 0.5rem solid var(--color-accent);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.callout[data-tone="warn"] { border-left-color: var(--color-coral); }
.callout[data-tone="gold"] { border-left-color: var(--color-gold); }
.callout p { font-size: var(--type-xl); max-width: 44ch; }
.poll {
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.poll h3 { font-size: var(--type-2xl); margin-bottom: var(--space-md); }
.poll-options { display: grid; gap: var(--space-sm); }
.poll button {
  width: 100%;
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: var(--color-canvas);
  color: var(--color-ink);
  padding: var(--space-sm) var(--space-md);
  text-align: left;
  cursor: pointer;
}
.poll button:hover { border-color: var(--color-accent); }
.poll button.is-selected { background: rgba(var(--color-accent-rgb), 0.12); border-color: var(--color-accent); }
.timeline {
  display: grid;
  gap: var(--space-md);
  counter-reset: timeline;
}
.timeline-item {
  counter-increment: timeline;
  display: grid;
  grid-template-columns: 3rem minmax(0, 1fr);
  gap: var(--space-md);
  align-items: center;
}
.timeline-item::before {
  content: counter(timeline);
  display: grid;
  place-items: center;
  width: 3rem;
  height: 3rem;
  border-radius: 50%;
  background: var(--color-accent);
  color: var(--color-surface);
  font-weight: 800;
}
.pipeline {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: var(--space-md);
}
.pipeline-step {
  position: relative;
  min-height: 9rem;
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.pipeline-step span {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: 50%;
  background: var(--color-accent);
  color: var(--color-surface);
  font-weight: 800;
  margin-bottom: var(--space-md);
}
.pipeline-step strong { display: block; font-size: var(--type-lg); }
.pipeline-step em {
  display: block;
  margin-top: var(--space-xs);
  color: var(--color-muted);
  font-size: var(--type-sm);
  font-style: normal;
}
.parse-tree {
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.parse-tree figcaption {
  color: var(--color-muted);
  font-family: var(--font-mono);
  font-size: var(--type-sm);
  margin-bottom: var(--space-md);
}
.parse-tree ul {
  list-style: none;
  margin: 0;
  padding-left: var(--space-lg);
  display: grid;
  gap: var(--space-xs);
}
.parse-tree > ul { padding-left: 0; }
.parse-tree li {
  position: relative;
  font-size: var(--type-md);
  line-height: 1.25;
}
.parse-tree li::before {
  content: "";
  position: absolute;
  left: calc(-1 * var(--space-md));
  top: 0.75em;
  width: var(--space-sm);
  height: 1px;
  background: var(--color-line);
}
.parse-tree > ul > li::before { display: none; }
.parse-tree span {
  display: inline-block;
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: rgba(var(--color-accent-rgb), 0.08);
  padding: var(--space-2xs) var(--space-xs);
  font-family: var(--font-mono);
}
.benchmark {
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.benchmark figcaption {
  font-family: var(--font-display);
  font-size: var(--type-2xl);
  margin-bottom: var(--space-md);
}
.benchmark-row {
  display: grid;
  grid-template-columns: minmax(8rem, 0.35fr) minmax(12rem, 1fr) 5rem;
  gap: var(--space-md);
  align-items: center;
  margin-top: var(--space-sm);
}
.benchmark-row span, .benchmark-row strong {
  font-size: var(--type-sm);
  font-variant-numeric: tabular-nums;
}
.benchmark-row div {
  height: 1.1rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(var(--color-ink-rgb), 0.10);
}
.benchmark-row i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--color-accent), var(--color-gold));
}
.citation {
  display: flex;
  gap: var(--space-sm);
  align-items: center;
  border-top: 1px solid var(--color-line);
  padding-top: var(--space-sm);
  color: var(--color-muted);
  font-size: var(--type-sm);
}
.citation span {
  text-transform: uppercase;
  font-size: var(--type-xs);
  color: var(--color-secondary-text);
}
.takeaway {
  border: 1px solid var(--color-line);
  border-left: 0.5rem solid var(--color-gold);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.takeaway strong {
  display: block;
  color: var(--color-accent);
  font-size: var(--type-sm);
  text-transform: uppercase;
  margin-bottom: var(--space-xs);
}
.takeaway p { font-size: var(--type-xl); max-width: 40ch; }
.query-demo {
  width: min(72rem, 100%);
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.query-demo header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}
.query-demo header span, .checkpoint span {
  color: var(--color-accent);
  font-size: var(--type-xs);
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
}
.query-demo h3 { font-size: var(--type-xl); }
.query-demo-grid {
	display: grid;
	grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.25fr) minmax(12rem, 0.9fr);
	gap: var(--space-md);
	align-items: stretch;
}
.query-demo .code-frame {
  width: 100%;
  height: 100%;
  box-shadow: none;
}
.query-demo .code-frame code { font-size: var(--type-xs); }
.query-demo .code-line {
	grid-template-columns: 2rem minmax(0, 1fr);
	gap: var(--space-sm);
	padding-right: var(--space-sm);
}
.query-tree {
  margin: 0;
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: rgba(var(--color-accent-rgb), 0.06);
  padding: var(--space-md);
}
.query-tree figcaption {
  color: var(--color-muted);
  font-family: var(--font-mono);
  font-size: var(--type-xs);
  margin-bottom: var(--space-sm);
}
.query-tree ul {
  list-style: none;
  margin: 0;
  padding-left: var(--space-md);
  display: grid;
  gap: var(--space-xs);
}
.query-tree > ul { padding-left: 0; }
.query-tree li { font-size: var(--type-sm); }
.query-tree span {
  display: inline-block;
  border: 1px solid var(--color-line);
  border-radius: var(--space-2xs);
  background: var(--color-surface);
  padding: var(--space-2xs) var(--space-xs);
  font-family: var(--font-mono);
}
.capture-pills {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-xs);
  margin-top: var(--space-md);
}
.capture-pills span {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: rgba(var(--color-gold-rgb), 0.14);
  padding: var(--space-xs) var(--space-sm);
}
.capture-pills strong, .capture-pills em {
  font-size: var(--type-sm);
  font-style: normal;
}
.profile-buckets, .corpus-run {
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.profile-buckets figcaption, .corpus-run figcaption {
  font-family: var(--font-display);
  font-size: var(--type-2xl);
  margin-bottom: var(--space-md);
}
.profile-bucket {
  display: grid;
  grid-template-columns: minmax(9rem, 0.35fr) minmax(12rem, 1fr) 4rem;
  gap: var(--space-md);
  align-items: center;
  margin-top: var(--space-sm);
}
.profile-bucket span, .profile-bucket strong {
  font-size: var(--type-sm);
  font-variant-numeric: tabular-nums;
}
.profile-bucket div {
  height: 1rem;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(var(--color-ink-rgb), 0.10);
}
.profile-bucket i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--color-coral), var(--color-gold));
}
.parity-matrix {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
  gap: var(--space-md);
}
.parity-cell {
  min-height: 7.5rem;
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.parity-cell strong {
  display: block;
  font-size: var(--type-lg);
}
.parity-cell span, .corpus-run td:last-child {
  display: inline-block;
  margin-top: var(--space-sm);
  border-radius: var(--space-2xs);
  padding: var(--space-2xs) var(--space-xs);
  background: rgba(var(--color-accent-rgb), 0.12);
  color: var(--color-accent);
  font-size: var(--type-xs);
  font-weight: 800;
  text-transform: uppercase;
}
.parity-cell[data-status="warn"] span, .parity-cell[data-status="watch"] span, .corpus-run tr[data-status="warn"] td:last-child, .corpus-run tr[data-status="watch"] td:last-child {
  background: rgba(var(--color-gold-rgb), 0.18);
  color: var(--color-gold);
}
.parity-cell[data-status="fail"] span, .parity-cell[data-status="blocked"] span, .corpus-run tr[data-status="fail"] td:last-child, .corpus-run tr[data-status="blocked"] td:last-child {
  background: rgba(var(--color-coral-rgb), 0.16);
  color: var(--color-coral);
}
.corpus-run table {
  width: 100%;
  border-collapse: collapse;
}
.corpus-run th, .corpus-run td {
  border-bottom: 1px solid var(--color-line);
  padding: var(--space-sm);
  text-align: left;
  font-size: var(--type-sm);
}
.corpus-run th {
  color: var(--color-muted);
  text-transform: uppercase;
}
.grammar-blob {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: var(--space-md);
}
.grammar-blob-step {
  min-height: 8rem;
  border: 1px dashed var(--color-line);
  border-radius: var(--space-sm);
  background: rgba(var(--color-accent-rgb), 0.06);
  padding: var(--space-lg);
}
.grammar-blob-step span {
  display: grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: var(--space-2xs);
  background: var(--color-code);
  color: var(--color-code-text);
  font-family: var(--font-mono);
  margin-bottom: var(--space-md);
}
.grammar-blob-step strong { font-size: var(--type-lg); }
.checkpoint {
  display: inline-flex;
  width: auto;
  align-items: center;
  gap: var(--space-sm);
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: rgba(var(--color-accent-rgb), 0.08);
  padding: var(--space-xs) var(--space-sm);
}
.checkpoint strong { font-size: var(--type-sm); }
.toolbar {
  position: fixed;
  left: 50%;
  bottom: var(--space-lg);
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-xs);
  border: 1px solid rgba(var(--color-ink-rgb), 0.16);
  border-radius: var(--space-sm);
  background: rgba(var(--color-surface-rgb), 0.86);
  backdrop-filter: blur(0.875rem);
  box-shadow: var(--shadow-control);
  z-index: 10;
}
.toolbar button, .toolbar a, .remote button, .presenter-controls button, .session-controls button {
  min-width: 2.625rem;
  min-height: 2.375rem;
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: var(--color-surface);
  color: var(--color-ink);
  text-decoration: none;
  display: inline-grid;
  place-items: center;
  cursor: pointer;
}
.toolbar button:hover, .toolbar a:hover, .remote button:hover, .presenter-controls button:hover, .session-controls button:hover {
  border-color: var(--color-accent);
}
.counter { min-width: 5.75rem; text-align: center; color: var(--color-muted); font-variant-numeric: tabular-nums; }
.progress-meter {
  width: 8rem;
  height: 0.5rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(var(--color-ink-rgb), 0.12);
}
.progress-meter span {
  display: block;
  height: 100%;
  width: 0;
  border-radius: inherit;
  background: var(--color-accent);
  transition: width var(--duration-fast) var(--ease-out-expo);
}
.overview {
  position: fixed;
  inset: 0;
  display: none;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: var(--space-md);
  padding: var(--space-2xl);
  overflow: auto;
  background: rgba(var(--color-canvas-rgb), 0.94);
  z-index: 20;
}
.overview.is-open { display: grid; }
.overview-tile {
  height: 9.5rem;
  text-align: left;
  padding: var(--space-md);
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-surface);
  color: var(--color-ink);
}
.overview-tile span { color: var(--color-accent); font-weight: 800; }
.overview-tile strong { display: block; margin-top: var(--space-md); font-size: var(--type-lg); }
.presenter, .remote {
  min-height: 100vh;
  overflow: auto;
  padding: var(--space-xl);
  background: var(--color-canvas);
}
.presenter-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(20rem, 0.6fr);
  gap: var(--space-lg);
}
.presenter-panel, .remote-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  padding: var(--space-lg);
  box-shadow: var(--shadow-control);
}
.presenter h1, .remote h1 {
  margin: 0 0 var(--space-lg);
  font-family: var(--font-display);
  font-size: var(--type-3xl);
}
.presenter h2, .remote h2 {
  margin: 0 0 var(--space-md);
  font-family: var(--font-display);
  font-size: var(--type-2xl);
}
.presenter-preview {
  width: 100%;
  aspect-ratio: 16 / 9;
  border: 1px solid var(--color-line);
  border-radius: var(--space-sm);
  background: var(--color-canvas);
  box-shadow: var(--shadow-control);
  margin-bottom: var(--space-lg);
}
.presenter-controls, .remote-controls, .session-controls, .recording-controls {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
  margin-top: var(--space-md);
}
.presenter-controls input, .remote input {
  min-height: 2.375rem;
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: var(--color-surface);
  color: var(--color-ink);
  padding: 0 var(--space-sm);
}
.notes-panel { font-size: var(--type-lg); line-height: 1.4; color: var(--color-ink); }
.timer {
  font-family: var(--font-mono);
  font-size: var(--type-3xl);
  font-variant-numeric: tabular-nums;
}
.slide-list {
  display: grid;
  gap: var(--space-xs);
  max-height: 45vh;
  overflow: auto;
}
.slide-list button {
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: var(--color-canvas);
  color: var(--color-ink);
  padding: var(--space-sm);
  text-align: left;
  cursor: pointer;
}
.slide-list button.is-current { border-color: var(--color-accent); background: rgba(var(--color-accent-rgb), 0.12); }
.checkpoint-list {
  display: grid;
  gap: var(--space-xs);
  max-height: 18vh;
  overflow: auto;
}
.checkpoint-list button {
  border: 1px solid var(--color-line);
  border-radius: var(--space-xs);
  background: var(--color-canvas);
  color: var(--color-ink);
  padding: var(--space-sm);
  text-align: left;
  cursor: pointer;
}
.checkpoint-list button span {
  display: block;
  color: var(--color-muted);
  font-size: var(--type-xs);
}
.join-link {
  display: block;
  color: var(--color-accent);
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
  font-size: var(--type-sm);
}
@media (max-width: 760px) {
  .slide { padding: var(--space-lg); }
  .slide h1 { font-size: var(--type-4xl); }
  .slide p, .slide li, .slide blockquote { font-size: var(--type-xl); }
  .cols, .image-right, .presenter-grid, .query-demo-grid { grid-template-columns: 1fr; }
  .toolbar { width: calc(100vw - var(--space-lg)); justify-content: center; }
}
@media (prefers-reduced-motion: reduce) {
  .step { transition: none; transform: none; }
}
@media print {
  body { overflow: visible; }
  .deck-shell { height: auto; }
  .slide, .slide.is-active {
    position: relative;
    display: grid;
    width: 100vw;
    height: 100vh;
    page-break-after: always;
  }
  .toolbar, .overview { display: none; }
  .step { opacity: 1; transform: none; }
}
`
}

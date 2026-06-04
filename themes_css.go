package slides

// themes_css.go holds the stylesheet constant for each theme registered in
// themes.go. They are split out so themes.go stays a readable registry + resolver
// and the (long) CSS lives on its own. Each constant is the inner CSS text (no
// <style> wrapper) and is fully scoped under `main.deck[data-theme="<name>"]`, so
// it cannot leak into another theme or collide with nav.go's visibility rule.
//
// Shared shape every theme follows (see the themes.go header for the contract):
//   - a `:root`-equivalent token block on `main.deck[data-theme=...]` declaring
//     --bg/--surface/--accent/--fg/--fg-muted, the three --font-* families, the
//     type-scale steps --fs-1..--fs-6 + --fs-body (Perfect Fourth, 1.333,
//     clamp()'d to be readable across a room), the spacing steps --sp-*, --radius
//     and --shadow.
//   - element rules for the full slide markup surface.
//   - .slide.layout-center (vertically+horizontally centered content) and
//     .slide.layout-title (oversized, centered hero) layouts.
//   - the example Counter island (.counter/.counter-btn/.counter-label) styled to
//     match, with hover/focus-visible/active states.
//   - motion gated behind @media (prefers-reduced-motion: no-preference).
//
// Web fonts are loaded with a resilient `font-family` stack that names the
// designer font first and falls back to a high-quality system family, so a deck
// served offline still looks intentional. (A future slice can inject @font-face /
// Google Fonts links into the head; the stacks degrade gracefully until then.)

// auroraCSS — Dark Elegance. Near-black blue-undertone canvas, warm amber accent,
// soft glow on headings and interactive elements. The default theme.
const auroraCSS = `
main.deck[data-theme="aurora"] {
  --bg: #0c0f1a;
  --surface: #161b2e;
  --accent: #f6b352;
  --accent-soft: rgba(246,179,82,0.16);
  --fg: #eef1f8;
  --fg-muted: #9aa3bd;
  --line: rgba(255,255,255,0.08);
  --font-display: "Space Grotesk", "Segoe UI", system-ui, sans-serif;
  --font-body: "Plus Jakarta Sans", "Segoe UI", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "SFMono-Regular", ui-monospace, monospace;
  --fs-1: clamp(2.6rem, 6vw, 4.6rem);
  --fs-2: clamp(2.1rem, 4.6vw, 3.4rem);
  --fs-3: clamp(1.7rem, 3.4vw, 2.5rem);
  --fs-4: clamp(1.4rem, 2.6vw, 1.9rem);
  --fs-5: 1.4rem;
  --fs-6: 1.2rem;
  --fs-body: clamp(1.15rem, 1.7vw, 1.5rem);
  --sp-1: 0.5rem; --sp-2: 1rem; --sp-3: 1.5rem; --sp-4: 2.5rem; --sp-5: 4rem;
  --radius: 14px;
  --shadow: 0 24px 60px rgba(0,0,0,0.45);
  color: var(--fg);
  background:
    radial-gradient(1200px 700px at 78% -8%, rgba(246,179,82,0.10), transparent 60%),
    radial-gradient(900px 600px at 6% 108%, rgba(99,130,255,0.12), transparent 55%),
    var(--bg);
  font-family: var(--font-body);
  font-size: var(--fs-body);
  line-height: 1.6;
  -webkit-font-smoothing: antialiased;
}
main.deck[data-theme="aurora"] > .slide {
  box-sizing: border-box;
  min-height: 100vh;
  padding: clamp(2.5rem, 7vw, 7rem);
  max-width: 1100px;
  margin: 0 auto;
}
main.deck[data-theme="aurora"] h1,
main.deck[data-theme="aurora"] h2,
main.deck[data-theme="aurora"] h3,
main.deck[data-theme="aurora"] h4,
main.deck[data-theme="aurora"] h5,
main.deck[data-theme="aurora"] h6 {
  font-family: var(--font-display);
  font-weight: 600;
  line-height: 1.18;
  letter-spacing: -0.02em;
  margin: 0 0 var(--sp-3);
  text-wrap: balance;
}
main.deck[data-theme="aurora"] h1 {
  font-size: var(--fs-1);
  background: linear-gradient(180deg, #fff 0%, #cfd6ef 100%);
  -webkit-background-clip: text; background-clip: text; color: transparent;
  /* padding so background-clip:text doesn't clip the gradient glyph edges */
  padding-bottom: 0.14em;
}
main.deck[data-theme="aurora"] h2 { font-size: var(--fs-2); }
main.deck[data-theme="aurora"] h3 { font-size: var(--fs-3); color: var(--accent); }
main.deck[data-theme="aurora"] h4 { font-size: var(--fs-4); }
main.deck[data-theme="aurora"] p { margin: 0 0 var(--sp-3); max-width: 46ch; color: var(--fg); }
main.deck[data-theme="aurora"] strong { color: var(--accent); font-weight: 700; }
main.deck[data-theme="aurora"] em { color: #fff; font-style: italic; }
main.deck[data-theme="aurora"] del { color: var(--fg-muted); }
main.deck[data-theme="aurora"] a {
  color: var(--accent); text-decoration: none;
  border-bottom: 2px solid var(--accent-soft); padding-bottom: 1px;
}
main.deck[data-theme="aurora"] a:hover { border-bottom-color: var(--accent); }
main.deck[data-theme="aurora"] a:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
main.deck[data-theme="aurora"] ul,
main.deck[data-theme="aurora"] ol { margin: 0 0 var(--sp-3); padding-left: 0; list-style: none; }
main.deck[data-theme="aurora"] ol { counter-reset: li; }
main.deck[data-theme="aurora"] li {
  position: relative; padding-left: 1.9rem; margin: 0 0 var(--sp-2);
  max-width: 46ch;
}
main.deck[data-theme="aurora"] ul > li::before {
  content: ""; position: absolute; left: 0; top: 0.7em;
  width: 0.5rem; height: 0.5rem; border-radius: 50%;
  background: var(--accent); box-shadow: 0 0 12px var(--accent);
}
main.deck[data-theme="aurora"] ol > li { counter-increment: li; }
main.deck[data-theme="aurora"] ol > li::before {
  content: counter(li); position: absolute; left: 0; top: 0.05em;
  color: var(--accent); font-family: var(--font-mono); font-weight: 700; font-size: 0.95em;
}
main.deck[data-theme="aurora"] code {
  font-family: var(--font-mono); font-size: 0.88em;
  background: var(--surface); color: #ffd9a0;
  padding: 0.15em 0.45em; border-radius: 6px; border: 1px solid var(--line);
}
main.deck[data-theme="aurora"] pre.code-block {
  font-family: var(--font-mono); font-size: clamp(0.85rem, 1.15vw, 1.05rem);
  line-height: 1.6; tab-size: 2;
  margin: var(--sp-3) 0; padding: var(--sp-3) var(--sp-4);
  background:
    linear-gradient(180deg, rgba(255,255,255,0.03), transparent 60%),
    var(--surface);
  color: var(--fg);
  border: 1px solid var(--line); border-left: 3px solid var(--accent);
  border-radius: var(--radius); box-shadow: var(--shadow);
  overflow-x: auto; -webkit-overflow-scrolling: touch;
  max-width: 100%;
}
main.deck[data-theme="aurora"] pre.code-block code {
  font: inherit; background: none; color: inherit;
  padding: 0; border: 0; border-radius: 0; white-space: pre;
}
/* Line-range emphasis: only when the fence carried a {…} spec (data-emphasized).
   Each line is a block so a full-width accent band + left border can sit behind
   the emphasized lines; the rest dim. A plain fence has no [data-emphasized], so
   every .ts-line stays at full opacity (default below). */
main.deck[data-theme="aurora"] pre.code-block .ts-line { display: block; }
main.deck[data-theme="aurora"] pre.code-block[data-emphasized] .ts-line {
  opacity: 0.4; border-left: 3px solid transparent;
  margin: 0 calc(-1 * var(--sp-4)); padding: 0 calc(var(--sp-4) - 3px);
}
main.deck[data-theme="aurora"] pre.code-block[data-emphasized] .ts-line.emphasis {
  opacity: 1; background: var(--accent-soft); border-left-color: var(--accent);
}
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="aurora"] pre.code-block[data-emphasized] .ts-line {
    transition: opacity 200ms cubic-bezier(0.25,1,0.5,1);
  }
}
main.deck[data-theme="aurora"] pre.code-block .ts-keyword { color: #f6b352; font-weight: 600; }
main.deck[data-theme="aurora"] pre.code-block .ts-type,
main.deck[data-theme="aurora"] pre.code-block .ts-namespace { color: #8fd0ff; }
main.deck[data-theme="aurora"] pre.code-block .ts-builtin { color: #c4a5ff; }
main.deck[data-theme="aurora"] pre.code-block .ts-string { color: #9be7a4; }
main.deck[data-theme="aurora"] pre.code-block .ts-number,
main.deck[data-theme="aurora"] pre.code-block .ts-bool { color: #ffb6d5; }
main.deck[data-theme="aurora"] pre.code-block .ts-comment { color: var(--fg-muted); font-style: italic; }
main.deck[data-theme="aurora"] pre.code-block .ts-property,
main.deck[data-theme="aurora"] pre.code-block .ts-attr,
main.deck[data-theme="aurora"] pre.code-block .ts-tag { color: #ffd9a0; }
main.deck[data-theme="aurora"] pre.code-block .ts-operator,
main.deck[data-theme="aurora"] pre.code-block .ts-punctuation { color: var(--fg-muted); }
main.deck[data-theme="aurora"] blockquote {
  margin: var(--sp-3) 0; padding: var(--sp-2) var(--sp-4);
  border-left: 3px solid var(--accent);
  background: var(--surface); border-radius: 0 var(--radius) var(--radius) 0;
  color: #fff; font-size: 1.1em; box-shadow: var(--shadow);
}
main.deck[data-theme="aurora"] > .slide.layout-center {
  display: flex; flex-direction: column; justify-content: center; align-items: flex-start;
  text-align: left;
}
main.deck[data-theme="aurora"] > .slide.layout-center.deck-active { display: flex; }
main.deck[data-theme="aurora"] > .slide.layout-title {
  display: flex; flex-direction: column; justify-content: center; align-items: center;
  text-align: center;
}
main.deck[data-theme="aurora"] > .slide.layout-title.deck-active { display: flex; }
main.deck[data-theme="aurora"] > .slide.layout-title h1 { font-size: clamp(3rem, 9vw, 6.5rem); }
main.deck[data-theme="aurora"] > .slide.layout-title p { max-width: 32ch; color: var(--fg-muted); font-size: 1.25em; }
main.deck[data-theme="aurora"] .counter {
  display: inline-flex; align-items: center; gap: var(--sp-2);
  background: var(--surface); padding: var(--sp-2) var(--sp-3);
  border-radius: 999px; border: 1px solid var(--line); box-shadow: var(--shadow);
  margin: var(--sp-2) 0;
}
main.deck[data-theme="aurora"] .counter-label {
  font-family: var(--font-mono); font-size: 1.2rem; color: var(--fg); min-width: 9ch;
}
main.deck[data-theme="aurora"] .counter-btn {
  font: 700 1.4rem/1 var(--font-display); color: var(--bg);
  background: var(--accent); border: none; cursor: pointer;
  width: 2.6rem; height: 2.6rem; border-radius: 50%;
}
main.deck[data-theme="aurora"] .counter-btn:hover { box-shadow: 0 0 18px var(--accent); }
main.deck[data-theme="aurora"] .counter-btn:active { transform: translateY(1px); }
main.deck[data-theme="aurora"] .counter-btn:focus-visible { outline: 2px solid #fff; outline-offset: 2px; }
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="aurora"] a,
  main.deck[data-theme="aurora"] .counter-btn { transition: all 200ms cubic-bezier(0.25,1,0.5,1); }
}
`

// paperCSS — Editorial Luxe. Warm ivory canvas, terracotta accent, serif
// headlines, generous breathing margins and a pull-quote feel.
const paperCSS = `
main.deck[data-theme="paper"] {
  --bg: #f7f3ec;
  --surface: #efe7da;
  --accent: #b8502f;
  --accent-soft: rgba(184,80,47,0.14);
  --fg: #2b2620;
  --fg-muted: #7a7268;
  --line: rgba(43,38,32,0.14);
  --font-display: "Playfair Display", "Iowan Old Style", Georgia, serif;
  --font-body: "Source Serif 4", "Iowan Old Style", Georgia, serif;
  --font-mono: "IBM Plex Mono", "SFMono-Regular", ui-monospace, monospace;
  --fs-1: clamp(2.8rem, 6.2vw, 5rem);
  --fs-2: clamp(2.2rem, 4.6vw, 3.4rem);
  --fs-3: clamp(1.7rem, 3.2vw, 2.4rem);
  --fs-4: clamp(1.4rem, 2.4vw, 1.85rem);
  --fs-5: 1.4rem;
  --fs-6: 1.2rem;
  --fs-body: clamp(1.2rem, 1.7vw, 1.5rem);
  --sp-1: 0.5rem; --sp-2: 1rem; --sp-3: 1.6rem; --sp-4: 2.6rem; --sp-5: 4.5rem;
  --radius: 4px;
  --shadow: 0 14px 34px rgba(43,38,32,0.12);
  color: var(--fg);
  background: var(--bg);
  font-family: var(--font-body);
  font-size: var(--fs-body);
  line-height: 1.66;
  -webkit-font-smoothing: antialiased;
}
main.deck[data-theme="paper"] > .slide {
  box-sizing: border-box;
  min-height: 100vh;
  padding: clamp(3rem, 9vw, 9rem);
  max-width: 980px;
  margin: 0 auto;
}
main.deck[data-theme="paper"] h1,
main.deck[data-theme="paper"] h2,
main.deck[data-theme="paper"] h3,
main.deck[data-theme="paper"] h4,
main.deck[data-theme="paper"] h5,
main.deck[data-theme="paper"] h6 {
  font-family: var(--font-display);
  font-weight: 700;
  line-height: 1.06;
  margin: 0 0 var(--sp-3);
  color: var(--fg);
  text-wrap: balance;
}
main.deck[data-theme="paper"] h1 { font-size: var(--fs-1); letter-spacing: -0.015em; }
main.deck[data-theme="paper"] h2 { font-size: var(--fs-2); }
main.deck[data-theme="paper"] h3 { font-size: var(--fs-3); font-style: italic; color: var(--accent); }
main.deck[data-theme="paper"] h4 { font-size: var(--fs-4); }
main.deck[data-theme="paper"] p { margin: 0 0 var(--sp-3); max-width: 60ch; }
main.deck[data-theme="paper"] strong { color: var(--accent); font-weight: 700; }
main.deck[data-theme="paper"] em { font-style: italic; }
main.deck[data-theme="paper"] del { color: var(--fg-muted); }
main.deck[data-theme="paper"] a {
  color: var(--accent); text-decoration: none;
  background-image: linear-gradient(var(--accent), var(--accent));
  background-size: 100% 1px; background-repeat: no-repeat; background-position: 0 1.15em;
}
main.deck[data-theme="paper"] a:hover { background-color: var(--accent-soft); }
main.deck[data-theme="paper"] a:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
main.deck[data-theme="paper"] ul,
main.deck[data-theme="paper"] ol { margin: 0 0 var(--sp-3); padding-left: 1.4rem; }
main.deck[data-theme="paper"] ul { list-style: none; padding-left: 0; }
main.deck[data-theme="paper"] ul > li { position: relative; padding-left: 1.6rem; }
main.deck[data-theme="paper"] ul > li::before {
  content: "\2014"; position: absolute; left: 0; color: var(--accent); font-weight: 700;
}
main.deck[data-theme="paper"] li { margin: 0 0 var(--sp-2); max-width: 58ch; }
main.deck[data-theme="paper"] ol > li::marker { color: var(--accent); font-family: var(--font-mono); }
main.deck[data-theme="paper"] code {
  font-family: var(--font-mono); font-size: 0.85em;
  background: var(--surface); color: #8a3a1f;
  padding: 0.1em 0.4em; border-radius: 3px;
}
main.deck[data-theme="paper"] pre.code-block {
  font-family: var(--font-mono); font-size: clamp(0.82rem, 1.1vw, 1rem);
  line-height: 1.62; tab-size: 2;
  margin: var(--sp-3) 0; padding: var(--sp-3) var(--sp-4);
  background: var(--surface); color: #2b2620;
  border: 1px solid var(--line); border-left: 3px solid var(--accent);
  border-radius: var(--radius);
  overflow-x: auto; -webkit-overflow-scrolling: touch; max-width: 100%;
}
main.deck[data-theme="paper"] pre.code-block code {
  font: inherit; background: none; color: inherit;
  padding: 0; border: 0; border-radius: 0; white-space: pre;
}
/* Line-range emphasis (only with a {…} fence spec — see aurora for the rationale). */
main.deck[data-theme="paper"] pre.code-block .ts-line { display: block; }
main.deck[data-theme="paper"] pre.code-block[data-emphasized] .ts-line {
  opacity: 0.42; border-left: 3px solid transparent;
  margin: 0 calc(-1 * var(--sp-4)); padding: 0 calc(var(--sp-4) - 3px);
}
main.deck[data-theme="paper"] pre.code-block[data-emphasized] .ts-line.emphasis {
  opacity: 1; background: var(--accent-soft); border-left-color: var(--accent);
}
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="paper"] pre.code-block[data-emphasized] .ts-line {
    transition: opacity 200ms cubic-bezier(0.25,1,0.5,1);
  }
}
main.deck[data-theme="paper"] pre.code-block .ts-keyword { color: #9a3412; font-weight: 700; }
main.deck[data-theme="paper"] pre.code-block .ts-type,
main.deck[data-theme="paper"] pre.code-block .ts-namespace { color: #1d4ed8; }
main.deck[data-theme="paper"] pre.code-block .ts-builtin { color: #6d28d9; }
main.deck[data-theme="paper"] pre.code-block .ts-string { color: #15803d; }
main.deck[data-theme="paper"] pre.code-block .ts-number,
main.deck[data-theme="paper"] pre.code-block .ts-bool { color: #b8502f; }
main.deck[data-theme="paper"] pre.code-block .ts-comment { color: #8a8478; font-style: italic; }
main.deck[data-theme="paper"] pre.code-block .ts-property,
main.deck[data-theme="paper"] pre.code-block .ts-attr,
main.deck[data-theme="paper"] pre.code-block .ts-tag { color: #8a3a1f; }
main.deck[data-theme="paper"] pre.code-block .ts-operator,
main.deck[data-theme="paper"] pre.code-block .ts-punctuation { color: #5f574c; }
main.deck[data-theme="paper"] blockquote {
  margin: var(--sp-4) 0; padding: 0 var(--sp-4);
  border-left: 4px solid var(--accent);
  font-family: var(--font-display); font-style: italic; font-size: 1.35em;
  line-height: 1.3; color: var(--fg);
}
main.deck[data-theme="paper"] > .slide.layout-center {
  display: flex; flex-direction: column; justify-content: center; align-items: flex-start;
}
main.deck[data-theme="paper"] > .slide.layout-center.deck-active { display: flex; }
main.deck[data-theme="paper"] > .slide.layout-title {
  display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center;
}
main.deck[data-theme="paper"] > .slide.layout-title.deck-active { display: flex; }
main.deck[data-theme="paper"] > .slide.layout-title h1 { font-size: clamp(3.2rem, 9.5vw, 7rem); }
main.deck[data-theme="paper"] > .slide.layout-title p { max-width: 34ch; color: var(--fg-muted); font-style: italic; }
main.deck[data-theme="paper"] .counter {
  display: inline-flex; align-items: center; gap: var(--sp-2);
  background: var(--surface); padding: var(--sp-2) var(--sp-3);
  border: 1px solid var(--line); border-radius: var(--radius); box-shadow: var(--shadow);
  margin: var(--sp-2) 0;
}
main.deck[data-theme="paper"] .counter-label {
  font-family: var(--font-mono); font-size: 1.15rem; color: var(--fg); min-width: 9ch;
}
main.deck[data-theme="paper"] .counter-btn {
  font: 700 1.4rem/1 var(--font-display); color: var(--bg);
  background: var(--accent); border: none; cursor: pointer;
  width: 2.5rem; height: 2.5rem; border-radius: var(--radius);
}
main.deck[data-theme="paper"] .counter-btn:hover { background: #9c4327; }
main.deck[data-theme="paper"] .counter-btn:active { transform: translateY(1px); }
main.deck[data-theme="paper"] .counter-btn:focus-visible { outline: 2px solid var(--fg); outline-offset: 2px; }
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="paper"] a,
  main.deck[data-theme="paper"] .counter-btn { transition: all 200ms cubic-bezier(0.25,1,0.5,1); }
}
`

// neonCSS — Electric. Deep indigo canvas, cyan + lime accents, bold uppercase
// display, geometric energy. The high-impact "demo" theme.
const neonCSS = `
main.deck[data-theme="neon"] {
  --bg: #14122b;
  --surface: #211d44;
  --accent: #2ff2d6;
  --accent-2: #c6ff4a;
  --accent-soft: rgba(47,242,214,0.16);
  --fg: #f3f1ff;
  --fg-muted: #a59fce;
  --line: rgba(255,255,255,0.10);
  --font-display: "Space Grotesk", "Segoe UI", system-ui, sans-serif;
  --font-body: "Plus Jakarta Sans", "Segoe UI", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "SFMono-Regular", ui-monospace, monospace;
  --fs-1: clamp(2.8rem, 6.4vw, 5rem);
  --fs-2: clamp(2.1rem, 4.6vw, 3.4rem);
  --fs-3: clamp(1.7rem, 3.4vw, 2.5rem);
  --fs-4: clamp(1.4rem, 2.6vw, 1.9rem);
  --fs-5: 1.4rem;
  --fs-6: 1.2rem;
  --fs-body: clamp(1.15rem, 1.7vw, 1.5rem);
  --sp-1: 0.5rem; --sp-2: 1rem; --sp-3: 1.5rem; --sp-4: 2.5rem; --sp-5: 4rem;
  --radius: 10px;
  --shadow: 0 0 0 1px var(--line), 0 18px 50px rgba(0,0,0,0.5);
  color: var(--fg);
  background:
    radial-gradient(800px 800px at 90% 0%, rgba(47,242,214,0.10), transparent 55%),
    radial-gradient(700px 700px at 0% 100%, rgba(198,255,74,0.08), transparent 55%),
    var(--bg);
  font-family: var(--font-body);
  font-size: var(--fs-body);
  line-height: 1.6;
  -webkit-font-smoothing: antialiased;
}
main.deck[data-theme="neon"] > .slide {
  box-sizing: border-box; min-height: 100vh;
  padding: clamp(2.5rem, 7vw, 7rem); max-width: 1100px; margin: 0 auto;
}
main.deck[data-theme="neon"] h1,
main.deck[data-theme="neon"] h2,
main.deck[data-theme="neon"] h3,
main.deck[data-theme="neon"] h4,
main.deck[data-theme="neon"] h5,
main.deck[data-theme="neon"] h6 {
  font-family: var(--font-display); font-weight: 700; line-height: 1.04;
  letter-spacing: -0.01em; text-transform: uppercase; margin: 0 0 var(--sp-3);
  text-wrap: balance;
}
main.deck[data-theme="neon"] h1 {
  font-size: var(--fs-1); color: #fff; text-shadow: 0 0 28px rgba(47,242,214,0.55);
}
main.deck[data-theme="neon"] h2 { font-size: var(--fs-2); color: var(--accent); }
main.deck[data-theme="neon"] h3 { font-size: var(--fs-3); color: var(--accent-2); }
main.deck[data-theme="neon"] h4 { font-size: var(--fs-4); }
main.deck[data-theme="neon"] p { margin: 0 0 var(--sp-3); max-width: 46ch; text-transform: none; }
main.deck[data-theme="neon"] strong { color: var(--accent-2); font-weight: 700; }
main.deck[data-theme="neon"] em { color: #fff; }
main.deck[data-theme="neon"] del { color: var(--fg-muted); }
main.deck[data-theme="neon"] a {
  color: var(--accent); text-decoration: none; font-weight: 600;
  border-bottom: 2px solid var(--accent-soft);
}
main.deck[data-theme="neon"] a:hover { color: #fff; border-bottom-color: var(--accent); text-shadow: 0 0 12px var(--accent); }
main.deck[data-theme="neon"] a:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
main.deck[data-theme="neon"] ul,
main.deck[data-theme="neon"] ol { margin: 0 0 var(--sp-3); padding-left: 0; list-style: none; }
main.deck[data-theme="neon"] ol { counter-reset: li; }
main.deck[data-theme="neon"] li { position: relative; padding-left: 2rem; margin: 0 0 var(--sp-2); max-width: 46ch; }
main.deck[data-theme="neon"] ul > li::before {
  content: "\25B8"; position: absolute; left: 0; top: 0; color: var(--accent); font-weight: 700;
}
main.deck[data-theme="neon"] ol > li { counter-increment: li; }
main.deck[data-theme="neon"] ol > li::before {
  content: counter(li, decimal-leading-zero); position: absolute; left: 0; top: 0.1em;
  color: var(--accent-2); font-family: var(--font-mono); font-weight: 700; font-size: 0.85em;
}
main.deck[data-theme="neon"] code {
  font-family: var(--font-mono); font-size: 0.88em;
  background: var(--surface); color: var(--accent);
  padding: 0.15em 0.45em; border-radius: 6px; border: 1px solid var(--line);
}
main.deck[data-theme="neon"] pre.code-block {
  font-family: var(--font-mono); font-size: clamp(0.85rem, 1.15vw, 1.05rem);
  line-height: 1.6; tab-size: 2;
  margin: var(--sp-3) 0; padding: var(--sp-3) var(--sp-4);
  background: linear-gradient(180deg, rgba(47,242,214,0.04), transparent 55%), var(--surface);
  color: var(--fg);
  border: 1px solid var(--accent); border-radius: var(--radius);
  box-shadow: 0 0 28px rgba(47,242,214,0.16);
  overflow-x: auto; -webkit-overflow-scrolling: touch; max-width: 100%;
}
main.deck[data-theme="neon"] pre.code-block code {
  font: inherit; background: none; color: inherit;
  padding: 0; border: 0; border-radius: 0; white-space: pre;
}
/* Line-range emphasis (only with a {…} fence spec — see aurora for the rationale). */
main.deck[data-theme="neon"] pre.code-block .ts-line { display: block; }
main.deck[data-theme="neon"] pre.code-block[data-emphasized] .ts-line {
  opacity: 0.38; border-left: 3px solid transparent;
  margin: 0 calc(-1 * var(--sp-4)); padding: 0 calc(var(--sp-4) - 3px);
}
main.deck[data-theme="neon"] pre.code-block[data-emphasized] .ts-line.emphasis {
  opacity: 1; background: var(--accent-soft); border-left-color: var(--accent);
}
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="neon"] pre.code-block[data-emphasized] .ts-line {
    transition: opacity 200ms cubic-bezier(0.25,1,0.5,1);
  }
}
main.deck[data-theme="neon"] pre.code-block .ts-keyword { color: #c6ff4a; font-weight: 700; }
main.deck[data-theme="neon"] pre.code-block .ts-type,
main.deck[data-theme="neon"] pre.code-block .ts-namespace { color: #2ff2d6; }
main.deck[data-theme="neon"] pre.code-block .ts-builtin { color: #7dd3fc; }
main.deck[data-theme="neon"] pre.code-block .ts-string { color: #f9f871; }
main.deck[data-theme="neon"] pre.code-block .ts-number,
main.deck[data-theme="neon"] pre.code-block .ts-bool { color: #ff8ad8; }
main.deck[data-theme="neon"] pre.code-block .ts-comment { color: var(--fg-muted); font-style: italic; }
main.deck[data-theme="neon"] pre.code-block .ts-property,
main.deck[data-theme="neon"] pre.code-block .ts-attr,
main.deck[data-theme="neon"] pre.code-block .ts-tag { color: #2ff2d6; }
main.deck[data-theme="neon"] pre.code-block .ts-operator,
main.deck[data-theme="neon"] pre.code-block .ts-punctuation { color: var(--fg-muted); }
main.deck[data-theme="neon"] blockquote {
  margin: var(--sp-3) 0; padding: var(--sp-3) var(--sp-4);
  border: 1px solid var(--accent); border-radius: var(--radius);
  background: var(--surface); color: #fff; box-shadow: 0 0 30px rgba(47,242,214,0.2);
}
main.deck[data-theme="neon"] > .slide.layout-center {
  display: flex; flex-direction: column; justify-content: center; align-items: flex-start;
}
main.deck[data-theme="neon"] > .slide.layout-center.deck-active { display: flex; }
main.deck[data-theme="neon"] > .slide.layout-title {
  display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center;
}
main.deck[data-theme="neon"] > .slide.layout-title.deck-active { display: flex; }
main.deck[data-theme="neon"] > .slide.layout-title h1 { font-size: clamp(3rem, 9.5vw, 7rem); }
main.deck[data-theme="neon"] > .slide.layout-title p { max-width: 32ch; color: var(--fg-muted); }
main.deck[data-theme="neon"] .counter {
  display: inline-flex; align-items: center; gap: var(--sp-2);
  background: var(--surface); padding: var(--sp-2) var(--sp-3);
  border: 1px solid var(--accent); border-radius: var(--radius);
  box-shadow: 0 0 24px rgba(47,242,214,0.18); margin: var(--sp-2) 0;
}
main.deck[data-theme="neon"] .counter-label {
  font-family: var(--font-mono); font-size: 1.2rem; color: var(--fg); min-width: 9ch;
}
main.deck[data-theme="neon"] .counter-btn {
  font: 700 1.4rem/1 var(--font-display); color: var(--bg);
  background: var(--accent); border: none; cursor: pointer;
  width: 2.6rem; height: 2.6rem; border-radius: 8px;
}
main.deck[data-theme="neon"] .counter-btn:hover { background: var(--accent-2); box-shadow: 0 0 20px var(--accent-2); }
main.deck[data-theme="neon"] .counter-btn:active { transform: translateY(1px); }
main.deck[data-theme="neon"] .counter-btn:focus-visible { outline: 2px solid #fff; outline-offset: 2px; }
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="neon"] a,
  main.deck[data-theme="neon"] .counter-btn { transition: all 200ms cubic-bezier(0.25,1,0.5,1); }
}
`

// swissCSS — Swiss Precision. Pure white, black ink + one red accent, tight grid,
// flush-left rhythm, minimal ornament. The clean professional theme.
const swissCSS = `
main.deck[data-theme="swiss"] {
  --bg: #ffffff;
  --surface: #f2f2f2;
  --accent: #e3000f;
  --accent-soft: rgba(227,0,15,0.10);
  --fg: #0a0a0a;
  --fg-muted: #6b6b6b;
  --line: #e0e0e0;
  --font-display: "Space Grotesk", "Helvetica Neue", Arial, sans-serif;
  --font-body: "Work Sans", "Helvetica Neue", Arial, sans-serif;
  --font-mono: "JetBrains Mono", "SFMono-Regular", ui-monospace, monospace;
  --fs-1: clamp(2.6rem, 6vw, 4.8rem);
  --fs-2: clamp(2rem, 4.4vw, 3.2rem);
  --fs-3: clamp(1.6rem, 3.2vw, 2.3rem);
  --fs-4: clamp(1.35rem, 2.4vw, 1.8rem);
  --fs-5: 1.35rem;
  --fs-6: 1.15rem;
  --fs-body: clamp(1.15rem, 1.6vw, 1.45rem);
  --sp-1: 0.5rem; --sp-2: 1rem; --sp-3: 1.5rem; --sp-4: 2.5rem; --sp-5: 4rem;
  --radius: 0px;
  --shadow: none;
  color: var(--fg);
  background: var(--bg);
  font-family: var(--font-body);
  font-size: var(--fs-body);
  line-height: 1.55;
  -webkit-font-smoothing: antialiased;
}
main.deck[data-theme="swiss"] > .slide {
  box-sizing: border-box; min-height: 100vh;
  padding: clamp(2.5rem, 7vw, 6.5rem); max-width: 1080px; margin: 0 auto;
  border-top: 6px solid var(--accent);
}
main.deck[data-theme="swiss"] h1,
main.deck[data-theme="swiss"] h2,
main.deck[data-theme="swiss"] h3,
main.deck[data-theme="swiss"] h4,
main.deck[data-theme="swiss"] h5,
main.deck[data-theme="swiss"] h6 {
  font-family: var(--font-display); font-weight: 700; line-height: 1.02;
  letter-spacing: -0.02em; margin: 0 0 var(--sp-3); text-wrap: balance;
}
main.deck[data-theme="swiss"] h1 { font-size: var(--fs-1); }
main.deck[data-theme="swiss"] h1::after {
  content: ""; display: block; width: 3.5rem; height: 6px; background: var(--accent); margin-top: var(--sp-3);
}
main.deck[data-theme="swiss"] h2 { font-size: var(--fs-2); }
main.deck[data-theme="swiss"] h3 { font-size: var(--fs-3); color: var(--accent); }
main.deck[data-theme="swiss"] h4 { font-size: var(--fs-4); }
main.deck[data-theme="swiss"] p { margin: 0 0 var(--sp-3); max-width: 56ch; }
main.deck[data-theme="swiss"] strong { color: var(--accent); font-weight: 700; }
main.deck[data-theme="swiss"] em { font-style: italic; }
main.deck[data-theme="swiss"] del { color: var(--fg-muted); }
main.deck[data-theme="swiss"] a {
  color: var(--accent); text-decoration: none; font-weight: 600;
  box-shadow: inset 0 -0.12em var(--accent-soft);
}
main.deck[data-theme="swiss"] a:hover { box-shadow: inset 0 -0.7em var(--accent-soft); }
main.deck[data-theme="swiss"] a:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
main.deck[data-theme="swiss"] ul,
main.deck[data-theme="swiss"] ol { margin: 0 0 var(--sp-3); padding-left: 0; list-style: none; }
main.deck[data-theme="swiss"] ol { counter-reset: li; }
main.deck[data-theme="swiss"] li {
  position: relative; padding-left: 1.8rem; margin: 0 0 var(--sp-2);
  max-width: 54ch; border-left: 0;
}
main.deck[data-theme="swiss"] ul > li::before {
  content: ""; position: absolute; left: 0; top: 0.55em;
  width: 0.7rem; height: 0.7rem; background: var(--accent);
}
main.deck[data-theme="swiss"] ol > li { counter-increment: li; }
main.deck[data-theme="swiss"] ol > li::before {
  content: counter(li) "."; position: absolute; left: 0; top: 0;
  color: var(--accent); font-family: var(--font-display); font-weight: 700;
}
main.deck[data-theme="swiss"] code {
  font-family: var(--font-mono); font-size: 0.86em;
  background: var(--surface); color: var(--fg);
  padding: 0.12em 0.4em;
}
main.deck[data-theme="swiss"] pre.code-block {
  font-family: var(--font-mono); font-size: clamp(0.82rem, 1.1vw, 1rem);
  line-height: 1.58; tab-size: 2;
  margin: var(--sp-3) 0; padding: var(--sp-3) var(--sp-4);
  background: var(--surface); color: var(--fg);
  border: 0; border-left: 4px solid var(--accent); border-radius: 0;
  overflow-x: auto; -webkit-overflow-scrolling: touch; max-width: 100%;
}
main.deck[data-theme="swiss"] pre.code-block code {
  font: inherit; background: none; color: inherit;
  padding: 0; border: 0; border-radius: 0; white-space: pre;
}
/* Line-range emphasis (only with a {…} fence spec — see aurora for the rationale).
   The block already has a 4px accent left rule; the per-line rule sits inside it. */
main.deck[data-theme="swiss"] pre.code-block .ts-line { display: block; }
main.deck[data-theme="swiss"] pre.code-block[data-emphasized] .ts-line {
  opacity: 0.4; border-left: 3px solid transparent;
  margin: 0 calc(-1 * var(--sp-4)); padding: 0 calc(var(--sp-4) - 3px);
}
main.deck[data-theme="swiss"] pre.code-block[data-emphasized] .ts-line.emphasis {
  opacity: 1; background: var(--accent-soft); border-left-color: var(--accent);
}
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="swiss"] pre.code-block[data-emphasized] .ts-line {
    transition: opacity 150ms cubic-bezier(0.25,1,0.5,1);
  }
}
main.deck[data-theme="swiss"] pre.code-block .ts-keyword { color: #e3000f; font-weight: 700; }
main.deck[data-theme="swiss"] pre.code-block .ts-type,
main.deck[data-theme="swiss"] pre.code-block .ts-namespace { color: #1a1a1a; font-weight: 600; }
main.deck[data-theme="swiss"] pre.code-block .ts-builtin { color: #1a1a1a; font-weight: 600; }
main.deck[data-theme="swiss"] pre.code-block .ts-string { color: #555555; }
main.deck[data-theme="swiss"] pre.code-block .ts-number,
main.deck[data-theme="swiss"] pre.code-block .ts-bool { color: #e3000f; }
main.deck[data-theme="swiss"] pre.code-block .ts-comment { color: #9b9b9b; font-style: italic; }
main.deck[data-theme="swiss"] pre.code-block .ts-property,
main.deck[data-theme="swiss"] pre.code-block .ts-attr,
main.deck[data-theme="swiss"] pre.code-block .ts-tag { color: #0a0a0a; }
main.deck[data-theme="swiss"] pre.code-block .ts-operator,
main.deck[data-theme="swiss"] pre.code-block .ts-punctuation { color: #6b6b6b; }
main.deck[data-theme="swiss"] blockquote {
  margin: var(--sp-3) 0; padding: var(--sp-1) 0 var(--sp-1) var(--sp-3);
  border-left: 4px solid var(--accent); font-size: 1.2em; color: var(--fg);
}
main.deck[data-theme="swiss"] > .slide.layout-center {
  display: flex; flex-direction: column; justify-content: center; align-items: flex-start;
}
main.deck[data-theme="swiss"] > .slide.layout-center.deck-active { display: flex; }
main.deck[data-theme="swiss"] > .slide.layout-title {
  display: flex; flex-direction: column; justify-content: center; align-items: flex-start;
}
main.deck[data-theme="swiss"] > .slide.layout-title.deck-active { display: flex; }
main.deck[data-theme="swiss"] > .slide.layout-title h1 { font-size: clamp(3rem, 9vw, 6.5rem); }
main.deck[data-theme="swiss"] > .slide.layout-title p { max-width: 40ch; color: var(--fg-muted); }
main.deck[data-theme="swiss"] .counter {
  display: inline-flex; align-items: center; gap: var(--sp-2);
  background: var(--surface); padding: var(--sp-2) var(--sp-3);
  border: 2px solid var(--fg); margin: var(--sp-2) 0;
}
main.deck[data-theme="swiss"] .counter-label {
  font-family: var(--font-mono); font-size: 1.15rem; color: var(--fg); min-width: 9ch;
}
main.deck[data-theme="swiss"] .counter-btn {
  font: 700 1.4rem/1 var(--font-display); color: var(--bg);
  background: var(--fg); border: none; cursor: pointer;
  width: 2.5rem; height: 2.5rem;
}
main.deck[data-theme="swiss"] .counter-btn:hover { background: var(--accent); }
main.deck[data-theme="swiss"] .counter-btn:active { transform: translateY(1px); }
main.deck[data-theme="swiss"] .counter-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media (prefers-reduced-motion: no-preference) {
  main.deck[data-theme="swiss"] a,
  main.deck[data-theme="swiss"] .counter-btn { transition: all 150ms cubic-bezier(0.25,1,0.5,1); }
}
`

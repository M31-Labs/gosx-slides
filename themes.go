package slides

// themes.go is the real lane's VISUAL SYSTEM (Phase 1, Slice 7). The real lane
// (serve.go) lowers every slide to `<section class="slide" data-slide="N">…` and
// hides all but the active one (nav.go); on its own that is unstyled HTML. This
// file is the single, obvious home for the presentation-grade design: a
// name -> CSS registry of distinct, polished THEMES plus a small set of per-slide
// LAYOUTS, all targeting the markup the lowering lanes emit.
//
// ── How a deck picks a theme ─────────────────────────────────────────────────
// The deck headmatter (the leading `---` block of deck.md) carries `theme: <name>`:
//
//	---
//	title: My Talk
//	theme: aurora      # one of Themes(); unknown/absent -> defaultTheme
//	---
//
// serve.go reads it (deckFrontmatterValues), resolves it through themeName, sets
// `data-theme="<name>"` on `<main class="deck">`, and injects themeCSS(name) into
// the single document <head> — the same RawHTML-into-ctx.AddHead mechanism that
// places the nav visibility style. Every theme's CSS is SCOPED under
// `main.deck[data-theme="<name>"]` so themes never leak into each other and never
// fight the nav rule (which lives under the unqualified `main.deck`).
//
// ── How a slide picks a layout ───────────────────────────────────────────────
// Per-slide frontmatter carries `layout: <name>`:
//
//	---
//	layout: title      # default | center | title  (see layoutClass)
//	---
//	# Big Opening
//
// slidegen.go's lowerSlideToGSX turns it into a `layout-<name>` class on the
// `<section>`, and every theme styles `.slide.layout-center` / `.slide.layout-title`.
//
// ── HOW TO ADD A THEME (this is the whole job) ───────────────────────────────
// Add ONE entry to the themeRegistry map below: a name -> CSS string. The CSS
// MUST be scoped under `main.deck[data-theme="<your-name>"]` (use the same shape
// as the existing themes) and should style the full markup surface:
//
//	.slide  h1..h6  p  ul/ol/li  code  strong  em  del  blockquote  a
//	.slide.layout-center  .slide.layout-title
//	.counter / .counter-btn / .counter-label   (the example island)
//
// That is it — the name is now returned by Themes(), accepted in headmatter, and
// listed by `slides themes`. No other file needs touching.
//
// ── Visual System (the design contract for these themes) ─────────────────────
// Type scale (Perfect Fourth, 1.333, presentation-sized so it reads across a
// room) and an 8px spacing rhythm are shared via CSS custom properties declared
// per theme. Tokens (per theme): --bg (60% canvas), --surface (30% panels),
// --accent (10% highlight), --fg / --fg-muted (text hierarchy), --font-display /
// --font-body / --font-mono, --radius, --shadow. Motion is Minimal (transitions
// only) with ease-out-quart `cubic-bezier(0.25,1,0.5,1)`; all motion is wrapped in
// `prefers-reduced-motion: no-preference` so reduced-motion users get none.
//
//	aurora   Dark Elegance — near-black blue-undertone canvas, warm amber accent,
//	         soft glow. Space Grotesk / Plus Jakarta Sans / JetBrains Mono. DEFAULT.
//	paper    Editorial Luxe — warm ivory canvas, terracotta accent, serif
//	         headlines, generous margins. Playfair Display / Source Serif 4 / IBM Plex Mono.
//	neon     Electric — deep indigo canvas, cyan + lime accents, bold geometry,
//	         uppercase display. Space Grotesk / Plus Jakarta Sans / JetBrains Mono.
//	swiss    Swiss Precision — pure white, black ink + one red accent, tight grid,
//	         flush-left rhythm. Space Grotesk / Work Sans / JetBrains Mono.

import (
	"sort"
	"strings"
)

// defaultTheme is the theme applied when a deck's headmatter has no `theme:` key
// or names a theme that is not in the registry. Kept as a single source of truth
// so serve.go, the resolver, and tests all agree.
const defaultTheme = "aurora"

// themeRegistry maps a theme name to its complete, self-contained stylesheet
// (inner CSS text, no <style> wrapper). This is the ONE place themes live: adding
// a theme = adding an entry here (see the file header for the contract). Every
// value is scoped under `main.deck[data-theme="<name>"]`.
var themeRegistry = map[string]string{
	"aurora": auroraCSS,
	"paper":  paperCSS,
	"neon":   neonCSS,
	"swiss":  swissCSS,
}

// Themes returns the sorted list of registered theme names. It backs the
// `slides themes` CLI command and any UI that lists selectable themes.
func Themes() []string {
	names := make([]string, 0, len(themeRegistry))
	for name := range themeRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// themeName normalizes a raw headmatter `theme:` value to a registered theme
// name, falling back to defaultTheme when the value is empty or unknown. The
// result is always a key present in themeRegistry, so it is safe to use directly
// as the `data-theme` attribute value and to pass to themeCSS.
func themeName(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return defaultTheme
	}
	if _, ok := themeRegistry[name]; ok {
		return name
	}
	return defaultTheme
}

// themeCSS returns the stylesheet for a theme. The name is resolved through
// themeName first, so an empty/unknown name yields the default theme's CSS rather
// than an empty string — the served head always carries a real theme.
func themeCSS(name string) string {
	return themeRegistry[themeName(name)]
}

// baseContentStyle styles the content elements every theme renders the same way —
// images, figures, and tables — once, theme-agnostically. It reads the active
// theme's design tokens (--line/--surface/--accent/--radius/--sp-*), which cascade
// from main.deck[data-theme], so a single rule adapts to every theme (with literal
// fallbacks so it still works if a token is absent). Kept out of the four theme
// blobs so a new content element is styled in ONE place, not four. Images are
// height-capped and object-fit:contain so they stay inside the locked viewport.
func baseContentStyle() string {
	return `main.deck img { max-width: 100%; max-height: 58vh; height: auto; object-fit: contain; display: block; margin: var(--sp-3, 1rem) auto; border-radius: var(--radius, 10px); }
main.deck figure { margin: var(--sp-3, 1rem) 0; }
main.deck figcaption { margin-top: 0.5em; font-size: 0.85em; text-align: center; color: var(--fg-muted, currentColor); }
main.deck table { border-collapse: collapse; width: 100%; margin: var(--sp-3, 1rem) 0; font-size: 0.95em; }
main.deck th, main.deck td { border: 1px solid var(--line, rgba(128,128,128,0.3)); padding: 0.5em 0.85em; text-align: left; vertical-align: top; }
main.deck th { background: var(--surface, rgba(128,128,128,0.12)); color: var(--accent, currentColor); font-weight: 700; }
main.deck tr:nth-child(even) td { background: color-mix(in srgb, var(--surface, gray) 35%, transparent); }
main.deck pre.code-block { position: relative; }
main.deck pre.code-block .code-copy { position: absolute; top: 0.55rem; right: 0.55rem; opacity: 0; transition: opacity 150ms; cursor: pointer; font: 600 0.7rem var(--font-mono, ui-monospace, monospace); color: var(--fg-muted, #999); background: var(--surface, rgba(128,128,128,0.2)); border: 1px solid var(--line, rgba(128,128,128,0.3)); border-radius: 6px; padding: 0.25rem 0.6rem; }
main.deck pre.code-block:hover .code-copy, main.deck pre.code-block .code-copy:focus-visible { opacity: 0.9; }
main.deck pre.code-block .code-copy:hover { color: var(--accent, currentColor); }
main.deck[data-line-numbers="1"] pre.code-block .ts-line::before { content: attr(data-line); display: inline-block; width: 2.4ch; margin-right: 1.25ch; text-align: right; opacity: 0.35; -webkit-user-select: none; user-select: none; }
main.deck pre.code-block .ts-diff-add { background: rgba(80,200,120,0.14); }
main.deck pre.code-block .ts-diff-del { background: rgba(255,107,107,0.14); }
main.deck pre.code-block .ts-diff-meta { color: var(--accent, currentColor); opacity: 0.8; }
main.deck > .slide { position: relative; isolation: isolate; }
main.deck > .slide .slide-header, main.deck > .slide .slide-footer { position: absolute; left: 1.4rem; right: 1.4rem; z-index: 5; font: 600 0.78rem/1.3 var(--font-mono, ui-monospace, monospace); color: var(--fg-muted, #888); opacity: 0.85; }
main.deck > .slide .slide-header { top: 1.1rem; }
main.deck > .slide .slide-footer { bottom: 1.1rem; right: 5rem; }
@media print { main.deck > .slide .slide-header, main.deck > .slide .slide-footer { opacity: 1; } }
/* Scene layer: an island rendered full-bleed BEHIND the slide's content. The
   slide isolates its stacking context (above), so z-index -1 paints the scene
   over the slide's own background but under every in-flow child. Decorative
   by contract: no pointer events, hidden under prefers-reduced-motion. */
main.deck > .slide > .slide-scene { position: absolute; inset: 0; z-index: -1; overflow: hidden; pointer-events: none; }
main.deck > .slide > .slide-scene > div { width: 100%; height: 100%; }
@media (prefers-reduced-motion: reduce) { main.deck > .slide > .slide-scene { display: none; } }
@media print { main.deck > .slide > .slide-scene { display: none; } }
/* Preset: parse-forest — drifting branch strokes in theme colors, the parser's
   fork forest as ambient texture. Pure compositing (transform-only keyframes). */
main.deck .scene-parse-forest { position: absolute; inset: 0; opacity: 0.6; }
main.deck .scene-parse-forest .spf { position: absolute; inset: -30%; }
main.deck .scene-parse-forest .spf-1 { background: repeating-linear-gradient(112deg, transparent 0 140px, color-mix(in srgb, var(--accent, #888) 22%, transparent) 140px 142px, transparent 142px 280px); animation: spfDrift1 80s linear infinite alternate; }
main.deck .scene-parse-forest .spf-2 { background: repeating-linear-gradient(68deg, transparent 0 190px, color-mix(in srgb, var(--fg-muted, #888) 18%, transparent) 190px 192px, transparent 192px 380px); animation: spfDrift2 110s linear infinite alternate; }
main.deck .scene-parse-forest .spf-3 { background: repeating-linear-gradient(91deg, transparent 0 260px, color-mix(in srgb, var(--accent, #888) 12%, transparent) 260px 261px, transparent 261px 520px); animation: spfDrift3 140s linear infinite alternate; }
@keyframes spfDrift1 { from { transform: translate3d(-2.5%, -1.5%, 0); } to { transform: translate3d(2.5%, 1.5%, 0); } }
@keyframes spfDrift2 { from { transform: translate3d(2%, -2%, 0); } to { transform: translate3d(-2%, 2%, 0); } }
@keyframes spfDrift3 { from { transform: translate3d(-1.5%, 2%, 0); } to { transform: translate3d(1.5%, -2%, 0); } }`
}

// knownLayouts is the set of per-slide layouts every theme styles. A slide's
// `layout:` frontmatter is matched against this set (case-insensitively); an
// empty or unknown value resolves to "default".
var knownLayouts = map[string]bool{
	"default":  true,
	"center":   true,
	"title":    true,
	"quote":    true, // centered oversized pull-quote
	"section":  true, // section divider: big heading + accent rule
	"two-cols": true, // body flows into two balanced columns (headings span)
	"full":     true, // full-bleed: no padding (e.g. a cover image)
}

// layoutClass maps a slide's raw `layout:` frontmatter value to the CSS class
// slidegen.go adds to the `<section>` (e.g. "center" -> "layout-center"). An
// empty or unknown layout resolves to the default layout, so the section always
// carries exactly one well-formed layout class and unknown values degrade
// gracefully instead of producing an unstyled class.
func layoutClass(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" || !knownLayouts[name] {
		name = "default"
	}
	return "layout-" + name
}

// baseLayoutStyle styles the layouts that compose the same way across every theme
// (quote / section / two-cols / full), once, token-driven. center and title are
// styled per-theme in themes_css.go; these read the active theme's tokens so a
// single rule fits all four. serve.go injects this AFTER the theme CSS so it wins
// equal-specificity ties (e.g. full-bleed clearing the themed slide padding).
func baseLayoutStyle() string {
	return `main.deck > .slide.layout-quote { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
main.deck > .slide.layout-quote blockquote { border: 0; padding: 0; margin: 0; max-width: 22ch; font-weight: 600; font-size: clamp(1.6rem, 3.6vw, 2.9rem); line-height: 1.3; }
main.deck > .slide.layout-section { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
main.deck > .slide.layout-section h1, main.deck > .slide.layout-section h2 { font-size: clamp(2.6rem, 7vw, 5rem); }
main.deck > .slide.layout-section h1::after, main.deck > .slide.layout-section h2::after { content: ""; display: block; width: 4rem; height: 4px; margin: 1.2rem auto 0; border-radius: 2px; background: var(--accent, currentColor); }
main.deck > .slide.layout-two-cols { column-count: 2; column-gap: clamp(2rem, 4vw, 4rem); }
main.deck > .slide.layout-two-cols h1, main.deck > .slide.layout-two-cols h2, main.deck > .slide.layout-two-cols h3 { column-span: all; }
main.deck > .slide.layout-full { padding: 0 !important; }
main.deck > .slide.layout-full img { width: 100%; height: 100vh; max-height: 100vh; margin: 0; border-radius: 0; object-fit: cover; }`
}

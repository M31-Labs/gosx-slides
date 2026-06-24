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
main.deck tr:nth-child(even) td { background: color-mix(in srgb, var(--surface, gray) 35%, transparent); }`
}

// knownLayouts is the set of per-slide layouts every theme styles. A slide's
// `layout:` frontmatter is matched against this set (case-insensitively); an
// empty or unknown value resolves to "default".
var knownLayouts = map[string]bool{
	"default": true,
	"center":  true,
	"title":   true,
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

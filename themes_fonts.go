package slides

// themes_fonts.go is the real lane's WEB-FONT layer (Phase 1 polish). Each theme's
// CSS (themes_css.go) names a designer font in its --font-display / --font-body /
// --font-mono stacks (e.g. aurora: Space Grotesk / Plus Jakarta Sans / JetBrains
// Mono) with a high-quality SYSTEM fallback at the end of every stack. Without the
// actual webfont loaded the browser renders the fallback; this file loads the
// matching designer faces so the themes read as designed.
//
// The single job: declare, in ONE place next to the theme registry, the Google
// Fonts families each theme needs, and emit the matching <link> tags for the
// SELECTED theme only. serve.go's renderPageBody injects fontLinks(theme) into the
// document <head> via the same ctx.AddHead(gosx.RawHTML(...)) path it uses for the
// theme <style> — so exactly one theme's fonts load per page, and the CSS keeps its
// system fallback so an offline deck still looks intentional.
//
// ── How to give a theme its fonts (the whole job) ────────────────────────────
// Add ONE entry to themeFonts below: the theme name -> the Google Fonts `family=`
// query segments matching the families its CSS names (and the weights it uses).
// fontStylesheetHref stitches them into a single css2 request; a theme with no
// entry simply loads no webfont and renders its CSS fallback. Keep this in sync
// with the --font-* stacks in themes_css.go.

import "strings"

// fontPreconnect is the pair of preconnect hints Google Fonts recommends so the
// font CSS + the WOFF2 binaries on the separate gstatic origin start their TCP +
// TLS handshakes early. It is emitted once per page (alongside the stylesheet
// link) for any theme that declares fonts. The gstatic preconnect MUST carry
// crossorigin (fonts are fetched as CORS requests); the apis preconnect must not.
const fontPreconnect = `<link rel="preconnect" href="https://fonts.googleapis.com">` +
	`<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>`

// themeFonts maps a theme name to the ordered Google Fonts `family=…` query
// segments it needs — one per distinct designer family in the theme's --font-*
// stacks (themes_css.go), with the weights that theme actually uses. The segments
// are pre-encoded (`+` for spaces, `:wght@…` for weight axes) exactly as a css2
// request expects, so fontStylesheetHref can join them directly.
//
// A theme whose families are all system fonts (none here today) would simply have
// no entry and load no webfont. Keep this list aligned with themes_css.go: the
// family names here MUST match the first (quoted) family in each --font-* stack.
var themeFonts = map[string][]string{
	// aurora — Space Grotesk (display 500/600/700), Plus Jakarta Sans
	// (body 400/500/600/700), JetBrains Mono (mono 400/600/700).
	"aurora": {
		"Space+Grotesk:wght@500;600;700",
		"Plus+Jakarta+Sans:wght@400;500;600;700",
		"JetBrains+Mono:wght@400;600;700",
	},
	// paper — Playfair Display (display 700, +italic for h3/blockquote),
	// Source Serif 4 (body 400/600, +italic), IBM Plex Mono (mono 400).
	"paper": {
		"Playfair+Display:ital,wght@0,700;1,700",
		"Source+Serif+4:ital,wght@0,400;0,600;1,400",
		"IBM+Plex+Mono:wght@400",
	},
	// neon — same designer triad as aurora (Space Grotesk / Plus Jakarta Sans /
	// JetBrains Mono); display is used uppercase at heavy weights.
	"neon": {
		"Space+Grotesk:wght@500;600;700",
		"Plus+Jakarta+Sans:wght@400;500;600;700",
		"JetBrains+Mono:wght@400;600;700",
	},
	// swiss — Space Grotesk (display 700), Work Sans (body 400/600), JetBrains
	// Mono (mono 400/700).
	"swiss": {
		"Space+Grotesk:wght@500;600;700",
		"Work+Sans:wght@400;500;600;700",
		"JetBrains+Mono:wght@400;700",
	},
}

// fontStylesheetHref builds the single Google Fonts css2 stylesheet URL for a
// theme by joining its family segments with `&family=` and appending
// `&display=swap` (so text paints immediately in the fallback and swaps to the
// webfont when it arrives — no invisible-text flash). It returns "" when the theme
// declares no fonts, so the caller emits no stylesheet link for a webfont-less
// theme.
func fontStylesheetHref(theme string) string {
	families := themeFonts[themeName(theme)]
	if len(families) == 0 {
		return ""
	}
	return "https://fonts.googleapis.com/css2?family=" +
		strings.Join(families, "&family=") + "&display=swap"
}

// fontLinks returns the <head> markup that loads the SELECTED theme's webfonts:
// the two preconnect hints plus the one css2 stylesheet <link>. The theme name is
// resolved through themeName first, so an empty/unknown value yields the default
// theme's fonts (matching the CSS the head also carries). It returns "" when the
// resolved theme declares no fonts (nothing to load — the CSS fallback stands).
//
// serve.go injects this into the document <head> via gosx.RawHTML, the same path
// the theme <style> takes, so only the active theme's fonts load. The CSS keeps a
// system fallback at the end of every --font-* stack, so a deck served offline (or
// before the webfont loads) still renders in an intentional family.
func fontLinks(theme string) string {
	href := fontStylesheetHref(theme)
	if href == "" {
		return ""
	}
	return fontPreconnect +
		`<link href="` + href + `" rel="stylesheet">`
}

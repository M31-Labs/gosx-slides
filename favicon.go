package slides

import (
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxFaviconBytes caps a deck's own favicon file. The icon is inlined into every
// page as a data: URI, so a large file would bloat each response and export.
const maxFaviconBytes = 64 * 1024

// defaultFaviconSVG is the icon for decks that set no `favicon:`: two stacked
// slide cards in the default theme's amber accent. Amber reads on both light and
// dark browser tabs, and the geometry holds at 16 and 32 px.
const defaultFaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
	`<rect x="6" y="5" width="22" height="15" rx="3" fill="#f6b352" opacity=".45"/>` +
	`<rect x="3" y="10" width="24" height="17" rx="3" fill="#f6b352"/>` +
	`<rect x="7" y="15" width="11" height="2.5" rx="1.25" fill="#101820"/>` +
	`<rect x="7" y="20" width="16" height="2.5" rx="1.25" fill="#101820"/></svg>`

var (
	faviconColorRe  = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	themeAccentRe   = regexp.MustCompile(`--accent:\s*(#[0-9a-fA-F]{6})`)
	faviconMimeByEx = map[string]string{
		".svg": "image/svg+xml",
		".png": "image/png",
		".ico": "image/x-icon",
	}
)

// faviconLink returns the <link rel="icon"> tag for the deck. It reads the
// `favicon:` headmatter key in one of three forms:
//
//	favicon: path/to/icon.svg|png|ico        the deck's own file (relative to deck.md)
//	favicon: "🎤"                            one emoji, centred in an SVG
//	favicon: {text: "GT", color: "#10b981"}  a 1-2 letter monogram (color optional)
//
// Every form is inlined as a data: URI, so the icon works offline, in static
// exports, and in single-file snapshots. An unset key yields the default icon.
func faviconLink(d *IslandDeck) (string, error) {
	raw := strings.TrimSpace(deckFrontmatterString(d, "favicon"))
	if raw == "" {
		return iconLink("image/svg+xml", []byte(defaultFaviconSVG)), nil
	}
	switch {
	case strings.HasPrefix(raw, "{"):
		svg, err := monogramFromInline(raw, themeAccent(themeName(deckTheme(d))))
		if err != nil {
			return "", err
		}
		return iconLink("image/svg+xml", []byte(svg)), nil
	case faviconMimeByEx[strings.ToLower(filepath.Ext(raw))] != "":
		return fileFaviconLink(d.Dir, raw)
	case isEmojiFavicon(raw):
		return iconLink("image/svg+xml", []byte(emojiSVG(raw))), nil
	}
	return "", fmt.Errorf("favicon: %q is not a .svg/.png/.ico path, a single emoji, or {text: ..., color: ...}", raw)
}

func iconLink(mime string, data []byte) string {
	return `<link rel="icon" type="` + mime + `" href="data:` + mime + `;base64,` +
		base64.StdEncoding.EncodeToString(data) + `">`
}

func fileFaviconLink(dir, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("favicon: %q must be a path inside the deck directory", rel)
	}
	data, err := os.ReadFile(filepath.Join(dir, clean))
	if err != nil {
		return "", fmt.Errorf("favicon: cannot read %q: %w", rel, err)
	}
	if len(data) > maxFaviconBytes {
		return "", fmt.Errorf("favicon: %q is %d bytes; the limit is %d", rel, len(data), maxFaviconBytes)
	}
	return iconLink(faviconMimeByEx[strings.ToLower(filepath.Ext(clean))], data), nil
}

// isEmojiFavicon accepts a short non-ASCII string (ZWJ and variation-selector
// sequences included) so a typo such as a bare word is rejected, not rendered.
func isEmojiFavicon(s string) bool {
	n := utf8.RuneCountInString(s)
	if n == 0 || n > 10 {
		return false
	}
	for _, r := range s {
		if r < 0x80 {
			return false
		}
	}
	return true
}

func emojiSVG(emoji string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
		`<text x="16" y="17" font-size="26" text-anchor="middle" dominant-baseline="central">` +
		html.EscapeString(emoji) + `</text></svg>`
}

// monogramSVG draws 1-2 letters on a rounded square. text is XML-escaped and
// color must be a #rgb/#rrggbb hex value, so neither can inject markup.
func monogramSVG(text, color string) (string, error) {
	n := utf8.RuneCountInString(text)
	if n < 1 || n > 2 {
		return "", fmt.Errorf("favicon: text %q must be 1 or 2 characters", text)
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("favicon: text contains a control character")
		}
	}
	if !faviconColorRe.MatchString(color) {
		return "", fmt.Errorf("favicon: color %q must be a hex value like #10b981", color)
	}
	size := "19"
	if n == 2 {
		size = "15"
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
		`<rect width="32" height="32" rx="7" fill="` + color + `"/>` +
		`<text x="16" y="17" font-family="system-ui,-apple-system,Segoe UI,sans-serif" font-weight="700" font-size="` + size +
		`" fill="` + contrastInk(color) + `" text-anchor="middle" dominant-baseline="central">` +
		html.EscapeString(text) + `</text></svg>`, nil
}

// monogramFromInline parses `{text: "GT", color: "#10b981"}`. The headmatter
// parser is flat, so this reads the one inline-map form directly.
func monogramFromInline(raw, defaultColor string) (string, error) {
	if !strings.HasSuffix(raw, "}") {
		return "", fmt.Errorf("favicon: %q is missing a closing }", raw)
	}
	text, color := "", defaultColor
	for _, part := range splitOutsideQuotes(raw[1 : len(raw)-1]) {
		key, val, ok := strings.Cut(part, ":")
		if !ok {
			return "", fmt.Errorf("favicon: %q is not key: value", strings.TrimSpace(part))
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		switch strings.TrimSpace(key) {
		case "text":
			text = val
		case "color":
			color = val
		default:
			return "", fmt.Errorf("favicon: unknown key %q (use text and color)", strings.TrimSpace(key))
		}
	}
	return monogramSVG(text, color)
}

func splitOutsideQuotes(s string) []string {
	var parts []string
	var quote rune
	start := 0
	for i, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ',':
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		parts = append(parts, rest)
	}
	return parts
}

// themeAccent returns the theme's --accent hex colour, or the default amber.
func themeAccent(theme string) string {
	if m := themeAccentRe.FindStringSubmatch(themeCSS(theme)); m != nil {
		return m[1]
	}
	return "#f6b352"
}

// contrastInk picks dark or white text for a hex background by relative luma.
func contrastInk(hex string) string {
	h := strings.TrimPrefix(hex, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	var r, g, b int
	fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b)
	if (299*r+587*g+114*b)/1000 > 150 {
		return "#101820"
	}
	return "#ffffff"
}

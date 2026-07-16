package slides

// deck_css.go is the first-class per-deck stylesheet lane. A deck restyles or
// extends its theme by dropping a `deck.css` (or `style.css`) next to deck.md,
// or naming files explicitly in headmatter:
//
//	---
//	css: brand.css, overrides/dark.css
//	---
//
// The resolved CSS is inlined into the document <head> AFTER the theme
// stylesheet, so at equal specificity the deck's rules win the cascade. It is
// inlined (not linked) so it rides both static export formats for free and so
// dev-mode's per-request re-render picks up edits on a plain browser refresh.
//
// This replaces the previous workaround of a hand-written island that emitted
// a <link> into the body (the GopherCon deck's DeckStyle.gsx) — a stylesheet
// is deck configuration, not a component.

import (
	"os"
	"path/filepath"
	"strings"
)

// deckDefaultCSSFiles are the conventional stylesheet names picked up
// automatically when headmatter names nothing, in priority order (the first
// that exists wins — they are alternatives, not a chain).
var deckDefaultCSSFiles = []string{"deck.css", "style.css"}

// deckCSSFiles resolves which stylesheet files apply to the deck: the
// headmatter `css:` list when present (comma- or space-separated, relative to
// the deck dir), else the first conventional default that exists. Paths that
// escape the deck directory (absolute, or ../ traversal) are rejected.
// Returned paths are deck-relative; missing named files are kept in the list
// (doctor reports them) but skipped at read time.
func deckCSSFiles(deck *IslandDeck) []string {
	if deck == nil {
		return nil
	}
	if raw := strings.TrimSpace(deckFrontmatterString(deck, "css")); raw != "" {
		var files []string
		for _, name := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			if name = strings.TrimSpace(name); name != "" && safeDeckRelPath(name) {
				files = append(files, name)
			}
		}
		return files
	}
	for _, name := range deckDefaultCSSFiles {
		if _, err := os.Stat(filepath.Join(deck.Dir, name)); err == nil {
			return []string{name}
		}
	}
	return nil
}

// safeDeckRelPath reports whether a headmatter-named path stays inside the
// deck directory: relative, and free of ../ traversal after cleaning.
func safeDeckRelPath(name string) bool {
	if filepath.IsAbs(name) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

// deckCustomCSS reads and concatenates the deck's resolved stylesheet files.
// A missing or unreadable file contributes nothing (fail-soft — doctor is the
// loud path). The single unsafe sequence for inline embedding, `</style`, is
// neutralized so a stylesheet can never break out of its <style> element.
func deckCustomCSS(deck *IslandDeck) string {
	var b strings.Builder
	for _, name := range deckCSSFiles(deck) {
		data, err := os.ReadFile(filepath.Join(deck.Dir, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.Write(data)
	}
	return strings.ReplaceAll(b.String(), "</style", "<\\/style")
}

// deckFrontmatterString reads one deck-headmatter value ("" when absent).
func deckFrontmatterString(deck *IslandDeck, key string) string {
	headmatter, _, err := splitHeadmatter(string(deck.Source))
	if err != nil {
		return ""
	}
	return parseFrontmatter(headmatter)[key]
}

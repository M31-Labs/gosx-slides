package slides

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SplitDeck writes a parsed deck as deck.md plus slides/*.md.
func SplitDeck(inputPath, outDir string) error {
	deck, err := ParseFile(inputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "slides"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "deck.md"), []byte(deckConfigSource(deck)+"\n"), 0o644); err != nil {
		return err
	}
	for _, slide := range deck.Slides {
		name := fmt.Sprintf("%02d-%s.md", slide.Index+1, slugify(slide.Title))
		if err := os.WriteFile(filepath.Join(outDir, "slides", name), []byte(slideSource(deck, slide)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// MergeDeck writes any single-file or directory deck as one split-by-separator file.
func MergeDeck(inputPath, outPath string) error {
	deck, err := ParseFile(inputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(MergedSource(deck)), 0o644)
}

// MergedSource serializes a deck as a single markdown file.
func MergedSource(deck *Deck) string {
	var buf strings.Builder
	buf.WriteString(deckConfigSource(deck))
	for _, slide := range deck.Slides {
		buf.WriteString("\n---\n")
		buf.WriteString(strings.TrimSpace(slideSource(deck, slide)))
		buf.WriteString("\n")
	}
	return FormatSource(buf.String())
}

func deckConfigSource(deck *Deck) string {
	config := map[string]string{}
	for k, v := range deck.Config {
		config[k] = v
	}
	config["title"] = deck.Title
	config["theme"] = deck.Theme
	config["transition"] = deck.Transition
	return frontmatterSource(config)
}

func slideSource(deck *Deck, slide Slide) string {
	fm := map[string]string{}
	for k, v := range slide.Frontmatter {
		fm[k] = v
	}
	if slide.Layout != "" && slide.Layout != "default" {
		fm["layout"] = slide.Layout
	}
	if slide.Clicks > 0 {
		fm["clicks"] = itoa(slide.Clicks)
	}
	if slide.Class != "" {
		fm["class"] = slide.Class
	}
	if slide.Transition != "" && slide.Transition != deck.Transition {
		fm["transition"] = slide.Transition
	}
	var buf strings.Builder
	if len(fm) > 0 {
		buf.WriteString(frontmatterSource(fm))
		buf.WriteString("\n")
	}
	buf.WriteString(strings.TrimSpace(slide.Body))
	if slide.Notes != "" {
		buf.WriteString("\n\n<Notes>\n")
		buf.WriteString(strings.TrimSpace(slide.Notes))
		buf.WriteString("\n</Notes>")
	}
	buf.WriteString("\n")
	return FormatSource(buf.String())
}

func frontmatterSource(values map[string]string) string {
	var buf strings.Builder
	buf.WriteString("---\n")
	written := map[string]bool{}
	for _, key := range []string{"title", "theme", "transition", "layout", "clicks", "class"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			buf.WriteString(key)
			buf.WriteString(": ")
			buf.WriteString(value)
			buf.WriteString("\n")
			written[key] = true
		}
	}
	var rest []string
	for key := range values {
		if !written[key] && strings.TrimSpace(values[key]) != "" {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(strings.TrimSpace(values[key]))
		buf.WriteString("\n")
	}
	buf.WriteString("---\n")
	return buf.String()
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var buf strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			buf.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			buf.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(buf.String(), "-")
	if out == "" {
		return "slide"
	}
	return out
}

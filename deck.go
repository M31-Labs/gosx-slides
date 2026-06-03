package slides

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Deck is the parsed, render-ready model for one presentation.
type Deck struct {
	Title       string
	Theme       string
	Transition  string
	Config      map[string]string
	Slides      []Slide
	SourcePath  string
	SourceFiles []string
	BaseDir     string
}

// Slide is one slide plus the metadata needed by presenter and export views.
type Slide struct {
	Index       int
	Layout      string
	Title       string
	Body        string
	Notes       string
	Clicks      int
	Class       string
	Transition  string
	SourcePath  string
	Frontmatter map[string]string
}

// Summary is returned by Check for concise CLI reporting.
type Summary struct {
	Title       string
	SlideCount  int
	TotalClicks int
	Layouts     map[string]int
	Notes       int
}

// ParseOptions carries source identity through parser errors.
type ParseOptions struct {
	SourcePath string
}

// ParseFile reads and parses a deck file.
func ParseFile(path string) (*Deck, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return ParseDirectory(path)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(src), ParseOptions{SourcePath: path})
}

// ParseDirectory reads a multi-file deck directory.
func ParseDirectory(dir string) (*Deck, error) {
	dir = filepath.Clean(dir)
	entry := filepath.Join(dir, "deck.md")
	if _, err := os.Stat(entry); err != nil {
		entry = filepath.Join(dir, "index.md")
		if _, indexErr := os.Stat(entry); indexErr != nil {
			return nil, fmt.Errorf("%s must contain deck.md or index.md", dir)
		}
	}

	data, err := os.ReadFile(entry)
	if err != nil {
		return nil, err
	}
	src := normalizeNewlines(string(data))
	src, included, err := expandIncludes(src, filepath.Dir(entry), map[string]bool{})
	if err != nil {
		return nil, err
	}
	headmatter, body, err := splitHeadmatter(src)
	if err != nil {
		return nil, err
	}
	config := parseFrontmatter(headmatter)
	deck := newDeck(config, ParseOptions{SourcePath: entry})
	deck.BaseDir = dir
	deck.SourceFiles = uniqueStrings(append([]string{entry}, included...))

	var rawSlides []rawSlide
	if strings.TrimSpace(body) != "" {
		slides, err := splitSlideSections(body)
		if err != nil {
			return nil, err
		}
		for _, slide := range slides {
			slide.SourcePath = entry
			rawSlides = append(rawSlides, slide)
		}
	}

	slideFiles, err := filepath.Glob(filepath.Join(dir, "slides", "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(slideFiles)
	for _, file := range slideFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		src, included, err := expandIncludes(normalizeNewlines(string(data)), filepath.Dir(file), map[string]bool{})
		if err != nil {
			return nil, err
		}
		deck.SourceFiles = uniqueStrings(append(append(deck.SourceFiles, file), included...))
		slides, err := splitSlideFileSections(src, file)
		if err != nil {
			return nil, err
		}
		rawSlides = append(rawSlides, slides...)
	}
	if len(rawSlides) == 0 {
		return nil, fmt.Errorf("deck has no slides")
	}
	if err := populateSlides(deck, rawSlides); err != nil {
		return nil, err
	}
	return deck, nil
}

// Parse converts Markdown++-shaped deck source into a deck model.
func Parse(src string, opts ParseOptions) (*Deck, error) {
	src = normalizeNewlines(src)
	var included []string
	if opts.SourcePath != "" {
		expanded, files, err := expandIncludes(src, filepath.Dir(opts.SourcePath), map[string]bool{})
		if err != nil {
			return nil, err
		}
		src = expanded
		included = files
	}
	headmatter, body, err := splitHeadmatter(src)
	if err != nil {
		return nil, err
	}
	config := parseFrontmatter(headmatter)
	rawSlides, err := splitSlideSections(body)
	if err != nil {
		return nil, err
	}
	if len(rawSlides) == 0 {
		return nil, fmt.Errorf("deck has no slides")
	}

	deck := newDeck(config, opts)
	if opts.SourcePath != "" {
		deck.BaseDir = filepath.Dir(opts.SourcePath)
		deck.SourceFiles = uniqueStrings(append([]string{opts.SourcePath}, included...))
	}
	for i := range rawSlides {
		rawSlides[i].SourcePath = opts.SourcePath
	}
	if err := populateSlides(deck, rawSlides); err != nil {
		return nil, err
	}

	return deck, nil
}

func newDeck(config map[string]string, opts ParseOptions) *Deck {
	return &Deck{
		Title:      valueOr(config["title"], "Untitled Deck"),
		Theme:      valueOr(config["theme"], "m31"),
		Transition: valueOr(config["transition"], "slide"),
		Config:     config,
		SourcePath: opts.SourcePath,
	}
}

func populateSlides(deck *Deck, rawSlides []rawSlide) error {
	for i, raw := range rawSlides {
		fm := parseFrontmatter(raw.Frontmatter)
		body, notes := extractNotes(raw.Body)
		slide := Slide{
			Index:       i,
			Layout:      valueOr(fm["layout"], "default"),
			Body:        strings.TrimSpace(body),
			Notes:       strings.TrimSpace(notes),
			Class:       fm["class"],
			Transition:  valueOr(fm["transition"], deck.Transition),
			SourcePath:  raw.SourcePath,
			Frontmatter: fm,
		}
		if title := firstHeading(slide.Body); title != "" {
			slide.Title = title
		} else {
			slide.Title = fmt.Sprintf("Slide %d", i+1)
		}
		if rawClicks := fm["clicks"]; rawClicks != "" {
			clicks, err := strconv.Atoi(rawClicks)
			if err != nil || clicks < 0 {
				return fmt.Errorf("slide %d has invalid clicks value %q", i+1, rawClicks)
			}
			slide.Clicks = clicks
		} else {
			slide.Clicks = inferClicks(slide.Body)
		}
		deck.Slides = append(deck.Slides, slide)
	}
	return nil
}

// Check parses a deck and returns a small quality summary.
func Check(path string) (*Summary, error) {
	deck, err := ParseFile(path)
	if err != nil {
		return nil, err
	}
	summary := &Summary{
		Title:      deck.Title,
		SlideCount: len(deck.Slides),
		Layouts:    map[string]int{},
	}
	for _, slide := range deck.Slides {
		summary.TotalClicks += slide.Clicks
		summary.Layouts[slide.Layout]++
		if slide.Notes != "" {
			summary.Notes++
		}
	}
	return summary, nil
}

// FormatSource normalizes deck source while preserving author content.
func FormatSource(src string) string {
	src = normalizeNewlines(src)
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	out := strings.Join(lines, "\n")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out) + "\n"
}

func normalizeNewlines(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return src
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

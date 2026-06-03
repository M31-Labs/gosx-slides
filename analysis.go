package slides

import (
	"regexp"
	"sort"
	"strings"
)

// DeckAnalysis is a structured authoring report for one deck.
type DeckAnalysis struct {
	Title            string          `json:"title"`
	Theme            string          `json:"theme"`
	SourceFiles      []string        `json:"sourceFiles"`
	SlideCount       int             `json:"slideCount"`
	TotalClicks      int             `json:"totalClicks"`
	WordCount        int             `json:"wordCount"`
	EstimatedSeconds int             `json:"estimatedSeconds"`
	Layouts          map[string]int  `json:"layouts"`
	Components       map[string]int  `json:"components"`
	Citations        []CitationRef   `json:"citations"`
	Checkpoints      []CheckpointRef `json:"checkpoints"`
	Warnings         []string        `json:"warnings"`
	Slides           []SlideAnalysis `json:"slides"`
}

// CitationRef is one source reference found in a deck.
type CitationRef struct {
	SlideIndex int    `json:"slideIndex"`
	Label      string `json:"label"`
	Href       string `json:"href"`
}

// CheckpointRef is one named presenter jump target found in a deck.
type CheckpointRef struct {
	SlideIndex int    `json:"slideIndex"`
	ID         string `json:"id"`
	Label      string `json:"label"`
}

// SlideAnalysis captures per-slide authoring signals.
type SlideAnalysis struct {
	Index            int             `json:"index"`
	Title            string          `json:"title"`
	Layout           string          `json:"layout"`
	Clicks           int             `json:"clicks"`
	Words            int             `json:"words"`
	EstimatedSeconds int             `json:"estimatedSeconds"`
	Components       []string        `json:"components"`
	Citations        []CitationRef   `json:"citations"`
	Checkpoints      []CheckpointRef `json:"checkpoints"`
	HasNotes         bool            `json:"hasNotes"`
}

// Analyze returns a structured deck report.
func Analyze(deck *Deck) DeckAnalysis {
	out := DeckAnalysis{
		Title:       deck.Title,
		Theme:       deck.Theme,
		SourceFiles: append([]string(nil), deck.SourceFiles...),
		SlideCount:  len(deck.Slides),
		Layouts:     map[string]int{},
		Components:  map[string]int{},
	}
	if !isKnownTheme(deck.Theme) {
		out.Warnings = append(out.Warnings, "deck: unknown built-in theme "+deck.Theme)
	}
	for _, slide := range deck.Slides {
		components := detectComponents(slide.Body)
		citations := extractCitations(slide.Body, slide.Index)
		checkpoints := extractCheckpoints(slide.Body, slide.Index)
		words := countWords(stripMarkup(slide.Body))
		estimated := 18 + slide.Clicks*5 + words/3
		out.TotalClicks += slide.Clicks
		out.WordCount += words
		out.EstimatedSeconds += estimated
		out.Layouts[slide.Layout]++
		for _, component := range components {
			out.Components[component]++
		}
		out.Citations = append(out.Citations, citations...)
		out.Checkpoints = append(out.Checkpoints, checkpoints...)
		out.Slides = append(out.Slides, SlideAnalysis{
			Index:            slide.Index,
			Title:            slide.Title,
			Layout:           slide.Layout,
			Clicks:           slide.Clicks,
			Words:            words,
			EstimatedSeconds: estimated,
			Components:       components,
			Citations:        citations,
			Checkpoints:      checkpoints,
			HasNotes:         slide.Notes != "",
		})
		if !knownLayout(slide.Layout) {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": unknown layout "+slide.Layout)
		}
		if words > 115 {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": dense slide with "+itoa(words)+" words")
		}
		if slide.Clicks > 8 {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": high click count "+itoa(slide.Clicks))
		}
		if slide.Notes == "" {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": no presenter notes")
		}
	}
	sort.Strings(out.Warnings)
	return out
}

func detectComponents(src string) []string {
	names := []string{"Scene3D", "Canvas", "Diagram", "Agenda", "Pipeline", "ParseTree", "Benchmark", "Citation", "Takeaway", "Metric", "Metrics", "Callout", "Poll", "Timeline", "Step", "Steps", "QueryDemo", "ProfileBuckets", "ParityMatrix", "CorpusRun", "GrammarBlob", "Checkpoint"}
	var found []string
	for _, name := range names {
		if strings.Contains(src, "<"+name) {
			found = append(found, name)
		}
	}
	if regexp.MustCompile("(?m)^```").FindStringIndex(src) != nil {
		found = append(found, "Code")
	}
	sort.Strings(found)
	return found
}

func extractCitations(src string, slideIndex int) []CitationRef {
	re := regexp.MustCompile(`(?is)<Citation\b[^>]*>`)
	var refs []CitationRef
	for _, match := range re.FindAllString(src, -1) {
		attrs := parseComponentAttrs(match)
		href := attrs["href"]
		refs = append(refs, CitationRef{
			SlideIndex: slideIndex,
			Label:      citationLabel(href, attrs["label"]),
			Href:       href,
		})
	}
	return refs
}

func extractCheckpoints(src string, slideIndex int) []CheckpointRef {
	re := regexp.MustCompile(`(?is)<Checkpoint\b[^>]*>`)
	var refs []CheckpointRef
	for _, match := range re.FindAllString(src, -1) {
		attrs := parseComponentAttrs(match)
		id := valueOr(attrs["id"], "slide-"+itoa(slideIndex+1))
		label := valueOr(attrs["label"], id)
		refs = append(refs, CheckpointRef{SlideIndex: slideIndex, ID: id, Label: label})
	}
	return refs
}

func citationLabel(href, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	href = strings.TrimSpace(href)
	if href == "" {
		return "Source"
	}
	if strings.HasPrefix(href, "hypha://") {
		return "Hyphae: " + humanizeRef(href)
	}
	return href
}

func humanizeRef(ref string) string {
	last := ref
	if idx := strings.LastIndex(last, "/"); idx >= 0 {
		last = last[idx+1:]
	}
	for _, prefix := range []string{"concept.", "decision.", "initiative.", "protocol.", "spore.", "plan.", "docs."} {
		last = strings.TrimPrefix(last, prefix)
	}
	last = strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(last)
	words := strings.Fields(last)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func stripMarkup(src string) string {
	src = regexp.MustCompile("(?s)<Notes>.*?</Notes>").ReplaceAllString(src, " ")
	src = regexp.MustCompile("(?s)<!--.*?-->").ReplaceAllString(src, " ")
	src = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(src, " ")
	src = regexp.MustCompile("<[^>]+>").ReplaceAllString(src, " ")
	src = regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(src, " ")
	return src
}

func countWords(src string) int {
	return len(regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]*`).FindAllString(src, -1))
}

func knownLayout(layout string) bool {
	switch layout {
	case "default", "center", "cover", "section", "two-cols", "image-right", "quote", "full":
		return true
	default:
		return false
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

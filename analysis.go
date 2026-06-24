package slides

import (
	"regexp"
	"strings"
)

// analysis.go holds the analysis result types and the small lane-agnostic helpers
// shared by the deck analysis (deck_analysis.go). The previous string-based Analyze and
// its helpers were removed with the fallback lane.

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

// Summary is the concise per-deck report returned by Check.
type Summary struct {
	Title       string
	SlideCount  int
	TotalClicks int
	Layouts     map[string]int
	Notes       int
}

// citationLabel resolves a human label for a <Citation/>: an explicit label wins,
// else a hypha:// ref is humanized, else the raw href, else "Source".
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

// wordRe matches a prose word for the density/word-count heuristic. Hoisted to
// package scope so it compiles once, not per slide.
var wordRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]*`)

func countWords(src string) int {
	return len(wordRe.FindAllString(src, -1))
}

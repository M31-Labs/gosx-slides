package slides

import (
	"fmt"
	"strings"
)

// RehearsalScript renders a compact speaker run sheet.
func RehearsalScript(deck *Deck) string {
	analysis := Analyze(deck)
	var buf strings.Builder
	buf.WriteString(deck.Title)
	buf.WriteString("\n")
	buf.WriteString("estimated: ")
	buf.WriteString(formatSeconds(analysis.EstimatedSeconds))
	buf.WriteString("\n\n")
	for _, slide := range analysis.Slides {
		buf.WriteString(fmt.Sprintf("%02d. %s\n", slide.Index+1, slide.Title))
		buf.WriteString("    layout: ")
		buf.WriteString(slide.Layout)
		buf.WriteString("  clicks: ")
		buf.WriteString(itoa(slide.Clicks))
		buf.WriteString("  estimate: ")
		buf.WriteString(formatSeconds(slide.EstimatedSeconds))
		buf.WriteString("\n")
		if len(slide.Components) > 0 {
			buf.WriteString("    components: ")
			buf.WriteString(strings.Join(slide.Components, ", "))
			buf.WriteString("\n")
		}
		notes := strings.TrimSpace(deck.Slides[slide.Index].Notes)
		if notes == "" {
			notes = "No notes."
		}
		for _, line := range strings.Split(notes, "\n") {
			buf.WriteString("    ")
			buf.WriteString(strings.TrimSpace(line))
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}
	if len(analysis.Warnings) > 0 {
		buf.WriteString("warnings:\n")
		for _, warning := range analysis.Warnings {
			buf.WriteString("- ")
			buf.WriteString(warning)
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func formatSeconds(total int) string {
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%dm%02ds", total/60, total%60)
}

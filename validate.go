package slides

import "strings"

// ValidateOptions configures profile-aware deck validation.
type ValidateOptions struct {
	Profile string
}

// ValidationReport is the CI-friendly validation result.
type ValidationReport struct {
	Profile  string       `json:"profile"`
	Analysis DeckAnalysis `json:"analysis"`
	Warnings []string     `json:"warnings"`
	Errors   []string     `json:"errors"`
}

// Validate applies the named profile to a parsed deck.
func Validate(deck *Deck, opts ValidateOptions) ValidationReport {
	profile := strings.ToLower(strings.TrimSpace(opts.Profile))
	if profile == "" {
		profile = "standard"
	}
	report := ValidationReport{
		Profile:  profile,
		Analysis: Analyze(deck),
	}
	report.Warnings = append(report.Warnings, report.Analysis.Warnings...)
	switch profile {
	case "standard":
	case "conference":
		validateConference(deck, &report)
	case "demo":
		validateDemo(deck, &report)
	case "lecture":
		validateLecture(deck, &report)
	default:
		report.Errors = append(report.Errors, "unknown validation profile "+profile)
	}
	report.Warnings = uniqueStrings(report.Warnings)
	report.Errors = uniqueStrings(report.Errors)
	return report
}

// Passed reports whether validation succeeds for normal or strict mode.
func (report ValidationReport) Passed(strict bool) bool {
	if len(report.Errors) > 0 {
		return false
	}
	return !strict || len(report.Warnings) == 0
}

func validateConference(deck *Deck, report *ValidationReport) {
	if report.Analysis.EstimatedSeconds > 45*60 {
		report.Errors = append(report.Errors, "conference: estimated runtime exceeds 45 minutes")
	}
	for _, slide := range report.Analysis.Slides {
		if !slide.HasNotes {
			report.Errors = append(report.Errors, "conference: slide "+itoa(slide.Index+1)+" needs presenter notes")
		}
		if slide.Words > 100 {
			report.Warnings = append(report.Warnings, "conference: slide "+itoa(slide.Index+1)+" is dense for a talk track")
		}
		if hasAny(slide.Components, []string{"Benchmark", "ProfileBuckets", "CorpusRun", "ParityMatrix"}) && !hasComponent(slide.Components, "Citation") {
			report.Errors = append(report.Errors, "conference: slide "+itoa(slide.Index+1)+" has evidence without a Citation")
		}
	}
}

func validateDemo(deck *Deck, report *ValidationReport) {
	for _, slide := range report.Analysis.Slides {
		if hasAny(slide.Components, []string{"Scene3D", "Canvas", "Poll", "QueryDemo"}) && !slide.HasNotes {
			report.Warnings = append(report.Warnings, "demo: slide "+itoa(slide.Index+1)+" has an interactive surface without fallback notes")
		}
	}
	_ = deck
}

func validateLecture(deck *Deck, report *ValidationReport) {
	withNotes := 0
	for _, slide := range report.Analysis.Slides {
		if slide.HasNotes {
			withNotes++
		}
		if slide.Words > 165 {
			report.Warnings = append(report.Warnings, "lecture: slide "+itoa(slide.Index+1)+" is very dense")
		}
	}
	if len(deck.Slides) > 0 && withNotes*100/len(deck.Slides) < 80 {
		report.Errors = append(report.Errors, "lecture: at least 80% of slides need notes for handout coverage")
	}
}

func hasAny(values, wanted []string) bool {
	for _, want := range wanted {
		if hasComponent(values, want) {
			return true
		}
	}
	return false
}

func hasComponent(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

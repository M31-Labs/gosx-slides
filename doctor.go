package slides

import (
	"os"
	"strings"
)

// DoctorItem is one environment or deck health check.
type DoctorItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// DoctorReport is returned by Doctor.
type DoctorReport struct {
	Items []DoctorItem `json:"items"`
}

// HasFailures reports whether any doctor item failed.
func (report DoctorReport) HasFailures() bool {
	for _, item := range report.Items {
		if item.Status == "fail" {
			return true
		}
	}
	return false
}

// Doctor checks the local deck and export environment.
func Doctor(deckPath string) (DoctorReport, error) {
	var report DoctorReport
	deck, err := ParseFile(deckPath)
	if err != nil {
		report.Items = append(report.Items, DoctorItem{Name: "deck", Status: "fail", Detail: err.Error()})
		return report, nil
	}
	analysis := Analyze(deck)
	report.Items = append(report.Items, DoctorItem{Name: "deck", Status: "ok", Detail: deck.Title + " / " + itoa(len(deck.Slides)) + " slides"})
	if len(deck.SourceFiles) > 1 {
		report.Items = append(report.Items, DoctorItem{Name: "sources", Status: "ok", Detail: itoa(len(deck.SourceFiles)) + " source files"})
	} else {
		report.Items = append(report.Items, DoctorItem{Name: "sources", Status: "ok", Detail: "single-file deck"})
	}
	notes := 0
	for _, slide := range analysis.Slides {
		if slide.HasNotes {
			notes++
		}
	}
	status := "ok"
	if notes < len(deck.Slides) {
		status = "warn"
	}
	report.Items = append(report.Items, DoctorItem{Name: "notes", Status: status, Detail: itoa(notes) + "/" + itoa(len(deck.Slides)) + " slides have notes"})
	if len(analysis.Components) > 0 {
		report.Items = append(report.Items, DoctorItem{Name: "components", Status: "ok", Detail: strings.Join(componentNames(analysis.Components), ", ")})
	}
	if chrome, err := findChrome(); err == nil {
		report.Items = append(report.Items, DoctorItem{Name: "chrome", Status: "ok", Detail: chrome})
	} else {
		report.Items = append(report.Items, DoctorItem{Name: "chrome", Status: "warn", Detail: err.Error()})
	}
	if _, err := os.Stat(deckBaseDir(deckPath)); err == nil {
		report.Items = append(report.Items, DoctorItem{Name: "baseDir", Status: "ok", Detail: deckBaseDir(deckPath)})
	}
	return report, nil
}

package slides

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"m31labs.dev/mdpp"
)

// analysis_island.go is the REAL-lane authoring analysis: check / inspect /
// validate / rehearse / doctor / components computed over an IslandDeck (the
// mdpp-parsed real deck), not the fallback Deck. It replaces the fallback
// analysis path so these tools stop reporting wrong themes/layouts/components on
// a real-lane deck. Every datum (layout, notes, clicks, title, components) is
// recovered from the slide's mdpp subtree via the helpers below — IslandSlide
// needs no extra fields.

// braceExprRe matches an inline {expr} so prose word counts exclude it (mirrors
// the fallback stripMarkup, which dropped `{…}` before counting).
var braceExprRe = regexp.MustCompile(`\{[^}]*\}`)

// slidePlainText returns a slide's prose with code blocks, components, raw HTML
// (which carries <Notes>/comment speaker notes), and {expr} interpolations
// excluded — the real-lane analog of the fallback stripMarkup. Used for word
// counts and density warnings.
func slidePlainText(slide IslandSlide) string {
	if slide.Node == nil {
		return ""
	}
	var b strings.Builder
	slide.Node.Walk(func(n *mdpp.Node) bool {
		switch n.Type {
		case mdpp.NodeCodeBlock, mdpp.NodeComponent, mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline, mdpp.NodeExpression:
			return false // skip the whole subtree: not prose
		case mdpp.NodeText:
			b.WriteString(n.Literal)
			b.WriteByte(' ')
		}
		return true
	})
	return braceExprRe.ReplaceAllString(b.String(), " ")
}

// slideWordCount counts prose words on a slide (excludes code/components/notes).
func slideWordCount(slide IslandSlide) int {
	return countWords(slidePlainText(slide))
}

// slideTitle is a slide's first heading text, else "Slide N" (1-based).
func slideTitle(slide IslandSlide) string {
	if slide.Node != nil {
		var found string
		slide.Node.Walk(func(n *mdpp.Node) bool {
			if found != "" {
				return false
			}
			if n.Level() >= 1 {
				found = strings.TrimSpace(n.Text())
				return false
			}
			return true
		})
		if found != "" {
			return found
		}
	}
	return "Slide " + itoa(slide.Index+1)
}

// slideLayoutInfo returns a slide's resolved layout name and whether the authored
// value is one the themes style. An absent layout resolves to "default" (known);
// a present-but-unknown layout is reported by name with known=false so callers
// can warn — validated against the REAL knownLayouts (themes.go), not the
// fallback layout set.
func slideLayoutInfo(slide IslandSlide) (name string, known bool) {
	raw, _ := slideFrontmatterValues(slide)["layout"].(string)
	name = strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return "default", true
	}
	return name, knownLayouts[name]
}

// slideClickCount is a slide's click budget: the max number of reveal steps over
// its stepped code blocks. Mirrors the client stepCountFor (nav.go), which takes
// the max data-steps across the slide's <pre> elements.
func slideClickCount(slide IslandSlide) int {
	if slide.Node == nil {
		return 0
	}
	max := 0
	for _, cb := range slide.Node.Find(mdpp.NodeCodeBlock) {
		if n := len(parseHighlightSteps(cb.Attr("highlights"))); n > max {
			max = n
		}
	}
	return max
}

// slideComponentNames returns the distinct, sorted island names a slide uses
// (its actual <Name/> .gsx references), not a fallback registry.
func slideComponentNames(slide IslandSlide) []string {
	seen := map[string]bool{}
	var out []string
	for _, ref := range slide.Components {
		if !seen[ref.Name] {
			seen[ref.Name] = true
			out = append(out, ref.Name)
		}
	}
	sort.Strings(out)
	return out
}

// slideCitations / slideCheckpoints surface <Citation/> and <Checkpoint/> island
// references (with their props) from a slide, so the conference profile and
// presenter jumps keep working over the real component model.
func slideCitations(slide IslandSlide) []CitationRef {
	var refs []CitationRef
	for _, ref := range slide.Components {
		if ref.Name != "Citation" {
			continue
		}
		props := parseProps(ref.Props)
		href, _ := props["href"].(string)
		label, _ := props["label"].(string)
		refs = append(refs, CitationRef{
			SlideIndex: slide.Index,
			Label:      citationLabel(href, label),
			Href:       href,
		})
	}
	return refs
}

func slideCheckpoints(slide IslandSlide) []CheckpointRef {
	var refs []CheckpointRef
	for _, ref := range slide.Components {
		if ref.Name != "Checkpoint" {
			continue
		}
		props := parseProps(ref.Props)
		id, _ := props["id"].(string)
		label, _ := props["label"].(string)
		if strings.TrimSpace(id) == "" {
			id = "slide-" + itoa(slide.Index+1)
		}
		if strings.TrimSpace(label) == "" {
			label = id
		}
		refs = append(refs, CheckpointRef{SlideIndex: slide.Index, ID: id, Label: label})
	}
	return refs
}

// Analyze is the real-lane Analyze: a structured authoring report computed
// from an IslandDeck. Themes and layouts are validated against the REAL
// vocabularies (themeRegistry / knownLayouts), and components are the deck's
// actual .gsx islands.
func Analyze(d *IslandDeck) DeckAnalysis {
	theme := deckTheme(d)
	out := DeckAnalysis{
		Title:       d.title(),
		Theme:       theme,
		SourceFiles: []string{DeckFileName},
		SlideCount:  len(d.Slides),
		Layouts:     map[string]int{},
		Components:  map[string]int{},
	}
	if norm := strings.TrimSpace(strings.ToLower(theme)); norm != "" && themeName(theme) != norm {
		out.Warnings = append(out.Warnings, "deck: unknown theme "+theme+" (using "+defaultTheme+")")
	}
	for _, slide := range d.Slides {
		layoutName, layoutKnown := slideLayoutInfo(slide)
		words := slideWordCount(slide)
		clicks := slideClickCount(slide)
		notes := extractSlideNotes(slide)
		components := slideComponentNames(slide)
		citations := slideCitations(slide)
		checkpoints := slideCheckpoints(slide)
		estimated := 18 + clicks*5 + words/3
		out.TotalClicks += clicks
		out.WordCount += words
		out.EstimatedSeconds += estimated
		out.Layouts[layoutName]++
		for _, component := range components {
			out.Components[component]++
		}
		out.Citations = append(out.Citations, citations...)
		out.Checkpoints = append(out.Checkpoints, checkpoints...)
		out.Slides = append(out.Slides, SlideAnalysis{
			Index:            slide.Index,
			Title:            slideTitle(slide),
			Layout:           layoutName,
			Clicks:           clicks,
			Words:            words,
			EstimatedSeconds: estimated,
			Components:       components,
			Citations:        citations,
			Checkpoints:      checkpoints,
			HasNotes:         notes != "",
		})
		if !layoutKnown {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": unknown layout "+layoutName)
		}
		if words > 115 {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": dense slide with "+itoa(words)+" words")
		}
		if clicks > 8 {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": high click count "+itoa(clicks))
		}
		if notes == "" {
			out.Warnings = append(out.Warnings, "slide "+itoa(slide.Index+1)+": no presenter notes")
		}
	}
	sort.Strings(out.Warnings)
	return out
}

// Check is the real-lane Check: concise per-deck counts from an IslandDeck.
func Check(dir string) (*Summary, error) {
	d, err := LoadIslandDeck(dir)
	if err != nil {
		return nil, err
	}
	summary := &Summary{Title: d.title(), SlideCount: len(d.Slides), Layouts: map[string]int{}}
	for _, slide := range d.Slides {
		name, _ := slideLayoutInfo(slide)
		summary.Layouts[name]++
		summary.TotalClicks += slideClickCount(slide)
		if extractSlideNotes(slide) != "" {
			summary.Notes++
		}
	}
	return summary, nil
}

// Validate is the real-lane Validate: profile rules over Analyze.
func Validate(d *IslandDeck, opts ValidateOptions) ValidationReport {
	profile := strings.ToLower(strings.TrimSpace(opts.Profile))
	if profile == "" {
		profile = "standard"
	}
	report := ValidationReport{Profile: profile, Analysis: Analyze(d)}
	report.Warnings = append(report.Warnings, report.Analysis.Warnings...)
	switch profile {
	case "standard":
	case "conference":
		validateConference(&report)
	case "demo":
		validateDemo(&report)
	case "lecture":
		validateLecture(&report)
	default:
		report.Errors = append(report.Errors, "unknown validation profile "+profile)
	}
	report.Warnings = uniqueStrings(report.Warnings)
	report.Errors = uniqueStrings(report.Errors)
	return report
}

// RehearsalScript is the real-lane RehearsalScript: a speaker run sheet
// from an IslandDeck, with notes read from the real slide subtrees.
func RehearsalScript(d *IslandDeck) string {
	analysis := Analyze(d)
	return rehearsalScript(d.title(), analysis, func(i int) string {
		if i < 0 || i >= len(d.Slides) {
			return ""
		}
		return extractSlideNotes(d.Slides[i])
	})
}

// DeckComponentInfo describes one island a real-lane deck actually uses.
type DeckComponentInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Compiles bool   `json:"compiles"`
	Error    string `json:"error,omitempty"`
	Slides   []int  `json:"slides"`
}

// DeckComponents lists the deck's own .gsx islands, with their source path,
// first-seen slides, and compile status.
func DeckComponents(d *IslandDeck) []DeckComponentInfo {
	order := []string{}
	slidesByName := map[string][]int{}
	for _, slide := range d.Slides {
		for _, name := range slideComponentNames(slide) {
			if _, ok := slidesByName[name]; !ok {
				order = append(order, name)
			}
			slidesByName[name] = append(slidesByName[name], slide.Index)
		}
	}
	sort.Strings(order)
	_, failures := d.compileComponents()
	out := make([]DeckComponentInfo, 0, len(order))
	for _, name := range order {
		info := DeckComponentInfo{
			Name:   name,
			Path:   filepath.Join(d.Dir, name+".gsx"),
			Slides: slidesByName[name],
		}
		if err, failed := failures[name]; failed {
			info.Error = err.Error()
		} else {
			info.Compiles = true
		}
		out = append(out, info)
	}
	return out
}

// Doctor checks a real-lane deck's health and the prerequisites the real
// serve/build path actually needs (the Go toolchain for the GOOS=js wasm build,
// a gosx-requiring go.mod, and that every island compiles) — not the dropped
// Chrome/PDF prereq.
func Doctor(dir string) (DoctorReport, error) {
	var report DoctorReport
	d, err := LoadIslandDeck(dir)
	if err != nil {
		report.Items = append(report.Items, DoctorItem{Name: "deck", Status: "fail", Detail: err.Error()})
		return report, nil
	}
	report.Items = append(report.Items, DoctorItem{Name: "deck", Status: "ok", Detail: d.title() + " / " + itoa(len(d.Slides)) + " slides"})

	// Go toolchain — the real lane builds runtime.wasm with `go build GOOS=js`.
	if goBin, err := exec.LookPath("go"); err == nil {
		report.Items = append(report.Items, DoctorItem{Name: "go", Status: "ok", Detail: goBin})
	} else {
		report.Items = append(report.Items, DoctorItem{Name: "go", Status: "fail", Detail: "go toolchain not found (needed to build the wasm runtime)"})
	}

	// gosx module resolution — the deck must be (or live inside) a module that
	// requires m31labs.dev/gosx so the GOOS=js build resolves.
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		if strings.Contains(string(data), "m31labs.dev/gosx") {
			report.Items = append(report.Items, DoctorItem{Name: "gomod", Status: "ok", Detail: "go.mod requires m31labs.dev/gosx (portable deck)"})
		} else {
			report.Items = append(report.Items, DoctorItem{Name: "gomod", Status: "warn", Detail: "go.mod present but does not require m31labs.dev/gosx"})
		}
	} else {
		report.Items = append(report.Items, DoctorItem{Name: "gomod", Status: "warn", Detail: "no go.mod — deck serves only from inside a gosx module (run `slides init` for a portable deck)"})
	}

	// Island compile health — the highest-value check: a broken .gsx degrades to an
	// inert placeholder at serve time, so catch it here.
	_, failures := d.compileComponents()
	if len(failures) == 0 {
		report.Items = append(report.Items, DoctorItem{Name: "islands", Status: "ok", Detail: "all components compile"})
	} else {
		for name, e := range failures {
			report.Items = append(report.Items, DoctorItem{Name: "island:" + name, Status: "fail", Detail: e.Error()})
		}
	}

	// Notes coverage.
	notes := 0
	for _, slide := range d.Slides {
		if extractSlideNotes(slide) != "" {
			notes++
		}
	}
	notesStatus := "ok"
	if notes < len(d.Slides) {
		notesStatus = "warn"
	}
	report.Items = append(report.Items, DoctorItem{Name: "notes", Status: notesStatus, Detail: itoa(notes) + "/" + itoa(len(d.Slides)) + " slides have notes"})

	// Theme resolves to a real theme.
	theme := deckTheme(d)
	if norm := strings.TrimSpace(strings.ToLower(theme)); norm != "" && themeName(theme) != norm {
		report.Items = append(report.Items, DoctorItem{Name: "theme", Status: "warn", Detail: "unknown theme " + theme + " (using " + defaultTheme + ")"})
	} else {
		report.Items = append(report.Items, DoctorItem{Name: "theme", Status: "ok", Detail: themeName(theme)})
	}

	return report, nil
}

// rehearsalScript formats a speaker run sheet from a deck analysis. notesFor
// returns the raw note text for slide i (0-based). Shared by the real and
// fallback rehearse paths.
func rehearsalScript(title string, analysis DeckAnalysis, notesFor func(i int) string) string {
	var buf strings.Builder
	buf.WriteString(title)
	buf.WriteString("\nestimated: ")
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
		notes := strings.TrimSpace(notesFor(slide.Index))
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

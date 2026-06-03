package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDeckFrontmatterSlidesNotesAndClicks(t *testing.T) {
	src := `---
title: Demo Deck
theme: m31
---

# First

<Step n={2}>Later</Step>

<!-- first note -->

---
layout: two-cols
clicks: 3
---

## Second

` + "```go {1-2|4|all}\nfunc main() {}\n```\n" + `

::right::

Content
`
	deck, err := Parse(src, ParseOptions{SourcePath: "/tmp/deck.md"})
	if err != nil {
		t.Fatal(err)
	}
	if deck.Title != "Demo Deck" {
		t.Fatalf("title = %q", deck.Title)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("slide count = %d", len(deck.Slides))
	}
	if deck.Slides[0].Clicks != 2 {
		t.Fatalf("slide 1 clicks = %d", deck.Slides[0].Clicks)
	}
	if deck.Slides[0].Notes != "first note" {
		t.Fatalf("slide notes = %q", deck.Slides[0].Notes)
	}
	if deck.Slides[1].Layout != "two-cols" {
		t.Fatalf("layout = %q", deck.Slides[1].Layout)
	}
	if deck.Slides[1].Clicks != 3 {
		t.Fatalf("slide 2 clicks = %d", deck.Slides[1].Clicks)
	}
}

func TestSlideSplitIgnoresFenceSeparators(t *testing.T) {
	src := "# One\n\n```text\n---\n```\n\n---\n\n# Two\n"
	deck, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("slide count = %d", len(deck.Slides))
	}
}

func TestParseRangeSpec(t *testing.T) {
	steps := ParseRangeSpec("{1-3|5|all}")
	if len(steps) != 3 {
		t.Fatalf("step count = %d", len(steps))
	}
	if got := lineSteps(2, steps); got != "1,3" {
		t.Fatalf("line 2 steps = %q", got)
	}
	if got := lineSteps(5, steps); got != "2,3" {
		t.Fatalf("line 5 steps = %q", got)
	}
}

func TestRenderDeckHTMLIncludesRuntimeSurfaces(t *testing.T) {
	deck, err := Parse(sampleDeck("m31"), ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	html := RenderDeckHTML(deck, RenderOptions{Mode: "deck"})
	for _, want := range []string{"theme-m31", "class=\"slide", "class=\"agenda\"", "class=\"step\"", "class=\"bind-step\"", "class=\"metric-grid\"", "class=\"poll\"", "class=\"code-frame\"", "class=\"scene3d\"", "window.__SLIDES_DECK__"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered html missing %q", want)
		}
	}
}

func TestExportSPADeterministic(t *testing.T) {
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(deckPath, []byte(sampleDeck("m31")), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "dist")
	if err := ExportSPA(deckPath, out); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deck.json", "notes.html", "handout.html"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing export artifact %s: %v", name, err)
		}
	}
	if err := ExportSPA(deckPath, out); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("SPA export is not deterministic")
	}
}

func TestAnalyzeDeck(t *testing.T) {
	deck, err := Parse(sampleDeck("m31"), ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	analysis := Analyze(deck)
	if analysis.SlideCount != len(deck.Slides) {
		t.Fatalf("slide count = %d", analysis.SlideCount)
	}
	if analysis.Components["Scene3D"] == 0 {
		t.Fatal("expected Scene3D component count")
	}
	if analysis.Components["Poll"] == 0 {
		t.Fatal("expected Poll component count")
	}
	if analysis.Components["Agenda"] == 0 {
		t.Fatal("expected Agenda component count")
	}
	if analysis.EstimatedSeconds <= 0 {
		t.Fatalf("estimated seconds = %d", analysis.EstimatedSeconds)
	}
}

func TestGoTreeSitterTemplateComponents(t *testing.T) {
	deck, err := Parse(gotreesitterDeck("noir"), ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	html := RenderDeckHTML(deck, RenderOptions{Mode: "deck"})
	for _, want := range []string{"theme-noir", "class=\"pipeline\"", "class=\"parse-tree\"", "class=\"benchmark\"", "class=\"takeaway\""} {
		if !strings.Contains(html, want) {
			t.Fatalf("gotreesitter render missing %q", want)
		}
	}
	analysis := Analyze(deck)
	for _, component := range []string{"Pipeline", "ParseTree", "Benchmark", "Takeaway"} {
		if analysis.Components[component] == 0 {
			t.Fatalf("expected component %s", component)
		}
	}
	script := RehearsalScript(deck)
	if !strings.Contains(script, "GoTreeSitter Without CGo") || !strings.Contains(script, "components:") {
		t.Fatalf("unexpected rehearsal script: %q", script)
	}
}

func TestDirectoryDeckSplitAndMerge(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.md")
	if err := os.WriteFile(source, []byte(gotreesitterDeck("noir")), 0o644); err != nil {
		t.Fatal(err)
	}
	multi := filepath.Join(dir, "multi")
	if err := SplitDeck(source, multi); err != nil {
		t.Fatal(err)
	}
	deck, err := ParseFile(multi)
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.SourceFiles) < 2 {
		t.Fatalf("expected source files, got %v", deck.SourceFiles)
	}
	if len(deck.Slides) != 5 {
		t.Fatalf("slide count = %d", len(deck.Slides))
	}
	if deck.Slides[2].Title != "Parser Pipeline" {
		t.Fatalf("slide 3 title = %q", deck.Slides[2].Title)
	}
	merged := filepath.Join(dir, "merged.md")
	if err := MergeDeck(multi, merged); err != nil {
		t.Fatal(err)
	}
	mergedDeck, err := ParseFile(merged)
	if err != nil {
		t.Fatal(err)
	}
	if len(mergedDeck.Slides) != len(deck.Slides) {
		t.Fatalf("merged slide count = %d, want %d", len(mergedDeck.Slides), len(deck.Slides))
	}
	if analysis := Analyze(mergedDeck); analysis.Components["Pipeline"] == 0 {
		t.Fatal("merged deck lost Pipeline component")
	}
}

func TestIncludesExpandAndTrackSourceFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "partials"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partials", "intro.md"), []byte("Included body\n\n<Checkpoint id=\"intro\" label=\"Intro\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	deckPath := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(deckPath, []byte(`---
title: Include Demo
---

# Include Demo

{{include "partials/intro.md"}}

<Notes>
Notes.
</Notes>
`), 0o644); err != nil {
		t.Fatal(err)
	}
	deck, err := ParseFile(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deck.Slides[0].Body, "Included body") {
		t.Fatalf("include was not expanded: %q", deck.Slides[0].Body)
	}
	if len(deck.SourceFiles) != 2 {
		t.Fatalf("source files = %v", deck.SourceFiles)
	}
	if analysis := Analyze(deck); len(analysis.Checkpoints) != 1 {
		t.Fatalf("checkpoints = %+v", analysis.Checkpoints)
	}
}

func TestExtendedComponentsValidationAndSingleExport(t *testing.T) {
	src := `---
title: Generic Deep Deck
theme: noir
---

# Generic Deep Deck

<QueryDemo lang="go" captures="@fn main">
` + "```go\nfunc main() {}\n```\n```query\n(function_declaration name: (identifier) @fn)\n```\n" + `</QueryDemo>

<ProfileBuckets buckets="scan 20|reduce 42|materialize 18"/>

<ParityMatrix rows="Go pass|JavaScript watch"/>

<CorpusRun rows="Go stdlib 1.02x pass|JavaScript corpus 4.41x pass"/>

<GrammarBlob steps="parser.c|tables|blob|registry"/>

<Citation href="hypha://m31labs/gotreesitter/object/concept.glr-fork-reduction"/>

<Checkpoint id="demo" label="Demo jump"/>

<Notes>
Run the demo and call out the source citation.
</Notes>
`
	deck, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	html := RenderDeckHTML(deck, RenderOptions{Mode: "deck"})
	for _, want := range []string{"class=\"query-demo\"", "class=\"profile-buckets\"", "class=\"parity-matrix\"", "class=\"corpus-run\"", "class=\"grammar-blob\"", "class=\"checkpoint\""} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered html missing %q", want)
		}
	}
	analysis := Analyze(deck)
	for _, component := range []string{"QueryDemo", "ProfileBuckets", "ParityMatrix", "CorpusRun", "GrammarBlob", "Checkpoint", "Citation"} {
		if analysis.Components[component] == 0 {
			t.Fatalf("expected component %s", component)
		}
	}
	report := Validate(deck, ValidateOptions{Profile: "conference"})
	if !report.Passed(true) {
		t.Fatalf("conference validation failed: errors=%v warnings=%v", report.Errors, report.Warnings)
	}
	dir := t.TempDir()
	deckPath := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(deckPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "single")
	if err := Export(deckPath, ExportOptions{Format: "single", OutDir: out}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "deck.html")); err != nil {
		t.Fatalf("missing single export: %v", err)
	}
}

func TestComponentRegistryAndPresenterFeatures(t *testing.T) {
	components := BuiltInComponents()
	if len(components) == 0 {
		t.Fatal("empty component registry")
	}
	registry := componentRegistryByName()
	for _, name := range []string{"QueryDemo", "Checkpoint", "CorpusRun"} {
		if registry[name].Name == "" {
			t.Fatalf("missing registry component %s", name)
		}
	}
	deck, err := Parse(`# Presenter

<Checkpoint id="jump" label="Jump point"/>

<Notes>
Notes.
</Notes>
`, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	html := RenderPresenterHTML(deck, `{"slideIndex":0,"clickStep":0}`)
	for _, want := range []string{"data-record=\"toggle\"", "Download Rehearsal", "checkpoint-list", "Jump point"} {
		if !strings.Contains(html, want) {
			t.Fatalf("presenter html missing %q", want)
		}
	}
}

func TestPresenterCommands(t *testing.T) {
	deck, err := Parse(sampleDeck("m31"), ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	state := &presentationState{}
	applyCommand(deck, state, "next", 0)
	if state.SlideIndex != 0 || state.ClickStep != 1 {
		t.Fatalf("after next: slide=%d step=%d", state.SlideIndex, state.ClickStep)
	}
	applyCommand(deck, state, "goto", 2)
	if state.SlideIndex != 2 || state.ClickStep != 0 {
		t.Fatalf("after goto: slide=%d step=%d", state.SlideIndex, state.ClickStep)
	}
	applyCommand(deck, state, "prev", 0)
	if state.SlideIndex != 1 {
		t.Fatalf("after prev: slide=%d step=%d", state.SlideIndex, state.ClickStep)
	}
}

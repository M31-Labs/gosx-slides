package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	slides "m31labs.dev/gosx-slides"
)

func storyCommand(args []string) error {
	if len(args) == 0 || (args[0] != "inspect" && args[0] != "assert") {
		return fmt.Errorf("usage: slides story inspect|assert [deck-dir] [--audience name] [--json] [--browser]")
	}
	command := args[0]
	jsonOut, rest := takeBoolFlag(args[1:], "json")
	browser, rest := takeBoolFlag(rest, "browser")
	audience, rest, err := takeStringFlag(rest, "audience", "")
	if err != nil {
		return err
	}
	if len(rest) > 1 || (browser && command != "assert") {
		return fmt.Errorf("--browser requires story assert; specify at most one deck directory")
	}
	deck, err := slides.LoadIslandDeckAudience(deckDir(rest), audience)
	if err != nil {
		return err
	}
	if deck.Story == nil {
		return fmt.Errorf("deck has no story: manifest")
	}
	if command == "inspect" {
		if jsonOut {
			return printSemanticJSON(deck.Story)
		}
		fmt.Printf("%s: %d beats across %d graphs\n", deck.Story.File, len(deck.Story.Beats), len(deck.Story.Graphs))
		for _, beat := range deck.Story.Beats {
			fmt.Printf("%s/%s · slide %d step %d · %d ms · %s\n", beat.Slide, beat.Cue, beat.SlideIndex+1, beat.Step, beat.DurationMS, beat.Caption)
		}
		return nil
	}
	var report slides.StoryAssertionReport
	if browser {
		report, err = slides.AssertStoryBrowser(deck)
	} else {
		report, err = slides.AssertStory(deck)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		if err := printSemanticJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("%d beats · %d checks · %d rendered states\n", report.Beats, report.Checks, report.RenderedStates)
		for _, failure := range report.Errors {
			fmt.Printf("error: %s\n", failure)
		}
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("story assertions failed")
	}
	return nil
}

func audiencesCommand(args []string) error {
	jsonOut, rest := takeBoolFlag(args, "json")
	if len(rest) > 1 {
		return fmt.Errorf("usage: slides audiences [deck-dir] [--json]")
	}
	deck, err := slides.LoadIslandDeck(deckDir(rest))
	if err != nil {
		return err
	}
	names, err := slides.DeckAudiences(deck)
	if err != nil {
		return err
	}
	if jsonOut {
		return printSemanticJSON(names)
	}
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

func tourCommand(args []string) error {
	jsonOut, rest := takeBoolFlag(args, "json")
	out, rest, err := takeStringFlag(rest, "out", "architecture-tour")
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: slides tour <before.sir> <after.sir> [--out new-deck-dir] [--json]")
	}
	read := func(path string) ([]byte, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, fmt.Errorf("architecture snapshot must be a regular file below 1 MiB")
		}
		data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if len(data) > 1<<20 {
			return nil, fmt.Errorf("architecture snapshot exceeds 1 MiB")
		}
		return data, err
	}
	before, err := read(rest[0])
	if err != nil {
		return err
	}
	after, err := read(rest[1])
	if err != nil {
		return err
	}
	tour, err := slides.ArchitectureTour(before, after)
	if err != nil {
		return err
	}
	if err := writeArchitectureTour(out, tour); err != nil {
		return err
	}
	if jsonOut {
		return printSemanticJSON(tour)
	}
	fmt.Printf("created %s: %d added, %d removed, %d changed actors\n", out, len(tour.Added), len(tour.Removed), len(tour.Changed))
	return nil
}

func writeArchitectureTour(out string, tour slides.ArchitectureChangeTour) error {
	dest, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	// Reserve a fresh destination before authoring; never replace user files.
	if err := os.Mkdir(dest, 0755); err != nil {
		return fmt.Errorf("tour needs an absent destination with an existing parent: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			for _, name := range []string{"deck.md", "story.yaml", "Counter.gsx", "go.mod", ".gitignore", "README"} {
				_ = os.Remove(filepath.Join(dest, name))
			}
			_ = os.Remove(dest)
		}
	}()
	if err := slides.ScaffoldRealLane(dest, slides.ScaffoldRealOptions{Theme: "aurora"}); err != nil {
		return err
	}
	for name, source := range map[string]string{"deck.md": tour.Markdown, "story.yaml": tour.StoryYAML, "README": "Generated architecture tour. Run slides story assert . and slides serve .\n"} {
		if err := os.WriteFile(filepath.Join(dest, name), []byte(source), 0644); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(dest, "Counter.gsx")); err != nil {
		return err
	}
	complete = true
	return nil
}

func printSemanticJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	if len(args) > 0 && args[0] == "history" {
		return historyTourCommand(args[1:])
	}
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
	return writeArchitectureTourFiles(out, tour.Markdown, tour.StoryYAML, nil)
}

func writeArchitectureTourFiles(out, markdown, story string, extra map[string][]byte) error {
	dest, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	// Reserve a fresh destination before authoring; never replace user files.
	if err := os.Mkdir(dest, 0755); err != nil {
		return fmt.Errorf("tour needs an absent destination with an existing parent: %w", err)
	}
	complete := false
	names := []string{"deck.md", "story.yaml", "Counter.gsx", "go.mod", ".gitignore", "README"}
	for name := range extra {
		if filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
			_ = os.Remove(dest)
			return fmt.Errorf("invalid tour output name")
		}
		names = append(names, name)
	}
	defer func() {
		if !complete {
			for _, name := range names {
				_ = os.Remove(filepath.Join(dest, name))
			}
			_ = os.Remove(dest)
		}
	}()
	if err := slides.ScaffoldRealLane(dest, slides.ScaffoldRealOptions{Theme: "aurora"}); err != nil {
		return err
	}
	for name, source := range map[string]string{"deck.md": markdown, "story.yaml": story, "README": "Generated architecture tour. Run slides story assert . and slides serve .\n"} {
		if err := os.WriteFile(filepath.Join(dest, name), []byte(source), 0644); err != nil {
			return err
		}
	}
	for name, data := range extra {
		if err := os.WriteFile(filepath.Join(dest, name), data, 0600); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(dest, "Counter.gsx")); err != nil {
		return err
	}
	complete = true
	return nil
}

type tourHistoryManifest struct {
	Version   int    `yaml:"version"`
	Title     string `yaml:"title"`
	Curation  string `yaml:"curation"`
	Snapshots []struct {
		ID       string `yaml:"id"`
		Label    string `yaml:"label"`
		Path     string `yaml:"path"`
		Source   string `yaml:"source"`
		Revision string `yaml:"revision"`
	} `yaml:"snapshots"`
}

func readTourFile(file *os.File, limit int64) ([]byte, error) {
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("tour source must be a regular file below %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("tour source exceeds size limit")
	}
	return data, err
}

func decodeTourDocument(data []byte, value any) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one versioned tour document")
	}
	return nil
}

func historyTourCommand(args []string) error {
	jsonOut, rest := takeBoolFlag(args, "json")
	out, rest, err := takeStringFlag(rest, "out", "architecture-history")
	if err != nil {
		return err
	}
	curationPath, rest, err := takeStringFlag(rest, "curation", "")
	if err != nil {
		return err
	}
	repository, rest, err := takeStringFlag(rest, "repo", "")
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: slides tour history <history.yaml> [--repo local-repo] [--curation previous/curation.json] [--out fresh-dir] [--json]")
	}
	path, err := filepath.Abs(rest[0])
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	data, err := readTourFile(file, 128<<10)
	if err != nil {
		return err
	}
	var manifest tourHistoryManifest
	if err := decodeTourDocument(data, &manifest); err != nil {
		return fmt.Errorf("history manifest: %w", err)
	}
	if manifest.Version != 1 || len(manifest.Snapshots) < 2 || len(manifest.Snapshots) > 32 {
		return fmt.Errorf("history manifest needs version 1 and 2–32 snapshots")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	readLocal := func(name string, limit int64) ([]byte, error) {
		if name == "" || !filepath.IsLocal(filepath.FromSlash(name)) || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("history references must be local relative paths")
		}
		info, err := root.Lstat(filepath.FromSlash(name))
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("history reference %q must be a regular local file", name)
		}
		file, err := root.Open(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		return readTourFile(file, limit)
	}
	snapshots := make([]slides.ArchitectureSnapshot, 0, len(manifest.Snapshots))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var total int
	for _, snapshot := range manifest.Snapshots {
		if filepath.Ext(snapshot.Path) != ".sir" {
			return fmt.Errorf("history snapshot %s must name a .sir file", snapshot.ID)
		}
		var data []byte
		var err error
		source := snapshot.Source
		if snapshot.Revision != "" {
			if repository == "" {
				return fmt.Errorf("revision snapshots require --repo local-repo")
			}
			var commit string
			data, commit, err = readGitTourSnapshot(ctx, repository, snapshot.Revision, snapshot.Path)
			canonical := commit + ":" + filepath.ToSlash(filepath.Clean(snapshot.Path))
			if source != "" {
				canonical += " (" + source + ")"
			}
			source = canonical
		} else {
			data, err = readLocal(snapshot.Path, 1<<20)
		}
		if err != nil {
			return fmt.Errorf("snapshot %s: %w", snapshot.ID, err)
		}
		total += len(data)
		if total > 4<<20 {
			return fmt.Errorf("history source exceeds 4 MiB")
		}
		if source == "" {
			source = snapshot.Path
		}
		snapshots = append(snapshots, slides.ArchitectureSnapshot{ID: snapshot.ID, Label: snapshot.Label, Source: source, Sirena: data})
	}
	var curation *slides.ArchitectureTourCuration
	if curationPath != "" || manifest.Curation != "" {
		var data []byte
		if curationPath != "" {
			file, err := os.Open(curationPath)
			if err != nil {
				return err
			}
			data, err = readTourFile(file, 512<<10)
			if err != nil {
				return err
			}
		} else {
			data, err = readLocal(manifest.Curation, 512<<10)
			if err != nil {
				return err
			}
		}
		curation = &slides.ArchitectureTourCuration{}
		if err := decodeTourDocument(data, curation); err != nil {
			return fmt.Errorf("tour curation: %w", err)
		}
	}
	tour, err := slides.ArchitectureTourHistory(snapshots, slides.ArchitectureTourOptions{Title: manifest.Title, Curation: curation})
	if err != nil {
		return err
	}
	report, err := json.MarshalIndent(tour, "", "  ")
	if err != nil {
		return err
	}
	curated, err := json.MarshalIndent(tour.Curation, "", "  ")
	if err != nil {
		return err
	}
	if err := writeArchitectureTourFiles(out, tour.Markdown, tour.StoryYAML, map[string][]byte{"tour.json": report, "curation.json": curated}); err != nil {
		return err
	}
	if jsonOut {
		return printSemanticJSON(tour)
	}
	fmt.Printf("created %s: %d revisions, %d transitions, %d preserved explanations\n", out, len(tour.Snapshots), len(tour.Transitions), len(tour.Curation.Explanations))
	return nil
}

func printSemanticJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

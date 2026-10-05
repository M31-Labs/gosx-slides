package slides

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Embed authored files explicitly: serving an example may create ignored
// build/ or dist/ trees, which must never become part of a shipped starter.
//
//go:embed examples/starters/*/deck.md examples/starters/*/README.md examples/starters/*/packs/studio/*.css examples/starters/*/packs/studio/pack.json examples/starters/*/*.yaml
var starterFiles embed.FS

// DeckTemplate describes a versioned, curated starter shipped with the binary.
// Starters contain local authoring files; discovery never requires a network.
type DeckTemplate struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Theme       string   `json:"theme"`
	Slides      int      `json:"slides"`
	Features    []string `json:"features"`
}

var starterCatalog = []DeckTemplate{
	{"architecture-review", "Architecture review", "An ADR-shaped review: system boundaries, tradeoffs, evidence and a decision.", "1.0.0", "aurora", 5, []string{"sirena", "semantic-story", "cues", "brand-pack", "speaker-notes"}},
	{"technical-talk", "Technical talk", "A focused technical narrative with stepped code, diagrams and an evidence slide.", "1.0.0", "paper", 6, []string{"sirena", "code-highlights", "cues", "brand-pack", "speaker-notes"}},
	{"teaching", "Teaching and exploration", "Predict, observe and explain an offline, seeded simulation with a practice prompt.", "1.0.0", "swiss", 5, []string{"simulation", "fragments", "cues", "brand-pack", "speaker-notes"}},
}

func DeckTemplates() []DeckTemplate {
	out := append([]DeckTemplate(nil), starterCatalog...)
	for i := range out {
		out[i].Features = append([]string(nil), out[i].Features...)
	}
	return out
}

type TemplateOptions struct {
	Template string // required catalog name
	Theme    string // optional built-in theme override; empty uses the starter theme
}

// ScaffoldTemplate atomically publishes a fresh, portable starter directory.
// Any existing destination is rejected; no install hooks or downloads run.
func ScaffoldTemplate(destination string, opts TemplateOptions) error {
	var selected *DeckTemplate
	for i := range starterCatalog {
		if starterCatalog[i].Name == opts.Template {
			selected = &starterCatalog[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("unknown template %q; use slides templates", opts.Template)
	}
	if opts.Theme != "" && !isRealLaneTheme(opts.Theme) {
		return fmt.Errorf("unknown theme %q (choose %s)", opts.Theme, themesList())
	}
	stage, dest, err := adoptionStage(destination)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	root := "examples/starters/" + selected.Name
	err = fs.WalkDir(starterFiles, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(path, root+"/")
		content, err := starterFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if relative == DeckFileName && opts.Theme != "" {
			content = []byte(strings.Replace(string(content), "theme: "+selected.Theme, "theme: "+opts.Theme, 1))
		}
		return adoptionWrite(stage, relative, content, 0644)
	})
	if err != nil {
		return err
	}
	for name, content := range map[string]string{"go.mod": realLaneGoMod(dest), ".gitignore": realLaneGitignore} {
		if err := adoptionWrite(stage, name, []byte(content), 0644); err != nil {
			return err
		}
	}
	if _, err := LoadIslandDeck(stage); err != nil {
		return fmt.Errorf("validate starter: %w", err)
	}
	return adoptionPublish(stage, dest)
}

func adoptionStage(destination string) (stage, dest string, err error) {
	if strings.TrimSpace(destination) == "" {
		return "", "", fmt.Errorf("fresh destination is required")
	}
	dest, err = filepath.Abs(destination)
	if err != nil {
		return "", "", err
	}
	if _, err = os.Lstat(dest); err == nil {
		return "", "", fmt.Errorf("destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dest))
	if err != nil {
		return "", "", fmt.Errorf("destination parent must exist: %w", err)
	}
	dest = filepath.Join(parent, filepath.Base(dest))
	stage, err = os.MkdirTemp(parent, ".slides-adoption-*")
	return stage, dest, err
}

func adoptionWrite(stage, relative string, content []byte, mode os.FileMode) error {
	if !safeDeckRelPath(relative) {
		return fmt.Errorf("invalid generated path %q", relative)
	}
	path := filepath.Join(stage, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, mode)
}

func adoptionPublish(stage, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	return publishAdoptionDirectory(stage, destination)
}

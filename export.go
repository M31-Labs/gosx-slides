package slides

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExportOptions configures deck export.
type ExportOptions struct {
	Format string
	OutDir string
}

// Export writes a deck in one of the supported formats.
func Export(deckPath string, opts ExportOptions) error {
	if opts.Format == "" {
		opts.Format = "spa"
	}
	if opts.OutDir == "" {
		opts.OutDir = "dist"
	}
	switch opts.Format {
	case "spa":
		return ExportSPA(deckPath, opts.OutDir)
	case "single":
		return ExportSingle(deckPath, opts.OutDir)
	case "pdf":
		return exportPDF(deckPath, opts.OutDir)
	case "png":
		return exportPNG(deckPath, opts.OutDir)
	default:
		return fmt.Errorf("unsupported export format %q", opts.Format)
	}
}

// ExportSingle writes one portable HTML file. Referenced local images remain external.
func ExportSingle(deckPath, outDir string) error {
	deck, err := ParseFile(deckPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "deck.html"), []byte(RenderDeckHTML(deck, RenderOptions{Mode: "deck"})), 0o644)
}

// ExportSPA writes a self-contained static SPA plus public assets.
func ExportSPA(deckPath, outDir string) error {
	deck, err := ParseFile(deckPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	html := RenderDeckHTML(deck, RenderOptions{Mode: "deck"})
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0o644); err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(exportManifest(deck), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "deck.json"), append(manifest, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "notes.html"), []byte(renderNotesHTML(deck)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "handout.html"), []byte(renderHandoutHTML(deck)), 0o644); err != nil {
		return err
	}
	public := filepath.Join(deckBaseDir(deckPath), "public")
	if info, err := os.Stat(public); err == nil && info.IsDir() {
		return copyDir(public, filepath.Join(outDir, "public"))
	}
	return nil
}

func exportManifest(deck *Deck) map[string]any {
	slides := make([]map[string]any, 0, len(deck.Slides))
	for _, slide := range deck.Slides {
		slides = append(slides, map[string]any{
			"index":      slide.Index,
			"title":      slide.Title,
			"layout":     slide.Layout,
			"clicks":     slide.Clicks,
			"transition": slide.Transition,
			"hasNotes":   slide.Notes != "",
			"sourcePath": slide.SourcePath,
		})
	}
	return map[string]any{
		"title":       deck.Title,
		"theme":       deck.Theme,
		"transition":  deck.Transition,
		"sourcePath":  deck.SourcePath,
		"sourceFiles": deck.SourceFiles,
		"components":  BuiltInComponents(),
		"slides":      slides,
		"analysis":    Analyze(deck),
	}
}

func renderNotesHTML(deck *Deck) string {
	var buf strings.Builder
	buf.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>")
	buf.WriteString(html.EscapeString(deck.Title))
	buf.WriteString(" notes</title><style>")
	buf.WriteString(baseCSS())
	buf.WriteString("body{overflow:auto}.notes-export{max-width:72rem;margin:0 auto;padding:var(--space-2xl)}.notes-export article{border-bottom:1px solid var(--color-line);padding:var(--space-lg) 0}</style></head><body><main class=\"notes-export\"><h1>")
	buf.WriteString(html.EscapeString(deck.Title))
	buf.WriteString(" speaker notes</h1>")
	for _, slide := range deck.Slides {
		buf.WriteString("<article><h2>")
		buf.WriteString(fmt.Sprintf("%02d. ", slide.Index+1))
		buf.WriteString(html.EscapeString(slide.Title))
		buf.WriteString("</h2><p>")
		if slide.Notes == "" {
			buf.WriteString("<em>No notes.</em>")
		} else {
			buf.WriteString(strings.ReplaceAll(html.EscapeString(slide.Notes), "\n", "<br>"))
		}
		buf.WriteString("</p></article>")
	}
	buf.WriteString("</main></body></html>\n")
	return buf.String()
}

func renderHandoutHTML(deck *Deck) string {
	var buf strings.Builder
	buf.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>")
	buf.WriteString(html.EscapeString(deck.Title))
	buf.WriteString(" handout</title><style>")
	buf.WriteString(baseCSS())
	buf.WriteString("body{overflow:auto}.handout{max-width:80rem;margin:0 auto;padding:var(--space-2xl)}.handout .slide{position:relative;display:grid;min-height:45rem;margin-bottom:var(--space-xl);border:1px solid var(--color-line);border-radius:var(--space-sm);overflow:hidden;page-break-inside:avoid}.handout .notes{display:block;margin:calc(-1 * var(--space-lg)) 0 var(--space-xl);padding:var(--space-lg);border:1px solid var(--color-line);border-radius:var(--space-sm);background:var(--color-surface)}.handout-title{margin-bottom:var(--space-xl)}</style></head><body class=\"theme-")
	buf.WriteString(html.EscapeString(themeClass(deck.Theme)))
	buf.WriteString("\"><main class=\"handout\"><h1 class=\"handout-title\">")
	buf.WriteString(html.EscapeString(deck.Title))
	buf.WriteString("</h1>")
	for _, slide := range deck.Slides {
		buf.WriteString(renderSlide(deck, slide))
		buf.WriteString("<aside class=\"notes\"><strong>Notes</strong><p>")
		if slide.Notes == "" {
			buf.WriteString("<em>No notes.</em>")
		} else {
			buf.WriteString(strings.ReplaceAll(html.EscapeString(slide.Notes), "\n", "<br>"))
		}
		buf.WriteString("</p></aside>")
	}
	buf.WriteString("</main></body></html>\n")
	return buf.String()
}

func exportPDF(deckPath, outDir string) error {
	if err := ExportSPA(deckPath, outDir); err != nil {
		return err
	}
	chrome, err := findChrome()
	if err != nil {
		return err
	}
	index := fileURL(filepath.Join(outDir, "index.html"))
	out := filepath.Join(outDir, "deck.pdf")
	cmd := exec.Command(chrome, "--headless=new", "--disable-gpu", "--no-sandbox", "--print-to-pdf="+out, index)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chrome pdf export failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func exportPNG(deckPath, outDir string) error {
	deck, err := ParseFile(deckPath)
	if err != nil {
		return err
	}
	if err := ExportSPA(deckPath, outDir); err != nil {
		return err
	}
	chrome, err := findChrome()
	if err != nil {
		return err
	}
	index := fileURL(filepath.Join(outDir, "index.html"))
	for _, slide := range deck.Slides {
		out := filepath.Join(outDir, fmt.Sprintf("slide-%02d.png", slide.Index+1))
		target := index + "?slide=" + fmt.Sprint(slide.Index+1)
		cmd := exec.Command(chrome, "--headless=new", "--disable-gpu", "--no-sandbox", "--window-size=1600,900", "--screenshot="+out, target)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("chrome png export failed for slide %d: %w: %s", slide.Index+1, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func findChrome() (string, error) {
	if custom := os.Getenv("SLIDES_CHROME"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom, nil
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("pdf/png export requires Chrome or Chromium; set SLIDES_CHROME to the executable path")
}

func fileURL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return (&url.URL{Scheme: "file", Path: abs}).String()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

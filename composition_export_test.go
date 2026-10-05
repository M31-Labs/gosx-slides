package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposedAssetsSurviveSnapshotHandoutAndSPABundles(t *testing.T) {
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":                 "# Shared\n\n<!-- slides:include sections/intro.md -->\n",
		"sections/intro.md":       "![Shared diagram](chart.svg)\n",
		"sections/chart.svg":      `<svg xmlns="http://www.w3.org/2000/svg"><text>Shared asset</text></svg>`,
		"build/gosx-runtime.wasm": "WASM",
	})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"single", "handout"} {
		out := t.TempDir()
		if err := ExportStatic(dir, ExportOptions{Format: format, OutDir: out}); err != nil {
			t.Fatal(format, err)
		}
		name := "deck.html"
		if format == "handout" {
			name = "handout.html"
		}
		content, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "data:image/svg+xml;base64,") {
			t.Fatalf("%s left included image external", format)
		}
	}
	out := t.TempDir()
	if err := exportSPA(dir, deck, `<img src="/public/_slides/sections/chart.svg">`, out); err != nil {
		t.Fatal(err)
	}
	asset, err := os.ReadFile(filepath.Join(out, "public", "_slides", "sections", "chart.svg"))
	if err != nil || !strings.Contains(string(asset), "Shared asset") {
		t.Fatalf("SPA composition asset: %s, %v", asset, err)
	}
	if _, err := inlineSnapshotAssets(deck, `<img src="/public/_slides/sections/private.svg">`); err == nil {
		t.Fatal("unregistered composition asset accepted")
	}
}

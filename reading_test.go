package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleSnapshotEmbedsLocalAssetsWithoutChangingCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "public"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "public", "picture.png"), []byte("picture"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "public", "font.woff2"), []byte("font"), 0644); err != nil {
		t.Fatal(err)
	}
	deck := &IslandDeck{Dir: dir}
	source := `<style>@font-face{src:url('/public/font.woff2')}</style><img src="/public/picture.png?v=2" srcset="public/picture.png 1x, /public/picture.png 2x"><pre><code>src="/public/picture.png"</code></pre><script>let path="/public/picture.png";</script><img src="https://example.test/picture.png">`
	result, err := inlineSnapshotAssets(deck, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"data:image/png;base64,cGljdHVyZQ==", "data:font/woff2;base64,Zm9udA==", `<code>src="/public/picture.png"</code>`, `let path="/public/picture.png"`, `src="https://example.test/picture.png"`} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing %q in %s", expected, result)
		}
	}
}

func TestSnapshotRejectsAssetEscapesAndOversize(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "public"), 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "public", "escape.png")); err == nil {
		if _, err := inlineSnapshotAssets(&IslandDeck{Dir: dir}, `<img src="/public/escape.png">`); err == nil {
			t.Fatal("outside symlink was read")
		}
	}
	file, err := os.Create(filepath.Join(dir, "public", "large.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(snapshotAssetLimit + 1); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := inlineSnapshotAssets(&IslandDeck{Dir: dir}, `<img src="/public/large.png">`); err == nil {
		t.Fatal("oversize image accepted")
	}
	if _, err := inlineSnapshotAssets(&IslandDeck{Dir: dir}, `<img src="/public/../../secret.png">`); err == nil {
		t.Fatal("path escape accepted")
	}
}

func TestHandoutIncludesNotesOnlyWhenRequested(t *testing.T) {
	dir := t.TempDir()
	source := "# First\n\nReadable content.\n\n<!-- Private speaker note -->\n\n---\n\n# Second\n\nMore content.\n"
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	for _, notes := range []bool{false, true} {
		out := t.TempDir()
		if err := ExportStatic(dir, ExportOptions{Format: "handout", OutDir: out, Notes: notes}); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(out, "handout.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), `data-reading="1"`) {
			t.Fatal("handout does not start in reading view")
		}
		if strings.Contains(string(content), "Private speaker note") != notes {
			t.Fatal("speaker notes inclusion does not match request")
		}
		if strings.Contains(string(content), `id="gosx-document"`) {
			t.Fatal("handout contains island runtime")
		}
	}
}

func TestDiagramReadingTranscriptContainsRenderedLabelsOnly(t *testing.T) {
	got := diagramVisibleText(`<svg><metadata>private source</metadata><title>Request flow</title><text>Public <tspan>API</tspan></text><!-- private comment --></svg>`)
	if got != "Request flow · Public API" {
		t.Fatalf("transcript = %q", got)
	}
}

func TestHandoutRejectsCaptureOptions(t *testing.T) {
	deck := loadDeckFromSource(t, "# Reading\n", nil)
	for _, opts := range []ExportOptions{{Format: "handout", Capture: true}, {Format: "handout", Steps: true}} {
		if err := ExportStatic(deck.Dir, opts); err == nil || !strings.Contains(err.Error(), "handout is a reading document") {
			t.Fatalf("invalid capture options: %v", err)
		}
	}
}

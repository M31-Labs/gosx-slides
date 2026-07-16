package slides

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// export_pdf_test.go proves `export --format pdf` prints a real PDF through a
// system Chrome. Chrome is an optional dependency, so the test skips (never
// fails) on machines without one — the CI gate for this path is any box with
// a browser.
func TestExportPDF(t *testing.T) {
	chromeAvailable := os.Getenv("SLIDES_CHROME") != ""
	if !chromeAvailable {
		for _, c := range pdfChromeCandidates {
			if _, err := exec.LookPath(c); err == nil {
				chromeAvailable = true
				break
			}
		}
	}
	if !chromeAvailable {
		t.Skip("no chrome/chromium on PATH; pdf export untestable here")
	}

	dir := newDeckDirUnderModule(t, "---\ntitle: T\ntheme: aurora\n---\n\n# One\n\nfirst\n\n---\n\n# Two\n\nsecond\n", nil)
	out := filepath.Join(t.TempDir(), "handout.pdf")
	if err := ExportStatic(dir, ExportOptions{Format: "pdf", OutDir: out}); err != nil {
		t.Fatalf("ExportStatic pdf: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		t.Fatalf("output is not a PDF (got %d bytes, prefix %q)", len(data), data[:min(4, len(data))])
	}
	// One slide per page: the page-tree /Count must equal the slide count.
	// (Chrome writes the Pages object uncompressed; the largest /Count is the
	// root page tree. This is what caught the print-cascade bug where
	// layout-default slides were display:none in print and the PDF folded.)
	counts := regexp.MustCompile(`/Count (\d+)`).FindAllSubmatch(data, -1)
	pages := 0
	for _, m := range counts {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > pages {
			pages = n
		}
	}
	if pages != 2 {
		t.Fatalf("expected 2 PDF pages for a 2-slide deck, got %d", pages)
	}
}

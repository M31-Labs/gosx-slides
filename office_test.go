package slides

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// This integration check is opt-in because the normal Go suite does not require
// a browser installation. It checks final package geometry and actual pixels.
func TestOfficeCapturedDimensions(t *testing.T) {
	if os.Getenv("SLIDES_CHROME") == "" {
		t.Skip("set SLIDES_CHROME for capture integration")
	}
	out := filepath.Join(t.TempDir(), "captured.pptx")
	if err := ExportStatic("examples/office-interop", ExportOptions{Format: "pptx", Editable: true, Width: 900, Height: 600, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	p, err := openOfficePackage(out)
	if err != nil {
		t.Fatal(err)
	}
	defer p.archive.Close()
	_, presentation, _ := p.presentation()
	size := presentation.child("sldSz")
	if size.attr("cx") != fmt.Sprint(900*9525) || size.attr("cy") != fmt.Sprint(600*9525) {
		t.Fatal("capture and PPTX dimensions differ")
	}
	for i := 1; i <= 3; i++ {
		data, err := p.read(fmt.Sprintf("ppt/media/slide%d.png", i), 16<<20)
		if err != nil {
			t.Fatal(err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width != 900 || config.Height != 600 {
			t.Fatalf("image size: %+v %v", config, err)
		}
	}
	first, _ := p.node("ppt/slides/slide1.xml")
	if len(first.descendants("tbl")) != 1 {
		t.Fatal("captured PPTX lost native table")
	}
	for i := 2; i <= 3; i++ {
		s, _ := p.node(fmt.Sprintf("ppt/slides/slide%d.xml", i))
		if len(s.descendants("chart")) != 1 {
			t.Fatalf("slide %d missing chart", i)
		}
	}
	pdf := filepath.Join(t.TempDir(), "custom.pdf")
	if err := ExportStatic("examples/office-interop", ExportOptions{Format: "pdf", Capture: true, Width: 900, Height: 600, OutDir: pdf}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatal(err)
	}
	box := regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)\s*\]`).FindSubmatch(data)
	if len(box) != 3 {
		t.Fatal("PDF missing page dimensions")
	}
	w, _ := strconv.ParseFloat(string(box[1]), 64)
	h, _ := strconv.ParseFloat(string(box[2]), 64)
	if math.Abs(w-675) > .5 || math.Abs(h-450) > .5 {
		t.Fatalf("captured PDF dimensions: %s", box[0])
	}
	if count := len(regexp.MustCompile(`/Type /Page\b`).FindAll(data, -1)); count != 3 {
		t.Fatalf("captured PDF has %d pages", count)
	}
}

func TestOfficeExportSize(t *testing.T) {
	deck := loadDeckFromSource(t, "---\naspect-ratio: 4:3\n---\n\n# Size\n", nil)
	for _, test := range []struct {
		opts ExportOptions
		w, h int
		bad  bool
	}{
		{ExportOptions{}, 960, 720, false}, {ExportOptions{Aspect: "16/9"}, 1280, 720, false},
		{ExportOptions{Aspect: "1:1"}, 720, 720, false}, {ExportOptions{Width: 1400, Height: 1000}, 1400, 1000, false},
		{ExportOptions{Width: 1000}, 0, 0, true}, {ExportOptions{Width: 4096, Height: 4096}, 0, 0, true},
		{ExportOptions{Aspect: "NaN:1"}, 0, 0, true}, {ExportOptions{Aspect: "Inf:1"}, 0, 0, true},
		{ExportOptions{Aspect: "0:1"}, 0, 0, true}, {ExportOptions{Aspect: "4:1"}, 0, 0, true},
	} {
		w, h, e := exportSize(deck, test.opts)
		if (e != nil) != test.bad || !test.bad && (w != test.w || h != test.h) {
			t.Errorf("%+v =%dx%d,%v", test.opts, w, h, e)
		}
	}
}

func TestPPTXSemanticOfficeObjectsAndTheme(t *testing.T) {
	out := filepath.Join(t.TempDir(), "native.pptx")
	w, err := newPPTXConfigured(out, 960, 720, "testdata/office-common.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer w.abort()
	var pixels bytes.Buffer
	png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 960, 720)))
	objects := []pptxObject{
		{Kind: "table", X: 100, Y: 120, Width: 300, Height: 100, Table: &pptxTable{ColumnWidths: []float64{150, 150}, RowHeights: []float64{50, 50}, Rows: [][]pptxCell{{{Text: "Name <&>", FontSize: 24, Bold: true}, {Text: "Value", FontSize: 24}}, {{Text: "Alpha", FontSize: 20}, {Text: "7", FontSize: 20}}}}},
		{Kind: "chart", X: 450, Y: 120, Width: 400, Height: 300, Chart: &pptxChart{Type: "bar", Categories: []string{"Alpha <&>", "Beta"}, Values: []float64{7, 11}, Colors: []string{"123456", "ABCDEF"}}},
		{Kind: "text", X: math.NaN(), Text: "Must be skipped"},
	}
	if err = w.addEditable(pixels.Bytes(), "Native", "Notes", objects); err != nil {
		t.Fatal(err)
	}
	if err = w.finish("Office"); err != nil {
		t.Fatal(err)
	}
	p, err := openOfficePackage(out)
	if err != nil {
		t.Fatal(err)
	}
	defer p.archive.Close()
	for name := range p.files {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			if _, err = p.node(name); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	_, presentation, err := p.presentation()
	if err != nil {
		t.Fatal(err)
	}
	size := presentation.child("sldSz")
	if size.attr("cx") != "9144000" || size.attr("cy") != "6858000" || size.attr("type") != "screen4x3" {
		t.Fatalf("size: %+v", size.Attrs)
	}
	slide, _ := p.node("ppt/slides/slide1.xml")
	if len(slide.descendants("tbl")) != 1 || len(slide.descendants("chart")) != 1 {
		t.Fatal("missing native table/chart")
	}
	data, _ := p.read("ppt/slides/slide1.xml", officeMaxXML)
	if bytes.Contains(data, []byte("Must be skipped")) {
		t.Fatal("invalid object accepted")
	}
	theme, _ := p.read("ppt/theme/theme1.xml", officeMaxXML)
	if !bytes.Contains(theme, []byte("Office")) {
		t.Fatal("template theme not reused")
	}
	xlsx, _ := p.read("ppt/embeddings/chart1.xlsx", 16<<20)
	sheet, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range sheet.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			r, _ := f.Open()
			content, _ := io.ReadAll(r)
			r.Close()
			found = bytes.Contains(content, []byte("Alpha &lt;&amp;&gt;")) && bytes.Contains(content, []byte("<v>11</v>"))
		}
	}
	if !found {
		t.Fatal("missing editable chart workbook data")
	}
	if target := os.Getenv("SLIDES_OFFICE_TEST_OUTPUT"); target != "" {
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(target, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportPPTXCommonContentLiteralSafety(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "imported")
	report, err := ImportPPTX("testdata/office-common.pptx", dest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Slides != 2 || report.Images != 1 {
		t.Fatalf("report %+v", report)
	}
	source, err := os.ReadFile(filepath.Join(dest, "deck.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"First &lt;&amp;&gt;", "Second: chart", "aspect-ratio: \"4:3\"", "Ordinary body", "<table>", "<td>Native</td>", "Alpha", "<td>11</td>", "Remember the evidence.", "/public/imported-image-1.png"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("missing %q", want)
		}
	}
	deck, err := LoadIslandDeck(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("slides %d", len(deck.Slides))
	}
	if len(deck.Includes) != 0 || len(deck.Slides[0].Components) != 0 {
		t.Fatal("imported text became active authoring")
	}
	body := graphicsBody(t, deck)
	if !strings.Contains(body, "{strings.Repeat") || !strings.Contains(body, "&lt;Counter/&gt;") {
		t.Fatal("imported literal text was evaluated or interpreted")
	}
	if !strings.Contains(body, "/public/imported-image-1.png") {
		t.Fatal("image reference missing")
	}
	for _, code := range []string{"layout-content-only", "chart-data-only", "notes-delimiter-escaped", "shape-omitted"} {
		found := false
		for _, warning := range report.Warnings {
			found = found || warning.Code == code
		}
		if !found {
			t.Errorf("missing warning %s", code)
		}
	}
	if notes, err := os.ReadFile(filepath.Join(dest, "imported-notes-2.txt")); err != nil || !strings.Contains(string(notes), "-->") {
		t.Fatal("original notes not preserved")
	}
	if _, err := os.Stat(filepath.Join(dest, "public", "imported-notes-2.txt")); !os.IsNotExist(err) {
		t.Fatal("private original notes published as a public asset", err)
	}
	info, err := os.Stat(filepath.Join(dest, "imported-notes-2.txt"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("original notes must remain private", err)
	}
	if notes := extractSlideNotes(deck.Slides[1]); !strings.Contains(notes, "-- >") || !strings.Contains(notes, "Chart notes") {
		t.Fatal("escaped presenter notes lost their text", notes)
	}
	// SPA copies only public assets; the private recovery sidecar must never
	// become an audience download, even when ordinary presenter notes export.
	if err := os.Mkdir(filepath.Join(dest, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := exportSPA(dest, deck, "<html><head></head><body>Audience deck</body></html>", out); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"imported-notes-2.txt", filepath.Join("public", "imported-notes-2.txt")} {
		if _, err := os.Stat(filepath.Join(out, path)); !os.IsNotExist(err) {
			t.Fatal("SPA exposed private note sidecar", path, err)
		}
	}
	if _, err = os.Stat(filepath.Join(dest, "go.mod")); err != nil {
		t.Fatal("imported deck not portable")
	}
	if _, err = ImportPPTX("testdata/office-common.pptx", dest); err == nil {
		t.Fatal("overwrote existing deck")
	}
	after, _ := os.ReadFile(filepath.Join(dest, "deck.md"))
	if !bytes.Equal(after, source) {
		t.Fatal("authored files changed")
	}
}

func TestImportPPTXRasterValidationAndStaticGIF(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 24, 16), palette)
	second := image.NewPaletted(image.Rect(0, 0, 24, 16), palette)
	second.SetColorIndex(0, 0, 1)
	var animated bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		data   []byte
		images int
		code   string
	}{{"gif", animated.Bytes(), 1, "image-static"}, {"invalid", []byte("not an image"), 0, "image-unsupported"}} {
		t.Run(test.name, func(t *testing.T) {
			fixture := officeFixtureMutation(t, func(name string, data []byte) (string, []byte) {
				if name == "ppt/media/image1.png" {
					data = test.data
				}
				return name, data
			}, nil)
			dest := filepath.Join(t.TempDir(), "deck")
			report, err := ImportPPTX(fixture, dest)
			if err != nil {
				t.Fatal(err)
			}
			if report.Images != test.images {
				t.Fatalf("images=%d", report.Images)
			}
			found := false
			for _, warning := range report.Warnings {
				found = found || warning.Code == test.code
			}
			if !found {
				t.Fatal("missing image diagnostic")
			}
			if test.images > 0 {
				data, err := os.ReadFile(filepath.Join(dest, "public", "imported-image-1.png"))
				if err != nil {
					t.Fatal(err)
				}
				if _, format, err := image.Decode(bytes.NewReader(data)); err != nil || format != "png" {
					t.Fatal("animation not converted to PNG")
				}
			}
		})
	}
}

func officeFixtureMutation(t *testing.T, mutate func(string, []byte) (string, []byte), extras map[string][]byte) string {
	t.Helper()
	source, err := zip.OpenReader("testdata/office-common.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	name := filepath.Join(t.TempDir(), "mutated.pptx")
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(file)
	for _, f := range source.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		entry, content := mutate(f.Name, data)
		if entry == "" {
			continue
		}
		w, _ := z.Create(entry)
		w.Write(content)
	}
	for name, data := range extras {
		w, _ := z.Create(name)
		w.Write(data)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	return name
}

func TestImportPPTXUsesRelationshipOrderAndNeverFetchesExternalAssets(t *testing.T) {
	var fetched atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fetched.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	fixture := officeFixtureMutation(t, func(name string, data []byte) (string, []byte) {
		s := string(data)
		if name == "ppt/presentation.xml" {
			// The ZIP filenames stay unchanged; only presentation ordering changes.
			s = strings.Replace(s, `<p:sldId id="256" r:id="rId7"/><p:sldId id="257" r:id="rId9"/>`, `<p:sldId id="257" r:id="rId9"/><p:sldId id="256" r:id="rId7"/>`, 1)
		}
		if name == "ppt/slides/_rels/slide1.xml.rels" {
			s = strings.Replace(s, `Target="../media/image1.png"`, `Target="`+server.URL+`/image.png" TargetMode="External"`, 1)
		}
		return name, []byte(s)
	}, nil)
	dest := filepath.Join(t.TempDir(), "ordered")
	report, err := ImportPPTX(fixture, dest)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "deck.md"))
	first, second := strings.Index(string(data), "First &lt;"), strings.Index(string(data), "Second: chart")
	if first < 0 || second < 0 || second > first {
		t.Fatal("presentation relationship order ignored")
	}
	if fetched.Load() != 0 || report.Images != 0 {
		t.Fatal("external content fetched or copied")
	}
	found := false
	for _, w := range report.Warnings {
		found = found || w.Code == "external-omitted"
	}
	if !found {
		t.Fatal("external relationship not reported")
	}
}

func TestOfficeArchiveAndXMLBounds(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string, []byte) (string, []byte)
		extras map[string][]byte
	}{
		{"escape", nil, map[string][]byte{"../escape": []byte("bad")}},
		{"duplicate", nil, map[string][]byte{"ppt/presentation.xml": []byte("<p/>")}},
		{"oversize", nil, map[string][]byte{"ppt/oversized.xml": bytes.Repeat([]byte("x"), 16<<20+1)}},
		{"bad-rel", func(name string, data []byte) (string, []byte) {
			if name == "ppt/_rels/presentation.xml.rels" {
				data = bytes.ReplaceAll(data, []byte(`Target="slides/slide1.xml"`), []byte(`Target="../../escape.xml"`))
			}
			return name, data
		}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			mutate := test.mutate
			if mutate == nil {
				mutate = func(n string, d []byte) (string, []byte) { return n, d }
			}
			fixture := officeFixtureMutation(t, mutate, test.extras)
			dest := filepath.Join(t.TempDir(), "rejected")
			if _, err := ImportPPTX(fixture, dest); err == nil {
				t.Fatal("unsafe input accepted")
			}
			if _, err := os.Lstat(dest); !os.IsNotExist(err) {
				t.Fatal("failed import published destination")
			}
		})
	}
	for _, src := range []string{strings.Repeat("<a>", 70) + strings.Repeat("</a>", 70), `<!DOCTYPE a [<!ENTITY x "hello">]><a>&x;</a>`, `<?unsafe body?><a/>`} {
		if _, err := parseOfficeXML([]byte(src)); err == nil {
			t.Errorf("unsafe XML accepted %s", src[:min(30, len(src))])
		}
	}
	empty := filepath.Join(t.TempDir(), "empty")
	os.Mkdir(empty, 0755)
	if _, err := ImportPPTX("testdata/office-common.pptx", empty); err == nil {
		t.Fatal("existing empty directory overwritten")
	}
}

func TestPPTXTemplateRejectsNonSelfContainedTheme(t *testing.T) {
	fixture := officeFixtureMutation(t, func(name string, data []byte) (string, []byte) {
		if name == "ppt/theme/theme1.xml" {
			data = bytes.Replace(data, []byte(`<a:theme `), []byte(`<a:theme xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="unsafe" `), 1)
		}
		return name, data
	}, nil)
	if _, err := officeTemplateTheme(fixture); err == nil {
		t.Fatal("theme with relationship accepted")
	}
	for _, o := range []pptxObject{{Kind: "table", Width: math.Inf(1)}, {Kind: "chart", Chart: &pptxChart{Type: "pie", Categories: []string{"A"}, Values: []float64{-1}}}} {
		if pptxValidObject(o) && validPPTXChart(o.Chart) {
			t.Fatal("invalid object accepted")
		}
	}
	if got := pptxTableXML(3, pptxObject{Table: &pptxTable{Rows: [][]pptxCell{{{Text: "bad"}}}, ColumnWidths: []float64{math.NaN()}, RowHeights: []float64{10}}}); got != "" {
		t.Fatal(fmt.Sprintf("invalid table accepted: %s", got))
	}
}

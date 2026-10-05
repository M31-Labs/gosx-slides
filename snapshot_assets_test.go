package slides

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestSnapshotMixedDataAndLocalSrcset(t *testing.T) {
	deck := snapshotFixture(t, map[string]string{"public/image.png": "picture"})
	got, err := inlineSnapshotAssets(deck, `<img srcset="data:image/png;base64,YQ== 1x, /public/image.png 2x">`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "data:image/png;base64,YQ== 1x, data:image/png;base64,cGljdHVyZQ== 2x") {
		t.Fatalf("mixed srcset not embedded: %s", got)
	}
}

func snapshotFixture(t *testing.T, files map[string]string) *IslandDeck {
	t.Helper()
	dir := t.TempDir()
	for name, contents := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return &IslandDeck{Dir: dir}
}

func snapshotDataURL(t *testing.T, value string) string {
	t.Helper()
	prefix, encoded, found := strings.Cut(value, ",")
	if !found || !strings.HasSuffix(prefix, ";base64") {
		t.Fatalf("not a base64 data URL: %.100s", value)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSnapshotRecursiveCSSUsesStylesheetRelativePaths(t *testing.T) {
	deck := snapshotFixture(t, map[string]string{
		"public/styles/main.css":          `@import "nested/colors.css" screen; @font-face{src:url('../fonts/text.woff2')} .logo{background:url('../icons/mark.svg#arrow')}`,
		"public/styles/nested/colors.css": `@import url("../detail.css"); .color{color:blue}`,
		"public/styles/detail.css":        `.detail{background:url('../pixel.png')}`,
		"public/fonts/text.woff2":         "font bytes",
		"public/icons/mark.svg":           `<svg><path id="arrow"/></svg>`,
		"public/pixel.png":                "pixel bytes",
	})
	result, err := inlineSnapshotAssets(deck, `<link rel="alternate Stylesheet" href="/public/styles/main.css">`)
	if err != nil {
		t.Fatal(err)
	}
	tokenizer := html.NewTokenizer(strings.NewReader(result))
	tokenizer.Next()
	main := snapshotDataURL(t, tokenAttribute(tokenizer.Token(), "href"))
	if !strings.Contains(main, `data:font/woff2;base64,`+base64.StdEncoding.EncodeToString([]byte("font bytes"))) {
		t.Fatal("stylesheet-relative font was not embedded: " + main)
	}
	if !strings.Contains(main, "#arrow") || !strings.Contains(main, " screen;") {
		t.Fatal("embedding lost SVG fragment or import media suffix")
	}
	imports := snapshotCSSURL.FindAllStringSubmatch(main, -1)
	if len(imports) < 1 {
		t.Fatal("missing imported stylesheet")
	}
	nested := snapshotDataURL(t, imports[0][1])
	detailImports := snapshotCSSURL.FindAllStringSubmatch(nested, -1)
	if len(detailImports) != 1 {
		t.Fatal("nested stylesheet import was not retained: " + nested)
	}
	detail := snapshotDataURL(t, detailImports[0][1])
	if !strings.Contains(detail, `data:image/png;base64,`+base64.StdEncoding.EncodeToString([]byte("pixel bytes"))) {
		t.Fatal("nested stylesheet image was not embedded: " + detail)
	}
}

func TestSnapshotSVGImagesAndFragmentReferences(t *testing.T) {
	deck := snapshotFixture(t, map[string]string{"public/pixel.png": "pixel bytes"})
	source := `<svg xmlns="http://www.w3.org/2000/svg"><image href="/public/pixel.png"/><image xlink:href="public/pixel.png"/><image href="#local-symbol"/></svg><pre><code>&lt;image href="/public/pixel.png"&gt;</code></pre><script>const image="/public/pixel.png";</script>`
	result, err := inlineSnapshotAssets(deck, source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(result, "data:image/png;base64,") != 2 {
		t.Fatal("SVG href and xlink:href images were not both embedded: " + result)
	}
	for _, want := range []string{`href="#local-symbol"`, `<code>&lt;image href="/public/pixel.png"&gt;</code>`, `const image="/public/pixel.png";`} {
		if !strings.Contains(result, want) {
			t.Errorf("snapshot changed non-resource content %q", want)
		}
	}
}

func TestSnapshotCSSImportsBoundCyclesNestingAndEscapes(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"cycle":          {"public/a.css": `@import "b.css";`, "public/b.css": `@import "a.css";`},
		"outside public": {"public/a.css": `.secret{background:url('../secret.png')}`, "secret.png": "private"},
		"outside deck":   {"public/a.css": `.secret{background:url('../../secret.png')}`},
	} {
		t.Run(name, func(t *testing.T) {
			deck := snapshotFixture(t, files)
			if _, err := inlineSnapshotAssets(deck, `<style>@import '/public/a.css';</style>`); err == nil {
				t.Fatal("unsafe recursive import was accepted")
			}
		})
	}
	files := map[string]string{}
	for i := 0; i <= snapshotCSSDepthLimit; i++ {
		files[fmt.Sprintf("public/level%d.css", i)] = fmt.Sprintf(`@import "level%d.css";`, i+1)
	}
	deck := snapshotFixture(t, files)
	if _, err := inlineSnapshotAssets(deck, `<style>@import '/public/level0.css';</style>`); err == nil || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("CSS import nesting was not bounded: %v", err)
	}
}

func TestSnapshotExpandedStylesheetBudget(t *testing.T) {
	// Raw resources fit the per-file budget, but repeated references would
	// inflate one embedded stylesheet far beyond that budget.
	deck := snapshotFixture(t, map[string]string{
		"public/a.css":     strings.Repeat(`.x{background:url('pixel.png')}`, 16),
		"public/pixel.png": strings.Repeat("x", 1<<20),
	})
	if _, err := inlineSnapshotAssets(deck, `<style>@import '/public/a.css';</style>`); err == nil || !strings.Contains(err.Error(), "size budget") {
		t.Fatalf("expanded CSS size was not bounded: %v", err)
	}
}

func TestSnapshotCSSCommentsAndContentStayLiteral(t *testing.T) {
	deck := snapshotFixture(t, map[string]string{"public/pixel.png": "pixel bytes"})
	source := `<style>/* @import '/public/missing.css'; url('/public/missing.png') */ .example::after{content:"url('/public/not-an-image.png')"} .live{background:url('/public/pixel.png')}</style>`
	result, err := inlineSnapshotAssets(deck, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`/* @import '/public/missing.css'; url('/public/missing.png') */`, `content:"url('/public/not-an-image.png')"`, "data:image/png;base64,"} {
		if !strings.Contains(result, want) {
			t.Errorf("snapshot changed CSS literal or lost live resource %q", want)
		}
	}
}

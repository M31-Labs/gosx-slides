package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/mdpp"
)

func writeCompositionFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompositionPreservesCRLFIncludesSectionsAndSourceLocations(t *testing.T) {
	dir := t.TempDir()
	root := "# Root\r\n\r\n<!-- root notes -->\r\n\r\n---\r\n\r\n<!-- slides:include parts/library.md#chosen -->\r\n\r\n---\r\n\r\n# Finish\r\n\r\n![Root](/public/root.svg)\r\n"
	fragment := "<!-- slides:section skip -->\r\n# Skip\r\n\r\n<!-- slides:section chosen -->\r\n# Chosen\r\n\r\n![Child](child.svg)\r\n\r\n<!-- child notes -->\r\n"
	writeCompositionFiles(t, dir, map[string]string{"deck.md": root, "parts/library.md": fragment, "parts/child.svg": "<svg></svg>"})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(deck.Source) != root || !strings.Contains(string(deck.ExpandedSource), "# Chosen\r\n") || strings.Contains(string(deck.ExpandedSource), "# Skip") {
		t.Fatal("CRLF content changed or wrong section selected")
	}
	images := deck.Document.AST().Find(mdpp.NodeImage)
	if len(images) != 2 {
		t.Fatalf("images = %d", len(images))
	}
	for i, expected := range []struct{ file, source, image string }{{"parts/library.md", fragment, "![Child](child.svg)"}, {"deck.md", root, "![Root](/public/root.svg)"}} {
		where, ok := deck.SourceLocation(images[i].Range.StartByte, images[i].Range.EndByte)
		if !ok || where.File != expected.file || !strings.Contains(expected.source[where.StartByte:where.EndByte], expected.image) {
			t.Fatalf("wrong original CRLF range: %+v %v", where, ok)
		}
	}
	plain, err := parseIslandDeck(dir, []byte("# Plain\r\n\r\n![Root](/public/root.svg)\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	image := plain.Document.AST().Find(mdpp.NodeImage)[0]
	where, ok := plain.SourceLocation(image.Range.StartByte, image.Range.EndByte)
	if !ok || !strings.HasPrefix(string(plain.Source[where.StartByte:where.EndByte]), "![Root](/public/root.svg)") {
		t.Fatalf("CRLF root range = %+v", where)
	}
}

func TestPackInstallationRejectsSymlinkedDestinationParent(t *testing.T) {
	source, deck, outside := t.TempDir(), t.TempDir(), t.TempDir()
	writeCompositionFiles(t, source, map[string]string{"pack.json": `{"schema":1,"name":"labs","version":"1.0.0"}`})
	if err := os.Symlink(outside, filepath.Join(deck, "packs")); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if _, err := InstallDeckPack(deck, source); err == nil {
		t.Fatal("installed through outside packs symlink")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside destination modified: %v %v", entries, err)
	}
}

func TestCompositionIncludesSourceLocationsAndAssets(t *testing.T) {
	dir := t.TempDir()
	root := "# Intro\n\n<!-- root notes -->\n\n---\n\n<!-- slides:include sections/chapter.md#demo -->\n\n---\n\n# Finish\n\n:::motion {preset=fade}\nRoot motion\n:::\n"
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":              root,
		"sections/chapter.md":  "<!-- slides:section unused -->\n# Unused\n\n<!-- slides:section demo -->\n# Reused\n\n![Local diagram](./chart.svg)\n\n<Scene3D Src=\"./diagram.sir\" Steps=\"./steps.json\"/>\n\n```go\n<<< ../code.go 1-2\n```\n\n:::motion {preset=fade}\nIncluded motion\n:::\n\n<!-- fragment notes -->\n",
		"sections/chart.svg":   "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"sections/diagram.sir": "node: \"Node\"",
		"sections/steps.json":  "[]",
		"code.go":              "package demo\nvar Value = 1\n",
	})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(deck.Source) != root {
		t.Fatal("root author source changed")
	}
	if len(deck.Slides) != 3 {
		t.Fatalf("slides = %d, want 3; expanded=%s", len(deck.Slides), deck.ExpandedSource)
	}
	if strings.Contains(string(deck.ExpandedSource), "# Unused") {
		t.Fatal("section selection included unrelated section")
	}
	image := deck.Document.AST().Find(mdpp.NodeImage)[0]
	if got := image.Attr("src"); got != "/public/_slides/sections/chart.svg" {
		t.Fatalf("src=%s", got)
	}
	where, ok := deck.SourceLocation(image.Range.StartByte, image.Range.EndByte)
	if !ok || where.File != "sections/chapter.md" {
		t.Fatalf("source location=%+v %v", where, ok)
	}
	chapter, _ := os.ReadFile(filepath.Join(dir, where.File))
	if !strings.Contains(string(chapter[where.StartByte:where.EndByte]), "chart.svg") {
		t.Fatalf("wrong author range %+v", where)
	}
	var found bool
	for _, ref := range deck.Slides[1].Components {
		if ref.Name == "Scene3D" {
			props := parseProps(ref.Props)
			if props["Src"] != "sections/diagram.sir" || props["Steps"] != "sections/steps.json" {
				t.Fatalf("props=%v", props)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing graphic")
	}
	if got := sourceMotionRanges(deck); len(got) != 1 || !strings.Contains(string(deck.Source[got[0].End:]), "Root motion") {
		t.Fatalf("root ranges=%+v", got)
	}
	var snippet string
	for _, node := range deck.Document.AST().Find(mdpp.NodeCodeBlock) {
		if path, _, ok := parseSnippetDirective(node.Literal); ok {
			snippet = path
		}
	}
	if snippet != "code.go" {
		t.Fatalf("snippet=%s", snippet)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, image.Attr("src"), nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("asset status=%d body=%s", rec.Code, rec.Body.String())
	}
	out := t.TempDir()
	if err := copyCompositionAssets(deck, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "public", "_slides", "sections", "chart.svg")); err != nil {
		t.Fatal(err)
	}
}

func TestCompositionDiagnosticsAndCodeSafety(t *testing.T) {
	cases := []struct{ name, root, fragment, want string }{
		{"missing", "# T\n\n<!-- slides:include missing.md -->\n", "", "missing.md"},
		{"cycle", "<!-- slides:include part.md -->\n", "<!-- slides:include deck.md -->\n", "cycle"},
		{"escape", "<!-- slides:include ../outside.md -->\n", "", "escapes deck"},
		{"section", "<!-- slides:include part.md#absent -->\n", "# Part\n", "section \"absent\""},
		{"headmatter", "<!-- slides:include part.md -->\n", "---\ntitle: Wrong\n---\n# Part\n", "headmatter"},
		{"bad_asset", "<!-- slides:include part.md -->\n", "![missing](../outside.png)\n", "outside.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCompositionFiles(t, dir, map[string]string{"deck.md": tc.root, "part.md": tc.fragment})
			_, err := LoadIslandDeck(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	dir := t.TempDir()
	source := "# T\n\n```md\n<!-- slides:include missing.md -->\n```\n\nInline `<!-- slides:include missing.md -->` stays literal.\n"
	writeCompositionFiles(t, dir, map[string]string{"deck.md": source})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Includes) != 0 || string(deck.ExpandedSource) != source {
		t.Fatal("expanded literal example code")
	}
}

func TestCompositionRejectsSymlinkEscape(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	writeCompositionFiles(t, outside, map[string]string{"private.md": "secret"})
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	writeCompositionFiles(t, dir, map[string]string{"deck.md": "<!-- slides:include link/private.md -->\n"})
	_, err := LoadIslandDeck(dir)
	if err == nil || !strings.Contains(err.Error(), "escapes deck") {
		t.Fatalf("error=%v", err)
	}
}

func TestCompositionFragmentComponentsAndConflicts(t *testing.T) {
	const island = "package main\n\n//gosx:island\nfunc Widget(props any) Node {\n return <div>from fragment</div>\n}\n"
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":      "<!-- slides:include a/part.md -->\n",
		"a/part.md":    "# Reusable\n\n<Widget/>\n",
		"a/Widget.gsx": island,
	})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := deck.CompileComponent("Widget"); err != nil {
		t.Fatal(err)
	}
	componentInfo, err := os.Stat(DeckComponents(deck)[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	expectedInfo, err := os.Stat(filepath.Join(dir, "a", "Widget.gsx"))
	if err != nil || !os.SameFile(componentInfo, expectedInfo) {
		t.Fatalf("component resolved to a different file: %s (%v)", DeckComponents(deck)[0].Path, err)
	}
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":      "<!-- slides:include a/part.md -->\n\n---\n\n<!-- slides:include b/part.md -->\n",
		"b/part.md":    "# Other\n\n<Widget/>\n",
		"b/Widget.gsx": island,
	})
	if _, err := LoadIslandDeck(dir); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict error=%v", err)
	}
	writeCompositionFiles(t, dir, map[string]string{"Widget.gsx": strings.ReplaceAll(island, "from fragment", "deck override")})
	deck, err = LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := deck.componentSources["Widget"]; got != "Widget.gsx" {
		t.Fatalf("override=%s", got)
	}
}

func TestDeckPackInstallationCSSLayoutsAndComponents(t *testing.T) {
	source, dir := t.TempDir(), t.TempDir()
	writeCompositionFiles(t, source, map[string]string{
		"pack.json":        `{"schema":1,"name":"labs","version":"1.2.0","baseTheme":"paper","css":["styles/theme.css"],"layouts":["comparison"],"components":{"Badge":"Badge.gsx"}}`,
		"styles/theme.css": `main.deck { --accent: #345; background-image: url("../mark.svg"); } .layout-comparison { display:grid; }`,
		"mark.svg":         "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"Badge.gsx":        "package main\n\n//gosx:island\nfunc Badge(props any) Node {\n return <strong>Pack badge</strong>\n}\n",
	})
	manifest, err := InstallDeckPack(dir, source)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.0" {
		t.Fatal(manifest)
	}
	if _, err := InstallDeckPack(dir, source); err == nil {
		t.Fatal("replaced existing pack")
	}
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":  "---\ntitle: Pack deck\ntheme: labs\npacks: labs@1.2.0\n---\n\n```yaml\nlayout: comparison\n```\n\n# Pack\n\n<Badge/>\n",
		"deck.css": "main.deck { --accent: red; }",
	})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	if deckTheme(deck) != "paper" {
		t.Fatalf("theme=%s", deckTheme(deck))
	}
	if slideLayoutClass(deck.Slides[0]) != "layout-comparison" {
		t.Fatal("lost registered layout")
	}
	if _, known := slideLayoutInfo(deck.Slides[0]); !known {
		t.Fatal("pack layout reported unknown")
	}
	css := deckCustomCSS(deck)
	if !strings.Contains(css, `url("/public/_slides/packs/labs/mark.svg")`) {
		t.Fatalf("css=%s", css)
	}
	if strings.Index(css, "--accent: red") < strings.Index(css, "--accent: #345") {
		t.Fatal("deck CSS does not override pack")
	}
	if _, _, err := deck.CompileComponent("Badge"); err != nil {
		t.Fatal(err)
	}
	report, err := Doctor(dir)
	if err != nil {
		t.Fatal(err)
	}
	var discovered bool
	for _, item := range report.Items {
		if item.Name == "pack:labs" && item.Status == "ok" {
			discovered = true
		}
	}
	if !discovered {
		t.Fatal("doctor did not discover pack")
	}
	// A copied deck has no reference back to the installation source.
	copyDir := t.TempDir()
	if err := copyTree(dir, copyDir); err != nil {
		t.Fatal(err)
	}
	moved, err := LoadIslandDeck(copyDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := moved.CompileComponent("Badge"); err != nil {
		t.Fatal(err)
	}
}

func TestPackInvalidPinsManifestsAndInstallSymlinks(t *testing.T) {
	for _, pin := range []string{"labs", "../labs@1.0.0", "labs@latest", "labs@1.0.0, labs@1.0.0", "labs@2.0.0"} {
		t.Run(pin, func(t *testing.T) {
			dir := t.TempDir()
			writeCompositionFiles(t, dir, map[string]string{
				"deck.md":              "---\npacks: " + pin + "\n---\n\n# T\n",
				"packs/labs/pack.json": `{"schema":1,"name":"labs","version":"1.0.0"}`,
			})
			if _, err := LoadIslandDeck(dir); err == nil {
				t.Fatal("accepted invalid pin")
			}
		})
	}
	for _, manifest := range []string{
		`{"schema":2,"name":"labs","version":"1.0.0"}`,
		`{"schema":1,"name":"labs","version":"1.0.0","components":{"Badge":"../Badge.gsx"}}`,
		`{"schema":1,"name":"labs","version":"1.0.0","unknown":"future"}`,
	} {
		dir := t.TempDir()
		writeCompositionFiles(t, dir, map[string]string{"pack.json": manifest})
		if _, err := readPackManifest(dir); err == nil {
			t.Fatal("accepted invalid manifest")
		}
	}
	source, outside := t.TempDir(), t.TempDir()
	writeCompositionFiles(t, source, map[string]string{"pack.json": `{"schema":1,"name":"labs","version":"1.0.0"}`})
	writeCompositionFiles(t, outside, map[string]string{"private": "secret"})
	if err := os.Symlink(filepath.Join(outside, "private"), filepath.Join(source, "private")); err != nil {
		t.Skip(err)
	}
	if _, err := InstallDeckPack(t.TempDir(), source); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("install error=%v", err)
	}
}

func TestCompositionRootRangesAfterInclude(t *testing.T) {
	dir := t.TempDir()
	root := "# Opening\n\n<!-- slides:include part.md -->\n\n:::motion {preset=slide-up}\nRoot body\n:::\n"
	writeCompositionFiles(t, dir, map[string]string{"deck.md": root, "part.md": "A much longer fragment of content with enough bytes to shift the following directive.\n"})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	ranges := sourceMotionRanges(deck)
	if len(ranges) != 1 || ranges[0].Start != strings.Index(root, ":::motion") {
		t.Fatalf("ranges=%+v", ranges)
	}
}

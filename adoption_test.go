package slides

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"m31labs.dev/mdpp"
)

func TestCuratedTemplatesArePortableAndSemantic(t *testing.T) {
	catalog := DeckTemplates()
	catalog[0].Features[0] = "mutated"
	if DeckTemplates()[0].Features[0] == "mutated" {
		t.Fatal("catalog exposes mutable slices")
	}
	for _, template := range DeckTemplates() {
		t.Run(template.Name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "starter")
			if err := ScaffoldTemplate(dest, TemplateOptions{Template: template.Name}); err != nil {
				t.Fatal(err)
			}
			for _, generated := range []string{"build", "dist"} {
				if _, err := os.Stat(filepath.Join(dest, generated)); !os.IsNotExist(err) {
					t.Fatal("starter embedded generated runtime files", generated, err)
				}
			}
			deck, err := LoadIslandDeck(dest)
			if err != nil {
				t.Fatal(err)
			}
			if len(deck.Slides) != template.Slides || themeName(deckFrontmatterString(deck, "theme")) != template.Theme || len(deck.Packs) != 1 {
				t.Fatal("catalog and starter disagree", len(deck.Slides), themeName(deckFrontmatterString(deck, "theme")), deck.Packs)
			}
			if report := Validate(deck, ValidateOptions{}); len(report.Errors) > 0 {
				t.Fatal(report.Errors)
			}
			if _, err := compileDeckProgram(deck); err != nil {
				t.Fatal("starter does not compile", err)
			}
			if template.Name == "architecture-review" {
				if deck.Story == nil || len(deck.Story.Beats) != 3 {
					t.Fatal("semantic story absent")
				}
				if report, err := AssertStory(deck); err != nil || len(report.Errors) > 0 {
					t.Fatal(report, err)
				}
			}
			if template.Name == "teaching" && (deck.Simulations == nil || len(deck.Simulations.Simulations) != 1) {
				t.Fatal("simulation absent")
			}
			app, err := deck.NewServer(ServeOptions{Static: true})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != 200 || strings.Contains(response.Body.String(), "data-gosx-unresolved") || !strings.Contains(response.Body.String(), "studio-kicker") {
				t.Fatal("starter rendered incompletely")
			}
			module, err := os.ReadFile(filepath.Join(dest, "go.mod"))
			if err != nil || !strings.Contains(string(module), "require m31labs.dev/gosx") {
				t.Fatal("starter is not portable", err)
			}
			before, _ := os.ReadFile(filepath.Join(dest, DeckFileName))
			if err := ScaffoldTemplate(dest, TemplateOptions{Template: template.Name}); err == nil {
				t.Fatal("overwrote existing starter")
			}
			after, _ := os.ReadFile(filepath.Join(dest, DeckFileName))
			if !bytes.Equal(before, after) {
				t.Fatal("authored work changed")
			}
		})
	}
	dest := filepath.Join(t.TempDir(), "invalid")
	if err := ScaffoldTemplate(dest, TemplateOptions{Template: "unknown"}); err == nil {
		t.Fatal("unknown template accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed scaffold published directory")
	}
}

func TestMarkdownMigrationSupportedFormsAndPrivateProvenance(t *testing.T) {
	for _, test := range []struct {
		format, file string
		slides       int
		note         string
	}{{"slidev", "slidev.md", 2, "PRIVATE SLIDEV"}, {"marp", "marp.md", 2, "PRIVATE MARP"}, {"quarto", "quarto.qmd", 4, "PRIVATE QUARTO"}} {
		t.Run(test.format, func(t *testing.T) {
			root := t.TempDir()
			source, err := os.ReadFile(filepath.Join("testdata/migration", test.file))
			if err != nil {
				t.Fatal(err)
			}
			logo, _ := os.ReadFile("testdata/migration/logo.svg")
			writeCompositionFiles(t, root, map[string]string{test.file: string(source), "assets/logo.svg": string(logo), "public/logo.svg": string(logo)})
			dest := filepath.Join(root, "migrated")
			report, err := MigrateMarkdown(filepath.Join(root, test.file), dest, MigrationOptions{Format: test.format})
			if err != nil {
				t.Fatal(err)
			}
			if report.Slides != test.slides || report.Assets != 1 || len(report.Origins) != test.slides || len(report.Diagnostics) == 0 {
				t.Fatal("incomplete report", report)
			}
			for _, diagnostic := range report.Diagnostics {
				if diagnostic.File != test.file || diagnostic.Range.StartByte < 0 || diagnostic.Range.EndByte > len(source) || diagnostic.Range.StartLine < 1 {
					t.Fatal("invalid original range", diagnostic)
				}
			}
			original, _ := os.ReadFile(filepath.Join(dest, "migration/source.md"))
			if !bytes.Equal(original, source) {
				t.Fatal("original not preserved exactly")
			}
			if runtime.GOOS != "windows" {
				for path, mode := range map[string]os.FileMode{"migration": 0700, "migration/source.md": 0600, "migration/report.json": 0600} {
					info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(path)))
					if err != nil || info.Mode().Perm() != mode {
						t.Fatal("private migration permissions", path, info, err)
					}
				}
			}
			deck, err := LoadIslandDeck(dest)
			if err != nil {
				t.Fatal(err)
			}
			if len(deck.Slides) != test.slides {
				t.Fatal("wrong slide boundaries", len(deck.Slides))
			}
			notes := ""
			for _, slide := range deck.Slides {
				notes += extractSlideNotes(slide)
				if len(slide.Components) != 0 {
					t.Fatal("imported component became executable")
				}
			}
			if !strings.Contains(notes, test.note) {
				t.Fatal("private notes lost", notes)
			}
			if len(deck.Document.AST().Find(mdpp.NodeExpression)) > 0 {
				t.Fatal("source expression became executable")
			}
			if test.format == "slidev" {
				if parseFrontmatter(deck.Slides[0].Node.Attr("frontmatter"))["layout"] != "title" || parseFrontmatter(deck.Slides[1].Node.Attr("frontmatter"))["layout"] != "center" || !slideHasReveal(deck.Slides[1]) {
					t.Fatal("Slidev metadata/reveals lost")
				}
			}
			if test.format == "marp" {
				if deckFrontmatterString(deck, "aspect-ratio") != "4:3" {
					t.Fatal("Marp aspect lost")
				}
				if len(deck.Slides[1].Node.Find(mdpp.NodeCodeBlock)) != 1 {
					t.Fatal("code block was split")
				}
			}
			if test.format == "quarto" {
				if slideIdentityAttrs(deck.Slides[2])["data-slide-id"] != "prediction" {
					t.Fatal("Quarto heading identity lost")
				}
				found := false
				for _, d := range report.Diagnostics {
					if d.Code == "MIGRATION-EXECUTION" {
						found = true
					}
				}
				if !found {
					t.Fatal("execution loss was not reported")
				}
				if _, err := os.Stat(filepath.Join(root, "SHOULD_NOT_EXIST")); !os.IsNotExist(err) {
					t.Fatal("source cell executed")
				}
			}
			app, err := deck.NewServer(ServeOptions{Static: true})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if strings.Contains(response.Body.String(), test.note) {
				t.Fatal("speaker notes leaked into audience HTML")
			}
			out := filepath.Join(root, "snapshot")
			if err := exportSingleSnapshot(deck, response.Body.String(), out); err != nil {
				t.Fatal(err)
			}
			page, _ := os.ReadFile(filepath.Join(out, "deck.html"))
			if !strings.Contains(string(page), "data:image/svg+xml") || strings.Contains(string(page), test.note) {
				t.Fatal("portable snapshot or notes privacy failed")
			}
			if _, err := MigrateMarkdown(filepath.Join(root, test.file), dest, MigrationOptions{Format: test.format}); err == nil {
				t.Fatal("migration overwrote destination")
			}
			payload, err := os.ReadFile(filepath.Join(dest, "migration/report.json"))
			var saved MarkdownMigrationReport
			if err != nil || json.Unmarshal(payload, &saved) != nil || saved.SHA256 != report.SHA256 {
				t.Fatal("provenance report missing", err)
			}
		})
	}
}

func TestMarkdownMigrationRejectsActiveAndOversizedAssets(t *testing.T) {
	root := t.TempDir()
	assets := map[string]string{
		"script.svg":               `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"external.svg":             `<svg xmlns="http://www.w3.org/2000/svg"><use href="https://example.invalid/private.svg#id"/></svg>`,
		"css.svg":                  `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:u\72l(https://example.invalid)"/></svg>`,
		"invalid.svg":              `<svg/><svg/>`,
		".private/credentials.png": "secret",
		"safe.svg":                 `<svg xmlns="http://www.w3.org/2000/svg"><defs><path id="p" d="M0 0L1 1"/></defs><use href="#p" fill="url(#p)"/></svg>`,
	}
	var source strings.Builder
	source.WriteString("# Assets\n\n")
	for _, name := range adoptionSortedKeys(assets) {
		source.WriteString("![asset](" + name + ")\n\n")
	}
	assets["source.md"] = source.String() + "![large](large.png)\n\n![link](linked.svg)\n"
	writeCompositionFiles(t, root, assets)
	file, err := os.Create(filepath.Join(root, "large.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(migrationAssetLimit + 1); err != nil {
		t.Fatal(err)
	}
	file.Close()
	// A failed symlink creation on Windows remains a missing resource, which is
	// also rejected; Linux additionally exercises the explicit symlink boundary.
	_ = os.Symlink(filepath.Join(root, "safe.svg"), filepath.Join(root, "linked.svg"))
	report, err := MigrateMarkdown(filepath.Join(root, "source.md"), filepath.Join(root, "output"), MigrationOptions{Format: "marp"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Assets != 1 {
		t.Fatal("unsafe assets were copied", report.Assets, report.Diagnostics)
	}
	assetWarnings := 0
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "MIGRATION-ASSET" {
			assetWarnings++
		}
	}
	if assetWarnings != 7 {
		t.Fatal("asset losses missing", report.Diagnostics)
	}
}

func TestMarkdownMigrationDoesNotExposeCommentsFromUnsupportedHTML(t *testing.T) {
	root := t.TempDir()
	writeCompositionFiles(t, root, map[string]string{"source.md": "# Import\n\n<div>\n<!-- PRIVATE HTML NOTE -->\n<script>window.executed=true</script>\n</div>\n"})
	dest := filepath.Join(root, "output")
	if _, err := MigrateMarkdown(filepath.Join(root, "source.md"), dest, MigrationOptions{Format: "slidev"}); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeck(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(extractSlideNotes(deck.Slides[0]), "PRIVATE HTML NOTE") {
		t.Fatal("HTML note lost")
	}
	app, err := deck.NewServer(ServeOptions{Static: true})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(response.Body.String(), "PRIVATE HTML NOTE") || strings.Contains(response.Body.String(), "<script>window.executed") {
		t.Fatal("private comment or executable script leaked")
	}
}

func TestMarkdownMigrationMixedQuartoNotesRemainPrivate(t *testing.T) {
	root := t.TempDir()
	writeCompositionFiles(t, root, map[string]string{"source.qmd": "## Content\n\nVisible.\n\n::: {.notes .incremental}\n\nPRIVATE MIXED CLASSES\n\n::: {.anything}\n\nPRIVATE NESTED\n\n:::\n\n:::\n"})
	dest := filepath.Join(root, "output")
	if _, err := MigrateMarkdown(filepath.Join(root, "source.qmd"), dest, MigrationOptions{Format: "quarto"}); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeck(dest)
	if err != nil {
		t.Fatal(err)
	}
	notes := extractSlideNotes(deck.Slides[0])
	if !strings.Contains(notes, "PRIVATE MIXED CLASSES") || !strings.Contains(notes, "PRIVATE NESTED") {
		t.Fatal("mixed-class notes lost", notes)
	}
	app, err := deck.NewServer(ServeOptions{Static: true})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(response.Body.String(), "PRIVATE MIXED CLASSES") || strings.Contains(response.Body.String(), "PRIVATE NESTED") {
		t.Fatal("mixed-class notes exposed")
	}
}

func TestMarkdownMigrationGeneratedBudgetAndDiagnosticLimit(t *testing.T) {
	root := t.TempDir()
	writeCompositionFiles(t, root, map[string]string{"source.md": "# Large\n\n```text\n" + strings.Repeat("x", maxSourceBytes) + "\n```\n"})
	dest := filepath.Join(root, "output")
	if _, err := MigrateMarkdown(filepath.Join(root, "source.md"), dest, MigrationOptions{Format: "marp"}); err == nil || !strings.Contains(err.Error(), "editor limit") {
		t.Fatal("uneditable migration accepted", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("oversized deck published")
	}
	source := "# Warnings\n\n" + strings.Repeat("{unsafe()}\n\n", 600)
	writeCompositionFiles(t, root, map[string]string{"warnings.md": source})
	report, err := MigrateMarkdown(filepath.Join(root, "warnings.md"), filepath.Join(root, "warnings"), MigrationOptions{Format: "slidev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 500 || report.Diagnostics[499].Code != "MIGRATION-DIAGNOSTICS" {
		t.Fatal("diagnostic truncation was not explicit", len(report.Diagnostics))
	}
}

func TestMarkdownMigrationBoundsUnsafeAssetsAndCRLFRanges(t *testing.T) {
	root := t.TempDir()
	source := "# Safe\r\n\r\n{{ unsafe() }}\r\n\r\n![secret](../outside.png)\r\n\r\n![credentials](credentials.json)\r\n\r\n<!-- slides:include ../private.md -->\r\n"
	writeCompositionFiles(t, root, map[string]string{"source.md": source, "credentials.json": "PRIVATE"})
	report, err := MigrateMarkdown(filepath.Join(root, "source.md"), filepath.Join(root, "safe"), MigrationOptions{Format: "slidev"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Assets != 0 {
		t.Fatal("copied unsafe asset")
	}
	found := false
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "MIGRATION-RUNTIME" {
			if !strings.Contains(source[diagnostic.Range.StartByte:diagnostic.Range.EndByte], "unsafe()") || diagnostic.Range.StartLine != 3 {
				t.Fatal("CRLF diagnostic does not address original source", diagnostic)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing source-ranged runtime loss")
	}
	deck, err := LoadIslandDeck(filepath.Join(root, "safe"))
	if err != nil || len(deck.Slides) != 1 {
		t.Fatal("include-like note became active", err)
	}
	writeCompositionFiles(t, root, map[string]string{"huge.md": strings.Repeat("x", migrationSourceLimit+1)})
	dest := filepath.Join(root, "huge-out")
	if _, err := MigrateMarkdown(filepath.Join(root, "huge.md"), dest, MigrationOptions{Format: "marp"}); err == nil {
		t.Fatal("unbounded source accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed migration published partial directory")
	}
}

func TestAdoptionConcurrentFreshPublicationDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "starter")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- ScaffoldTemplate(dest, TemplateOptions{Template: "technical-talk"})
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("fresh publication did not have exactly one winner", successes)
	}
	if _, err := LoadIslandDeck(dest); err != nil {
		t.Fatal("published directory incomplete", err)
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".slides-adoption-") {
			t.Fatal("failed publication left staging files")
		}
	}
}

func TestAdoptionPublishPreservesConcurrentEmptyDestination(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("no-replace primitive is platform specific")
	}
	dest := filepath.Join(t.TempDir(), "destination")
	stage, dest, err := adoptionStage(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	if err := os.WriteFile(filepath.Join(stage, "authored.md"), []byte("draft"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := publishAdoptionDirectory(stage, dest); err == nil {
		t.Fatal("concurrently created directory replaced")
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 0 {
		t.Fatal("concurrent destination changed", entries, err)
	}
}

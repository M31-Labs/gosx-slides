package slides

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/mdpp"
)

const audienceTestSource = "---\ntitle: Audience demo\naudiences: [engineers, leaders, sharedOnly]\n---\n\n" +
	"```yaml\nid: intro\ncues: start, ready\n```\n\n# Shared introduction\n\n[Conclusion](#4/1) <a href='#4'>Final</a>\n\n<!-- shared notes -->\n\n---\n\n" +
	"```yaml\nid: internals\ncues: start, details\naudiences: engineers\n```\n\n# Engine internals\n\n<Counter Initial={2}/>\n\n<!-- engineer notes -->\n\n---\n\n" +
	"```yaml\nid: economics\naudiences:\n  - leaders\n```\n\n# Business economics\n\n<!-- leader notes -->\n\n---\n\n" +
	"```yaml\nid: finish\ncues: start, questions\n```\n\n# Shared conclusion\n\nVisible index {slide.index}.\n\n[Introduction](#intro/ready)\n\n<!-- final notes -->\n"

func TestAudienceSelectionCopiesReindexesAndRenders(t *testing.T) {
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":     audienceTestSource,
		"Counter.gsx": "package main\n\n//gosx:island\nfunc Counter(props any) Node {\n return <span>count {props.Initial}</span>\n}\n",
	})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := DeckAudiences(deck)
	if err != nil || !reflect.DeepEqual(names, []string{"engineers", "leaders", "sharedOnly"}) {
		t.Fatal(names, err)
	}
	engineers, err := SelectAudience(deck, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	leaders, err := SelectAudience(deck, "leaders")
	if err != nil {
		t.Fatal(err)
	}
	shared, err := LoadIslandDeckAudience(dir, "sharedOnly")
	if err != nil || len(shared.Slides) != 2 {
		t.Fatal("declared audience with shared slides", shared, err)
	}
	if engineers.Audience != "engineers" || len(engineers.Slides) != 3 || len(leaders.Slides) != 3 || len(deck.Slides) != 4 || len(engineers.Document.Slides()) != 3 {
		t.Fatal("incorrect slide selection")
	}
	for i, slide := range engineers.Slides {
		if slide.Index != i {
			t.Fatal("slide index not recomputed", slide.Index, i)
		}
	}
	if slideIdentityAttrs(engineers.Slides[2])["data-slide-id"] != "finish" || !reflect.DeepEqual(slideCueNames(engineers.Slides[1]), []string{"start", "details"}) {
		t.Fatal("stable addresses changed")
	}
	if got := engineers.Slides[0].Node.Find(mdpp.NodeLink)[0].Attr("href"); got != "#3/1" {
		t.Fatal("numeric link not remapped", got)
	}
	if got := deck.Slides[0].Node.Find(mdpp.NodeLink)[0].Attr("href"); got != "#4/1" {
		t.Fatal("original link mutated", got)
	}
	if !strings.Contains(engineers.Slides[0].Node.Find(mdpp.NodeHTMLInline)[0].Literal, `href="#3"`) {
		t.Fatal("raw HTML link not remapped")
	}
	if _, _, err := engineers.CompileComponent("Counter"); err != nil {
		t.Fatal(err)
	}
	if len(loadIslandDefs(leaders)) != 0 || Analyze(leaders).Components["Counter"] != 0 || Analyze(engineers).Components["Counter"] != 1 {
		t.Fatal("excluded component compiled/indexed")
	}
	for _, variant := range []*IslandDeck{engineers, leaders} {
		program, err := compileDeckProgram(variant)
		if err != nil || program.slideCount != 3 {
			t.Fatal("variant compilation", err)
		}
		app, err := variant.NewServer(ServeOptions{Static: true, IncludeNotes: true})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRecorder()
		app.Build().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/", nil))
		body := r.Body.String()
		if r.Code != 200 || !strings.Contains(body, "Visible index 2.") || strings.Count(body, `data-slide-id=`) != 3 {
			t.Fatal("variant SSR or numbering wrong", r.Code)
		}
		if variant == engineers && (strings.Contains(body, "Business economics") || strings.Contains(body, "leader notes")) {
			t.Fatal("excluded audience content or notes leaked")
		}
		if variant == leaders && (strings.Contains(body, "Engine internals") || strings.Contains(body, "engineer notes")) {
			t.Fatal("excluded engineer content or notes leaked")
		}
		if strings.Contains(notesHTML(variant), "engineer notes") != (variant == engineers) || strings.Contains(RehearsalScript(variant), "slide 4") {
			t.Fatal("notes/rehearsal retained excluded slides")
		}
	}
	engineers.Document.Frontmatter()["title"] = "changed"
	engineers.Source[0] = '!'
	engineers.Slides[1].Components[0].Name = "Changed"
	engineers.Slides[1].Node.Attrs["frontmatter"] = "id: changed"
	if deck.Document.Frontmatter()["title"] != "Audience demo" || string(deck.Source) != audienceTestSource || deck.Slides[1].Components[0].Name != "Counter" || strings.Contains(deck.Slides[1].Node.Attr("frontmatter"), "changed") {
		t.Fatal("variant mutation changed the author model")
	}
	if _, err := SelectAudience(leaders, "engineers"); err == nil {
		t.Fatal("changed variant without reloading complete source")
	}
}

func TestAudienceMetadataValidation(t *testing.T) {
	for _, tc := range []struct {
		source  string
		want    []string
		invalid bool
	}{
		{"id: demo", nil, false},
		{"# metadata comment", nil, false},
		{"audiences: engineers, leaders", []string{"engineers", "leaders"}, false},
		{"audiences: 'engineers, leaders'", []string{"engineers", "leaders"}, false},
		{"audiences: [engineers, 'leaders', engineers]", []string{"engineers", "leaders"}, false},
		{"audiences:\n - engineers\n - leaders", []string{"engineers", "leaders"}, false},
		{"audiences: []", nil, true}, {"audiences:", nil, true},
		{"audiences: engineers,", nil, true}, {"audiences: [1]", nil, true},
		{"audiences: true", nil, true}, {"audiences: [engineers, {id: leaders}]", nil, true},
		{"audiences: engineers\naudiences: leaders", nil, true},
		{"names: &names [engineers]\naudiences: *names", nil, true},
		{"audiences: [engineers, 'two words']", nil, true},
		{"audiences: engineers\nunknown: [", nil, true},
		{"audiences: engineers\n#" + strings.Repeat("x", 65536), nil, true},
	} {
		t.Run(tc.source[:min(len(tc.source), 45)], func(t *testing.T) {
			got, _, err := parseAudienceMetadata(tc.source)
			if (err != nil) != tc.invalid || !reflect.DeepEqual(got, tc.want) {
				t.Fatal(got, err)
			}
		})
	}
	for _, source := range []string{
		"---\naudiences: [engineers]\n---\n\n```yaml\naudiences: leaders\n```\n\n# Unknown tag\n",
		"---\naudiences: [engineers, leaders]\n---\n\n```yaml\naudiences: leaders\n```\n\n# Empty engineers variant\n",
	} {
		deck := loadDeckFromSource(t, source, nil)
		if _, err := SelectAudience(deck, "engineers"); err == nil {
			t.Fatal("invalid or empty audience selection accepted")
		}
	}
	deck := loadDeckFromSource(t, "```yaml\naudiences: engineers, leaders\n```\n\n# Tagged\n", nil)
	for _, name := range []string{"", " engineers", "ghost", "../escape", strings.Repeat("a", 65)} {
		if _, err := SelectAudience(deck, name); err == nil {
			t.Fatal("invalid name accepted", name)
		}
	}
	if _, err := SelectAudience(nil, "engineers"); err == nil {
		t.Fatal("nil deck accepted")
	}
	if names, err := DeckAudiences(deck); err != nil || !reflect.DeepEqual(names, []string{"engineers", "leaders"}) {
		t.Fatal("audience discovery", names, err)
	}
}

func TestAudienceExampleVariantsCompile(t *testing.T) {
	for _, audience := range []string{"engineers", "leaders", "workshop"} {
		deck, err := LoadIslandDeckAudience("examples/audience-variants", audience)
		if err != nil {
			t.Fatal(err)
		}
		count := 3
		if audience == "workshop" {
			count = 4
		}
		if len(deck.Slides) != count {
			t.Fatal(audience, len(deck.Slides))
		}
		if _, err := compileDeckProgram(deck); err != nil {
			t.Fatal(audience, err)
		}
		if diagnostics := Analyze(deck).Diagnostics; len(diagnostics) != 0 {
			t.Fatal(audience, diagnostics)
		}
	}
}

func TestAudienceIncludeOriginsAndOmittedLinks(t *testing.T) {
	for _, link := range []string{"[Hidden](#internal/details)", "[Hidden](#2)", `<a href="#2/1">Hidden</a>`} {
		t.Run(link, func(t *testing.T) {
			dir := t.TempDir()
			root := "---\naudiences: [engineers, leaders]\n---\n\n<!-- slides:include parts/common.md -->\n\n---\n\n```yaml\nid: internal\naudiences: engineers\n```\n\n# Internals\n"
			part := "# Shared\r\n\r\n" + link + "\r\n"
			writeCompositionFiles(t, dir, map[string]string{"deck.md": root, "parts/common.md": part})
			_, err := LoadIslandDeckAudience(dir, "leaders")
			var selection *AudienceSelectionError
			if !errors.As(err, &selection) || len(selection.Diagnostics) != 1 {
				t.Fatal("omitted link not rejected", err)
			}
			d := selection.Diagnostics[0]
			if d.Code != "AUDIENCE-LINK" || d.File != "parts/common.md" || d.Range.StartLine != 3 || !strings.Contains(part[d.Range.StartByte:d.Range.EndByte], "#") {
				t.Fatal("wrong include error location", d)
			}
			if !strings.Contains(err.Error(), "parts/common.md:3:") {
				t.Fatal("error lost source origin", err)
			}
		})
	}
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{
		"deck.md":            "# Intro\n\n<!-- notes -->\n\n---\n\n<!-- slides:include parts/technical.md -->\n\n---\n\n```yaml\naudiences: leaders\n```\n\n# Leaders\n",
		"parts/technical.md": "```yaml\naudiences: engineers\n```\n\n# Technical\r\n\r\n![Local](plot.svg)\r\n\r\n<Counter/>\n",
		"parts/plot.svg":     "<svg></svg>",
		"parts/Counter.gsx":  "package main\n\n//gosx:island\nfunc Counter(props any) Node {\n return <span>included</span>\n}\n",
	})
	deck, err := LoadIslandDeckAudience(dir, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 2 || len(deck.Slides[1].Components) != 1 {
		t.Fatal("wrong included variant")
	}
	image := deck.Slides[1].Node.Find(mdpp.NodeImage)[0]
	where, ok := deck.SourceLocation(image.Range.StartByte, image.Range.EndByte)
	if !ok || where.File != "parts/technical.md" {
		t.Fatal("included origin changed", where)
	}
	if _, ok := resolveCompositionAsset(deck, image.Attr("src")); !ok {
		t.Fatal("included asset registry lost")
	}
	if _, _, err := deck.CompileComponent("Counter"); err != nil {
		t.Fatal("included component origin lost", err)
	}
}

func TestAudienceDiagnosticsUseSelectedSemanticScopes(t *testing.T) {
	source := "```yaml\nid: excluded\naudiences: leaders\n```\n\n# Excluded\n\n:::motion {preset=fade after=missing}\nExcluded bad reference\n:::\n\n<!-- notes -->\n\n---\n\n" +
		"```yaml\nid: retained\naudiences: engineers\n```\n\n# Retained\n\n:::motion {preset=fade after=missingActive}\nActive bad reference\n:::\n"
	deck := loadDeckFromSource(t, source, nil)
	variant, err := SelectAudience(deck, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Analyze(variant).Diagnostics
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "missingActive") || diagnostics[0].File != "deck.md" {
		t.Fatal("semantic index restored excluded scopes", diagnostics)
	}
	if len(Analyze(deck).Diagnostics) != 2 {
		t.Fatal("original diagnostics changed")
	}
}

func TestAudienceCompiledStoryIsFilteredWithoutMutation(t *testing.T) {
	base, err := os.ReadFile("examples/semantic-story/deck.md")
	if err != nil {
		t.Fatal(err)
	}
	graph, _ := os.ReadFile("examples/semantic-story/request.sir")
	story, _ := os.ReadFile("examples/semantic-story/story.yaml")
	// Prepend an excluded slide so both story graphs and generated scene steps
	// must survive movement from original slide 1 to selected slide 0.
	head, body, _ := splitHeadmatter(string(base))
	source := "---\n" + head + "audiences: [engineers, leaders]\n---\n\n```yaml\nid: lead\naudiences: leaders\n```\n\n# Leadership only\n\n<!-- notes -->\n\n---\n\n" + strings.Replace(body, "id: request", "id: request\naudiences: engineers", 1)
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": source, "request.sir": string(graph), "story.yaml": string(story)})
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	variant, err := SelectAudience(deck, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	if variant.Story == nil || len(variant.Story.Beats) != 3 || variant.Story.Beats[0].SlideIndex != 0 || deck.Story.Beats[0].SlideIndex != 1 || variant.Story.Beats[0].GraphKey != deck.Story.Beats[0].GraphKey {
		t.Fatal("story reindex or graph key changed")
	}
	for _, graph := range variant.Story.Graphs {
		if graph.SlideIndex != 0 {
			t.Fatal("story graph index changed incorrectly")
		}
	}
	ref := variant.Slides[0].Components[0]
	graphic := compileDeckGraphics(variant)[graphicsKey(ref.Name, ref.Props)]
	if graphic.err != nil || !strings.Contains(ref.Props, `StorySlide="1"`) || len(variant.storySceneSteps) != 1 {
		t.Fatal("private generated scene steps lost", ref, graphic.err)
	}
	compiled, err := CompileStory(variant)
	if err != nil || len(compiled.Beats) != 3 || compiled.Beats[0].SlideIndex != 0 {
		t.Fatal("story cannot validate after selection", err)
	}
	if assertions, err := AssertStory(variant); err != nil || len(assertions.Errors) != 0 {
		t.Fatal("filtered story assertion failed", assertions, err)
	}
	leaders, err := LoadIslandDeckAudience(dir, "leaders")
	if err != nil || len(leaders.Slides) != 1 || len(leaders.Story.Beats) != 0 || len(leaders.Story.Graphs) != 0 {
		t.Fatal("excluded known story slides were not skipped", leaders, err)
	}
	variant.Story.Beats[1].Focus[0] = "changed"
	variant.storySceneSteps[graphicsKey(ref.Name, ref.Props)][0] = '!'
	if deck.Story.Beats[1].Focus[0] == "changed" || deck.storySceneSteps[graphicsKey(ref.Name, ref.Props)][0] == '!' {
		t.Fatal("copied story shares mutable data")
	}
	// Root-source bytes stay author-owned and unchanged after every selection.
	if saved, _ := os.ReadFile(filepath.Join(dir, DeckFileName)); string(saved) != source {
		t.Fatal("selection wrote author source")
	}
}

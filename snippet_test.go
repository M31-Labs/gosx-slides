package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snippet_test.go proves the `<<< ./path` snippet-import fence: a talk shows
// REAL source read from a file next to the deck at render time (never a
// stale paste), with an optional line window, composing with the click-step
// highlight spec, and sandboxed to the deck directory.

// snippetDeck loads a deck from md with extra plain files on disk.
func snippetDeck(t *testing.T, md string, files map[string]string) *IslandDeck {
	t.Helper()
	dir := newDeckDirUnderModule(t, md, nil)
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatalf("LoadIslandDeck: %v", err)
	}
	return deck
}

// TestSnippetImportRendersFile proves the fence body `<<< ./snip/hello.go`
// renders the file's highlighted source.
func TestSnippetImportRendersFile(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Snip\n\n```go\n<<< ./snip/hello.go\n```\n"
	deck := snippetDeck(t, md, map[string]string{
		"snip/hello.go": "package snip\n\nfunc Hello() string { return \"hi\" }\n",
	})
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `<pre class="code-block" data-lang="go"`) {
		t.Fatalf("expected a highlighted go block, got:\n%s", html)
	}
	if !strings.Contains(html, "Hello") || !strings.Contains(html, `class="ts-keyword"`) {
		t.Errorf("expected the file's source, highlighted, got:\n%s", html)
	}
	if strings.Contains(html, "&lt;&lt;&lt;") || strings.Contains(html, "<<<") {
		t.Errorf("the snippet directive itself must not render, got:\n%s", html)
	}
}

// TestSnippetImportLineWindow proves `<<< ./f.go 3-4` slices the file to the
// inclusive line window before rendering.
func TestSnippetImportLineWindow(t *testing.T) {
	md := "---\ntitle: T\n---\n\n# Window\n\n```text\n<<< ./f.txt 3-4\n```\n"
	deck := snippetDeck(t, md, map[string]string{
		"f.txt": "one\ntwo\nthree\nfour\nfive\n",
	})
	html := renderSlidesHTML(t, deck)

	for _, want := range []string{"three", "four"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected windowed line %q, got:\n%s", want, html)
		}
	}
	for _, banned := range []string{"one", "two", "five"} {
		if strings.Contains(html, ">"+banned+"<") {
			t.Errorf("line %q is outside the window and must not render:\n%s", banned, html)
		}
	}
}

// TestSnippetImportComposesWithSteps proves the highlight spec on the fence
// still drives click steps when the body is a snippet import.
func TestSnippetImportComposesWithSteps(t *testing.T) {
	md := "---\ntitle: T\n---\n\n# Steps\n\n```go {1|3}\n<<< ./s.go\n```\n"
	deck := snippetDeck(t, md, map[string]string{
		"s.go": "package s\n\nfunc A() {}\n",
	})
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `data-steps="2"`) {
		t.Fatalf("expected two click steps on the snippet block, got:\n%s", html)
	}
}

// TestSnippetImportSandboxed proves a traversal path renders the blocked
// placeholder instead of reading outside the deck.
func TestSnippetImportSandboxed(t *testing.T) {
	md := "---\ntitle: T\n---\n\n# Escape\n\n```text\n<<< ../outside.txt\n```\n"
	deck := snippetDeck(t, md, nil)
	dir := deck.Dir
	outside := filepath.Join(dir, "..", "outside.txt")
	if err := os.WriteFile(outside, []byte("SECRET-CONTENT"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	t.Cleanup(func() { os.Remove(outside) })
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "SECRET-CONTENT") {
		t.Fatalf("snippet import escaped the deck directory:\n%s", html)
	}
	if !strings.Contains(html, "snippet blocked") {
		t.Errorf("expected the blocked placeholder, got:\n%s", html)
	}
}

// TestSnippetMissingFileFailsSoft proves a missing snippet file renders a
// visible placeholder, never a broken slide.
func TestSnippetMissingFileFailsSoft(t *testing.T) {
	md := "---\ntitle: T\n---\n\n# Missing\n\n```go\n<<< ./nope.go\n```\n"
	deck := snippetDeck(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "snippet not found") {
		t.Fatalf("expected the not-found placeholder, got:\n%s", html)
	}
}

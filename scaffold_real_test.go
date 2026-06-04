package slides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScaffoldRealLaneProducesServeableDeck is the headline scaffold guarantee:
// `slides init <name>` writes a deck that loads + renders the WHOLE real lane
// (island, evaluated {expr}, highlighted code block, theme) with no further
// setup. It scaffolds into a temp dir UNDER the repo and loads + renders the deck
// through the real Slice-4 flow (an in-process compile that reads the .gsx files
// directly — no subprocess `go build`), asserting each pillar appears in the
// output. (Portability — serving from outside the repo — is covered by
// TestScaffoldRealLaneWritesPortableGoMod plus the manual serve gate.)
func TestScaffoldRealLaneProducesServeableDeck(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	base := filepath.Join(repoDir, "testdata")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	parent, err := os.MkdirTemp(base, "scaffold-real-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parent) })

	name := filepath.Join(parent, "mydeck")
	if err := ScaffoldRealLane(name, ScaffoldRealOptions{Theme: "neon"}); err != nil {
		t.Fatalf("ScaffoldRealLane: %v", err)
	}

	// The three expected files exist.
	for _, f := range []string{"deck.md", "Counter.gsx", "README"} {
		if _, err := os.Stat(filepath.Join(name, f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}

	deck, err := LoadIslandDeck(name)
	if err != nil {
		t.Fatalf("LoadIslandDeck on scaffolded deck: %v", err)
	}
	html := renderSlidesHTML(t, deck)

	// Pillar 1: the island hydrates — its shell carries a gosx island marker, and
	// the counter markup is present (Initial={3} seeds count is 3).
	if !strings.Contains(html, "counter") {
		t.Errorf("scaffolded deck did not render the Counter island:\n%s", html)
	}
	if !strings.Contains(html, "count is") {
		t.Errorf("scaffolded deck Counter did not render its label:\n%s", html)
	}

	// Pillar 2: {expr} EVALUATES. The opener has {strings.ToUpper("my deck")} ->
	// MY DECK, and a slide has {2 + 3} -> 5. Raw expr source must NOT survive.
	if !strings.Contains(html, "MY DECK") {
		t.Errorf("scaffolded {strings.ToUpper(...)} did not evaluate to MY DECK:\n%s", html)
	}
	if strings.Contains(html, "{2 + 3}") {
		t.Errorf("scaffolded {2 + 3} expr was not evaluated (raw source present):\n%s", html)
	}
	if !strings.Contains(html, ">5<") && !strings.Contains(html, " 5") {
		t.Errorf("scaffolded {2 + 3} did not evaluate to 5:\n%s", html)
	}

	// Pillar 3: the fenced ```go block renders as a highlighted code block.
	if !strings.Contains(html, `<pre class="code-block"`) {
		t.Errorf("scaffolded deck did not render a styled code block:\n%s", html)
	}
	if !strings.Contains(html, `class="ts-keyword"`) {
		t.Errorf("scaffolded code block was not syntax-highlighted:\n%s", html)
	}

	// Pillar 4: the chosen theme resolves (headmatter theme: neon).
	if got := themeName(deckTheme(deck)); got != "neon" {
		t.Errorf("scaffolded deck theme = %q, want neon", got)
	}
}

// TestScaffoldRealLaneWritesPortableGoMod is the portability guarantee: `slides
// init <name>` writes a go.mod that makes the deck a self-contained Go module
// requiring gosx, plus a .gitignore for the staged build/. This is what lets a
// scaffolded deck `slides serve` from ANY directory (StageRuntimeAssets +
// resolveGoSXRoot run `go build`/`go list` with cmd.Dir = deck dir, so the deck
// must itself require gosx). The end-to-end serve-from-outside-the-repo proof is
// exercised manually (it fetches gosx and builds a GOOS=js wasm); here we assert
// the on-disk module shape that makes it possible.
func TestScaffoldRealLaneWritesPortableGoMod(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "portdeck")
	if err := ScaffoldRealLane(name, ScaffoldRealOptions{Theme: "neon"}); err != nil {
		t.Fatalf("ScaffoldRealLane: %v", err)
	}

	// All five files exist (deck.md, Counter.gsx, go.mod, .gitignore, README).
	for _, f := range []string{"deck.md", "Counter.gsx", "go.mod", ".gitignore", "README"} {
		if _, err := os.Stat(filepath.Join(name, f)); err != nil {
			t.Errorf("scaffold missing %s: %v", f, err)
		}
	}

	// go.mod declares the deck module, targets go 1.26, and REQUIRES gosx — the
	// load-bearing line for portability.
	gomod, err := os.ReadFile(filepath.Join(name, "go.mod"))
	if err != nil {
		t.Fatalf("read scaffolded go.mod: %v", err)
	}
	src := string(gomod)
	if !strings.Contains(src, "module portdeck") {
		t.Errorf("go.mod missing `module portdeck`:\n%s", src)
	}
	if !strings.Contains(src, "go 1.26") {
		t.Errorf("go.mod missing `go 1.26`:\n%s", src)
	}
	if !strings.Contains(src, "require "+gosxModuleImportPath+" v") {
		t.Errorf("go.mod must require %s at a vN.N.N version:\n%s", gosxModuleImportPath, src)
	}

	// .gitignore keeps the staged build/ (and *.test) out of version control.
	gi, err := os.ReadFile(filepath.Join(name, ".gitignore"))
	if err != nil {
		t.Fatalf("read scaffolded .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), "build/") {
		t.Errorf(".gitignore must ignore build/:\n%s", gi)
	}
}

// TestModuleNameFromDeck proves the deck path -> module path sanitization: it uses
// the base name, lowercases, collapses illegal runs to single hyphens, and falls
// back to "deck" for an empty/fully-stripped name, so realLaneGoMod always emits a
// valid module path.
func TestModuleNameFromDeck(t *testing.T) {
	cases := []struct{ in, want string }{
		{"portdeck", "portdeck"},
		{"/tmp/portdeck", "portdeck"},
		{"/tmp/My Deck", "my-deck"},
		{"My_Cool.Deck", "my-cool-deck"},
		{"weird!!!name", "weird-name"},
		{"--leading-trailing--", "leading-trailing"},
		{"!!!", "deck"},
		{"", "deck"},
		{".", "deck"},
	}
	for _, c := range cases {
		if got := moduleNameFromDeck(c.in); got != c.want {
			t.Errorf("moduleNameFromDeck(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGoSXScaffoldVersion proves the pinned gosx version is a valid require target
// (starts with "v") — whether sourced from the running binary's build info or the
// fallback const — so a scaffolded go.mod never pins "(devel)" or an empty string.
func TestGoSXScaffoldVersion(t *testing.T) {
	v := gosxScaffoldVersion()
	if !strings.HasPrefix(v, "v") {
		t.Errorf("gosxScaffoldVersion() = %q, want a vN.N.N version (got non-release/empty)", v)
	}
}

// TestScaffoldRealLaneRejectsUnknownTheme proves an invalid --theme fails loudly
// rather than silently scaffolding a deck that renders with the default theme.
func TestScaffoldRealLaneRejectsUnknownTheme(t *testing.T) {
	dir := t.TempDir()
	err := ScaffoldRealLane(filepath.Join(dir, "x"), ScaffoldRealOptions{Theme: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an unknown theme, got nil")
	}
	if !strings.Contains(err.Error(), "unknown theme") {
		t.Errorf("expected an 'unknown theme' error, got: %v", err)
	}
}

// TestScaffoldRealLaneRefusesOverwrite proves re-running init never clobbers an
// existing deck.md.
func TestScaffoldRealLaneRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "deck")
	if err := ScaffoldRealLane(name, ScaffoldRealOptions{}); err != nil {
		t.Fatalf("first ScaffoldRealLane: %v", err)
	}
	if err := ScaffoldRealLane(name, ScaffoldRealOptions{}); err == nil {
		t.Fatal("expected an error scaffolding over an existing deck, got nil")
	}
}

// TestScaffoldRealLaneDefaultsTheme proves an empty theme falls back to the
// default (aurora) rather than erroring.
func TestScaffoldRealLaneDefaultsTheme(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "deck")
	if err := ScaffoldRealLane(name, ScaffoldRealOptions{}); err != nil {
		t.Fatalf("ScaffoldRealLane with empty theme: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(name, "deck.md"))
	if err != nil {
		t.Fatalf("read scaffolded deck: %v", err)
	}
	if !strings.Contains(string(src), "theme: "+defaultTheme) {
		t.Errorf("expected default theme %q in scaffolded deck, got:\n%s", defaultTheme, src)
	}
}

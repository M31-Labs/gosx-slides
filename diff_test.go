package slides

import (
	"strings"
	"testing"
)

// TestDiffCodeBlockColoring verifies that a ```diff fenced code block emits
// ts-diff-add on lines starting with '+' and ts-diff-del on lines starting
// with '-', while leaving context lines and +++ / --- file headers uncolored.
func TestDiffCodeBlockColoring(t *testing.T) {
	deckMD := "# Diff\n\n" + "```diff\n" +
		"+added line\n" +
		"-removed line\n" +
		" context line\n" +
		"+++ b/file.go\n" +
		"--- a/file.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		"```\n"

	deck := loadDeckFromSource(t, deckMD, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "ts-diff-add") {
		t.Fatalf("expected ts-diff-add class for '+added line' but got:\n%s", html)
	}
	if !strings.Contains(html, "ts-diff-del") {
		t.Fatalf("expected ts-diff-del class for '-removed line' but got:\n%s", html)
	}
	if !strings.Contains(html, "ts-diff-meta") {
		t.Fatalf("expected ts-diff-meta class for '@@ hunk header' but got:\n%s", html)
	}
}

// TestDiffFileHeadersUncolored verifies that +++ and --- file-header lines are
// NOT given the ts-diff-add / ts-diff-del coloring (they are prose, not hunks).
func TestDiffFileHeadersUncolored(t *testing.T) {
	deckMD := "# Diff headers\n\n" + "```diff\n" +
		"+++ b/new.go\n" +
		"--- a/old.go\n" +
		"```\n"

	deck := loadDeckFromSource(t, deckMD, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "ts-diff-add") {
		t.Fatalf("ts-diff-add must NOT appear for +++ file headers:\n%s", html)
	}
	if strings.Contains(html, "ts-diff-del") {
		t.Fatalf("ts-diff-del must NOT appear for --- file headers:\n%s", html)
	}
}

// TestDiffWithHighlightSteps verifies that diff coloring and emphasis stepping
// are additive: a diff block with a step spec carries both ts-diff-add/del AND
// the emphasis/data-step attributes on stepped lines.
func TestDiffWithHighlightSteps(t *testing.T) {
	deckMD := "# Stepped diff\n\n" + "```diff{1}\n" +
		"+added line\n" +
		"-removed line\n" +
		"```\n"

	deck := loadDeckFromSource(t, deckMD, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "ts-diff-add") {
		t.Fatalf("ts-diff-add missing in stepped diff block:\n%s", html)
	}
	if !strings.Contains(html, "ts-diff-del") {
		t.Fatalf("ts-diff-del missing in stepped diff block:\n%s", html)
	}
	// Line 1 (+added) should also carry emphasis from the step spec.
	if !strings.Contains(html, "emphasis") {
		t.Fatalf("emphasis class missing from stepped line in diff block:\n%s", html)
	}
}

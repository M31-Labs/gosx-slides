package slides

import (
	"strings"
	"testing"
)

// TestInlineJoinSpacePreserved proves the whitespace-only text node mdpp emits
// between two adjacent inline nodes survives the lowering. quoteTextExpr used
// to drop whitespace-only segments, fusing `**from zero to** *self-hosted*`
// into "…toself-hosted" (caught rendering the GopherCon 2026 deck).
func TestInlineJoinSpacePreserved(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Hero\n\n**from zero to** *self-hosted* infra\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "</strong><em>") {
		t.Fatalf("inline join space was dropped (strong/em fused):\n%s", html)
	}
	if !strings.Contains(html, "</strong> <em>") {
		t.Errorf("expected a joining space between </strong> and <em>, got:\n%s", html)
	}
	// The space after the closing *em* (before "infra") is a non-empty text
	// segment and must also survive.
	if !strings.Contains(html, "</em> infra") {
		t.Errorf("expected the text after the em to keep its leading space, got:\n%s", html)
	}
}

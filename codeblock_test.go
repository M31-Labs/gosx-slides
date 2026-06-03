package slides

import (
	"strings"
	"testing"
)

// TestCodeBlockRendersHighlightedPre proves a fenced ```go block lowers to a real
// <pre class="code-block"> with syntax-highlighted token spans (the headline
// nicey: code blocks are styled + highlighted, not plain quoted text). It renders
// through the real Slice-4 flow (compile + evaluate), so it also confirms the
// generated `{__slidesCode.Block(...)}` call resolves and its RawHTML Node rides
// the expression evaluator unescaped.
func TestCodeBlockRendersHighlightedPre(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Code\n\n" +
		"```go\npackage main\n\nfunc main() {}\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `<pre class="code-block"`) {
		t.Fatalf("expected a styled <pre class=\"code-block\">, got:\n%s", html)
	}
	if !strings.Contains(html, `data-lang="go"`) {
		t.Errorf("expected data-lang=\"go\" on the code block, got:\n%s", html)
	}
	// Token highlighting: gosx's highlighter emits ts-* span classes. If any are
	// present, the block is genuinely highlighted (not plain escaped text).
	if !strings.Contains(html, `class="ts-keyword"`) {
		t.Errorf("expected highlighted token spans (ts-keyword) in the code block, got:\n%s", html)
	}
	// The <pre> wraps a <code> element (semantic + the themes style pre.code-block code).
	if !strings.Contains(html, "<code>") {
		t.Errorf("expected a <code> inside the code block, got:\n%s", html)
	}
}

// TestCodeBlockEscapesDangerousSource proves code text containing markup-special
// characters is escaped, never injected as raw HTML — the safety property of the
// lowering (the highlighter escapes the code text; only the token <span>s are
// markup). A literal `<script>` in a fence must NOT appear unescaped.
func TestCodeBlockEscapesDangerousSource(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Danger\n\n" +
		"```text\n<script>alert(1)</script>\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("dangerous code was injected unescaped:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected the code's < and > to be escaped (&lt;script&gt;), got:\n%s", html)
	}
}

// TestBareFenceRendersAsCodeBlock proves a fence with NO language still renders as
// a styled code block (plain escaped text, no token spans), so unlabeled snippets
// are styled too rather than falling through to plain prose.
func TestBareFenceRendersAsCodeBlock(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Plain\n\n" +
		"```\njust some text\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `<pre class="code-block"`) {
		t.Fatalf("expected a styled <pre class=\"code-block\"> for a bare fence, got:\n%s", html)
	}
	if !strings.Contains(html, "just some text") {
		t.Errorf("expected the fence body in the code block, got:\n%s", html)
	}
}

// TestThemeCSSCarriesCodeBlockStyling proves every theme's stylesheet styles
// pre.code-block and at least one token class, so code looks intentional in all
// four themes (not just aurora).
func TestThemeCSSCarriesCodeBlockStyling(t *testing.T) {
	for _, theme := range Themes() {
		css := themeCSS(theme)
		if !strings.Contains(css, "pre.code-block") {
			t.Errorf("theme %q CSS missing pre.code-block styling", theme)
		}
		if !strings.Contains(css, ".ts-keyword") {
			t.Errorf("theme %q CSS missing .ts-keyword token color", theme)
		}
	}
}

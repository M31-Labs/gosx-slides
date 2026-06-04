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

// TestCodeBlockEmphasizesHighlightedLines proves a fence with `{1-3}` meta renders
// PER-LINE (ts-line wrappers) and marks lines 1-3 with the `emphasis` class while
// leaving the others un-emphasized — the static line-range emphasis feature. The
// <pre> also carries data-emphasized so the theme CSS dims the rest.
func TestCodeBlockEmphasizesHighlightedLines(t *testing.T) {
	// 5-line Go body; emphasize lines 1-3.
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Code\n\n" +
		"```go {1-3}\nline1 := 1\nline2 := 2\nline3 := 3\nline4 := 4\nline5 := 5\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `data-emphasized="true"`) {
		t.Fatalf("expected the code block to be marked data-emphasized:\n%s", html)
	}
	// Per-line wrappers from highlight.HTMLLines.
	if !strings.Contains(html, `class="ts-line`) || !strings.Contains(html, `data-line="1"`) {
		t.Fatalf("expected per-line ts-line wrappers with data-line, got:\n%s", html)
	}
	// Lines 1-3 emphasized; line 4 and 5 NOT.
	for _, ln := range []string{"1", "2", "3"} {
		marker := `class="ts-line emphasis" data-line="` + ln + `"`
		if !strings.Contains(html, marker) {
			t.Errorf("expected line %s to carry the emphasis class (%q):\n%s", ln, marker, html)
		}
	}
	for _, ln := range []string{"4", "5"} {
		bad := `class="ts-line emphasis" data-line="` + ln + `"`
		if strings.Contains(html, bad) {
			t.Errorf("line %s should NOT be emphasized but was:\n%s", ln, html)
		}
		// It must still be present as a plain ts-line.
		if !strings.Contains(html, `class="ts-line" data-line="`+ln+`"`) {
			t.Errorf("line %s missing its plain ts-line wrapper:\n%s", ln, html)
		}
	}
}

// TestCodeBlockNoHighlightsRendersPlain proves a fence with NO {…} meta renders
// exactly as before — no data-emphasized marker and (for backward compatibility)
// no per-line ts-line wrappers, so every line shows at full opacity.
func TestCodeBlockNoHighlightsRendersPlain(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Code\n\n" +
		"```go\nfunc main() {}\nx := 1\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "data-emphasized") {
		t.Errorf("a plain fence must not be marked data-emphasized:\n%s", html)
	}
	if strings.Contains(html, "ts-line") {
		t.Errorf("a plain fence must not emit per-line ts-line wrappers (no emphasis spec):\n%s", html)
	}
	// Still a real, highlighted code block (func -> ts-keyword).
	if !strings.Contains(html, `<pre class="code-block"`) || !strings.Contains(html, `class="ts-keyword"`) {
		t.Errorf("plain fence lost its highlighted code block:\n%s", html)
	}
}

// TestCodeBlockEmphasisStaysEscaped proves the per-line emphasis path is still
// XSS-safe: dangerous source in an emphasized fence is escaped, never injected.
func TestCodeBlockEmphasisStaysEscaped(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Danger\n\n" +
		"```text {1}\n<script>alert(1)</script>\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("dangerous code injected unescaped in the emphasis path:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("expected the code's < and > escaped (&lt;script&gt;):\n%s", html)
	}
}

// TestParseHighlightLines covers the line-range mini-DSL parser directly: comma
// lists, N-M ranges, | groups (unioned for static emphasis), `all`, and garbage.
func TestParseHighlightLines(t *testing.T) {
	cases := []struct {
		in   string
		want map[int]bool
	}{
		{"", nil},
		{"   ", nil},
		{"2", map[int]bool{2: true}},
		{"1-3", map[int]bool{1: true, 2: true, 3: true}},
		{"1,4", map[int]bool{1: true, 4: true}},
		{"1-3|5", map[int]bool{1: true, 2: true, 3: true, 5: true}}, // groups unioned
		{" 1 - 2 , 4 ", map[int]bool{1: true, 2: true, 4: true}},    // whitespace tolerant
		{"all", map[int]bool{allLinesSentinel: true}},
		{"2|all", map[int]bool{allLinesSentinel: true}}, // all wins
		{"0", nil},                       // non-positive dropped
		{"3-1", nil},                     // inverted range dropped
		{"abc", nil},                     // garbage dropped
		{"x,2,y", map[int]bool{2: true}}, // garbage items skipped, valid kept
	}
	for _, c := range cases {
		got := parseHighlightLines(c.in)
		if !sameIntSet(got, c.want) {
			t.Errorf("parseHighlightLines(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func sameIntSet(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

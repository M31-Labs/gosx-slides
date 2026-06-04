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
// leaving the others un-emphasized — the static line-range emphasis feature. A
// single (no-`|`) group is ONE click step, so the <pre> carries data-emphasized +
// data-steps="1" and each emphasized line carries data-step="1".
func TestCodeBlockEmphasizesHighlightedLines(t *testing.T) {
	// 5-line Go body; emphasize lines 1-3.
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Code\n\n" +
		"```go {1-3}\nline1 := 1\nline2 := 2\nline3 := 3\nline4 := 4\nline5 := 5\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `data-emphasized="true"`) {
		t.Fatalf("expected the code block to be marked data-emphasized:\n%s", html)
	}
	// A single group is one click step.
	if !strings.Contains(html, `data-steps="1"`) {
		t.Fatalf("expected a single-group fence to record data-steps=\"1\":\n%s", html)
	}
	// Per-line wrappers from highlight.HTMLLines.
	if !strings.Contains(html, `class="ts-line`) || !strings.Contains(html, `data-line="1"`) {
		t.Fatalf("expected per-line ts-line wrappers with data-line, got:\n%s", html)
	}
	// Lines 1-3 emphasized and tagged with step 1; line 4 and 5 NOT.
	for _, ln := range []string{"1", "2", "3"} {
		marker := `class="ts-line emphasis" data-step="1" data-line="` + ln + `"`
		if !strings.Contains(html, marker) {
			t.Errorf("expected line %s to carry the emphasis class + data-step=\"1\" (%q):\n%s", ln, marker, html)
		}
	}
	for _, ln := range []string{"4", "5"} {
		bad := `data-line="` + ln + `"`
		if strings.Contains(html, `emphasis" data-step="1" data-line="`+ln+`"`) {
			t.Errorf("line %s should NOT be emphasized but was:\n%s", ln, html)
		}
		// It must still be present as a plain ts-line (no emphasis, no data-step).
		if !strings.Contains(html, `class="ts-line" `+bad) {
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

// TestCodeBlockStepsEmitDataSteps is the marquee lowering test: a `{2-3|6}` fence
// (the showcase's slide-4 spec) lowers to an ordered CLICK-STEP block — the <pre>
// records data-steps="2" (two `|`-groups), lines 2-3 carry data-step="1" and line
// 6 carries data-step="2", and the un-stepped lines (1,4,5) stay plain ts-line.
// This is what navScript reads to step through the groups one ArrowRight at a time.
func TestCodeBlockStepsEmitDataSteps(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Code\n\n" +
		"```go {2-3|6}\na := 1\nb := 2\nc := 3\nd := 4\ne := 5\nf := 6\n```\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	// Two ordered steps recorded on the <pre>.
	if !strings.Contains(html, `data-steps="2"`) {
		t.Fatalf("expected data-steps=\"2\" for a two-group {2-3|6} fence:\n%s", html)
	}
	if !strings.Contains(html, `data-emphasized="true"`) {
		t.Errorf("a stepped fence must still carry data-emphasized:\n%s", html)
	}
	// Step 1 lights lines 2 and 3.
	for _, ln := range []string{"2", "3"} {
		marker := `class="ts-line emphasis" data-step="1" data-line="` + ln + `"`
		if !strings.Contains(html, marker) {
			t.Errorf("expected line %s in step 1 (%q):\n%s", ln, marker, html)
		}
	}
	// Step 2 lights line 6.
	if !strings.Contains(html, `class="ts-line emphasis" data-step="2" data-line="6"`) {
		t.Errorf("expected line 6 in step 2:\n%s", html)
	}
	// Lines 1, 4, 5 are in no step: plain ts-line, no data-step.
	for _, ln := range []string{"1", "4", "5"} {
		if strings.Contains(html, `data-step="`) && strings.Contains(html, `data-step="1" data-line="`+ln+`"`) {
			t.Errorf("line %s should carry no step:\n%s", ln, html)
		}
		if !strings.Contains(html, `class="ts-line" data-line="`+ln+`"`) {
			t.Errorf("line %s should be a plain ts-line:\n%s", ln, html)
		}
	}
}

// TestParseHighlightSteps covers the ordered-group parser directly: it must
// PRESERVE `|`-group order as separate steps (not union them like
// parseHighlightLines), drop empty/garbage groups, and turn an "all" group into
// the sentinel set.
func TestParseHighlightSteps(t *testing.T) {
	cases := []struct {
		in   string
		want []map[int]bool
	}{
		{"", nil},
		{"   ", nil},
		{"2-3|6", []map[int]bool{{2: true, 3: true}, {6: true}}}, // ordered, NOT unioned
		{"1-3", []map[int]bool{{1: true, 2: true, 3: true}}},     // single group = one step
		{"1|2|3", []map[int]bool{{1: true}, {2: true}, {3: true}}},
		{"|2|", []map[int]bool{{2: true}}},                            // empty groups dropped
		{"1|all", []map[int]bool{{1: true}, {allLinesSentinel: true}}}, // all -> sentinel step
		{"abc|2", []map[int]bool{{2: true}}},                          // garbage group dropped
		{"0|3-1", nil},                                                // all-garbage -> nil
	}
	for _, c := range cases {
		got := parseHighlightSteps(c.in)
		if !sameStepList(got, c.want) {
			t.Errorf("parseHighlightSteps(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func sameStepList(a, b []map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameIntSet(a[i], b[i]) {
			return false
		}
	}
	return true
}

// TestServeCarriesStepSpotlightCSS proves the served page carries the
// theme-agnostic click-through spotlight stylesheet: the dim-the-rest rule keyed
// on the slide's data-active-step + the per-step re-light rule that matches a
// line's data-step word. This is what makes the active step's lines pop while the
// others dim, in every theme.
func TestServeCarriesStepSpotlightCSS(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)
	for _, want := range []string{
		"data-active-step",                 // the attr navScript sets on the active slide
		"pre.code-block[data-steps]",       // scoped to stepped blocks
		`.ts-line.emphasis[data-step~="1"]`, // per-step re-light (word match)
		"var(--accent",                      // theme-agnostic: inherits theme tokens
	} {
		if !strings.Contains(body, want) {
			t.Errorf("step spotlight CSS missing %q:\n(searched served body)", want)
		}
	}
}

// TestNavScriptCarriesStepModel proves the served nav controller carries the
// step-then-slide model: it reads data-steps to size each slide's step budget,
// writes data-active-step, and broadcasts the step alongside the index so the
// presenter and audience step together.
func TestNavScriptCarriesStepModel(t *testing.T) {
	body := serveBody(t, twoSlideDeck, nil)
	script := extractFirstScript(t, body)
	for _, want := range []string{
		"data-steps",        // sizes the per-slide step budget
		"data-active-step",  // writes the active step on the slide
		"stepCountFor",      // per-slide step count helper
		"maxStep",           // step-then-slide budget check
		"step: step",        // broadcasts the step with the index
		"data.step",         // applies a remote step from the peer
	} {
		if !strings.Contains(script, want) {
			t.Errorf("nav step model missing %q:\n%s", want, script)
		}
	}
}

package slides

import (
	"strings"
	"testing"
)

// html_raw_test.go proves the sanitized raw-HTML passthrough lane: ordinary
// HTML written in deck.md now RENDERS (div/span/style compose freely, like
// Slidev) instead of being dropped, while script-capable constructs are
// stripped. The threat model is accidental injection (pasted content,
// generated markdown) — the sanitizer guarantees nothing executable survives,
// so a deck export is always safe to host.

// TestRawHTMLBlockPassesThrough proves a block-level <div> with class/style
// attributes survives lowering and rendering, and that markdown BETWEEN the
// open and close tags lands inside the div (the open/close literals and the
// markdown children are lowered in document order).
func TestRawHTMLBlockPassesThrough(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# HTML\n\n" +
		"<div class=\"grid two\" style=\"gap: 1rem\">\n\n" +
		"some **bold** prose\n\n" +
		"</div>\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `<div class="grid two" style="gap: 1rem">`) {
		t.Fatalf("expected the div to pass through with its attributes, got:\n%s", html)
	}
	div := strings.Index(html, `<div class="grid two"`)
	bold := strings.Index(html, "<strong>bold</strong>")
	close_ := strings.Index(html[div:], "</div>")
	if div < 0 || bold < 0 || close_ < 0 || bold < div || bold > div+close_ {
		t.Errorf("expected the markdown to render inside the div, got:\n%s", html)
	}
}

// TestRawHTMLInlinePassesThrough proves inline HTML (a <br> and a styled
// <span>) inside a heading survives — the v2 GopherCon deck needed a CSS
// pseudo-element hack for a designed hero line break; raw inline HTML is the
// real answer.
func TestRawHTMLInlinePassesThrough(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n" +
		"# Line one<br>and a <span class=\"hm1\">colored run</span>\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "<br") {
		t.Errorf("expected the <br> to pass through, got:\n%s", html)
	}
	if !strings.Contains(html, `<span class="hm1">`) {
		t.Errorf("expected the styled span to pass through, got:\n%s", html)
	}
}

// TestRawHTMLStripsExecutable proves script-capable constructs never survive:
// <script> subtrees (tag AND content), inline on* handlers, and javascript:
// URLs are all removed while the surrounding safe markup still renders.
func TestRawHTMLStripsExecutable(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Danger\n\n" +
		"<div class=\"ok\" onclick=\"steal()\">\n\n" +
		"safe text\n\n" +
		"<script>alert(1)</script>\n\n" +
		"<a href=\"javascript:alert(2)\">bad link</a>\n" +
		"<a href=\"https://example.com\">good link</a>\n\n" +
		"</div>\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	for _, banned := range []string{"<script", "alert(1)", "onclick", "javascript:"} {
		if strings.Contains(html, banned) {
			t.Errorf("executable construct %q survived sanitization:\n%s", banned, html)
		}
	}
	if !strings.Contains(html, `<div class="ok">`) {
		t.Errorf("expected the div to survive with its safe attributes, got:\n%s", html)
	}
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Errorf("expected the https link to survive, got:\n%s", html)
	}
}

// TestRawHTMLStyleBlockPassesThrough proves a per-slide <style> block passes
// through with its CSS content UNESCAPED (style is a raw-text element — CSS
// selectors like `a > b` must not become `a &gt; b`). This is the Slidev
// per-slide-style story without a framework hook.
func TestRawHTMLStyleBlockPassesThrough(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Styled\n\n" +
		"<style>.hero > em { color: rebeccapurple; }</style>\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, "<style>.hero > em { color: rebeccapurple; }</style>") {
		t.Errorf("expected the style block to pass through raw, got:\n%s", html)
	}
}

// TestRawHTMLComponentInsideBlockStillMounts proves a <Component/> tag written
// inside a raw HTML block still lowers to a component (it must not be
// swallowed by the passthrough lane).
func TestRawHTMLComponentInsideBlockStillMounts(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Mixed\n\n" +
		"<div class=\"stage\"><Counter Initial={3}/></div>\n"
	deck := loadDeckFromSource(t, md, map[string]string{
		"Counter": counterGSX,
	})
	html := renderSlidesHTML(t, deck)

	if !strings.Contains(html, `<div class="stage">`) {
		t.Errorf("expected the wrapping div to pass through, got:\n%s", html)
	}
	if !strings.Contains(html, "counter") {
		t.Errorf("expected the Counter island to render inside the div, got:\n%s", html)
	}
}

// TestRawHTMLUnknownTagDroppedTextKept proves a tag outside the allowlist is
// removed while its inner text still renders (drop the wrapper, keep the
// words), and HTML comments vanish entirely.
func TestRawHTMLUnknownTagDroppedTextKept(t *testing.T) {
	md := "---\ntitle: T\ntheme: aurora\n---\n\n# Unknown\n\n" +
		"<blink>still readable</blink>\n\n<!-- gone -->\n"
	deck := loadDeckFromSource(t, md, nil)
	html := renderSlidesHTML(t, deck)

	if strings.Contains(html, "<blink") {
		t.Errorf("expected the unknown tag to be dropped, got:\n%s", html)
	}
	if !strings.Contains(html, "still readable") {
		t.Errorf("expected the unknown tag's text to survive, got:\n%s", html)
	}
	if strings.Contains(html, "gone") {
		t.Errorf("expected the HTML comment to be dropped, got:\n%s", html)
	}
}

// TestSanitizeDeckHTMLUnit exercises the sanitizer directly on the fragments
// the tokenizer must get right: attribute escaping, data-/aria- passthrough,
// void elements, unbalanced close tags, and img src schemes.
func TestSanitizeDeckHTMLUnit(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"data and aria attrs kept", `<div data-x="1" aria-label="ok">`, `<div data-x="1" aria-label="ok">`},
		{"attr value escaped", `<span title="a<b">`, `<span title="a&lt;b">`},
		{"stray close tag kept when allowed", `</div>`, `</div>`},
		{"void br self-close normalized", `<br/>`, `<br/>`},
		{"img https kept", `<img src="https://x/y.png" alt="a">`, `<img src="https://x/y.png" alt="a"/>`},
		{"img data-image kept", `<img src="data:image/png;base64,AA==">`, `<img src="data:image/png;base64,AA=="/>`},
		{"img data-text dropped", `<img src="data:text/html;base64,AA==">`, `<img/>`},
		{"iframe subtree dropped", `<iframe src="https://x"></iframe>after`, `after`},
		{"target rel kept on a", `<a href="/x" target="_blank" rel="noopener">`, `<a href="/x" target="_blank" rel="noopener">`},
	}
	for _, tc := range cases {
		if got := sanitizeDeckHTML(tc.in); got != tc.want {
			t.Errorf("%s: sanitizeDeckHTML(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

package slides

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
)

func TestMathTypesetsThroughBothLowerings(t *testing.T) {
	deck := loadDeckFromSource(t, `# Equations

Inline $\frac{a}{b} + \sqrt{x}$ stays in prose.

$$
\begin{bmatrix}1 & 2 \\ 3 & 4\end{bmatrix}
$$
`, nil)
	if !deckHasMath(deck) {
		t.Fatal("parsed equations were not detected")
	}
	compiled := renderSlidesHTML(t, deck)
	r := island.NewRenderer("math-fallback")
	var fallback strings.Builder
	for _, slide := range deck.Slides {
		fallback.WriteString(gosx.RenderHTML(renderIslandSlide(r, slide, nil, "")))
	}
	for lane, rendered := range map[string]string{"compiled": compiled, "fallback": fallback.String()} {
		for _, want := range []string{`class="math-inline"`, `class="math-block"`, `<math xmlns="http://www.w3.org/1998/Math/MathML"`, `<mfrac>`, `<msqrt>`, `<mtable`, `aria-hidden="true"`, `encoding="application/x-tex"`} {
			if !strings.Contains(rendered, want) {
				t.Errorf("%s math lowering missing %s", lane, want)
			}
		}
		if strings.Contains(rendered, `math-error`) {
			t.Errorf("%s valid math failed: %s", lane, rendered)
		}
	}
}

func TestMathFailuresAreVisibleAndEscaped(t *testing.T) {
	for _, source := range []string{`\unknowncommand{x}`, `\frac{a}{`, `\href{javascript:alert(1)}{click}`, `\includegraphics{https://example.com/leak.png}`, `\htmlClass{injected}{x}`, `\def\loop{\loop}\loop`} {
		rendered := deckMath.renderHTML(source, false)
		if !strings.Contains(rendered, `math-error`) || !strings.Contains(rendered, `Equation could not be typeset:`) {
			t.Errorf("invalid/untrusted math did not show a diagnostic: %.80s", source)
		}
		if !strings.Contains(rendered, htmlEscape(source)) {
			t.Errorf("math diagnostic lost the source: %.80s", source)
		}
		if strings.Contains(rendered, "<script>") || strings.Contains(rendered, "<a ") || strings.Contains(rendered, "<img ") {
			t.Errorf("unsafe content reached equation output: %.200s", rendered)
		}
	}
	// Angle brackets are ordinary math relations, never an HTML passthrough.
	if rendered := deckMath.renderHTML(`<script>alert("unsafe")</script>`, false); strings.Contains(rendered, "<script>") {
		t.Fatal("TeX containing HTML-like text injected HTML")
	}
	if rendered := deckMath.renderHTML("x+1", false); strings.Contains(rendered, `math-error`) {
		t.Fatal("renderer did not recover after bad equations: " + rendered)
	}
}

func TestMathRejectedSourcePreviewBounded(t *testing.T) {
	for _, source := range []string{strings.Repeat("<", 300000), strings.Repeat("a", mathMaxSource-1) + "🙂" + strings.Repeat("z", 300000)} {
		rendered := deckMath.renderHTML(source, true)
		if len(rendered) > mathMaxOutput {
			t.Fatalf("error output exceeded cap: %d bytes", len(rendered))
		}
		if !utf8.ValidString(rendered) {
			t.Fatal("truncated preview split a UTF-8 character")
		}
		if !strings.Contains(rendered, "source truncated") || !strings.Contains(rendered, "16 KiB source limit") {
			t.Fatal("rejected source has no truncation diagnostic")
		}
		if strings.Contains(rendered, htmlEscape(source)) {
			t.Fatal("oversized source was emitted in full")
		}
	}
	// Generated expression arguments bypass source-file size checks, so the
	// bound must hold at the renderer's public expression boundary too.
	deck := loadDeckFromSource(t, "# Equation\n\n{__slidesMath.Render(strings.Repeat(\"<\", 300000), true)}\n", nil)
	rendered := renderSlidesHTML(t, deck)
	if len(rendered) > mathMaxOutput || !strings.Contains(rendered, "source truncated") {
		t.Fatalf("expression produced an unbounded diagnostic: %d bytes", len(rendered))
	}
	if !strings.Contains(string(deck.Source), "300000") {
		t.Fatal("author source was altered")
	}
}

// Match Go's escaping without importing an alternative HTML rendering path.
func htmlEscape(source string) string {
	return gosx.RenderHTML(gosx.Text(source))
}

func TestMathCacheBoundedAndConcurrent(t *testing.T) {
	renderer := &mathRenderer{}
	var wg sync.WaitGroup
	errors := make(chan string, 8)
	for n := range 8 {
		wg.Go(func() {
			for i := range 40 {
				result := renderer.renderHTML(fmt.Sprintf("x^{%d}+%d", n, i), i%2 == 0)
				if strings.Contains(result, `math-error`) {
					errors <- result
					return
				}
			}
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if len(renderer.cache) > mathCacheEntries || renderer.bytes > mathCacheBytes {
		t.Fatalf("unbounded math cache: %d entries, %d bytes", len(renderer.cache), renderer.bytes)
	}
	oldest := renderer.order[0]
	renderer.renderHTML("newest", false)
	if _, ok := renderer.cache[oldest]; ok || len(renderer.cache) != mathCacheEntries {
		t.Fatal("cache did not evict its oldest entry")
	}
	// A global definition is scoped to one render invocation.
	renderer.renderHTML(`\gdef\custom{z}\custom`, false)
	if result := renderer.renderHTML(`\custom`, false); !strings.Contains(result, `math-error`) {
		t.Fatal("a formula leaked macro definitions into another formula")
	}
}

func TestMathDeadlineRecovers(t *testing.T) {
	renderer := &mathRenderer{vm: goja.New()}
	if _, err := renderer.withDeadline(10*time.Millisecond, func() (goja.Value, error) {
		return renderer.vm.RunString("for (;;) {}")
	}); err == nil {
		t.Fatal("runaway JavaScript did not hit the deadline")
	}
	// A completed interrupt callback cannot poison the next invocation.
	value, err := renderer.withDeadline(time.Second, func() (goja.Value, error) {
		return renderer.vm.RunString("1+2")
	})
	if err != nil || value.ToInteger() != 3 {
		t.Fatalf("runtime did not recover from deadline: %v", err)
	}
}

func TestMathAssetsOfflineAndFeatureGated(t *testing.T) {
	for _, hasMath := range []bool{false, true} {
		source := "# Plain\n\nNo equation.\n"
		if hasMath {
			source = "---\ntheme: swiss\n---\n\n# Equation\n\n$E=mc^2$\n"
		}
		deck := loadDeckFromSource(t, source, nil)
		app, err := deck.NewServer(ServeOptions{Static: true})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		body := rec.Body.String()
		if rec.Code != 200 {
			t.Fatal("deck did not serve: ", rec.Code)
		}
		if strings.Contains(body, `data-slides-math="katex-0.19.0"`) != hasMath {
			t.Errorf("math stylesheet gating is incorrect (math=%v)", hasMath)
		}
		if !hasMath {
			continue
		}
		for _, want := range []string{"data:font/woff2;base64,", "The MIT License (MIT)", "<mrow>", "<msup>"} {
			if !strings.Contains(body, want) {
				t.Errorf("offline math missing %q", want)
			}
		}
		if strings.Contains(body, "url(fonts/") || strings.Contains(body, "katex.render") || strings.Contains(body, "katex.min.js") {
			t.Error("math deck requires external assets or browser math execution")
		}
		out := t.TempDir()
		if err := ExportStatic(deck.Dir, ExportOptions{Format: "single", OutDir: out}); err != nil {
			t.Fatal(err)
		}
		snapshot, err := os.ReadFile(filepath.Join(out, "deck.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(snapshot), "<msup>") || !strings.Contains(string(snapshot), "data:font/woff2;base64,") {
			t.Error("single export lost offline typeset math")
		}
	}
}

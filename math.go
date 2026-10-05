package slides

import (
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx-slides/internal/mathassets"
	"m31labs.dev/mdpp"
)

const (
	mathNamespace    = "__slidesMath"
	mathRenderFunc   = "Render"
	mathMaxSource    = 16 << 10
	mathMaxOutput    = 1 << 20
	mathCacheEntries = 256
	mathCacheBytes   = 4 << 20
)

// KaTeX runs inside a pure-Go VM on the server. MathML and visual HTML are
// present before JavaScript executes, including in snapshots and printed decks.
// Plain decks never initialize the VM or emit the embedded font stylesheet.
var deckMath = mathRenderer{}

type mathKey struct {
	source  string
	display bool
}

type mathRenderer struct {
	mu     sync.Mutex
	once   sync.Once
	vm     *goja.Runtime
	render goja.Callable
	err    error
	cache  map[mathKey]string
	order  []mathKey
	bytes  int
}

func (r *mathRenderer) initialize() {
	source, err := mathassets.Files.ReadFile("katex.min.js")
	if err != nil {
		r.err = err
		return
	}
	r.vm = goja.New()
	r.vm.SetMaxCallStackSize(1024)
	_, r.err = r.withDeadline(2*time.Second, func() (goja.Value, error) {
		return r.vm.RunScript("katex-0.19.0.min.js", string(source))
	})
	if r.err != nil {
		return
	}
	r.render, _ = goja.AssertFunction(r.vm.Get("katex").ToObject(r.vm).Get("renderToString"))
	if r.render == nil {
		r.err = fmt.Errorf("bundled math renderer is unavailable")
	}
}

// Wait for an already-fired callback before clearing the interrupt. Without
// this handshake, a late callback could interrupt the next equation's render.
// The caller holds mu: a goja runtime may only execute on one goroutine.
func (r *mathRenderer) withDeadline(timeout time.Duration, run func() (goja.Value, error)) (goja.Value, error) {
	expired := make(chan struct{})
	timer := time.AfterFunc(timeout, func() {
		r.vm.Interrupt("equation rendering time limit exceeded")
		close(expired)
	})
	defer func() {
		if !timer.Stop() {
			<-expired
		}
		r.vm.ClearInterrupt()
	}()
	return run()
}

func (r *mathRenderer) renderHTML(source string, display bool) string {
	if len(source) > mathMaxSource {
		return mathErrorHTML(source, display, "equation exceeds the 16 KiB source limit")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := mathKey{source, display}
	if result, ok := r.cache[key]; ok {
		return result
	}
	r.once.Do(r.initialize)
	var result string
	if r.err != nil {
		result = mathErrorHTML(source, display, r.err.Error())
	} else {
		// Each equation has fresh macros. One equation cannot redefine commands
		// for another slide. Deny every trust-sensitive command; record the denial
		// because KaTeX can color such commands red without raising ParseError.
		blockedCommand := ""
		options := r.vm.ToValue(map[string]any{
			"displayMode":  display,
			"output":       "htmlAndMathml",
			"throwOnError": true,
			"strict":       "error",
			"trust": func(context map[string]any) bool {
				blockedCommand, _ = context["command"].(string)
				return false
			},
			"maxExpand": 1000,
			"maxSize":   20,
			"macros":    map[string]any{},
		})
		value, err := r.withDeadline(250*time.Millisecond, func() (goja.Value, error) {
			return r.render(goja.Undefined(), r.vm.ToValue(source), options)
		})
		if blockedCommand != "" {
			result = mathErrorHTML(source, display, "unsupported trust-sensitive command: "+blockedCommand)
		} else if err != nil {
			result = mathErrorHTML(source, display, err.Error())
		} else if rendered := value.String(); len(rendered) > mathMaxOutput {
			result = mathErrorHTML(source, display, "equation exceeds the 1 MiB output limit")
		} else {
			tag, class := mathWrapper(display)
			result = "<" + tag + ` class="` + class + `" data-math-source="` + html.EscapeString(source) + `">` + rendered + "</" + tag + ">"
		}
	}
	if r.cache == nil {
		r.cache = make(map[mathKey]string)
	}
	// FIFO bounds both count and retained bytes; an unbounded map grows forever
	// when the browser editor changes formulas during a long-running session.
	size := len(source) + len(result)
	if size <= mathCacheBytes {
		for len(r.order) >= mathCacheEntries || r.bytes+size > mathCacheBytes {
			old := r.order[0]
			r.order = r.order[1:]
			r.bytes -= len(old.source) + len(r.cache[old])
			delete(r.cache, old)
		}
		r.cache[key] = result
		r.order = append(r.order, key)
		r.bytes += size
	}
	return result
}

func mathWrapper(display bool) (string, string) {
	if display {
		return "div", "math-block"
	}
	return "span", "math-inline"
}

func mathErrorHTML(source string, display bool, message string) string {
	tag, class := mathWrapper(display)
	// Rejected input must not bypass the output bound through the diagnostic.
	// Keep the author source intact; only its rendered preview is shortened.
	if preview, truncated := mathPreview(source, mathMaxSource); truncated {
		source = preview + "\n… [source truncated]"
	}
	// Runtime errors include internal frames; authors need the first diagnostic.
	message = strings.SplitN(message, "\n", 2)[0]
	message, _ = mathPreview(message, 1024)
	return "<" + tag + ` class="` + class + ` math-error" data-math-error="` + html.EscapeString(message) + `"><code>` +
		html.EscapeString(source) + `</code><span class="math-error-message">Equation could not be typeset: ` +
		html.EscapeString(message) + "</span></" + tag + ">"
}

func mathPreview(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit], true
}

func mathNode(source string, display bool) gosx.Node {
	// Only KaTeX's generated output reaches RawHTML; author TeX is not HTML.
	return gosx.RawHTML(deckMath.renderHTML(source, display))
}

func deckHasMath(deck *IslandDeck) bool {
	if deck == nil {
		return false
	}
	var hasMath func(*mdpp.Node) bool
	hasMath = func(n *mdpp.Node) bool {
		if n == nil {
			return false
		}
		if n.Type == mdpp.NodeMathInline || n.Type == mdpp.NodeMathBlock {
			return true
		}
		for _, child := range n.Children {
			if hasMath(child) {
				return true
			}
		}
		return false
	}
	for _, slide := range deck.Slides {
		if hasMath(slide.Node) {
			return true
		}
	}
	return false
}

var mathCSS = sync.OnceValue(func() string {
	css, _ := mathassets.Files.ReadFile("katex.min.css")
	license, _ := mathassets.Files.ReadFile("LICENSE")
	// WOFF2 is supported by the browsers used by slides. Inline its bytes and
	// remove older fallback URLs so every font works from file:// and subpaths.
	fallbacks := regexp.MustCompile(`,url\(fonts/[^)]+\.(?:woff|ttf)\) format\("[^"]+"\)`)
	fontURLs := regexp.MustCompile(`url\(fonts/([^)]+\.woff2)\)`)
	text := fontURLs.ReplaceAllStringFunc(fallbacks.ReplaceAllString(string(css), ""), func(url string) string {
		filename := fontURLs.FindStringSubmatch(url)[1]
		data, err := mathassets.Files.ReadFile("fonts/" + filename)
		if err != nil {
			panic("missing bundled math font: " + filename)
		}
		return "url(data:font/woff2;base64," + base64.StdEncoding.EncodeToString(data) + ")"
	})
	return "/* KaTeX 0.19.0\n" + string(license) + "*/\n" + text + `
.math-block { margin: 1em 0; max-width: 100%; }
.math-inline, .math-block { color: inherit; }
.math-error code { white-space: pre-wrap; overflow-wrap: anywhere; }
.math-error-message { display: block; font: .65em/1.4 var(--font-body, sans-serif); color: var(--accent, #b31b1b); }
`
})

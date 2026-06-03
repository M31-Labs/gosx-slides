package slides

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// RenderOptions controls which runtime affordances are included.
type RenderOptions struct {
	Mode       string
	LiveReload bool
	BasePath   string
}

// RenderDeckHTML renders a complete HTML document.
func RenderDeckHTML(deck *Deck, opts RenderOptions) string {
	if opts.Mode == "" {
		opts.Mode = "deck"
	}
	var buf bytes.Buffer
	state := deckState(deck)
	stateJSON, _ := json.Marshal(state)
	title := html.EscapeString(deck.Title)
	buf.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString("<meta charset=\"utf-8\">\n")
	buf.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	buf.WriteString("<meta name=\"color-scheme\" content=\"light dark\">\n")
	buf.WriteString("<title>" + title + "</title>\n")
	buf.WriteString("<style>\n")
	buf.WriteString(baseCSS())
	buf.WriteString("</style>\n")
	buf.WriteString("</head>\n")
	buf.WriteString("<body class=\"slides-app mode-" + html.EscapeString(opts.Mode) + " theme-" + html.EscapeString(themeClass(deck.Theme)) + "\">\n")
	buf.WriteString("<main class=\"deck-shell\" aria-label=\"Slide deck\">\n")
	for _, slide := range deck.Slides {
		buf.WriteString(renderSlide(deck, slide))
	}
	buf.WriteString("</main>\n")
	buf.WriteString(renderToolbar(deck))
	buf.WriteString("<div class=\"overview\" id=\"overview\" aria-hidden=\"true\">")
	for _, slide := range deck.Slides {
		buf.WriteString("<button class=\"overview-tile\" type=\"button\" data-goto=\"" + strconv.Itoa(slide.Index) + "\">")
		buf.WriteString("<span>" + strconv.Itoa(slide.Index+1) + "</span>")
		buf.WriteString("<strong>" + html.EscapeString(slide.Title) + "</strong>")
		buf.WriteString("</button>")
	}
	buf.WriteString("</div>\n")
	buf.WriteString("<script>window.__SLIDES_DECK__=")
	buf.Write(stateJSON)
	buf.WriteString(";</script>\n")
	buf.WriteString("<script>\n")
	buf.WriteString(runtimeJS(opts))
	buf.WriteString("</script>\n")
	buf.WriteString("</body>\n</html>\n")
	return buf.String()
}

func renderSlide(deck *Deck, slide Slide) string {
	classes := []string{"slide", "layout-" + safeClass(slide.Layout)}
	if slide.Class != "" {
		classes = append(classes, safeClass(slide.Class))
	}
	var buf bytes.Buffer
	buf.WriteString("<section class=\"" + strings.Join(classes, " ") + "\" data-slide=\"" + strconv.Itoa(slide.Index) + "\" data-clicks=\"" + strconv.Itoa(slide.Clicks) + "\" data-title=\"" + html.EscapeString(slide.Title) + "\">\n")
	buf.WriteString("<div class=\"slide-inner\">\n")
	if slide.Layout == "two-cols" {
		left, right := splitMarker(slide.Body, "::right::")
		buf.WriteString("<div class=\"cols\"><div>")
		buf.WriteString(renderMarkdown(deck, slide, left))
		buf.WriteString("</div><div>")
		buf.WriteString(renderMarkdown(deck, slide, right))
		buf.WriteString("</div></div>")
	} else if slide.Layout == "image-right" {
		left, right := splitMarker(slide.Body, "::image::")
		buf.WriteString("<div class=\"image-right\"><div>")
		buf.WriteString(renderMarkdown(deck, slide, left))
		buf.WriteString("</div><figure>")
		buf.WriteString(renderMarkdown(deck, slide, right))
		buf.WriteString("</figure></div>")
	} else {
		buf.WriteString(renderMarkdown(deck, slide, slide.Body))
	}
	buf.WriteString("\n</div>\n")
	if slide.Notes != "" {
		buf.WriteString("<aside class=\"notes\" hidden>" + renderPlainLines(slide.Notes) + "</aside>\n")
	}
	buf.WriteString("</section>\n")
	return buf.String()
}

func splitMarker(src, marker string) (string, string) {
	parts := strings.SplitN(src, marker, 2)
	if len(parts) == 1 {
		return src, ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func renderMarkdown(deck *Deck, slide Slide, src string) string {
	lines := strings.Split(src, "\n")
	var buf bytes.Buffer
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		buf.WriteString("<p>")
		buf.WriteString(renderInline(deck, slide, strings.Join(para, " ")))
		buf.WriteString("</p>\n")
		para = nil
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushPara()
			continue
		}
		if isFenceStart(trimmed) {
			flushPara()
			info := strings.TrimSpace(strings.TrimLeft(trimmed, "`~"))
			var code []string
			i++
			for ; i < len(lines); i++ {
				if isFenceStart(strings.TrimSpace(lines[i])) {
					break
				}
				code = append(code, lines[i])
			}
			buf.WriteString(renderFence(info, strings.Join(code, "\n")))
			continue
		}
		if strings.HasPrefix(trimmed, "<Steps>") {
			flushPara()
			var items []string
			i++
			for ; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "</Steps>" {
					break
				}
				if strings.TrimSpace(lines[i]) != "" {
					items = append(items, strings.TrimSpace(lines[i]))
				}
			}
			buf.WriteString("<div class=\"steps-list\">")
			for idx, item := range items {
				buf.WriteString("<div class=\"step\" data-step=\"" + strconv.Itoa(idx+1) + "\">")
				buf.WriteString(renderInline(deck, slide, item))
				buf.WriteString("</div>")
			}
			buf.WriteString("</div>\n")
			continue
		}
		if isComponentLine(trimmed, "Scene3D") {
			flushPara()
			buf.WriteString(renderScene3D(slide.Index))
			continue
		}
		if isComponentLine(trimmed, "Canvas") {
			flushPara()
			buf.WriteString(renderCanvas(slide.Index))
			continue
		}
		if isComponentLine(trimmed, "Diagram") {
			flushPara()
			buf.WriteString(renderDiagram("diagram", trimmed))
			continue
		}
		if isComponentLine(trimmed, "Agenda") {
			flushPara()
			buf.WriteString(renderAgenda(deck, slide.Index))
			continue
		}
		if strings.HasPrefix(trimmed, "<Metrics>") {
			flushPara()
			var metricLines []string
			i++
			for ; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "</Metrics>" {
					break
				}
				if strings.TrimSpace(lines[i]) != "" {
					metricLines = append(metricLines, strings.TrimSpace(lines[i]))
				}
			}
			buf.WriteString(renderMetrics(metricLines))
			continue
		}
		if strings.HasPrefix(trimmed, "<Metric") {
			flushPara()
			buf.WriteString(renderMetrics([]string{trimmed}))
			continue
		}
		if strings.HasPrefix(trimmed, "<Callout") {
			flushPara()
			buf.WriteString(renderCallout(deck, slide, trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Poll") {
			flushPara()
			buf.WriteString(renderPoll(slide.Index, trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Timeline") {
			flushPara()
			buf.WriteString(renderTimeline(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Pipeline") {
			flushPara()
			buf.WriteString(renderPipeline(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<ParseTree") {
			flushPara()
			buf.WriteString(renderParseTree(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Benchmark") {
			flushPara()
			buf.WriteString(renderBenchmark(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Citation") {
			flushPara()
			buf.WriteString(renderCitation(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Takeaway") {
			flushPara()
			buf.WriteString(renderTakeaway(deck, slide, trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<QueryDemo") {
			flushPara()
			opening := trimmed
			block := ""
			if strings.Contains(trimmed, "</QueryDemo>") {
				re := regexp.MustCompile(`(?is)<QueryDemo[^>]*>(.*?)</QueryDemo>`)
				if match := re.FindStringSubmatch(trimmed); len(match) == 2 {
					block = match[1]
				}
			} else {
				var blockLines []string
				i++
				for ; i < len(lines); i++ {
					if strings.TrimSpace(lines[i]) == "</QueryDemo>" {
						break
					}
					blockLines = append(blockLines, lines[i])
				}
				block = strings.Join(blockLines, "\n")
			}
			buf.WriteString(renderQueryDemo(deck, slide, opening, block))
			continue
		}
		if strings.HasPrefix(trimmed, "<ProfileBuckets") {
			flushPara()
			buf.WriteString(renderProfileBuckets(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<ParityMatrix") {
			flushPara()
			buf.WriteString(renderParityMatrix(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<CorpusRun") {
			flushPara()
			buf.WriteString(renderCorpusRun(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<GrammarBlob") {
			flushPara()
			buf.WriteString(renderGrammarBlob(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "<Checkpoint") {
			flushPara()
			buf.WriteString(renderCheckpoint(trimmed))
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			flushPara()
			level := headingLevel(trimmed)
			if level > 0 {
				text := strings.TrimSpace(trimmed[level:])
				buf.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, renderInline(deck, slide, text), level))
				continue
			}
		}
		if strings.HasPrefix(trimmed, ">") {
			flushPara()
			var quotes []string
			for ; i < len(lines); i++ {
				t := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(t, ">") {
					i--
					break
				}
				quotes = append(quotes, strings.TrimSpace(strings.TrimPrefix(t, ">")))
			}
			buf.WriteString("<blockquote>" + renderInline(deck, slide, strings.Join(quotes, " ")) + "</blockquote>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushPara()
			buf.WriteString("<ul>")
			for ; i < len(lines); i++ {
				t := strings.TrimSpace(lines[i])
				if !(strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ")) {
					i--
					break
				}
				buf.WriteString("<li>" + renderInline(deck, slide, strings.TrimSpace(t[2:])) + "</li>")
			}
			buf.WriteString("</ul>\n")
			continue
		}
		para = append(para, trimmed)
	}
	flushPara()
	return buf.String()
}

func renderInline(deck *Deck, slide Slide, src string) string {
	stepRe := regexp.MustCompile(`(?is)<Step\s+[^>]*\bn\s*=\s*["{]?([0-9]+)["}]?[^>]*>(.*?)</Step>`)
	var out strings.Builder
	last := 0
	matches := stepRe.FindAllStringSubmatchIndex(src, -1)
	for _, m := range matches {
		out.WriteString(renderInlineNoSteps(deck, slide, src[last:m[0]]))
		step := src[m[2]:m[3]]
		body := src[m[4]:m[5]]
		out.WriteString("<span class=\"step\" data-step=\"" + html.EscapeString(step) + "\">")
		out.WriteString(renderInline(deck, slide, body))
		out.WriteString("</span>")
		last = m[1]
	}
	out.WriteString(renderInlineNoSteps(deck, slide, src[last:]))
	return out.String()
}

func renderInlineNoSteps(deck *Deck, slide Slide, src string) string {
	escaped := html.EscapeString(src)
	escaped = replaceExpressions(deck, slide, escaped)
	escaped = regexp.MustCompile("`([^`]+)`").ReplaceAllString(escaped, "<code>$1</code>")
	escaped = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(escaped, "<em>$1</em>")
	escaped = renderImages(escaped)
	escaped = renderLinks(escaped)
	return escaped
}

func replaceExpressions(deck *Deck, slide Slide, src string) string {
	replacements := map[string]string{
		"{deck.Title}":  html.EscapeString(deck.Title),
		"{deck.Theme}":  html.EscapeString(deck.Theme),
		"{slide.Title}": html.EscapeString(slide.Title),
		"{slide.Index}": strconv.Itoa(slide.Index + 1),
		"{$step}":       `<span class="bind-step" data-bind="step">0</span>`,
	}
	for from, to := range replacements {
		src = strings.ReplaceAll(src, from, to)
	}
	expr := regexp.MustCompile(`\{[A-Za-z_.$][^{}]*\}`)
	return expr.ReplaceAllStringFunc(src, func(match string) string {
		return "`" + match + "`"
	})
}

func renderImages(src string) string {
	img := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	return img.ReplaceAllStringFunc(src, func(match string) string {
		m := img.FindStringSubmatch(match)
		if len(m) != 3 {
			return match
		}
		return `<img src="` + attrURL(m[2]) + `" alt="` + html.EscapeString(m[1]) + `">`
	})
}

func renderLinks(src string) string {
	link := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	return link.ReplaceAllStringFunc(src, func(match string) string {
		m := link.FindStringSubmatch(match)
		if len(m) != 3 {
			return match
		}
		return `<a href="` + attrURL(m[2]) + `">` + m[1] + `</a>`
	})
}

func attrURL(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "/") {
		return html.EscapeString(raw)
	}
	u := url.URL{Path: raw}
	return html.EscapeString(u.String())
}

func renderFence(info, code string) string {
	lang := strings.Fields(info)
	langName := ""
	if len(lang) > 0 {
		langName = strings.TrimSpace(lang[0])
	}
	if langName == "sirena" || langName == "mermaid" {
		return renderDiagram(langName, code)
	}
	spec := rangeSpecFromInfo(info)
	return renderCode(langName, spec, code)
}

func renderCode(lang, spec, code string) string {
	steps := ParseRangeSpec(spec)
	lines := strings.Split(code, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"code-frame\" data-step-count=\"" + strconv.Itoa(len(steps)) + "\">")
	if lang != "" {
		buf.WriteString("<figcaption>" + html.EscapeString(lang) + "</figcaption>")
	}
	buf.WriteString("<pre><code>")
	for i, line := range lines {
		n := i + 1
		buf.WriteString("<span class=\"code-line\" data-line=\"" + strconv.Itoa(n) + "\"")
		if stepList := lineSteps(n, steps); stepList != "" {
			buf.WriteString(" data-steps=\"" + html.EscapeString(stepList) + "\"")
		}
		buf.WriteString("><span class=\"line-no\">" + strconv.Itoa(n) + "</span>")
		buf.WriteString(highlightCodeLine(lang, line))
		buf.WriteString("</span>\n")
	}
	buf.WriteString("</code></pre></figure>\n")
	return buf.String()
}

func highlightCodeLine(lang, line string) string {
	escaped := html.EscapeString(line)
	if lang != "go" {
		return escaped
	}
	keywords := []string{"func", "return", "if", "else", "for", "range", "type", "struct", "package", "import", "var", "const", "map"}
	for _, keyword := range keywords {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`)
		escaped = re.ReplaceAllString(escaped, `<span class="kw">`+keyword+`</span>`)
	}
	return escaped
}

func renderDiagram(kind, src string) string {
	lines := strings.Split(strings.TrimSpace(src), "\n")
	title := kind
	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		title = strings.TrimSpace(lines[0])
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"diagram\" data-kind=\"" + html.EscapeString(kind) + "\">")
	buf.WriteString("<svg viewBox=\"0 0 900 360\" role=\"img\" aria-label=\"" + html.EscapeString(kind) + " diagram\">")
	buf.WriteString("<rect x=\"18\" y=\"18\" width=\"864\" height=\"324\" rx=\"8\" class=\"diagram-bg\"></rect>")
	buf.WriteString("<path d=\"M150 180 C260 80 410 80 520 180 S710 300 820 180\" class=\"diagram-line\"></path>")
	for i, label := range diagramLabels(lines) {
		x := 130 + i*230
		buf.WriteString("<g class=\"diagram-node\"><circle cx=\"" + strconv.Itoa(x) + "\" cy=\"180\" r=\"48\"></circle>")
		buf.WriteString("<text x=\"" + strconv.Itoa(x) + "\" y=\"186\" text-anchor=\"middle\">" + html.EscapeString(label) + "</text></g>")
	}
	buf.WriteString("</svg>")
	buf.WriteString("<figcaption>" + html.EscapeString(title) + "</figcaption>")
	buf.WriteString("</figure>\n")
	return buf.String()
}

func diagramLabels(lines []string) []string {
	var labels []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "graph") || strings.HasPrefix(line, "flowchart") {
			continue
		}
		line = strings.NewReplacer("-->", " ", "->", " ", "[", " ", "]", " ", "(", " ", ")", " ").Replace(line)
		for _, token := range strings.Fields(line) {
			token = strings.Trim(token, ";")
			if token != "" && len(labels) < 4 {
				labels = append(labels, token)
			}
		}
		if len(labels) >= 4 {
			break
		}
	}
	if len(labels) == 0 {
		return []string{"input", "model", "output"}
	}
	return labels
}

func renderScene3D(index int) string {
	return `<div class="scene3d" data-scene="` + strconv.Itoa(index) + `"><canvas aria-label="Animated 3D scene"></canvas></div>` + "\n"
}

func renderCanvas(index int) string {
	return `<div class="canvas-board" data-canvas="` + strconv.Itoa(index) + `"><canvas aria-label="Canvas board"></canvas></div>` + "\n"
}

func renderAgenda(deck *Deck, activeIndex int) string {
	var buf bytes.Buffer
	buf.WriteString("<ol class=\"agenda\">")
	for _, slide := range deck.Slides {
		buf.WriteString("<li")
		if slide.Index == activeIndex {
			buf.WriteString(" class=\"is-current\"")
		}
		buf.WriteString("><a href=\"#")
		buf.WriteString(strconv.Itoa(slide.Index + 1))
		buf.WriteString("\"><span>")
		buf.WriteString(strconv.Itoa(slide.Index + 1))
		buf.WriteString("</span><strong>")
		buf.WriteString(html.EscapeString(slide.Title))
		buf.WriteString("</strong><em>")
		buf.WriteString(html.EscapeString(slide.Layout))
		buf.WriteString("</em></a></li>")
	}
	buf.WriteString("</ol>\n")
	return buf.String()
}

func renderMetrics(lines []string) string {
	var buf bytes.Buffer
	buf.WriteString("<div class=\"metric-grid\">")
	for _, line := range lines {
		if !strings.HasPrefix(line, "<Metric") {
			continue
		}
		attrs := parseComponentAttrs(line)
		label := valueOr(attrs["label"], "Metric")
		value := valueOr(attrs["value"], attrs["number"])
		if value == "" {
			value = "0"
		}
		delta := attrs["delta"]
		buf.WriteString("<div class=\"metric\"><span>")
		buf.WriteString(html.EscapeString(label))
		buf.WriteString("</span><strong>")
		buf.WriteString(html.EscapeString(value))
		buf.WriteString("</strong>")
		if delta != "" {
			buf.WriteString("<em>" + html.EscapeString(delta) + "</em>")
		}
		buf.WriteString("</div>")
	}
	buf.WriteString("</div>\n")
	return buf.String()
}

func renderCallout(deck *Deck, slide Slide, line string) string {
	attrs := parseComponentAttrs(line)
	tone := safeClass(valueOr(attrs["tone"], "info"))
	body := attrs["body"]
	re := regexp.MustCompile(`(?is)<Callout[^>]*>(.*?)</Callout>`)
	if match := re.FindStringSubmatch(line); len(match) == 2 {
		body = strings.TrimSpace(match[1])
	}
	if body == "" {
		body = "Callout"
	}
	return `<aside class="callout" data-tone="` + html.EscapeString(tone) + `"><p>` + renderInline(deck, slide, body) + `</p></aside>` + "\n"
}

func renderPoll(slideIndex int, line string) string {
	attrs := parseComponentAttrs(line)
	question := valueOr(attrs["question"], "Choose one")
	options := splitOptions(attrs["options"])
	if len(options) == 0 {
		options = []string{"Option A", "Option B"}
	}
	id := "poll-" + strconv.Itoa(slideIndex) + "-" + strconv.Itoa(len(question))
	var buf bytes.Buffer
	buf.WriteString("<section class=\"poll\" data-poll=\"" + html.EscapeString(id) + "\"><h3>")
	buf.WriteString(html.EscapeString(question))
	buf.WriteString("</h3><div class=\"poll-options\">")
	for _, option := range options {
		buf.WriteString("<button type=\"button\" data-poll-option=\"" + html.EscapeString(option) + "\">")
		buf.WriteString("<span>" + html.EscapeString(option) + "</span><strong data-poll-count=\"\">0</strong>")
		buf.WriteString("</button>")
	}
	buf.WriteString("</div></section>\n")
	return buf.String()
}

func renderTimeline(line string) string {
	attrs := parseComponentAttrs(line)
	items := splitOptions(attrs["items"])
	if len(items) == 0 {
		items = []string{"Start", "Build", "Ship"}
	}
	var buf bytes.Buffer
	buf.WriteString("<div class=\"timeline\">")
	for _, item := range items {
		buf.WriteString("<div class=\"timeline-item\"><p>")
		buf.WriteString(html.EscapeString(item))
		buf.WriteString("</p></div>")
	}
	buf.WriteString("</div>\n")
	return buf.String()
}

func renderPipeline(line string) string {
	attrs := parseComponentAttrs(line)
	steps := splitOptions(attrs["steps"])
	if len(steps) == 0 {
		steps = []string{"Source", "Parse", "Tree", "Query"}
	}
	var buf bytes.Buffer
	buf.WriteString("<div class=\"pipeline\" role=\"list\">")
	for i, step := range steps {
		parts := strings.SplitN(step, ":", 2)
		label := strings.TrimSpace(parts[0])
		detail := ""
		if len(parts) == 2 {
			detail = strings.TrimSpace(parts[1])
		}
		buf.WriteString("<div class=\"pipeline-step\" role=\"listitem\"><span>")
		buf.WriteString(strconv.Itoa(i + 1))
		buf.WriteString("</span><strong>")
		buf.WriteString(html.EscapeString(label))
		buf.WriteString("</strong>")
		if detail != "" {
			buf.WriteString("<em>" + html.EscapeString(detail) + "</em>")
		}
		buf.WriteString("</div>")
	}
	buf.WriteString("</div>\n")
	return buf.String()
}

func renderParseTree(line string) string {
	attrs := parseComponentAttrs(line)
	root := valueOr(attrs["root"], "source_file")
	tree := attrs["tree"]
	if tree == "" {
		tree = "package_clause>package,identifier;function_declaration>func,identifier,parameters,block"
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"parse-tree\"><figcaption>")
	buf.WriteString(html.EscapeString(root))
	buf.WriteString("</figcaption><ul><li><span>")
	buf.WriteString(html.EscapeString(root))
	buf.WriteString("</span>")
	buf.WriteString(renderParseTreeChildren(tree))
	buf.WriteString("</li></ul></figure>\n")
	return buf.String()
}

func renderParseTreeChildren(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("<ul>")
	for _, branch := range strings.Split(spec, ";") {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		parent := branch
		children := ""
		if parts := strings.SplitN(branch, ">", 2); len(parts) == 2 {
			parent = strings.TrimSpace(parts[0])
			children = strings.TrimSpace(parts[1])
		}
		buf.WriteString("<li><span>")
		buf.WriteString(html.EscapeString(parent))
		buf.WriteString("</span>")
		if children != "" {
			buf.WriteString("<ul>")
			for _, child := range strings.Split(children, ",") {
				child = strings.TrimSpace(child)
				if child == "" {
					continue
				}
				buf.WriteString("<li><span>")
				buf.WriteString(html.EscapeString(child))
				buf.WriteString("</span></li>")
			}
			buf.WriteString("</ul>")
		}
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul>")
	return buf.String()
}

func renderBenchmark(line string) string {
	attrs := parseComponentAttrs(line)
	title := valueOr(attrs["title"], "Benchmark")
	values := parseBenchmarkValues(attrs["values"])
	if len(values) == 0 {
		values = []benchmarkValue{{Label: "Baseline", Value: 1}, {Label: "Current", Value: 0.75}}
	}
	maxValue := 0.0
	for _, value := range values {
		if value.Value > maxValue {
			maxValue = value.Value
		}
	}
	if maxValue <= 0 {
		maxValue = 1
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"benchmark\"><figcaption>")
	buf.WriteString(html.EscapeString(title))
	buf.WriteString("</figcaption>")
	for _, value := range values {
		width := int((value.Value / maxValue) * 100)
		if width < 3 {
			width = 3
		}
		buf.WriteString("<div class=\"benchmark-row\"><span>")
		buf.WriteString(html.EscapeString(value.Label))
		buf.WriteString("</span><div><i style=\"width:")
		buf.WriteString(strconv.Itoa(width))
		buf.WriteString("%\"></i></div><strong>")
		buf.WriteString(html.EscapeString(value.Display))
		buf.WriteString("</strong></div>")
	}
	buf.WriteString("</figure>\n")
	return buf.String()
}

type benchmarkValue struct {
	Label   string
	Value   float64
	Display string
}

func parseBenchmarkValues(src string) []benchmarkValue {
	var values []benchmarkValue
	for _, part := range splitOptions(src) {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		raw := fields[len(fields)-1]
		label := strings.TrimSpace(strings.TrimSuffix(part, raw))
		if label == "" {
			label = "value"
		}
		numeric := strings.TrimSuffix(strings.TrimSuffix(raw, "x"), "%")
		value, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			continue
		}
		values = append(values, benchmarkValue{Label: label, Value: value, Display: raw})
	}
	return values
}

func renderCitation(line string) string {
	attrs := parseComponentAttrs(line)
	href := attrs["href"]
	if href == "" {
		href = "#"
	}
	label := citationLabel(href, attrs["label"])
	return `<aside class="citation"><span>source</span><a href="` + html.EscapeString(href) + `">` + html.EscapeString(label) + `</a></aside>` + "\n"
}

func renderTakeaway(deck *Deck, slide Slide, line string) string {
	attrs := parseComponentAttrs(line)
	body := attrs["body"]
	re := regexp.MustCompile(`(?is)<Takeaway[^>]*>(.*?)</Takeaway>`)
	if match := re.FindStringSubmatch(line); len(match) == 2 {
		body = strings.TrimSpace(match[1])
	}
	if body == "" {
		body = "Takeaway"
	}
	return `<aside class="takeaway"><strong>Takeaway</strong><p>` + renderInline(deck, slide, body) + `</p></aside>` + "\n"
}

type fencedBlock struct {
	Info string
	Code string
}

func renderQueryDemo(deck *Deck, slide Slide, line, block string) string {
	attrs := parseComponentAttrs(line)
	title := valueOr(attrs["title"], "Query Demo")
	lang := valueOr(attrs["lang"], "go")
	code := attrs["code"]
	query := attrs["query"]
	blocks := extractFencedBlocks(block)
	for idx, fenced := range blocks {
		info := strings.TrimSpace(fenced.Info)
		if query == "" && (info == "query" || info == "scm" || strings.Contains(info, "query")) {
			query = fenced.Code
			continue
		}
		if code == "" {
			code = fenced.Code
			if info != "" && info != "query" && info != "scm" {
				lang = strings.Fields(info)[0]
			}
			continue
		}
		if query == "" && idx > 0 {
			query = fenced.Code
		}
	}
	if code == "" {
		code = "func main() {\n    fmt.Println(\"hello\")\n}"
	}
	if query == "" {
		query = "(function_declaration name: (identifier) @fn)"
	}
	tree := valueOr(attrs["tree"], "function_declaration>func,identifier,parameter_list,block")
	captures := splitOptions(attrs["captures"])
	if len(captures) == 0 {
		captures = []string{"@fn main"}
	}
	var buf bytes.Buffer
	buf.WriteString("<section class=\"query-demo\"><header><span>query demo</span><h3>")
	buf.WriteString(html.EscapeString(title))
	buf.WriteString("</h3></header><div class=\"query-demo-grid\"><div>")
	buf.WriteString(renderCode(lang, "", code))
	buf.WriteString("</div><div>")
	buf.WriteString(renderCode("query", "", query))
	buf.WriteString("</div><figure class=\"query-tree\"><figcaption>tree shape</figcaption><ul><li><span>source_file</span>")
	buf.WriteString(renderParseTreeChildren(tree))
	buf.WriteString("</li></ul></figure></div><div class=\"capture-pills\">")
	for _, capture := range captures {
		parts := strings.Fields(capture)
		label := capture
		value := ""
		if len(parts) > 1 {
			label = parts[0]
			value = strings.Join(parts[1:], " ")
		}
		buf.WriteString("<span><strong>")
		buf.WriteString(html.EscapeString(label))
		buf.WriteString("</strong>")
		if value != "" {
			buf.WriteString("<em>" + renderInline(deck, slide, value) + "</em>")
		}
		buf.WriteString("</span>")
	}
	buf.WriteString("</div></section>\n")
	return buf.String()
}

func extractFencedBlocks(src string) []fencedBlock {
	lines := strings.Split(src, "\n")
	var blocks []fencedBlock
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !isFenceStart(trimmed) {
			continue
		}
		info := strings.TrimSpace(strings.TrimLeft(trimmed, "`~"))
		var code []string
		i++
		for ; i < len(lines); i++ {
			if isFenceStart(strings.TrimSpace(lines[i])) {
				break
			}
			code = append(code, lines[i])
		}
		blocks = append(blocks, fencedBlock{Info: info, Code: strings.Join(code, "\n")})
	}
	return blocks
}

func renderProfileBuckets(line string) string {
	attrs := parseComponentAttrs(line)
	title := valueOr(attrs["title"], "Profile Buckets")
	values := parseBenchmarkValues(attrs["buckets"])
	if len(values) == 0 {
		values = []benchmarkValue{{Label: "parse", Value: 42, Display: "42"}, {Label: "materialize", Value: 18, Display: "18"}}
	}
	maxValue := 0.0
	for _, value := range values {
		if value.Value > maxValue {
			maxValue = value.Value
		}
	}
	if maxValue <= 0 {
		maxValue = 1
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"profile-buckets\"><figcaption>")
	buf.WriteString(html.EscapeString(title))
	buf.WriteString("</figcaption>")
	for _, value := range values {
		width := int((value.Value / maxValue) * 100)
		if width < 4 {
			width = 4
		}
		buf.WriteString("<div class=\"profile-bucket\"><span>")
		buf.WriteString(html.EscapeString(value.Label))
		buf.WriteString("</span><div><i style=\"width:")
		buf.WriteString(strconv.Itoa(width))
		buf.WriteString("%\"></i></div><strong>")
		buf.WriteString(html.EscapeString(value.Display))
		buf.WriteString("</strong></div>")
	}
	buf.WriteString("</figure>\n")
	return buf.String()
}

func renderParityMatrix(line string) string {
	attrs := parseComponentAttrs(line)
	rows := splitOptions(attrs["rows"])
	if len(rows) == 0 {
		rows = []string{"Go pass", "JavaScript pass", "Python watch", "External scanner warn"}
	}
	var buf bytes.Buffer
	buf.WriteString("<div class=\"parity-matrix\" role=\"list\">")
	for _, row := range rows {
		fields := strings.Fields(row)
		status := "watch"
		label := strings.TrimSpace(row)
		if len(fields) > 1 {
			status = strings.ToLower(fields[len(fields)-1])
			label = strings.TrimSpace(strings.TrimSuffix(row, fields[len(fields)-1]))
		}
		buf.WriteString("<div class=\"parity-cell\" data-status=\"")
		buf.WriteString(html.EscapeString(safeClass(status)))
		buf.WriteString("\" role=\"listitem\"><strong>")
		buf.WriteString(html.EscapeString(label))
		buf.WriteString("</strong><span>")
		buf.WriteString(html.EscapeString(status))
		buf.WriteString("</span></div>")
	}
	buf.WriteString("</div>\n")
	return buf.String()
}

func renderCorpusRun(line string) string {
	attrs := parseComponentAttrs(line)
	title := valueOr(attrs["title"], "Corpus Run")
	rows := splitOptions(attrs["rows"])
	if len(rows) == 0 {
		rows = []string{"JavaScript corpus 4.41x pass", "Go stdlib 1.02x pass"}
	}
	var buf bytes.Buffer
	buf.WriteString("<figure class=\"corpus-run\"><figcaption>")
	buf.WriteString(html.EscapeString(title))
	buf.WriteString("</figcaption><table><thead><tr><th>Corpus</th><th>Result</th><th>Status</th></tr></thead><tbody>")
	for _, row := range rows {
		fields := strings.Fields(row)
		label := row
		result := ""
		status := "watch"
		if len(fields) >= 3 {
			status = fields[len(fields)-1]
			result = fields[len(fields)-2]
			label = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(row, status)), result))
		}
		buf.WriteString("<tr data-status=\"")
		buf.WriteString(html.EscapeString(safeClass(status)))
		buf.WriteString("\"><td>")
		buf.WriteString(html.EscapeString(label))
		buf.WriteString("</td><td>")
		buf.WriteString(html.EscapeString(result))
		buf.WriteString("</td><td>")
		buf.WriteString(html.EscapeString(status))
		buf.WriteString("</td></tr>")
	}
	buf.WriteString("</tbody></table></figure>\n")
	return buf.String()
}

func renderGrammarBlob(line string) string {
	attrs := parseComponentAttrs(line)
	steps := splitOptions(attrs["steps"])
	if len(steps) == 0 {
		steps = []string{"parser.c", "symbol tables", "grammar blob", "Go registry"}
	}
	var buf bytes.Buffer
	buf.WriteString("<div class=\"grammar-blob\" role=\"list\">")
	for i, step := range steps {
		buf.WriteString("<div class=\"grammar-blob-step\" role=\"listitem\"><span>")
		buf.WriteString(strconv.Itoa(i + 1))
		buf.WriteString("</span><strong>")
		buf.WriteString(html.EscapeString(step))
		buf.WriteString("</strong></div>")
	}
	buf.WriteString("</div>\n")
	return buf.String()
}

func renderCheckpoint(line string) string {
	attrs := parseComponentAttrs(line)
	id := valueOr(attrs["id"], "checkpoint")
	label := valueOr(attrs["label"], id)
	return `<aside class="checkpoint" data-checkpoint-id="` + html.EscapeString(id) + `" data-checkpoint-label="` + html.EscapeString(label) + `"><span>checkpoint</span><strong>` + html.EscapeString(label) + `</strong></aside>` + "\n"
}

func splitOptions(src string) []string {
	var out []string
	for _, part := range strings.Split(src, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseComponentAttrs(line string) map[string]string {
	attrs := map[string]string{}
	re := regexp.MustCompile(`([A-Za-z_:][A-Za-z0-9_:.-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|\{([^}]*)\})`)
	for _, match := range re.FindAllStringSubmatch(line, -1) {
		if len(match) != 5 {
			continue
		}
		value := match[2]
		if value == "" {
			value = match[3]
		}
		if value == "" {
			value = match[4]
		}
		attrs[match[1]] = value
	}
	return attrs
}

func renderPlainLines(src string) string {
	var parts []string
	for _, line := range strings.Split(strings.TrimSpace(src), "\n") {
		parts = append(parts, html.EscapeString(strings.TrimSpace(line)))
	}
	return strings.Join(parts, "<br>")
}

func headingLevel(trimmed string) int {
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
		return level
	}
	return 0
}

func isComponentLine(line, name string) bool {
	return strings.HasPrefix(line, "<"+name) && (strings.HasSuffix(line, "/>") || strings.HasPrefix(line, "<"+name+">"))
}

func safeClass(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "default"
	}
	var out strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			out.WriteRune(r)
		} else if r == ' ' {
			out.WriteByte('-')
		}
	}
	if out.Len() == 0 {
		return "default"
	}
	return out.String()
}

func deckState(deck *Deck) map[string]any {
	analysis := Analyze(deck)
	slides := make([]map[string]any, 0, len(deck.Slides))
	startSecond := 0
	for _, slide := range deck.Slides {
		estimated := 0
		var citations []CitationRef
		var checkpoints []CheckpointRef
		if slide.Index < len(analysis.Slides) {
			slideAnalysis := analysis.Slides[slide.Index]
			estimated = slideAnalysis.EstimatedSeconds
			citations = slideAnalysis.Citations
			checkpoints = slideAnalysis.Checkpoints
		}
		slides = append(slides, map[string]any{
			"index":            slide.Index,
			"title":            slide.Title,
			"clicks":           slide.Clicks,
			"notes":            slide.Notes,
			"layout":           slide.Layout,
			"citations":        citations,
			"checkpoints":      checkpoints,
			"estimatedSeconds": estimated,
			"startSecond":      startSecond,
			"endSecond":        startSecond + estimated,
		})
		startSecond += estimated
	}
	return map[string]any{
		"title":                 deck.Title,
		"theme":                 deck.Theme,
		"estimatedTotalSeconds": analysis.EstimatedSeconds,
		"citations":             analysis.Citations,
		"checkpoints":           analysis.Checkpoints,
		"slides":                slides,
	}
}

func publicPath(deck *Deck, rel string) string {
	if deck.BaseDir == "" {
		return rel
	}
	return filepath.Join(deck.BaseDir, rel)
}

package slides

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
	"m31labs.dev/mdpp"
)

const migrationSourceLimit = 2 << 20
const migrationAssetLimit = 8 << 20
const migrationTotalAssetLimit = 64 << 20

type MigrationOptions struct {
	Format string // slidev, marp or quarto; explicit to avoid guessing ambiguous Markdown
}

type MigrationSlideOrigin struct {
	Slide int        `json:"slide"` // zero-based destination slide
	Range mdpp.Range `json:"range"` // bytes in the unchanged original, including CRLF
}

type MarkdownMigrationReport struct {
	Version     int                    `json:"version"`
	Format      string                 `json:"format"`
	Source      string                 `json:"source"` // original basename; no private absolute path in the deck
	SHA256      string                 `json:"sha256"`
	Destination string                 `json:"destination"`
	Slides      int                    `json:"slides"`
	Assets      int                    `json:"assets"`
	Origins     []MigrationSlideOrigin `json:"origins"`
	Diagnostics []SourceDiagnostic     `json:"diagnostics"`
}

type markdownMigrator struct {
	format, root, file string
	original, source   []byte
	removedCR          []int
	report             MarkdownMigrationReport
	assets             map[string][]byte
	assetPaths         map[string]string
	assetBytes         int
	allowed            func(string) bool
	deckMeta           map[string]any
	defaults           map[string]any
	slideLevel         int
}

type migrationSlide struct {
	body       strings.Builder
	notes      []string
	meta       map[string]any
	start, end int
}

// MigrateMarkdown imports a bounded Markdown presentation without executing
// Vue, Quarto cells, expressions, includes, package managers or source code.
// Supported content is rewritten through mdpp's parsed node ranges. Losses are
// explicit; the exact original and report remain private under migration/.
// Only referenced, approved local media is copied; external URLs are never read.
func MigrateMarkdown(source, destination string, opts MigrationOptions) (MarkdownMigrationReport, error) {
	report := MarkdownMigrationReport{Version: 1, Diagnostics: []SourceDiagnostic{}, Origins: []MigrationSlideOrigin{}}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format != "slidev" && format != "marp" && format != "quarto" {
		return report, fmt.Errorf("migration format must be slidev, marp or quarto")
	}
	path, err := filepath.Abs(source)
	if err != nil {
		return report, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return report, fmt.Errorf("migration source must be a regular file without a symlink")
	}
	original, err := migrationRead(path, migrationSourceLimit)
	if err != nil {
		return report, err
	}
	if !utf8.Valid(original) {
		return report, fmt.Errorf("migration source must be UTF-8")
	}
	doc, err := mdpp.Parse(original)
	if err != nil {
		return report, err
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return report, err
	}
	m := &markdownMigrator{format: format, root: root, file: filepath.Base(path), original: original, source: doc.Source, removedCR: markdownCRPositions(original), assets: map[string][]byte{}, assetPaths: map[string]string{}, deckMeta: map[string]any{"theme": "paper", "transition": "none"}, defaults: map[string]any{}, slideLevel: 2, allowed: exportPublicPolicy(root)}
	m.report = report
	m.report.Format, m.report.Source = format, m.file
	m.warn(mdpp.Range{}, "MIGRATION-PRESENTATION", "Content was migrated to native layouts; source-engine typography, placement, transitions, plugins and runtime behavior require author review.")
	hash := sha256.Sum256(original)
	m.report.SHA256 = hex.EncodeToString(hash[:])
	for _, diagnostic := range doc.Diagnostics() {
		if diagnostic.Code == "MDPP-PARSE-005" {
			return report, fmt.Errorf("migration parser budget exceeded")
		}
		m.warn(diagnostic.Range, "MIGRATION-PARSE", diagnostic.Message)
	}
	content, err := m.convert(doc)
	if err != nil {
		return m.report, err
	}
	if len(content) > maxSourceBytes {
		return m.report, fmt.Errorf("generated deck exceeds the editor limit of %d bytes", maxSourceBytes)
	}
	// Check the resulting grammar before publishing. In particular, imported
	// interpolation or component syntax must not become a GoSX program.
	check, err := mdpp.Parse(content)
	if err != nil {
		return m.report, err
	}
	unsafe := false
	check.AST().Walk(func(n *mdpp.Node) bool {
		if n.Type == mdpp.NodeExpression || n.Type == mdpp.NodeComponent {
			unsafe = true
		}
		return true
	})
	if unsafe {
		return m.report, fmt.Errorf("migration could not neutralize executable authoring syntax")
	}
	if len(check.Slides()) != m.report.Slides {
		return m.report, fmt.Errorf("migration produced ambiguous slide boundaries")
	}
	stage, dest, err := adoptionStage(destination)
	if err != nil {
		return m.report, err
	}
	defer os.RemoveAll(stage)
	m.report.Destination = dest
	m.report.Assets = len(m.assets)
	files := map[string][]byte{DeckFileName: content, "go.mod": []byte(realLaneGoMod(dest)), ".gitignore": []byte(realLaneGitignore + "\n# Originals can contain private speaker notes.\nmigration/\n"), "README.md": []byte("# Migrated presentation\n\nRun `slides serve .`. Review every diagnostic in `migration/report.json` and compare with the unchanged `migration/source.md` before presenting.\n\nMigration preserves supported content, not source-engine layout or runtime behavior. Speaker notes remain private unless explicitly exported with --notes. Original source and provenance are kept outside public/ and ignored by Git.\n")}
	for name, data := range m.assets {
		files[name] = data
	}
	for _, name := range adoptionSortedKeys(files) {
		if err := adoptionWrite(stage, name, files[name], 0644); err != nil {
			return m.report, err
		}
	}
	if err := os.Mkdir(filepath.Join(stage, "migration"), 0700); err != nil {
		return m.report, err
	}
	if err := adoptionWrite(stage, "migration/source.md", original, 0600); err != nil {
		return m.report, err
	}
	payload, err := json.MarshalIndent(m.report, "", "  ")
	if err != nil {
		return m.report, err
	}
	if err := adoptionWrite(stage, "migration/report.json", append(payload, '\n'), 0600); err != nil {
		return m.report, err
	}
	if _, err := LoadIslandDeck(stage); err != nil {
		return m.report, fmt.Errorf("validate migrated deck: %w", err)
	}
	if err := adoptionPublish(stage, dest); err != nil {
		return m.report, err
	}
	return m.report, nil
}

func migrationRead(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("migration file %s exceeds %d bytes", filepath.Base(path), limit)
	}
	return content, nil
}

func (m *markdownMigrator) originalRange(r mdpp.Range) mdpp.Range {
	r.StartByte = originalMarkdownOffset(m.removedCR, r.StartByte)
	r.EndByte = originalMarkdownOffset(m.removedCR, r.EndByte)
	r.StartLine, r.StartCol = sourcePosition(m.original, r.StartByte)
	r.EndLine, r.EndCol = sourcePosition(m.original, r.EndByte)
	return r
}

func (m *markdownMigrator) warn(r mdpp.Range, code, message string) {
	if len(m.report.Diagnostics) >= 500 {
		m.report.Diagnostics[499] = SourceDiagnostic{Code: "MIGRATION-DIAGNOSTICS", Severity: "warning", Message: "Further diagnostics were omitted after the 500-entry budget; compare the complete preserved original.", Range: m.originalRange(r), File: m.file}
		return
	}
	if len(message) > 512 {
		message = message[:512]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
		message += "…"
	}
	m.report.Diagnostics = append(m.report.Diagnostics, SourceDiagnostic{Code: code, Severity: "warning", Message: message, Range: m.originalRange(r), File: m.file})
}

func (m *markdownMigrator) raw(n *mdpp.Node) string {
	return string(m.source[n.Range.StartByte:n.Range.EndByte])
}

func (m *markdownMigrator) convert(doc *mdpp.Document) ([]byte, error) {
	nodes := doc.AST().Children
	if len(nodes) > 0 && nodes[0].Type == mdpp.NodeFrontmatter {
		if len(nodes[0].Literal) > 65536 {
			return nil, fmt.Errorf("migration metadata exceeds 64 KiB")
		}
		m.metadata(doc.Frontmatter(), m.defaults, nodes[0].Range, true)
		nodes = nodes[1:]
	}
	var slides []*migrationSlide
	newSlide := func() *migrationSlide {
		meta := map[string]any{}
		for k, v := range m.defaults {
			meta[k] = v
		}
		return &migrationSlide{meta: meta, start: -1}
	}
	current := newSlide()
	finish := func() {
		if strings.TrimSpace(current.body.String()) == "" && len(current.notes) == 0 {
			return
		}
		slides = append(slides, current)
		current = newSlide()
	}
	if m.format == "quarto" {
		if title, ok := m.deckMeta["title"].(string); ok && title != "" {
			current.meta["layout"] = "title"
			current.body.WriteString("<h1>" + html.EscapeString(title) + "</h1>\n\n")
			for _, key := range []string{"subtitle", "author", "date"} {
				if value, ok := doc.Frontmatter()[key].(string); ok {
					current.body.WriteString("<p>" + html.EscapeString(value) + "</p>\n\n")
				}
			}
			current.start, current.end = 0, 0
			finish()
		}
	}
	var regions []string
	for i, n := range nodes {
		if len(slides) >= 1000 {
			return nil, fmt.Errorf("migration exceeds 1000 slides")
		}
		raw := strings.TrimSpace(m.raw(n))
		if m.format == "quarto" && n.Type == mdpp.NodeParagraph {
			if strings.HasPrefix(raw, "::: {") && strings.HasSuffix(raw, "}") {
				kind := "unsupported"
				attributes := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(raw, "::: {"), "}"))
				for _, value := range attributes {
					if value == ".notes" {
						kind = "notes"
					}
					if value == ".incremental" {
						if kind != "notes" {
							kind = "incremental"
						}
					}
				}
				if len(regions) >= 16 {
					return nil, fmt.Errorf("migration directive nesting exceeds 16")
				}
				regions = append(regions, kind)
				if kind == "incremental" {
					current.meta["reveal"] = true
					m.warn(n.Range, "MIGRATION-FRAGMENTS", "Incremental lists use native linear reveals; the first item is visible initially.")
				}
				if kind == "unsupported" {
					m.warn(n.Range, "MIGRATION-DIRECTIVE", "Unsupported Quarto div attributes were omitted; content was retained.")
				}
				continue
			}
			if raw == ":::" {
				if len(regions) > 0 {
					regions = regions[:len(regions)-1]
				} else {
					m.warn(n.Range, "MIGRATION-DIRECTIVE", "Unmatched Quarto div terminator omitted.")
				}
				continue
			}
		}
		private := false
		for _, region := range regions {
			if region == "notes" {
				private = true
			}
		}
		if private {
			if current.start < 0 {
				current.start = n.Range.StartByte
			}
			current.end = n.Range.EndByte
			current.notes = append(current.notes, m.raw(n))
			continue
		}
		if n.Type == mdpp.NodeThematicBreak {
			finish()
			// Slidev's --- / YAML / --- is a setext heading in CommonMark.
			// Recognize only a grammar-classified level-2 heading containing a
			// valid YAML mapping, immediately after a parsed thematic break.
			if m.format == "slidev" && i+1 < len(nodes) {
				candidate := nodes[i+1]
				if values, ok := m.slidevMetadata(candidate); ok {
					m.metadata(values, current.meta, candidate.Range, false)
				}
			}
			continue
		}
		if m.format == "slidev" && i > 0 && nodes[i-1].Type == mdpp.NodeThematicBreak {
			if _, ok := m.slidevMetadata(n); ok {
				continue
			}
		}
		if m.format == "quarto" && n.Type == mdpp.NodeHeading {
			level, _ := strconv.Atoi(n.Attr("level"))
			if level <= m.slideLevel {
				finish()
				if level < m.slideLevel {
					current.meta["layout"] = "section"
				}
			}
		}
		if current.start < 0 {
			current.start = n.Range.StartByte
		}
		current.end = n.Range.EndByte
		if n.Type == mdpp.NodeHTMLBlock || n.Type == mdpp.NodeHTMLInline {
			if strings.HasPrefix(raw, "<!--") && strings.HasSuffix(raw, "-->") {
				comment := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "<!--"), "-->"))
				if m.format == "marp" && m.marpDirective(comment, current, n.Range) {
					continue
				}
				current.notes = append(current.notes, comment)
				continue
			}
			if m.format == "slidev" && (raw == "<v-clicks>" || raw == "</v-clicks>") {
				current.meta["reveal"] = true
				if raw == "<v-clicks>" {
					m.warn(n.Range, "MIGRATION-FRAGMENTS", "v-clicks lists use native linear reveals; the first item is visible initially.")
				}
				continue
			}
		}
		current.body.WriteString(strings.TrimSpace(m.node(n, current)) + "\n\n")
	}
	if len(regions) > 0 {
		m.warn(mdpp.Range{StartByte: len(m.source), EndByte: len(m.source)}, "MIGRATION-DIRECTIVE", "Unclosed Quarto div; review its preserved content and notes.")
	}
	finish()
	if len(slides) == 0 {
		return nil, fmt.Errorf("migration source contains no presentation content")
	}
	if _, ok := m.deckMeta["title"]; !ok {
		m.deckMeta["title"] = strings.TrimSuffix(m.file, filepath.Ext(m.file))
	}
	head, err := yaml.Marshal(m.deckMeta)
	if err != nil {
		return nil, err
	}
	var output strings.Builder
	output.WriteString("---\n" + string(head) + "---\n\n")
	for i, slide := range slides {
		if i > 0 {
			output.WriteString("\n---\n\n")
		}
		if _, ok := slide.meta["id"]; !ok {
			slide.meta["id"] = fmt.Sprintf("slide-%d", i+1)
		}
		meta, err := yaml.Marshal(slide.meta)
		if err != nil {
			return nil, err
		}
		output.WriteString("```yaml\n" + string(meta) + "```\n\n" + slide.body.String())
		// A trailing comment both forces a clean mdpp slide boundary and keeps
		// all gathered notes private, even when source notes were mid-slide.
		notes := strings.Join(slide.notes, "\n\n")
		if strings.Contains(notes, "slides:") {
			notes = strings.ReplaceAll(notes, "slides:", "slides :")
			m.warn(mdpp.Range{StartByte: max(slide.start, 0), EndByte: slide.end}, "MIGRATION-INCLUDE", "Source include-like comments were retained as inert speaker notes.")
		}
		notes = strings.ReplaceAll(notes, "-->", "-- >")
		output.WriteString("<!--\n" + notes + "\n-->\n")
		m.report.Origins = append(m.report.Origins, MigrationSlideOrigin{i, m.originalRange(mdpp.Range{StartByte: max(slide.start, 0), EndByte: slide.end})})
	}
	m.report.Slides = len(slides)
	return []byte(output.String()), nil
}

func (m *markdownMigrator) slidevMetadata(n *mdpp.Node) (map[string]any, bool) {
	if n.Type != mdpp.NodeHeading || n.Attr("level") != "2" {
		return nil, false
	}
	raw := strings.TrimSpace(m.raw(n))
	pos := strings.LastIndex(raw, "\n")
	if pos < 0 || strings.TrimSpace(raw[pos+1:]) != "---" {
		return nil, false
	}
	var values map[string]any
	if len(raw) > 65536 || yaml.Unmarshal([]byte(raw[:pos]), &values) != nil || len(values) == 0 {
		return nil, false
	}
	return values, true
}

func (m *markdownMigrator) metadata(values map[string]any, slide map[string]any, r mdpp.Range, deck bool) {
	for _, key := range adoptionSortedKeys(values) {
		value := values[key]
		switch key {
		case "title":
			if deck {
				if text, ok := value.(string); ok {
					m.deckMeta["title"] = text
				}
			}
		case "subtitle", "author", "date", "marp":
			if !deck || m.format != "quarto" && key != "marp" {
				m.warn(r, "MIGRATION-METADATA", "Metadata "+key+" was retained only in the original source.")
			}
		case "theme":
			if deck {
				if text, ok := value.(string); ok && isRealLaneTheme(text) {
					m.deckMeta["theme"] = text
				} else {
					m.warn(r, "MIGRATION-THEME", "Source theme was replaced by paper; source-engine CSS and theme packages are not loaded.")
				}
			}
		case "layout":
			text, _ := value.(string)
			switch text {
			case "cover", "intro":
				slide["layout"] = "title"
			case "end":
				slide["layout"] = "center"
			case "default", "center", "title", "quote", "section", "two-cols", "split", "full":
				slide["layout"] = text
			default:
				m.warn(r, "MIGRATION-LAYOUT", "Unsupported source layout "+text+" uses the default layout.")
			}
		case "class", "_class":
			if text, ok := value.(string); ok {
				slide["class"] = text
				if m.format == "marp" && text == "lead" {
					slide["layout"] = "title"
				} else {
					m.warn(r, "MIGRATION-CLASS", "Class names were retained, but source-engine class styling was not imported.")
				}
			}
		case "backgroundColor", "_backgroundColor":
			if text, ok := value.(string); ok {
				slide["background"] = text
			}
		case "header", "footer", "_header", "_footer":
			if text, ok := value.(string); ok {
				if deck {
					m.deckMeta[strings.TrimPrefix(key, "_")] = text
				} else {
					slide[strings.TrimPrefix(key, "_")] = text
				}
			}
		case "size", "aspectRatio", "aspect-ratio":
			text := strings.ReplaceAll(fmt.Sprint(value), "/", ":")
			if _, _, err := exportSize(nil, ExportOptions{Aspect: text}); err == nil {
				m.deckMeta["aspect-ratio"] = text
			} else {
				m.warn(r, "MIGRATION-ASPECT", "Unsupported source aspect ratio uses 16:9.")
			}
		case "line-numbers":
			if enabled, ok := value.(bool); ok {
				m.deckMeta[key] = enabled
			}
		case "incremental":
			if enabled, ok := value.(bool); ok {
				slide["reveal"] = enabled
			}
		case "format":
			if m.format == "quarto" {
				if formats, ok := value.(map[string]any); ok {
					if options, ok := formats["revealjs"].(map[string]any); ok {
						for k, v := range options {
							if k == "slide-level" {
								if level, ok := v.(int); ok && level >= 1 && level <= 6 {
									m.slideLevel = level
								}
							} else if k == "incremental" {
								m.defaults["reveal"] = v
							} else {
								m.warn(r, "MIGRATION-METADATA", "Quarto revealjs option "+k+" is not supported.")
							}
						}
					}
				}
			} else {
				m.warn(r, "MIGRATION-METADATA", "Source format options were not imported.")
			}
		default:
			m.warn(r, "MIGRATION-METADATA", "Metadata "+key+" is not supported; the original source retains it.")
		}
	}
}

func (m *markdownMigrator) marpDirective(comment string, slide *migrationSlide, r mdpp.Range) bool {
	var values map[string]any
	if len(comment) > 65536 || yaml.Unmarshal([]byte(comment), &values) != nil {
		return false
	}
	known := false
	for key := range values {
		switch key {
		case "class", "_class", "backgroundColor", "_backgroundColor", "header", "_header", "footer", "_footer", "paginate", "_paginate", "theme", "size", "color", "_color":
			known = true
		}
	}
	if !known {
		return false
	}
	m.metadata(values, slide.meta, r, false)
	return true
}

type migrationEdit struct {
	start, end int
	text       string
}

func (m *markdownMigrator) node(n *mdpp.Node, slide *migrationSlide) string {
	if n.Type == mdpp.NodeCodeBlock || n.Type == mdpp.NodeDiagram {
		language := n.Attr("language")
		highlights := n.Attr("highlights")
		if m.format == "quarto" {
			first := strings.TrimSpace(strings.SplitN(m.raw(n), "\n", 2)[0])
			info := strings.TrimLeft(first, "`~")
			if strings.HasPrefix(info, "{") {
				tokens := strings.Fields(strings.Trim(info, "{} "))
				if len(tokens) > 0 {
					language = strings.TrimSuffix(tokens[0], ",")
				}
				highlights = ""
				m.warn(n.Range, "MIGRATION-EXECUTION", "Executable Quarto cell was converted to displayed source; computation and generated output were not run.")
			}
		}
		if language == "" && n.Type == mdpp.NodeDiagram {
			language = n.Attr("kind")
		}
		if !migrationLanguage(language) {
			language = "text"
			m.warn(n.Range, "MIGRATION-CODE", "Unsupported code-fence options were omitted.")
		}
		if highlights != "" && !migrationHighlights(highlights) {
			highlights = ""
			m.warn(n.Range, "MIGRATION-HIGHLIGHTS", "Unsupported code highlight specification was omitted.")
		}
		info := language
		if highlights != "" {
			info += " {" + highlights + "}"
		}
		return migrationFence(n.Literal, info)
	}
	var edits []migrationEdit
	var walk func(*mdpp.Node)
	walk = func(child *mdpp.Node) {
		replacement := ""
		replace := false
		switch child.Type {
		case mdpp.NodeExpression, mdpp.NodeComponent:
			replacement = migrationInline(m.raw(child))
			replace = true
			if m.format == "quarto" && n.Type == mdpp.NodeHeading && child.Type == mdpp.NodeExpression {
				tokens := strings.Fields(child.Literal)
				if len(tokens) > 0 && strings.HasPrefix(tokens[0], "#") && migrationID(tokens[0][1:]) {
					slide.meta["id"] = tokens[0][1:]
					replacement = ""
					if len(tokens) > 1 {
						m.warn(child.Range, "MIGRATION-CLASS", "Heading attributes beyond the stable ID were omitted.")
					}
				}
			}
			if replacement != "" {
				m.warn(child.Range, "MIGRATION-RUNTIME", "Source expression/component was retained as literal code; Vue and GoSX execution is disabled.")
			}
		case mdpp.NodeImage, mdpp.NodeLink:
			value := child.Attr("href")
			if child.Type == mdpp.NodeImage {
				value = child.Attr("src")
			}
			if rewritten, changed := m.assetURL(value, child.Range); changed {
				label := child.Attr("alt")
				prefix := "!"
				if child.Type == mdpp.NodeLink {
					prefix = ""
					label = ""
					for _, part := range child.Children {
						label += m.node(part, slide)
					}
				}
				label = strings.ReplaceAll(strings.ReplaceAll(label, "[", "\\["), "]", "\\]")
				replacement = prefix + "[" + label + "](" + rewritten + ")"
				replace = true
			}
		case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline:
			replacement = m.html(child, slide)
			replace = true
		case mdpp.NodeCodeSpan, mdpp.NodeCodeBlock, mdpp.NodeDiagram:
			return
		}
		if replace {
			edits = append(edits, migrationEdit{child.Range.StartByte, child.Range.EndByte, replacement})
			return
		}
		for _, nested := range child.Children {
			walk(nested)
		}
	}
	walk(n)
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var output strings.Builder
	position := n.Range.StartByte
	for _, edit := range edits {
		if edit.start < position {
			continue
		}
		output.Write(m.source[position:edit.start])
		output.WriteString(edit.text)
		position = edit.end
	}
	output.Write(m.source[position:n.Range.EndByte])
	return output.String()
}

func migrationInline(source string) string {
	width := 1
	run := 0
	for _, r := range source {
		if r == '`' {
			run++
			width = max(width, run+1)
		} else {
			run = 0
		}
	}
	mark := strings.Repeat("`", width)
	return mark + " " + strings.ReplaceAll(source, "\n", " ") + " " + mark
}
func migrationFence(source, info string) string {
	width := 3
	run := 0
	for _, r := range source {
		if r == '`' {
			run++
			width = max(width, run+1)
		} else {
			run = 0
		}
	}
	mark := strings.Repeat("`", width)
	return mark + info + "\n" + strings.TrimSuffix(source, "\n") + "\n" + mark
}
func migrationLanguage(language string) bool {
	for _, r := range language {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '+' || r == '.') {
			return false
		}
	}
	return len(language) <= 64
}
func migrationHighlights(value string) bool {
	if len(value) > 256 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r == '-' || r == ',' || r == '|' || r == ' ') {
			return false
		}
	}
	return true
}
func migrationID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (m *markdownMigrator) html(n *mdpp.Node, slide *migrationSlide) string {
	raw := m.raw(n)
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var output strings.Builder
	literal := false
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if kind == html.CommentToken {
			slide.notes = append(slide.notes, token.Data)
			continue
		}
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken || kind == html.EndTagToken {
			wire := strings.TrimLeft(string(tokenizer.Raw()), "</ ")
			unsupported := len(wire) > 0 && wire[0] >= 'A' && wire[0] <= 'Z' || strings.HasPrefix(token.Data, "v-") || rawHTMLDroppedSubtrees[token.Data] || token.Data == "style"
			for _, attr := range token.Attr {
				if strings.HasPrefix(attr.Key, "v-") || strings.HasPrefix(attr.Key, ":") || strings.HasPrefix(attr.Key, "@") || strings.HasPrefix(attr.Key, "on") {
					unsupported = true
				}
			}
			if unsupported {
				literal = true
			}
			for i := range token.Attr {
				attr := &token.Attr[i]
				if attr.Key == "src" || attr.Key == "poster" || attr.Key == "href" {
					if value, changed := m.assetURL(attr.Val, n.Range); changed {
						attr.Val = value
					}
				}
				if attr.Key == "srcset" || attr.Key == "style" {
					attr.Val = ""
					m.warn(n.Range, "MIGRATION-HTML", "Inline CSS/srcset was omitted; local URLs in these attributes require author review.")
				}
			}
		}
		output.WriteString(token.String())
	}
	if literal {
		m.warn(n.Range, "MIGRATION-HTML", "Runtime components, scripts, styles or bindings were retained as literal source; comments remain private notes.")
		if n.Type == mdpp.NodeHTMLInline {
			return migrationInline(output.String())
		}
		return migrationFence(output.String(), "html")
	}
	return output.String()
}

func (m *markdownMigrator) assetURL(value string, r mdpp.Range) (string, bool) {
	parsed, err := url.Parse(value)
	if err != nil {
		m.warn(r, "MIGRATION-ASSET", "Invalid asset URL was disabled.")
		return "#", true
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(value, "#") || value == "" {
		if parsed.Host != "" || parsed.Scheme == "http" || parsed.Scheme == "https" {
			m.warn(r, "MIGRATION-REMOTE", "External URL was retained without fetching; offline availability is not guaranteed.")
		} else if parsed.Scheme != "" && parsed.Scheme != "mailto" && parsed.Scheme != "tel" {
			m.warn(r, "MIGRATION-ASSET", "Unsupported URL scheme was disabled.")
			return "#", true
		}
		return value, false
	}
	relative := strings.TrimPrefix(parsed.Path, "/")
	if m.format == "slidev" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(relative, "public/") {
		relative = "public/" + relative
	}
	if !safeDeckRelPath(relative) || strings.Contains(relative, "\\") {
		m.warn(r, "MIGRATION-ASSET", "Escaping, missing or private local resource was disabled: "+value)
		return "#", true
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	for _, part := range strings.Split(relative, "/") {
		if strings.HasPrefix(part, ".") {
			m.warn(r, "MIGRATION-ASSET", "Hidden local resource was disabled: "+value)
			return "#", true
		}
	}
	ext := strings.ToLower(filepath.Ext(relative))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg", ".mp4", ".webm", ".mp3", ".wav", ".ogg", ".pdf", ".woff", ".woff2", ".ttf", ".otf":
	default:
		m.warn(r, "MIGRATION-ASSET", "Local resource type is not supported: "+value)
		return "#", true
	}
	if target, ok := m.assetPaths[relative]; ok {
		parsed.Path = target
		return parsed.String(), true
	}
	path := m.root
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(relative)), "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			m.warn(r, "MIGRATION-ASSET", "Missing or symlinked local resource was disabled: "+value)
			return "#", true
		}
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		m.warn(r, "MIGRATION-ASSET", "Nonregular local resource was disabled: "+value)
		return "#", true
	}
	if !m.allowed(filepath.FromSlash(relative)) {
		m.warn(r, "MIGRATION-ASSET", "Private local resource was disabled: "+value)
		return "#", true
	}
	content, err := migrationRead(path, migrationAssetLimit)
	if err != nil || len(m.assets) >= 256 || m.assetBytes+len(content) > migrationTotalAssetLimit {
		m.warn(r, "MIGRATION-ASSET", "Local resource exceeds migration limits: "+value)
		return "#", true
	}
	if ext == ".svg" && !migrationStaticSVG(content) {
		m.warn(r, "MIGRATION-ASSET", "SVG outside the static geometry allowlist (styles, scripts, external references or invalid XML) was disabled: "+value)
		return "#", true
	}
	hash := sha256.Sum256(content)
	name := "public/assets/" + hex.EncodeToString(hash[:]) + ext
	if _, ok := m.assets[name]; !ok {
		m.assets[name] = content
		m.assetBytes += len(content)
	}
	m.assetPaths[relative] = "/" + name
	parsed.Path = "/" + name
	return parsed.String(), true
}

func adoptionSortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

package slides

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"m31labs.dev/mdpp"
)

// SourceLocation identifies original author bytes, never generated content.
type SourceLocation struct {
	File      string `json:"file"`
	StartByte int    `json:"startByte"`
	EndByte   int    `json:"endByte"`
}

type sourceSegment struct {
	start, end, original int
	file                 string
}

// SourceLocation maps a contiguous expanded range to its author file.
// Ranges crossing an include boundary deliberately have no single location.
func (d *IslandDeck) SourceLocation(start, end int) (SourceLocation, bool) {
	if d == nil || start < 0 || end < start {
		return SourceLocation{}, false
	}
	start, end = originalMarkdownOffset(d.sourceCRPositions, start), originalMarkdownOffset(d.sourceCRPositions, end)
	if len(d.sourceSegments) == 0 {
		if end <= len(d.Source) {
			return SourceLocation{DeckFileName, start, end}, true
		}
		return SourceLocation{}, false
	}
	for _, seg := range d.sourceSegments {
		if start >= seg.start && start < seg.end && end <= seg.end {
			return SourceLocation{seg.file, seg.original + start - seg.start, seg.original + end - seg.start}, true
		}
	}
	return SourceLocation{}, false
}

func (d *IslandDeck) rootSourceRange(start, end int) (int, int, bool) {
	where, ok := d.SourceLocation(start, end)
	return where.StartByte, where.EndByte, ok && where.File == DeckFileName
}

var includeComment = regexp.MustCompile(`^<!--\s*slides:include\s+(.+?)\s*-->$`)
var sectionComment = regexp.MustCompile(`^<!--\s*slides:section\s+([A-Za-z][A-Za-z0-9_-]*)\s*-->$`)

// HasDeckIncludes checks parsed block comments, leaving literal code untouched.
// Editing tools can use it to avoid incomplete single-file semantic renames.
func HasDeckIncludes(src []byte) (bool, error) {
	comments, err := markdownComments(src)
	if err != nil {
		return false, err
	}
	for _, n := range comments {
		trimmed := strings.TrimSpace(n.Literal)
		if includeComment.MatchString(trimmed) {
			return true, nil
		}
		if strings.HasPrefix(trimmed, "<!-- slides:include") {
			return false, fmt.Errorf("invalid slides:include comment")
		}
	}
	return false, nil
}

type composition struct {
	dir      string
	segments []sourceSegment
	files    []string
	out      []byte
	active   map[string]bool
}

type compositionDirective struct {
	start, end int
	target     string
}

func markdownComments(src []byte) ([]*mdpp.Node, error) {
	doc, err := mdpp.Parse(src)
	if err != nil {
		return nil, err
	}
	var comments []*mdpp.Node
	removed := markdownCRPositions(src)
	doc.AST().Walk(func(n *mdpp.Node) bool {
		if n.Type == mdpp.NodeHTMLBlock && strings.HasPrefix(strings.TrimSpace(n.Literal), "<!--") {
			original := *n
			original.Range.StartByte = originalMarkdownOffset(removed, n.Range.StartByte)
			original.Range.EndByte = originalMarkdownOffset(removed, n.Range.EndByte)
			comments = append(comments, &original)
		}
		return true
	})
	return comments, nil
}

// mdpp changes CRLF to LF before assigning ranges. Retain a sparse list of
// removed CR offsets in normalized coordinates to recover original author bytes.
// Lone CR becomes LF without changing byte length.
func markdownCRPositions(src []byte) []int {
	var removed []int
	for i := 0; i+1 < len(src); i++ {
		if src[i] == '\r' && src[i+1] == '\n' {
			removed = append(removed, i-len(removed))
		}
	}
	return removed
}

func originalMarkdownOffset(removed []int, offset int) int {
	return offset + sort.Search(len(removed), func(i int) bool { return removed[i] > offset })
}

func (c *composition) append(src []byte, file string, original int) error {
	if len(c.out)+len(src) > 8<<20 {
		return fmt.Errorf("composed deck exceeds 8 MiB")
	}
	start := len(c.out)
	c.out = append(c.out, src...)
	if len(src) > 0 {
		c.segments = append(c.segments, sourceSegment{start, len(c.out), original, file})
	}
	return nil
}

func (c *composition) expand(file string, src []byte, original, depth int) error {
	if depth > 32 {
		return fmt.Errorf("include nesting exceeds 32 levels at %s", file)
	}
	if c.active[file] {
		return fmt.Errorf("include cycle at %s", file)
	}
	c.active[file] = true
	defer delete(c.active, file)
	comments, err := markdownComments(src)
	if err != nil {
		return fmt.Errorf("parse include %s: %w", file, err)
	}
	var directives []compositionDirective
	for _, n := range comments {
		trimmed := strings.TrimSpace(n.Literal)
		m := includeComment.FindStringSubmatch(trimmed)
		if m == nil {
			if strings.HasPrefix(trimmed, "<!-- slides:include") {
				return fmt.Errorf("%s: invalid slides:include comment", file)
			}
			continue
		}
		start, end := n.Range.StartByte, n.Range.EndByte
		if start < 0 || end > len(src) || end < start {
			return fmt.Errorf("invalid include source range in %s", file)
		}
		// mdpp may include the trailing newline in a block range. Replace only
		// the comment itself so author whitespace remains source mapped.
		trim := strings.TrimSpace(n.Literal)
		at := bytes.Index(src[start:end], []byte(trim))
		if at < 0 {
			return fmt.Errorf("invalid include source in %s", file)
		}
		directives = append(directives, compositionDirective{start + at, start + at + len(trim), m[1]})
	}
	sort.Slice(directives, func(i, j int) bool { return directives[i].start < directives[j].start })
	pos := 0
	for _, include := range directives {
		if include.start < pos {
			continue
		}
		if err := c.append(src[pos:include.start], file, original+pos); err != nil {
			return err
		}
		target := strings.TrimSpace(include.target)
		if strings.HasPrefix(target, `"`) {
			target, err = strconv.Unquote(target)
			if err != nil {
				return fmt.Errorf("%s: invalid quoted include: %w", file, err)
			}
		}
		path, section, selected := strings.Cut(target, "#")
		if selected && section == "" {
			return fmt.Errorf("%s: include section name is empty", file)
		}
		if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
			return fmt.Errorf("%s: include must name a relative Markdown file: %q", file, target)
		}
		rel := filepath.ToSlash(filepath.Join(filepath.Dir(file), filepath.FromSlash(path)))
		resolved, err := safeAuthorPath(c.dir, rel)
		if err != nil {
			return fmt.Errorf("%s: include %q: %w", file, target, err)
		}
		if !strings.EqualFold(filepath.Ext(rel), ".md") {
			return fmt.Errorf("%s: include %q must be Markdown", file, target)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return fmt.Errorf("%s: include %q must be a regular Markdown file below 8 MiB", file, target)
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("%s: include %q: %w", file, target, err)
		}
		if strings.HasPrefix(strings.TrimSpace(string(data)), "---\n") || strings.HasPrefix(strings.TrimSpace(string(data)), "---\r\n") {
			return fmt.Errorf("%s: included %s has deck headmatter; use slide YAML fences in fragments", file, rel)
		}
		offset := 0
		if section != "" {
			data, offset, err = selectMarkdownSection(data, section)
			if err != nil {
				return fmt.Errorf("%s: include %q: %w", file, target, err)
			}
		}
		if !containsString(c.files, rel) {
			c.files = append(c.files, rel)
		}
		if err := c.expand(rel, data, offset, depth+1); err != nil {
			return err
		}
		pos = include.end
	}
	return c.append(src[pos:], file, original+pos)
}

func selectMarkdownSection(src []byte, id string) ([]byte, int, error) {
	comments, err := markdownComments(src)
	if err != nil {
		return nil, 0, err
	}
	start, end := -1, len(src)
	seen := map[string]bool{}
	for _, n := range comments {
		match := sectionComment.FindStringSubmatch(strings.TrimSpace(n.Literal))
		if match == nil {
			continue
		}
		if seen[match[1]] {
			return nil, 0, fmt.Errorf("duplicate section %q", match[1])
		}
		seen[match[1]] = true
		if start >= 0 && end == len(src) {
			end = n.Range.StartByte
		}
		if match[1] == id {
			start = n.Range.EndByte
		}
	}
	if start < 0 {
		return nil, 0, fmt.Errorf("section %q not found", id)
	}
	return src[start:end], start, nil
}

func containsString(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}

// safeAuthorPath checks lexical traversal and symlink resolution before reads.
func safeAuthorPath(dir, rel string) (string, error) {
	if rel == "" || !safeDeckRelPath(rel) || strings.Contains(rel, "\\") {
		return "", fmt.Errorf("path escapes deck: %q", rel)
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Rel(root, abs)
	if err != nil || !safeDeckRelPath(resolved) {
		return "", fmt.Errorf("path escapes deck: %q", rel)
	}
	return abs, nil
}

func expandDeckSource(dir string, src []byte) (*composition, error) {
	c := &composition{dir: dir, active: map[string]bool{}}
	if !bytes.Contains(src, []byte("slides:include")) {
		if err := c.append(src, DeckFileName, 0); err != nil {
			return nil, err
		}
		return c, nil
	}
	if err := c.expand(DeckFileName, src, 0, 0); err != nil {
		return nil, err
	}
	return c, nil
}

func (d *IslandDeck) localSourcePath(origin, value string) (string, error) {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return "", fmt.Errorf("%s: invalid relative source %q", origin, value)
	}
	rel := filepath.ToSlash(filepath.Join(filepath.Dir(origin), filepath.FromSlash(value)))
	_, err := safeAuthorPath(d.Dir, rel)
	return rel, err
}

// assetURL registers just the referenced local file, avoiding publication of
// Markdown, component source, or other author files alongside static assets.
func (d *IslandDeck) assetURL(origin, value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if u.IsAbs() || u.Host != "" || strings.HasPrefix(value, "/") || u.Path == "" {
		return value, nil
	}
	rel, err := d.localSourcePath(origin, u.Path)
	if err != nil {
		return "", fmt.Errorf("%s: asset %q: %w", origin, value, err)
	}
	abs, err := safeAuthorPath(d.Dir, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s: asset %q is not a regular file", origin, value)
	}
	if d.compositionAssets == nil {
		d.compositionAssets = map[string]string{}
	}
	key := "/public/_slides/" + rel
	d.compositionAssets[key] = rel
	u.Path = key
	return u.String(), nil
}

var htmlLocalAsset = regexp.MustCompile(`(?i)\b(src|href|poster)\s*=\s*("([^"]*)"|'([^']*)')`)

func (d *IslandDeck) rebaseIncludedNodes() error {
	var problem error
	d.Document.AST().Walk(func(n *mdpp.Node) bool {
		if problem != nil {
			return false
		}
		where, ok := d.SourceLocation(n.Range.StartByte, n.Range.StartByte+1)
		if !ok || where.File == DeckFileName {
			return true
		}
		switch n.Type {
		case mdpp.NodeImage, mdpp.NodeLink:
			attr := "src"
			if n.Type == mdpp.NodeLink {
				attr = "href"
			}
			value, err := d.assetURL(where.File, n.Attr(attr))
			if err != nil {
				problem = err
				return false
			}
			if n.Attrs == nil {
				n.Attrs = map[string]string{}
			}
			n.Attrs[attr] = value
		case mdpp.NodeCodeBlock:
			if language := n.Attr("language"); language == "yaml" || language == "yml" {
				n.Literal, problem = d.rebasePackCSS(where.File, n.Literal)
				return problem == nil
			}
			if path, window, ok := parseSnippetDirective(n.Literal); ok {
				rel, err := d.localSourcePath(where.File, path)
				if err != nil {
					problem = fmt.Errorf("%s: snippet %q: %w", where.File, path, err)
					return false
				}
				n.Literal = "<<< " + rel
				if window != "" {
					n.Literal += " " + window
				}
			}
		case mdpp.NodeComponent:
			if isGraphicsComponent(n.Attr("name")) {
				n.Attrs["props"], problem = d.rebaseGraphicProps(where.File, n.Attr("props"))
			}
		case mdpp.NodeHTMLBlock, mdpp.NodeHTMLInline, mdpp.NodeText:
			if n.Type == mdpp.NodeText && !strings.Contains(n.Literal, "<") {
				return true
			}
			if n.Type == mdpp.NodeHTMLBlock && strings.HasPrefix(strings.TrimSpace(n.Literal), "<style") {
				n.Literal, problem = d.rebasePackCSS(where.File, n.Literal)
				return problem == nil
			}
			n.Literal = blockComponentRe.ReplaceAllStringFunc(n.Literal, func(tag string) string {
				match := blockComponentRe.FindStringSubmatch(tag)
				if isGraphicsComponent(match[1]) {
					props, err := d.rebaseGraphicProps(where.File, match[2])
					if err != nil {
						problem = err
						return tag
					}
					return "<" + match[1] + " " + props + match[3] + ">"
				}
				return tag
			})
			// Only lowercase HTML tags have ordinary URL attributes. GoSX
			// component props are author code and must not be HTML-tokenized.
			n.Literal = htmlTagRe.ReplaceAllStringFunc(n.Literal, func(tag string) string {
				return htmlLocalAsset.ReplaceAllStringFunc(tag, func(attr string) string {
					match := htmlLocalAsset.FindStringSubmatch(attr)
					value := match[3]
					quote := `"`
					if match[4] != "" {
						value = match[4]
						quote = "'"
					}
					rebased, err := d.assetURL(where.File, value)
					if err != nil {
						problem = err
						return attr
					}
					return match[1] + "=" + quote + rebased + quote
				})
			})
		}
		return true
	})
	return problem
}

func (d *IslandDeck) rebaseGraphicProps(origin, raw string) (string, error) {
	tokens := splitPropTokens(raw)
	for i, token := range tokens {
		name, value, ok := strings.Cut(token, "=")
		if !ok || (name != "Src" && name != "Steps" && name != "Shader" && name != "Uniforms") {
			continue
		}
		path, ok := parsePropValue(value).(string)
		if !ok {
			return "", fmt.Errorf("%s: graphic %s must be a relative path", origin, name)
		}
		rel, err := d.localSourcePath(origin, path)
		if err != nil {
			return "", fmt.Errorf("%s: graphic %s: %w", origin, name, err)
		}
		tokens[i] = name + "=" + strconv.Quote(rel)
	}
	return strings.Join(tokens, " "), nil
}

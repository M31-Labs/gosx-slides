package slides

// deck_grammars.go — deck-local grammar blobs for code-fence highlighting.
//
// A deck can showcase languages the built-in highlighter has never heard of —
// typically a project's own DSLs. Dropping `<name>.bin` (a gotreesitter
// grammar blob) and `<name>.scm` (its highlight query) into the deck's
// grammars/ directory makes ```<name> fences highlight through the language's
// real grammar. This is the artifact-not-toolchain contract: the deck loads
// compiled blobs; it never imports the language's module or build pipeline.

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"m31labs.dev/gosx"
)

// deckGrammar is one loaded deck-local language: the blob-decoded grammar and
// its compiled highlight query, executed per fence via the plain query cursor
// (the same lane the language projects' own highlight tests use).
type deckGrammar struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

var (
	deckGrammarsMu    sync.Mutex
	deckGrammarsCache = map[string]map[string]*deckGrammar{}
)

// deckGrammarsFor loads (once per deck dir) every <name>.bin / <name>.scm pair
// under grammars/. Pairs are required — a blob without its highlight query
// could parse but never style, so it is skipped. Unreadable or undecodable
// entries are also skipped rather than failing the deck: the fence then
// degrades to plain text exactly like any unknown language.
func deckGrammarsFor(deckDir string) map[string]*deckGrammar {
	deckGrammarsMu.Lock()
	defer deckGrammarsMu.Unlock()
	if set, ok := deckGrammarsCache[deckDir]; ok {
		return set
	}
	set := map[string]*deckGrammar{}
	dir := filepath.Join(deckDir, "grammars")
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".bin") {
				continue
			}
			base := strings.TrimSuffix(name, ".bin")
			blob, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			queryText, err := os.ReadFile(filepath.Join(dir, base+".scm"))
			if err != nil {
				continue
			}
			lang, err := gotreesitter.LoadLanguage(blob)
			if err != nil {
				continue
			}
			// Blob-loaded languages need the same aftercare the registry's own
			// extension path applies: lex-mode repair, and reattaching the host
			// external scanner (these DSLs are Go supersets, and Go's automatic
			// semicolons live in scanner code the blob cannot carry).
			gotreesitter.RepairNoLookaheadLexModes(lang)
			effective := string(queryText)
			parent := deckGrammarExtendsParent(effective)
			if !grammars.AdaptScannerForLanguage(base, lang) && parent != "" {
				grammars.AdaptScannerForLanguage(parent, lang)
			}
			// Compose the parent language's highlight query underneath the
			// extension's, mirroring the registry's InheritHighlights rule:
			// parent first, so the child's patterns win on identical ranges.
			if parent != "" {
				if entry := grammars.DetectLanguageByName(parent); entry != nil && strings.TrimSpace(entry.HighlightQuery) != "" {
					effective = entry.HighlightQuery + "\n" + effective
				}
			}
			query, err := gotreesitter.NewQuery(effective, lang)
			if err != nil {
				// The composed query can reference parent nodes a stale blob
				// lacks; fall back to the extension's own query alone.
				query, err = gotreesitter.NewQuery(string(queryText), lang)
				if err != nil {
					continue
				}
			}
			set[strings.ToLower(base)] = &deckGrammar{lang: lang, query: query}
		}
	}
	deckGrammarsCache[deckDir] = set
	return set
}

// deckGrammarExtendsRe matches the highlight-query header convention shared
// with the registry and Orchard: `;; Extension: name (extends go)`.
var deckGrammarExtendsRe = regexp.MustCompile(`\(extends ([a-z_0-9]+)\)`)

func deckGrammarExtendsParent(query string) string {
	m := deckGrammarExtendsRe.FindStringSubmatch(query)
	if m == nil {
		return ""
	}
	return m[1]
}

// deckGrammarCaptureClass maps tree-sitter highlight capture names onto the
// ts-* span classes the themes already style, so a deck-local language picks
// up the deck's existing code palette. Dotted captures fall back to their head
// segment; unmapped captures (plain variables, for instance) stay unstyled.
var deckGrammarCaptureClass = map[string]string{
	"keyword":     "ts-keyword",
	"string":      "ts-string",
	"number":      "ts-number",
	"comment":     "ts-comment",
	"type":        "ts-type",
	"constructor": "ts-type",
	"constant":    "ts-bool",
	"attribute":   "ts-attr",
	"property":    "ts-property",
	"function":    "ts-builtin",
	"operator":    "ts-operator",
	"punctuation": "ts-punctuation",
	"namespace":   "ts-namespace",
	"tag":         "ts-tag",
}

func deckGrammarClassFor(capture string) string {
	if class, ok := deckGrammarCaptureClass[capture]; ok {
		return class
	}
	if i := strings.IndexByte(capture, '.'); i > 0 {
		return deckGrammarCaptureClass[capture[:i]]
	}
	return ""
}

// deckGrammarHTML highlights source with a deck-local grammar and returns
// escaped span markup shaped like the built-in highlighter's output. ok is
// false when the deck ships no grammar under that fence name.
func deckGrammarHTML(deckDir, lang, source string) (string, bool) {
	g := deckGrammarsFor(deckDir)[strings.ToLower(strings.TrimSpace(lang))]
	if g == nil {
		return "", false
	}
	src := []byte(source)
	tree, err := gotreesitter.NewParser(g.lang).Parse(src)
	if err != nil || tree == nil {
		return html.EscapeString(source), true
	}
	type span struct {
		start, end uint32
		class      string
	}
	var spans []span
	cursor := g.query.Exec(tree.RootNode(), g.lang, src)
	for {
		capture, ok := cursor.NextCapture()
		if !ok {
			break
		}
		class := deckGrammarClassFor(capture.Name)
		if class == "" || capture.Node == nil {
			continue
		}
		spans = append(spans, span{start: capture.Node.StartByte(), end: capture.Node.EndByte(), class: class})
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var b strings.Builder
	pos := uint32(0)
	for _, s := range spans {
		if s.start < pos || s.end <= s.start || int(s.end) > len(src) {
			continue // overlapping or out-of-range span: first writer wins
		}
		deckGrammarLexGap(&b, string(src[pos:s.start]))
		b.WriteString(`<span class="`)
		b.WriteString(s.class)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(string(src[s.start:s.end])))
		b.WriteString(`</span>`)
		pos = s.end
	}
	deckGrammarLexGap(&b, string(src[pos:]))
	return b.String(), true
}

// deckGrammarGoKeywords is the lexical gap-fill vocabulary: Go's keywords,
// because every deck-local DSL so far is a Go superset and its highlight
// query concentrates on the EXTENSION nodes. Structural captures always win;
// this only colors text the grammar's query left unstyled.
var deckGrammarGoKeywords = map[string]string{
	"package": "ts-keyword", "import": "ts-keyword", "func": "ts-keyword",
	"return": "ts-keyword", "if": "ts-keyword", "else": "ts-keyword",
	"for": "ts-keyword", "range": "ts-keyword", "var": "ts-keyword",
	"const": "ts-keyword", "type": "ts-keyword", "struct": "ts-keyword",
	"interface": "ts-keyword", "map": "ts-keyword", "chan": "ts-keyword",
	"go": "ts-keyword", "defer": "ts-keyword", "switch": "ts-keyword",
	"select": "ts-keyword", "case": "ts-keyword", "default": "ts-keyword",
	"break": "ts-keyword", "continue": "ts-keyword", "fallthrough": "ts-keyword",
	"goto": "ts-keyword",
	"true": "ts-bool", "false": "ts-bool", "nil": "ts-bool",
}

func isDeckIdentByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// deckGrammarLexGap renders one unstyled gap segment: HTML-escaped, with a
// minimal lexical pass for strings, line comments, numbers, and Go keywords.
func deckGrammarLexGap(b *strings.Builder, text string) {
	emit := func(class, tok string) {
		if class == "" {
			b.WriteString(html.EscapeString(tok))
			return
		}
		b.WriteString(`<span class="`)
		b.WriteString(class)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(tok))
		b.WriteString(`</span>`)
	}
	i := 0
	for i < len(text) {
		c := text[i]
		switch {
		case c == '"' || c == '`':
			quote := c
			j := i + 1
			for j < len(text) {
				if text[j] == '\\' && quote == '"' && j+1 < len(text) {
					j += 2
					continue
				}
				if text[j] == quote || text[j] == '\n' {
					j++
					break
				}
				j++
			}
			emit("ts-string", text[i:j])
			i = j
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				j = len(text)
			} else {
				j += i
			}
			emit("ts-comment", text[i:j])
			i = j
		case c >= '0' && c <= '9' && (i == 0 || !isDeckIdentByte(text[i-1])):
			j := i
			for j < len(text) && (isDeckIdentByte(text[j]) || text[j] == '.') {
				j++
			}
			emit("ts-number", text[i:j])
			i = j
		case isDeckIdentByte(c) && !(c >= '0' && c <= '9'):
			j := i
			for j < len(text) && isDeckIdentByte(text[j]) {
				j++
			}
			word := text[i:j]
			emit(deckGrammarGoKeywords[word], word)
			i = j
		default:
			j := i + 1
			for j < len(text) && !isDeckIdentByte(text[j]) && text[j] != '"' && text[j] != '`' && text[j] != '/' {
				j++
			}
			emit("", text[i:j])
			i = j
		}
	}
}

// deckGrammarLangToken reduces a fence language to an attribute-safe token for
// data-lang, mirroring highlight.NormalizeLanguage's fixed-token guarantee.
func deckGrammarLangToken(lang string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return -1
		}
	}, strings.TrimSpace(lang))
}

// deckCodeBlockNode is codeBlockNode with deck-local grammar support layered
// in front. Emphasis stepping and diff coloring keep the built-in per-line
// path (deck grammars render whole blocks), as does every language the deck
// does not override.
func deckCodeBlockNode(deckDir, lang, source, highlights string) gosx.Node {
	if !strings.EqualFold(lang, "diff") && strings.TrimSpace(highlights) == "" {
		trimmed := strings.TrimRight(source, "\n")
		if hl, ok := deckGrammarHTML(deckDir, lang, trimmed); ok {
			var b strings.Builder
			b.WriteString(`<pre class="code-block" data-lang="`)
			b.WriteString(deckGrammarLangToken(lang))
			b.WriteString(`"><code>`)
			b.WriteString(hl)
			b.WriteString(`</code></pre>`)
			return gosx.RawHTML(b.String())
		}
	}
	return codeBlockNode(lang, source, highlights)
}

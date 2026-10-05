package slides

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

// SourceDiagnostic is a parser/story finding. Byte ranges refer to UTF-8 source;
// clients must convert them before selecting JavaScript UTF-16 text.
type SourceDiagnostic struct {
	Code     string     `json:"code"`
	Severity string     `json:"severity"`
	Message  string     `json:"message"`
	Range    mdpp.Range `json:"range"`
	File     string     `json:"file,omitempty"`
}

type sourceOutline struct {
	Index int        `json:"index"`
	Title string     `json:"title"`
	Range mdpp.Range `json:"range"`
}

type sourceAnalysis struct {
	Revision    string             `json:"revision"`
	Diagnostics []SourceDiagnostic `json:"diagnostics"`
	Symbols     []mdpp.StorySymbol `json:"symbols"`
	Outline     []sourceOutline    `json:"outline"`
	Editable    bool               `json:"editable"`
	Renameable  bool               `json:"renameable"`
	Source      *string            `json:"source,omitempty"`
}

// Deck diagnostics retain the original author file and byte range. A parsed
// finding spanning multiple origins has no single editable source location.
func deckSourceDiagnostics(deck *IslandDeck) []SourceDiagnostic {
	if deck == nil {
		return nil
	}
	out := sourceDiagnostics(deck.Document)
	sources := map[string][]byte{DeckFileName: deck.Source}
	for i := range out {
		diagnostic := &out[i]
		where, ok := deck.SourceLocation(diagnostic.Range.StartByte, diagnostic.Range.EndByte)
		if !ok {
			diagnostic.File = "<composed deck>"
			continue
		}
		source, loaded := sources[where.File]
		if !loaded {
			path, err := safeAuthorPath(deck.Dir, where.File)
			if err == nil {
				source, err = os.ReadFile(path)
			}
			if err != nil {
				diagnostic.File = "<composed deck>"
				continue
			}
			sources[where.File] = source
		}
		if where.StartByte < 0 || where.EndByte > len(source) {
			diagnostic.File = "<composed deck>"
			continue
		}
		diagnostic.File = where.File
		diagnostic.Range.StartByte, diagnostic.Range.EndByte = where.StartByte, where.EndByte
		diagnostic.Range.StartLine, diagnostic.Range.StartCol = sourcePosition(source, where.StartByte)
		diagnostic.Range.EndLine, diagnostic.Range.EndCol = sourcePosition(source, where.EndByte)
	}
	return out
}

func sourcePosition(source []byte, offset int) (line, column int) {
	line, column = 1, 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\r' || source[i] == '\n' {
			line++
			column = 1
			if source[i] == '\r' && i+1 < offset && source[i+1] == '\n' {
				i++
			}
		} else {
			column++
		}
	}
	return
}

func diagnosticMessage(diagnostic SourceDiagnostic) string {
	file := diagnostic.File
	if file == "" {
		file = DeckFileName
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", file, diagnostic.Range.StartLine, diagnostic.Range.StartCol, diagnostic.Code, diagnostic.Message)
}

func sourceDiagnostics(doc *mdpp.Document) []SourceDiagnostic {
	if doc == nil {
		return nil
	}
	diagnostics := append(doc.Diagnostics(), mdpp.IndexStory(doc).Diagnostics...)
	out := make([]SourceDiagnostic, 0, len(diagnostics))
	for _, d := range diagnostics {
		severity := "info"
		if d.Severity == mdpp.SeverityWarning {
			severity = "warning"
		}
		if d.Severity == mdpp.SeverityError {
			severity = "error"
		}
		out = append(out, SourceDiagnostic{d.Code, severity, d.Message, d.Range, ""})
	}
	return out
}

// Analyze raw source, never expanded includes: draft ranges can only describe
// bytes in the editor's deck.md. Disk saves retain the existing revision guard.
func analyzeSource(source string) (sourceAnalysis, mdpp.StoryIndex, error) {
	doc, err := mdpp.Parse([]byte(source))
	if err != nil {
		return sourceAnalysis{}, mdpp.StoryIndex{}, err
	}
	idx := mdpp.IndexStory(doc)
	hasIncludes, err := HasDeckIncludes([]byte(source))
	if err != nil {
		return sourceAnalysis{}, mdpp.StoryIndex{}, err
	}
	out := sourceAnalysis{Revision: sourceRevision([]byte(source)), Diagnostics: sourceDiagnostics(doc), Symbols: idx.Symbols, Outline: []sourceOutline{}, Editable: !doc.SourceHadCarriageReturns(), Renameable: !hasIncludes && !doc.SourceHadCarriageReturns()}
	if out.Symbols == nil {
		out.Symbols = []mdpp.StorySymbol{}
	}
	if !out.Editable {
		out.Diagnostics = append(out.Diagnostics, SourceDiagnostic{Code: "SOURCE-LF", Severity: "warning", Message: "Structured source edits require LF line endings. Convert this file before using rename.", Range: mdpp.Range{StartLine: 1, StartCol: 1}})
	}
	if hasIncludes {
		out.Diagnostics = append(out.Diagnostics, SourceDiagnostic{Code: "SOURCE-INCLUDES", Severity: "info", Message: "Single-file rename is unavailable while this deck includes fragments. Edit the author files directly.", Range: mdpp.Range{StartLine: 1, StartCol: 1}})
	}
	// NodeSlide ranges omit lifted YAML metadata. Group the original top-level
	// nodes instead, using parsed thematic breaks rather than textual separators.
	start, index, title := 0, 0, ""
	add := func(end int) {
		if title == "" {
			title = fmt.Sprintf("Slide %d", index+1)
		}
		out.Outline = append(out.Outline, sourceOutline{index, title, mdpp.Range{StartByte: start, EndByte: end}})
		index++
		title = ""
	}
	for _, n := range doc.Root.Children {
		if n.Type == mdpp.NodeFrontmatter {
			start = n.Range.EndByte
			continue
		}
		if n.Type == mdpp.NodeThematicBreak {
			add(n.Range.StartByte)
			start = n.Range.EndByte
			continue
		}
		if title == "" && n.Level() > 0 {
			title = strings.TrimSpace(n.Text())
		}
	}
	add(len(doc.Source))
	return out, idx, nil
}

func renameSource(source string, offset int, name string) (string, error) {
	hasIncludes, err := HasDeckIncludes([]byte(source))
	if err != nil {
		return "", err
	}
	if hasIncludes {
		return "", fmt.Errorf("single-file rename cannot update included fragment references; edit the author files directly")
	}
	_, idx, err := analyzeSource(source)
	if err != nil {
		return "", err
	}
	target, ok := idx.At(offset)
	if !ok {
		return "", fmt.Errorf("select a parsed slide, cue or actor name")
	}
	edits, err := idx.Rename(target, name)
	if err != nil {
		return "", err
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Range.StartByte > edits[j].Range.StartByte })
	for _, e := range edits {
		source = source[:e.Range.StartByte] + e.NewText + source[e.Range.EndByte:]
	}
	return source, nil
}

// Draft analysis/rename has no filesystem writes and does not publish navigation
// state. The same local-origin/token gate as persistent editing applies.
func mountSourceTools(app *server.App, token string) {
	app.Mount("/_slides/analyze", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			fail(405, "method not allowed")
			return
		}
		if err := authorizeSourceRequest(r, token, true); err != nil {
			fail(403, err.Error())
			return
		}
		var input struct {
			Source string `json:"source"`
			Rename *struct {
				Offset int    `json:"offset"`
				Name   string `json:"name"`
			} `json:"rename"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6*maxSourceBytes+16384)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || len(input.Source) > maxSourceBytes {
			fail(400, "invalid source or authoring size limit exceeded")
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			fail(400, "expected one draft analysis")
			return
		}
		source := input.Source
		if input.Rename != nil {
			var err error
			source, err = renameSource(source, input.Rename.Offset, input.Rename.Name)
			if err != nil {
				fail(422, err.Error())
				return
			}
			if len(source) > maxSourceBytes {
				fail(400, "authoring size limit exceeded")
				return
			}
		}
		out, _, err := analyzeSource(source)
		if err != nil {
			fail(422, err.Error())
			return
		}
		if input.Rename != nil {
			out.Source = &source
		}
		json.NewEncoder(w).Encode(out)
	}))
}

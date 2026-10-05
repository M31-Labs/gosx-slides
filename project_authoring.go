package slides

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
	"m31labs.dev/gosx"
	"m31labs.dev/mdpp"
	"m31labs.dev/sirena"
)

const maxProjectFiles = 512
const maxProjectBytes = 64 << 20

// AuthorProject offers the same bounded, revision-checked operations to the
// browser and local agents. It never accepts a command, module hook, or an
// absolute file path. One instance serializes edits; external edits use hashes.
type AuthorProject struct {
	dir string
	mu  *sync.Mutex
}

type ProjectFile struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Revision string `json:"revision"`
	Bytes    int    `json:"bytes"`
}
type ProjectIndex struct {
	Files           []ProjectFile `json:"files"`
	ContextRevision string        `json:"contextRevision"`
}
type ProjectDocument struct {
	ProjectFile
	Source          string `json:"source"`
	ContextRevision string `json:"contextRevision"`
}
type ProjectEdit struct {
	File            string `json:"file"`
	Source          string `json:"source"`
	Revision        string `json:"revision"`
	ContextRevision string `json:"contextRevision"`
}
type ProjectReport struct {
	ContextRevision string             `json:"contextRevision"`
	Diagnostics     []SourceDiagnostic `json:"diagnostics"`
	Outline         []ProjectOutline   `json:"outline"`
}
type ProjectOutline struct {
	Index int        `json:"index"`
	Title string     `json:"title"`
	File  string     `json:"file"`
	Range mdpp.Range `json:"range"`
}
type ProjectSave struct {
	Revision        string `json:"revision"`
	ContextRevision string `json:"contextRevision"`
	Recovery        string `json:"recovery"`
}
type ProjectError struct {
	Status  int
	Message string
}

func (e *ProjectError) Error() string { return e.Message }
func projectFailure(status int, message string) error {
	return &ProjectError{status, message}
}

func NewAuthorProject(dir string) (*AuthorProject, error) {
	abs, err := filepath.Abs(dir)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return &AuthorProject{dir: abs, mu: &sync.Mutex{}}, nil
}

// Hidden/recovery/session state, build outputs, public assets, packages and
// private folders are never editable. Asset copies for validation use the same
// private exclusions but may include public/. Symlinks are never followed.
func projectPathAllowed(name string) bool {
	if name == "" || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		lower := strings.ToLower(part)
		if strings.HasPrefix(part, ".") || lower == "build" || lower == "dist" || lower == "node_modules" || lower == "private" || lower == "secrets" || lower == "migration" || lower == "tour.json" {
			return false
		}
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".pem", ".key", ".p12", ".pfx", ".env":
		return false
	}
	return true
}

func projectKind(name string) string {
	if !projectPathAllowed(name) || strings.HasPrefix(strings.ToLower(name), "public/") || strings.HasPrefix(strings.ToLower(name), "packs/") {
		return ""
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".md":
		return "markdown"
	case ".gsx":
		return "gosx"
	case ".sir":
		return "sirena"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".css":
		return "css"
	case ".sel":
		return "shader"
	}
	return ""
}

func projectValidationFile(name string) bool {
	if projectKind(name) != "" {
		return true
	}
	if strings.HasPrefix(name, "packs/") {
		return true
	}
	if name == "go.mod" || name == "go.sum" {
		return true
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".go", ".txt", ".csv", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".gif", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".glb", ".gltf", ".bin", ".vtt", ".wav", ".mp3", ".mp4", ".webm", ".css", ".json":
		return true
	}
	return false
}

func projectReadRegular(root *os.Root, name string, limit int64) ([]byte, os.FileMode, error) {
	if !projectPathAllowed(name) {
		return nil, 0, projectFailure(403, "project path is private or invalid")
	}
	parts := strings.Split(name, "/")
	var expected os.FileInfo
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) {
			return nil, 0, projectFailure(403, "project files cannot use symlinks or nonregular paths")
		}
		expected = info
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(expected, info) || info.Size() > limit {
		return nil, 0, projectFailure(413, "project file is not regular or exceeds its size limit")
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, 0, projectFailure(413, "project file size limit exceeded")
	}
	return raw, info.Mode().Perm(), nil
}

type projectSnapshot struct {
	ProjectIndex
	data  map[string][]byte
	modes map[string]os.FileMode
}

func (p *AuthorProject) snapshot() (*projectSnapshot, error) {
	root, err := os.OpenRoot(p.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	s := &projectSnapshot{ProjectIndex: ProjectIndex{Files: []ProjectFile{}}, data: map[string][]byte{}, modes: map[string]os.FileMode{}}
	total := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if name == "." {
			return walkErr
		}
		if !projectPathAllowed(name) || entry != nil && entry.Type()&os.ModeSymlink != 0 {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !projectValidationFile(name) {
			return nil
		}
		if len(s.data) >= maxProjectFiles {
			return projectFailure(413, "project validation is limited to 512 files")
		}
		kind, limit := projectKind(name), int64(16<<20)
		if kind != "" {
			limit = maxSourceBytes
		}
		raw, mode, err := projectReadRegular(root, name, limit)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		total += len(raw)
		if total > maxProjectBytes {
			return projectFailure(413, "project validation is limited to 64 MiB")
		}
		s.data[name], s.modes[name] = raw, mode
		if kind != "" {
			if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
				return projectFailure(422, name+": author source must be UTF-8 text")
			}
			s.Files = append(s.Files, ProjectFile{name, kind, sourceRevision(raw), len(raw)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, ok := s.data[DeckFileName]; !ok {
		return nil, projectFailure(422, "project needs a regular deck.md")
	}
	// Stable context includes all validation inputs, assets and directory membership.
	names := make([]string, 0, len(s.data))
	for name := range s.data {
		names = append(names, name)
	}
	sort.Strings(names)
	var context bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&context, "%s\x00%s\n", name, sourceRevision(s.data[name]))
	}
	s.ContextRevision = sourceRevision(context.Bytes())
	return s, nil
}

func (p *AuthorProject) List() (ProjectIndex, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot()
	if err != nil {
		return ProjectIndex{}, err
	}
	return s.ProjectIndex, nil
}

func (p *AuthorProject) Read(name string) (ProjectDocument, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if projectKind(name) == "" {
		return ProjectDocument{}, projectFailure(403, "file is not editable author source")
	}
	s, err := p.snapshot()
	if err != nil {
		return ProjectDocument{}, err
	}
	for _, file := range s.Files {
		if file.Path == name {
			return ProjectDocument{file, string(s.data[name]), s.ContextRevision}, nil
		}
	}
	return ProjectDocument{}, projectFailure(404, "project file not found")
}

func (s *projectSnapshot) validate(overrides map[string]string) (ProjectReport, *IslandDeck, func(), error) {
	report := ProjectReport{ContextRevision: s.ContextRevision, Diagnostics: []SourceDiagnostic{}, Outline: []ProjectOutline{}}
	dir, err := os.MkdirTemp("", "slides-project-validation-*")
	if err != nil {
		return report, nil, func() {}, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	for name, raw := range s.data {
		if changed, ok := overrides[name]; ok {
			raw = []byte(changed)
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0700); err == nil {
			err = os.WriteFile(target, raw, 0600)
		}
		if err != nil {
			cleanup()
			return report, nil, func() {}, err
		}
	}
	addError := func(file, code string, err error) {
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, SourceDiagnostic{Code: code, Severity: "error", Message: strings.ReplaceAll(err.Error(), dir+string(filepath.Separator), ""), File: file, Range: mdpp.Range{StartLine: 1, StartCol: 1}})
		}
	}
	for _, file := range s.Files {
		raw := s.data[file.Path]
		if changed, ok := overrides[file.Path]; ok {
			raw = []byte(changed)
		}
		switch file.Kind {
		case "markdown":
			doc, err := mdpp.Parse(raw)
			addError(file.Path, "PROJECT-MARKDOWN", err)
			if doc != nil {
				for _, d := range doc.Diagnostics() {
					severity := "info"
					if d.Severity == mdpp.SeverityError {
						severity = "error"
					} else if d.Severity == mdpp.SeverityWarning {
						severity = "warning"
					}
					removed := markdownCRPositions(raw)
					d.Range.StartByte = originalMarkdownOffset(removed, d.Range.StartByte)
					d.Range.EndByte = originalMarkdownOffset(removed, d.Range.EndByte)
					d.Range.StartLine, d.Range.StartCol = sourcePosition(raw, d.Range.StartByte)
					d.Range.EndLine, d.Range.EndCol = sourcePosition(raw, d.Range.EndByte)
					report.Diagnostics = append(report.Diagnostics, SourceDiagnostic{d.Code, severity, d.Message, d.Range, file.Path})
				}
			}
		case "sirena":
			doc, err := sirena.Parse(raw)
			addError(file.Path, "PROJECT-SIRENA", err)
			if doc == nil {
				continue
			}
			for _, d := range doc.Diagnostics() {
				r := mdpp.Range{StartByte: d.Range.Start, EndByte: d.Range.End}
				r.StartLine, r.StartCol = sourcePosition(raw, r.StartByte)
				r.EndLine, r.EndCol = sourcePosition(raw, r.EndByte)
				report.Diagnostics = append(report.Diagnostics, SourceDiagnostic{d.Code, d.Severity.String(), d.Message, r, file.Path})
			}
		case "gosx":
			tree, _, err := gosx.Parse(raw)
			var syntaxRange *mdpp.Range
			if err == nil && tree.RootNode().HasErrorOrMissing() {
				err = fmt.Errorf("GoSX syntax error")
				node := tree.RootNode()
				for {
					var next = node
					for _, child := range node.Children() {
						if child.HasErrorOrMissing() {
							next = child
							break
						}
					}
					if next == node {
						break
					}
					node = next
				}
				r := mdpp.Range{StartByte: int(node.StartByte()), EndByte: int(node.EndByte())}
				r.StartLine, r.StartCol = sourcePosition(raw, r.StartByte)
				r.EndLine, r.EndCol = sourcePosition(raw, r.EndByte)
				syntaxRange = &r
			}
			if err == nil {
				_, err = gosx.Compile(raw)
			}
			addError(file.Path, "PROJECT-GOSX", err)
			if syntaxRange != nil {
				report.Diagnostics[len(report.Diagnostics)-1].Range = *syntaxRange
			}
		case "json":
			if !json.Valid(raw) {
				addError(file.Path, "PROJECT-JSON", fmt.Errorf("invalid JSON document"))
			}
		case "yaml":
			var value any
			decoder := yaml.NewDecoder(bytes.NewReader(raw))
			err := decoder.Decode(&value)
			if err == nil && decoder.Decode(&value) != io.EOF {
				err = fmt.Errorf("expected one YAML document")
			}
			addError(file.Path, "PROJECT-YAML", err)
		}
	}
	deck, err := LoadIslandDeck(dir)
	addError(DeckFileName, "PROJECT-DECK", err)
	if err == nil {
		if len(deck.Slides) == 0 {
			addError(DeckFileName, "PROJECT-EMPTY", fmt.Errorf("the deck needs at least one slide"))
		}
		report.Diagnostics = append(report.Diagnostics, deckSourceDiagnostics(deck)...)
		for _, slide := range deck.Slides {
			row := ProjectOutline{Index: slide.Index, Title: fmt.Sprintf("Slide %d", slide.Index+1)}
			for _, node := range slide.Node.Children {
				location, ok := deck.SourceLocation(node.Range.StartByte, node.Range.EndByte)
				if !ok {
					continue
				}
				if row.File == "" || node.Level() > 0 {
					row.File, row.Range.StartByte, row.Range.EndByte = location.File, location.StartByte, location.EndByte
					if raw := s.data[row.File]; raw != nil {
						if draft, ok := overrides[row.File]; ok {
							raw = []byte(draft)
						}
						row.Range.StartLine, row.Range.StartCol = sourcePosition(raw, location.StartByte)
						row.Range.EndLine, row.Range.EndCol = sourcePosition(raw, location.EndByte)
					}
				}
				if node.Level() > 0 {
					row.Title = strings.TrimSpace(node.Text())
					break
				}
			}
			report.Outline = append(report.Outline, row)
		}
		_, failures := deck.compileComponents()
		for name, err := range failures {
			addError(name+".gsx", "PROJECT-COMPONENT", err)
		}
		compiled, err := compileDeckProgram(deck)
		addError(DeckFileName, "PROJECT-COMPILE", err)
		if compiled != nil {
			for key, graphic := range compiled.graphics {
				if graphic.err != nil {
					ref := ComponentRef{}
					for _, candidate := range deckGraphicRefs(deck) {
						if graphicsKey(candidate.Name, candidate.Props) == key {
							ref = candidate
							break
						}
					}
					addError(graphicString(parseProps(ref.Props), "Src", DeckFileName), "PROJECT-GRAPHIC", graphic.err)
				}
			}
		}
	}
	sort.SliceStable(report.Diagnostics, func(i, j int) bool {
		a, b := report.Diagnostics[i], report.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Range.StartByte != b.Range.StartByte {
			return a.Range.StartByte < b.Range.StartByte
		}
		return a.Code < b.Code
	})
	return report, deck, cleanup, nil
}

// Diagnose accepts a single unsaved file replacement without touching author files.
func (p *AuthorProject) Diagnose(file, source string) (ProjectReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot()
	if err != nil {
		return ProjectReport{}, err
	}
	overrides := map[string]string{}
	if file != "" {
		if projectKind(file) == "" || s.data[file] == nil || len(source) > maxSourceBytes || !utf8.ValidString(source) || strings.ContainsRune(source, 0) {
			return ProjectReport{}, projectFailure(400, "invalid project draft")
		}
		overrides[file] = source
	}
	report, _, cleanup, err := s.validate(overrides)
	defer cleanup()
	return report, err
}

func (p *AuthorProject) Write(edit ProjectEdit) (ProjectSave, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if projectKind(edit.File) == "" || len(edit.Source) > maxSourceBytes || !utf8.ValidString(edit.Source) || strings.ContainsRune(edit.Source, 0) {
		return ProjectSave{}, projectFailure(400, "invalid project source edit")
	}
	// Keep the existing structured-authoring LF contract for every project source.
	if strings.ContainsRune(edit.Source, '\r') {
		return ProjectSave{}, projectFailure(422, "project saves require LF line endings")
	}
	s, err := p.snapshot()
	if err != nil {
		return ProjectSave{}, err
	}
	if s.data[edit.File] == nil {
		return ProjectSave{}, projectFailure(404, "project file not found")
	}
	if sourceRevision(s.data[edit.File]) != edit.Revision || s.ContextRevision != edit.ContextRevision {
		return ProjectSave{}, projectFailure(409, "project changed; reload the file and reconcile your draft")
	}
	report, _, cleanup, err := s.validate(map[string]string{edit.File: edit.Source})
	defer cleanup()
	if err != nil {
		return ProjectSave{}, err
	}
	for _, d := range report.Diagnostics {
		if d.Severity != "error" {
			continue
		}
		if d.File == edit.File {
			return ProjectSave{}, projectFailure(422, "project validation failed: "+diagnosticMessage(d))
		}
		// Existing errors in other files must not trap a project in a state
		// where neither file can be repaired. Only identical baseline errors
		// may remain; new dependency/compilation errors still block the save.
		baseline, _, baselineCleanup, baselineErr := s.validate(nil)
		baselineCleanup()
		if baselineErr != nil {
			return ProjectSave{}, baselineErr
		}
		remaining := make(map[SourceDiagnostic]int)
		for _, prior := range baseline.Diagnostics {
			if prior.Severity == "error" && prior.File != edit.File {
				remaining[prior]++
			}
		}
		for _, after := range report.Diagnostics {
			if after.Severity != "error" {
				continue
			}
			if remaining[after] == 0 {
				return ProjectSave{}, projectFailure(422, "project validation failed: "+diagnosticMessage(after))
			}
			remaining[after]--
		}
		break
	}
	latest, err := p.snapshot()
	if err != nil || latest.ContextRevision != edit.ContextRevision {
		return ProjectSave{}, projectFailure(409, "project changed during validation; reload and reconcile your draft")
	}
	recovery, err := p.publish(edit, s.modes[edit.File])
	if err != nil {
		return ProjectSave{}, err
	}
	latest, err = p.snapshot()
	if err != nil {
		return ProjectSave{}, err
	}
	return ProjectSave{sourceRevision([]byte(edit.Source)), latest.ContextRevision, recovery}, nil
}

// Pin the parent directory through os.Root, then retain the displaced inode and
// publish a no-overwrite hard link. Concurrent external writes are recoverable.
func (p *AuthorProject) publish(edit ProjectEdit, mode os.FileMode) (string, error) {
	root, err := os.OpenRoot(p.dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if _, _, err := projectReadRegular(root, edit.File, maxSourceBytes); err != nil {
		return "", err
	}
	parent, err := root.OpenRoot(filepath.FromSlash(path.Dir(edit.File)))
	if err != nil {
		return "", err
	}
	defer parent.Close()
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return "", err
	}
	history := ".slides-history-" + hex.EncodeToString(random[:])
	if err = parent.Mkdir(history, 0700); err != nil {
		return "", err
	}
	name := path.Base(edit.File)
	next, previous := history+"/next", history+"/"+name
	defer func() { parent.Remove(next); parent.Remove(history) }()
	f, err := parent.OpenFile(next, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return "", err
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.WriteString(edit.Source)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = parent.Link(next, history+"/probe"); err != nil {
		return "", fmt.Errorf("project saves require hard-link support: %w", err)
	}
	parent.Remove(history + "/probe")
	if err = parent.Rename(name, previous); err != nil {
		return "", err
	}
	recovery := path.Join(path.Dir(edit.File), previous)
	matches := func() bool {
		info, err := parent.Lstat(previous)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
			return false
		}
		file, err := parent.Open(previous)
		if err != nil {
			return false
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !os.SameFile(info, opened) {
			return false
		}
		raw, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
		return err == nil && len(raw) <= maxSourceBytes && sourceRevision(raw) == edit.Revision
	}
	conflict := func() error {
		return projectFailure(409, "source changed during save; previous file retained at "+recovery)
	}
	if !matches() {
		parent.Link(previous, name)
		return "", conflict()
	}
	if err = parent.Link(next, name); err != nil {
		parent.Link(previous, name)
		if errors.Is(err, os.ErrExist) {
			return "", conflict()
		}
		return "", fmt.Errorf("could not publish source; previous file retained at %s: %w", recovery, err)
	}
	if !matches() {
		return "", conflict()
	}
	return recovery, nil
}

// ResolveAddress validates a stable slide/cue address and returns a local URL
// fragment. It does not broadcast or alter anyone's presentation state.
func (p *AuthorProject) ResolveAddress(slide, cue string) (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot()
	if err != nil {
		return nil, err
	}
	_, deck, cleanup, err := s.validate(nil)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	if deck == nil {
		return nil, fmt.Errorf("deck cannot be parsed")
	}
	for _, row := range deck.Slides {
		id, _ := slideFrontmatterValues(row)["id"].(string)
		if id != slide || !cueNamePattern.MatchString(id) {
			continue
		}
		step := 0
		if cue != "" {
			found := false
			for index, name := range slideCueNames(row) {
				if name == cue {
					step, found = index, true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("unknown cue %q", cue)
			}
		}
		anchor := "#" + slide
		if cue != "" {
			anchor += "/" + cue
		}
		return map[string]any{"anchor": anchor, "slideIndex": row.Index, "step": step, "contextRevision": s.ContextRevision}, nil
	}
	return nil, fmt.Errorf("unknown slide %q", slide)
}

func (p *AuthorProject) AssertStory() (StoryAssertionReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot()
	if err != nil {
		return StoryAssertionReport{}, err
	}
	_, deck, cleanup, err := s.validate(nil)
	defer cleanup()
	if err != nil {
		return StoryAssertionReport{}, err
	}
	if deck == nil || deck.Story == nil {
		return StoryAssertionReport{}, fmt.Errorf("deck has no valid story manifest")
	}
	return AssertStory(deck)
}

// RenameDraft uses mdpp's scoped symbol edits. Cross-file rename remains
// unavailable until one transaction can validate/publish every affected file.
func (p *AuthorProject) RenameDraft(file, source string, offset int, name string) (map[string]string, error) {
	if file != DeckFileName || len(source) > maxSourceBytes || !utf8.ValidString(source) {
		return nil, fmt.Errorf("scoped rename supports deck.md without includes; use original-file edits for composed projects")
	}
	if _, err := p.Read(file); err != nil {
		return nil, err
	}
	renamed, err := renameSource(source, offset, name)
	if err != nil {
		return nil, err
	}
	return map[string]string{"file": file, "source": renamed}, nil
}

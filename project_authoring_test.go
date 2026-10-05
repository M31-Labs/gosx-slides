package slides

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func authorProjectFixture(t *testing.T) (*AuthorProject, string) {
	t.Helper()
	dir := t.TempDir()
	for name, source := range map[string]string{
		"deck.md":            "# Root\n\n<!-- notes -->\n\n---\n\n<!-- slides:include sections/body.md -->\n",
		"sections/body.md":   "```yaml\nid: detail\ncues: initial, next\n```\n\n# Detail\n\nCafé 🦊\n",
		"story.yaml":         "version: 1\nbeats: []\n",
		"graph.sir":          "service api\n",
		"public/logo.svg":    `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"private/notes.md":   "PRIVATE-NOTES",
		".slides-team.json":  `{"draft":"PRIVATE-DRAFT"}`,
		"build/private.json": `{"draft":"PRIVATE-BUILD"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := NewAuthorProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p, dir
}

func TestAuthorProjectOriginalFilesAndDiagnostics(t *testing.T) {
	p, dir := authorProjectFixture(t)
	index, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Files) != 4 {
		t.Fatalf("author file list: %+v", index.Files)
	}
	for _, denied := range []string{"../secret.md", "sections/../deck.md", "private/notes.md", ".slides-team.json", "build/private.json", "public/logo.svg", "C:/secret.md", "sections\\body.md"} {
		if _, err := p.Read(denied); err == nil {
			t.Errorf("read allowed %q", denied)
		}
	}
	doc, err := p.Read("sections/body.md")
	if err != nil {
		t.Fatal(err)
	}
	draft := doc.Source + "\n[Jump](#detail/missing)\n"
	report, err := p.Diagnose(doc.Path, draft)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range report.Diagnostics {
		if d.File == doc.Path && strings.Contains(d.Message, "missing") {
			found = true
			if d.Range.StartByte < 0 || d.Range.EndByte > len(draft) || draft[d.Range.StartByte:d.Range.EndByte] != "missing" {
				t.Errorf("original range %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("missing original-file cue finding: %+v", report.Diagnostics)
	}
	if len(report.Outline) != 2 || report.Outline[1].File != doc.Path {
		t.Fatalf("original outline: %+v", report.Outline)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, doc.Path))
	if string(raw) != doc.Source {
		t.Fatal("diagnosis modified source")
	}
	anchor, err := p.ResolveAddress("detail", "next")
	if err != nil || anchor["step"] != 1 || anchor["anchor"] != "#detail/next" {
		t.Fatalf("address: %+v %v", anchor, err)
	}
}

func TestAuthorProjectConflictValidationAndRecovery(t *testing.T) {
	p, dir := authorProjectFixture(t)
	doc, _ := p.Read("sections/body.md")
	edit := ProjectEdit{doc.Path, doc.Source + "\nChanged\n", doc.Revision, doc.ContextRevision}
	if err := os.WriteFile(filepath.Join(dir, "graph.sir"), []byte("service api\nservice worker\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := p.Write(edit)
	var problem *ProjectError
	if !errors.As(err, &problem) || problem.Status != 409 {
		t.Fatalf("dependency conflict: %v", err)
	}
	doc, _ = p.Read("sections/body.md")
	edit = ProjectEdit{doc.Path, doc.Source + "\n<!-- slides:include absent.md -->\n", doc.Revision, doc.ContextRevision}
	if _, err := p.Write(edit); !errors.As(err, &problem) || problem.Status != 422 {
		t.Fatalf("invalid reference accepted: %v", err)
	}
	current, _ := p.Read(doc.Path)
	if current.Revision != doc.Revision {
		t.Fatal("invalid edit changed file")
	}
	edit.Source = doc.Source + "\nChanged\n"
	saved, err := p.Write(edit)
	if err != nil {
		t.Fatal(err)
	}
	current, _ = p.Read(doc.Path)
	if current.Source != edit.Source || current.Revision != saved.Revision || saved.ContextRevision == doc.ContextRevision {
		t.Fatalf("save: %+v", saved)
	}
	previous, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(saved.Recovery)))
	if err != nil || string(previous) != doc.Source {
		t.Fatalf("recoverable original: %v", err)
	}
	if _, err := p.Write(edit); !errors.As(err, &problem) || problem.Status != 409 {
		t.Fatalf("stale file edit accepted: %v", err)
	}
	current, _ = p.Read(doc.Path)
	if _, err := p.Write(ProjectEdit{current.Path, strings.ReplaceAll(current.Source, "\n", "\r\n"), current.Revision, current.ContextRevision}); !errors.As(err, &problem) || problem.Status != 422 {
		t.Fatalf("CRLF edit accepted: %v", err)
	}
}

func TestAuthorProjectSequentialRepairAndGeneratedReports(t *testing.T) {
	p, dir := authorProjectFixture(t)
	for _, file := range []string{"a.json", "b.json"} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte("{invalid}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "migration"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"tour.json", "migration/original.md", "migration/report.json"} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(strings.Repeat("x", 2*maxSourceBytes)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	index, err := p.List()
	if err != nil {
		t.Fatalf("generated reports prevented discovery: %v", err)
	}
	for _, file := range index.Files {
		if file.Path == "tour.json" || strings.HasPrefix(file.Path, "migration/") {
			t.Fatalf("private generated provenance listed: %s", file.Path)
		}
	}
	for _, file := range []string{"a.json", "b.json"} {
		doc, err := p.Read(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Write(ProjectEdit{file, "{}\n", doc.Revision, doc.ContextRevision}); err != nil {
			t.Fatalf("sequential repair of %s failed: %v", file, err)
		}
	}
	report, err := p.Diagnose("", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range report.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("repaired project still has errors: %+v", d)
		}
	}
}

func TestAuthorProjectSymlinksLimitsAndDraftKinds(t *testing.T) {
	p, dir := authorProjectFixture(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.md"), []byte("PRIVATE-OUTSIDE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err == nil {
		if _, err := p.Read("linked/outside.md"); err == nil {
			t.Fatal("directory symlink read")
		}
		if err := os.Symlink(filepath.Join(dir, "sections/body.md"), filepath.Join(dir, "alias.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Read("alias.md"); err == nil {
			t.Fatal("local symlink read")
		}
		index, err := p.List()
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range index.Files {
			if strings.HasPrefix(file.Path, "linked/") || file.Path == "alias.md" {
				t.Fatal("symlink discovered")
			}
		}
	}
	for name, source := range map[string]string{"graph.sir": "service api\nservice api\n", "story.yaml": "version: [\n", "data.json": "{"} {
		if name == "data.json" {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		report, err := p.Diagnose(name, source)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range report.Diagnostics {
			if d.File == name && d.Severity == "error" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s finding: %+v", name, report.Diagnostics)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "huge.md"), []byte(strings.Repeat("x", maxSourceBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.List(); err == nil {
		t.Fatal("oversize author file accepted")
	}
}

func TestAuthorProjectExportExcludesPrivateState(t *testing.T) {
	p, dir := authorProjectFixture(t)
	output, err := p.ExportSnapshot("handout")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(output["file"])))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE-NOTES", "PRIVATE-DRAFT", "PRIVATE-BUILD", "<!-- notes -->"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("private export %s", secret)
		}
	}
	if !strings.Contains(string(raw), "Detail") {
		t.Fatal("handout missing included content")
	}
	if _, err := p.ExportSnapshot("video"); err == nil {
		t.Fatal("external capture allowed")
	}
	if _, err := p.Read(output["file"]); err == nil {
		t.Fatal("generated private output exposed as source")
	}
}

package slides

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ExportSnapshot produces a fresh snapshot/handout with private notes excluded.
// It executes no external process and accepts no author-supplied output path.
func (p *AuthorProject) ExportSnapshot(format string) (map[string]string, error) {
	if format != "single" && format != "handout" {
		return nil, fmt.Errorf("project export supports single or handout; captured/live formats use the slides export CLI")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot()
	if err != nil {
		return nil, err
	}
	report, deck, cleanup, err := s.validate(nil)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	for _, d := range report.Diagnostics {
		if d.Severity == "error" {
			return nil, fmt.Errorf("%s", diagnosticMessage(d))
		}
	}
	if deck == nil {
		return nil, fmt.Errorf("project cannot be parsed")
	}
	out := filepath.Join(deck.Dir, ".export")
	if err := ExportStatic(deck.Dir, ExportOptions{Format: format, OutDir: out}); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(p.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	name := ".slides-export-" + hex.EncodeToString(random[:])
	if err := root.Mkdir(name, 0700); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			root.RemoveAll(name)
		}
	}()
	files, total := 0, 0
	err = filepath.WalkDir(out, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(out, file)
		if err != nil {
			return err
		}
		target := filepath.Join(name, rel)
		if entry.IsDir() {
			return root.MkdirAll(target, 0700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot output cannot contain symlinks")
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files++
		total += len(raw)
		if files > maxProjectFiles || total > maxProjectBytes {
			return fmt.Errorf("snapshot exceeds project output limits")
		}
		return root.WriteFile(target, raw, 0600)
	})
	if err != nil {
		return nil, err
	}
	committed = true
	entry := "deck.html"
	if format == "handout" {
		entry = "handout.html"
	}
	return map[string]string{"file": filepath.ToSlash(filepath.Join(name, entry)), "contextRevision": s.ContextRevision, "format": format}, nil
}

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
)

func adoptionCommandOutput(t *testing.T, run func() error) (string, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original; reader.Close(); writer.Close() }()
	done := make(chan string, 1)
	go func() { content, _ := io.ReadAll(reader); done <- string(content) }()
	err = run()
	writer.Close()
	return <-done, err
}

func TestAdoptionCommandDiscoveryAndMigration(t *testing.T) {
	output, err := adoptionCommandOutput(t, func() error { return templatesCommand([]string{"--json"}) })
	var catalog []slides.DeckTemplate
	if err != nil || json.Unmarshal([]byte(output), &catalog) != nil || len(catalog) != 3 {
		t.Fatal("template discovery", output, err)
	}
	if err := templatesCommand([]string{"--unknown"}); err == nil {
		t.Fatal("unknown discovery option accepted")
	}
	for _, args := range [][]string{nil, {"source.md"}, {"source.md", "--from", "marp"}, {"source.md", "--out", "new"}, {"source.md", "--from"}} {
		if err := migrateCommand(args); err == nil {
			t.Fatal("incomplete migration arguments accepted", args)
		}
	}
	root := t.TempDir()
	source := filepath.Join(root, "source.md")
	if err := os.WriteFile(source, []byte("# Import\n\n<!-- PRIVATE CLI NOTE -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "new")
	output, err = adoptionCommandOutput(t, func() error { return migrateCommand([]string{source, "--from=marp", "--out=" + dest, "--json"}) })
	var report slides.MarkdownMigrationReport
	if err != nil || json.Unmarshal([]byte(output), &report) != nil || report.Slides != 1 || len(report.Diagnostics) == 0 || strings.Contains(output, "PRIVATE CLI NOTE") {
		t.Fatal("CLI migration/provenance", output, err)
	}
	// Publication pins the canonical parent. macOS /var aliases and Windows
	// short names can differ from the requested spelling while identifying the
	// same directory; still require the report to name the actual output.
	reported, reportErr := os.Stat(report.Destination)
	requested, requestErr := os.Stat(dest)
	if reportErr != nil || requestErr != nil || !os.SameFile(reported, requested) {
		t.Fatal("migration report points to a different destination", report.Destination, reportErr, requestErr)
	}
}

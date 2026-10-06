//go:build linux

package slides

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestMarkdownMigrationRejectsFIFOWithoutOpeningIt(t *testing.T) {
	root := t.TempDir()
	writeCompositionFiles(t, root, map[string]string{"source.md": "# Safe\n\n![pipe](pipe.png)\n"})
	if err := unix.Mkfifo(filepath.Join(root, "pipe.png"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		report, err := MigrateMarkdown(filepath.Join(root, "source.md"), filepath.Join(root, "output"), MigrationOptions{Format: "marp"})
		if err == nil && report.Assets != 0 {
			t.Error("nonregular asset copied")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("migration blocked opening a FIFO")
	}
}

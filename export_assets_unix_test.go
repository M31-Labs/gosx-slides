//go:build !windows

package slides

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSPARejectsNonregularAssetsWithoutBlocking(t *testing.T) {
	deck := exportAssetTestDeck(t)
	if err := unix.Mkfifo(filepath.Join(deck.Dir, "public/pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	writeCompositionFiles(t, out, map[string]string{"index.html": "PREVIOUS"})
	if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil {
		t.Fatal("named pipe accepted")
	}
	if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "PREVIOUS" {
		t.Fatal("nonregular preflight changed HTML")
	}
}

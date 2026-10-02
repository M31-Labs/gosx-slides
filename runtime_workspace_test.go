package slides

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeRootRespectsGoWorkspace(t *testing.T) {
	dir := t.TempDir()
	runtime := filepath.Join(dir, "gosx")
	deck := filepath.Join(dir, "deck")
	for _, d := range []string{runtime, deck} {
		if err := os.Mkdir(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{filepath.Join(runtime, "go.mod"): "module m31labs.dev/gosx\ngo 1.26\n", filepath.Join(deck, "go.mod"): "module example.test/deck\ngo 1.26\nrequire m31labs.dev/gosx v0.57.1\n", filepath.Join(dir, "go.work"): "go 1.26\nuse (\n ./gosx\n ./deck\n)\n"}
	for p, s := range files {
		if err := os.WriteFile(p, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOWORK", filepath.Join(dir, "go.work"))
	t.Setenv("GOFLAGS", "-mod=vendor")
	got, err := resolveGoSXRoot(deck)
	if err != nil || got != runtime {
		t.Fatalf("workspace runtime root %q, %v", got, err)
	}
}

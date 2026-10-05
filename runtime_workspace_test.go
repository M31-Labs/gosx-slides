package slides

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A fresh module can return successful go-list metadata with no source Dir.
// Serving must retry writable resolution before trying to stage the runtime.
func TestRuntimeRootResolvesFreshPortableDeck(t *testing.T) {
	dir := t.TempDir()
	mod := fmt.Sprintf("module example.test/freshdeck\ngo 1.26\nrequire m31labs.dev/gosx %s\n", gosxScaffoldVersion())
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=vendor")
	root, err := resolveGoSXRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "go.mod")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("resolved root %q: %v", root, err)
	}
}

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

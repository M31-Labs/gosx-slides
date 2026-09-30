package slides

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRuntimeCacheInvalidatesGoUpgradeAtSameRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake toolchain uses a POSIX executable")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	goScript := `#!/bin/sh
case "$1" in
list) printf 'm31labs.dev/gosx v0.57.1\n';;
env) printf '{"GOROOT":"/fixed/go","GOVERSION":"%s"}\n' "$SLIDES_TEST_GOVERSION";;
*) exit 1;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(goScript), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SLIDES_TEST_GOVERSION", "go1.26.0")
	before, err := runtimeCacheKey(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLIDES_TEST_GOVERSION", "go1.26.1")
	after, err := runtimeCacheKey(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("in-place Go upgrade reused old WASM cache identity")
	}
	if root, err := deckGoRoot(dir); err != nil || root != "/fixed/go" {
		t.Fatalf("selected toolchain root changed: %q %v", root, err)
	}
}

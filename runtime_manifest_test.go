package slides

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/gosx/buildmanifest"
)

func TestStageRuntimeManifestTracksPublishedBytes(t *testing.T) {
	dir := t.TempDir()
	build := filepath.Join(dir, "build")
	if err := os.MkdirAll(build, 0755); err != nil {
		t.Fatal(err)
	}
	wasm := []byte("runtime revision one")
	js := []byte("bootstrap revision one")
	for name, data := range map[string][]byte{"gosx-runtime.wasm": wasm, "bootstrap-runtime.js": js, "bootstrap-feature-motion-transport.js": js, "bootstrap-feature-motion-svg.js": js, "bootstrap-feature-motion-scene.js": js} {
		if err := os.WriteFile(filepath.Join(build, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want []byte) {
		t.Helper()
		if err := stageRuntimeManifest(dir, build); err != nil {
			t.Fatal(err)
		}
		manifest, err := buildmanifest.Load(filepath.Join(dir, "dist", "build.json"))
		if err != nil {
			t.Fatal(err)
		}
		if got := manifest.Runtime.WASM; got.Hash != buildmanifest.ContentHash(want) || got.Size != int64(len(want)) || got.Integrity != buildmanifest.ContentIntegrity(want) || got.File != "" {
			t.Fatalf("WASM metadata does not describe compatibility asset: %+v", got)
		}
		if got := manifest.Runtime.BootstrapRuntime; got.Hash != buildmanifest.ContentHash(js) || got.Integrity != buildmanifest.ContentIntegrity(js) {
			t.Fatalf("bootstrap metadata does not match staged bytes: %+v", got)
		}
		for _, name := range []string{"motion-transport", "motion-svg", "motion-scene"} {
			if got := manifest.Runtime.Features[name]; got.Hash != buildmanifest.ContentHash(js) || got.Integrity != buildmanifest.ContentIntegrity(js) {
				t.Fatalf("%s integrity does not match staged bytes: %+v", name, got)
			}
		}
		if manifest.Runtime.WASMIslands.Hash != "" {
			t.Fatal("manifest advertises an unstaged runtime variant")
		}
	}
	check(wasm)
	wasm = []byte("runtime revision two")
	if err := os.WriteFile(filepath.Join(build, "gosx-runtime.wasm"), wasm, 0644); err != nil {
		t.Fatal(err)
	}
	check(wasm)
}

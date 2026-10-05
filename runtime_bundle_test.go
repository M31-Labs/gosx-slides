package slides

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRuntimeBundle(t *testing.T) (string, RuntimeBundle) {
	t.Helper()
	dir := t.TempDir()
	wasm := make([]byte, 1<<20)
	copy(wasm, []byte("\x00asm\x01\x00\x00\x00"))
	files := map[string][]byte{"gosx-runtime.wasm": wasm, "wasm_exec.js": []byte("test wasm bridge"), "patch.js": []byte("test patch"), "bootstrap.js": []byte("test bootstrap"), "bootstrap-lite.js": []byte("test lite"), "bootstrap-runtime.js": []byte("test runtime")}
	for _, name := range requiredRuntimeBundleAssets {
		if _, ok := files[name]; !ok {
			files[name] = []byte("test feature")
		}
	}
	manifest := RuntimeBundle{Version: 1, GoSXVersion: gosxScaffoldVersion(), GoVersion: "go1.26", Assets: make(map[string]RuntimeBundleAsset)}
	for name, content := range files {
		sum := sha256.Sum256(content)
		manifest.Assets[name] = RuntimeBundleAsset{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeTestRuntimeManifest(t, dir, manifest)
	return dir, manifest
}

func writeTestRuntimeManifest(t *testing.T, dir string, manifest RuntimeBundle) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, runtimeBundleManifest), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBundledRuntimeServesWithoutGoOrModule(t *testing.T) {
	bundle, _ := testRuntimeBundle(t)
	t.Setenv("SLIDES_RUNTIME_DIR", bundle)
	t.Setenv("PATH", t.TempDir())
	deck := t.TempDir()
	if err := os.WriteFile(filepath.Join(deck, "deck.md"), []byte("---\noffline-required: true\n---\n\n# Portable\n"), 0644); err != nil {
		t.Fatal(err)
	}
	root, err := StageRuntimeAssets(deck, false)
	if err != nil || root != deck {
		t.Fatalf("portable stage: %q, %v", root, err)
	}
	if info, err := os.Stat(filepath.Join(deck, "build", "gosx-runtime.wasm")); err != nil || info.Size() != 1<<20 {
		t.Fatalf("staged WASM: %v, %v", info, err)
	}
	report, err := Doctor(deck)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range report.Items {
		if (item.Name == "runtime" || item.Name == "go" || item.Name == "gomod") && item.Status != "ok" {
			t.Fatalf("portable prerequisite: %+v", item)
		}
	}
}

func TestBundledRuntimeRejectsCorruptionBeforeChangingDeck(t *testing.T) {
	for _, kind := range []string{"hash", "version", "path", "missing", "feature", "size", "test"} {
		t.Run(kind, func(t *testing.T) {
			bundle, manifest := testRuntimeBundle(t)
			switch kind {
			case "hash":
				if err := os.WriteFile(filepath.Join(bundle, "patch.js"), []byte("corrupted"), 0644); err != nil {
					t.Fatal(err)
				}
			case "version":
				manifest.GoSXVersion = "v0.1.0"
			case "path":
				manifest.Assets["../bootstrap-escape.js"] = manifest.Assets["patch.js"]
			case "missing":
				delete(manifest.Assets, "bootstrap.js")
			case "feature":
				delete(manifest.Assets, "bootstrap-feature-scene3d-gltf.js")
			case "test":
				manifest.Assets["bootstrap-runtime.test.js"] = manifest.Assets["patch.js"]
			case "size":
				asset := manifest.Assets["patch.js"]
				asset.Size = 96 << 20
				manifest.Assets["patch.js"] = asset
			}
			writeTestRuntimeManifest(t, bundle, manifest)
			t.Setenv("SLIDES_RUNTIME_DIR", bundle)
			deck := t.TempDir()
			if err := os.Mkdir(filepath.Join(deck, "build"), 0755); err != nil {
				t.Fatal(err)
			}
			before := []byte("previous runtime")
			file := filepath.Join(deck, "build", "patch.js")
			if err := os.WriteFile(file, before, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := StageRuntimeAssets(deck, false); err == nil {
				t.Fatal("invalid bundle accepted")
			}
			after, err := os.ReadFile(file)
			if err != nil || string(after) != string(before) {
				t.Fatalf("prior cache changed: %q, %v", after, err)
			}
		})
	}
}

func TestBundledRuntimeRejectsSymlinkSourceAndDestination(t *testing.T) {
	for _, location := range []string{"source", "build", "destination"} {
		t.Run(location, func(t *testing.T) {
			bundle, _ := testRuntimeBundle(t)
			t.Setenv("SLIDES_RUNTIME_DIR", bundle)
			deck, outside := t.TempDir(), t.TempDir()
			canary := filepath.Join(outside, "patch.js")
			if err := os.WriteFile(canary, []byte("private canary"), 0644); err != nil {
				t.Fatal(err)
			}
			var target, link string
			switch location {
			case "source":
				link, target = filepath.Join(bundle, "patch.js"), canary
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			case "build":
				link, target = filepath.Join(deck, "build"), outside
			case "destination":
				if err := os.Mkdir(filepath.Join(deck, "build"), 0755); err != nil {
					t.Fatal(err)
				}
				link, target = filepath.Join(deck, "build", "patch.js"), canary
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if _, err := StageRuntimeAssets(deck, false); err == nil {
				t.Fatal("symlink accepted")
			}
			data, err := os.ReadFile(canary)
			if err != nil || string(data) != "private canary" {
				t.Fatalf("outside file changed: %q, %v", data, err)
			}
		})
	}
}

func TestBundledRuntimeExplicitMissingFailsRatherThanBuilding(t *testing.T) {
	t.Setenv("SLIDES_RUNTIME_DIR", filepath.Join(t.TempDir(), "absent"))
	_, err := StageRuntimeAssets(t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "bundled runtime") {
		t.Fatalf("missing explicit runtime: %v", err)
	}
}

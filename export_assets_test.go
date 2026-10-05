package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exportAssetTestDeck(t *testing.T) *IslandDeck {
	t.Helper()
	deck := loadDeckFromSource(t, "# Asset policy\n", nil)
	writeCompositionFiles(t, deck.Dir, map[string]string{
		"build/gosx-runtime.wasm": "WASM", "build/bootstrap.js": "BOOTSTRAP",
		"build/.private/nested.json": "PRIVATE BUILD STATE", "build/.module-dir": "PRIVATE ROOT",
		"public/ordinary.txt": "PUBLIC", "public/nested/font.woff2": "FONT",
	})
	return deck
}

func TestSPAPublicCopiesMatchNativePolicy(t *testing.T) {
	deck := exportAssetTestDeck(t)
	private := []string{".env", ".env.local", "secrets.json", "tls.key", "credentials.json", "state.db", "state.db-wal", ".git/config", ".ssh/config"}
	for _, name := range private {
		writeCompositionFiles(t, deck.Dir, map[string]string{"public/" + name: "PRIVATE"})
	}
	app, err := deck.NewServer(ServeOptions{Static: true})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := exportSPA(deck.Dir, deck, "<html><head></head><body>Safe</body></html>", out, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range private {
		response := httptest.NewRecorder()
		app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/public/"+name, nil))
		if response.Code != http.StatusNotFound {
			t.Fatal("fixture no longer denied by native policy", name, response.Code)
		}
		if _, err := os.Lstat(filepath.Join(out, "public", name)); !os.IsNotExist(err) {
			t.Fatal("live-policy denied asset exported", name, err)
		}
	}
	for _, name := range []string{"public/ordinary.txt", "public/nested/font.woff2", "gosx/runtime.wasm", "gosx/bootstrap.js"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal("ordinary resource missing", name, err)
		}
	}
	for _, name := range []string{"gosx/.private/nested.json", "gosx/.module-dir"} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatal("private build state copied", name, err)
		}
	}
}

func TestSPARejectsSourceSymlinksBeforeWriting(t *testing.T) {
	for _, name := range []string{"public/leaked.txt", "public/link-dir", "build/leaked.js", "build/link-dir"} {
		t.Run(name, func(t *testing.T) {
			deck := exportAssetTestDeck(t)
			outside := t.TempDir()
			writeCompositionFiles(t, outside, map[string]string{"secret.txt": "PRIVATE"})
			target := filepath.Join(outside, "secret.txt")
			if strings.HasSuffix(name, "-dir") {
				target = outside
			}
			if err := os.Symlink(target, filepath.Join(deck.Dir, name)); err != nil {
				t.Skip("symlink creation unavailable", err)
			}
			out := t.TempDir()
			writeCompositionFiles(t, out, map[string]string{"index.html": "PREVIOUS"})
			if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatal("unsafe export accepted", err)
			}
			if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "PREVIOUS" {
				t.Fatal("preflight changed existing HTML")
			}
			if _, err := os.Stat(filepath.Join(out, "gosx")); !os.IsNotExist(err) {
				t.Fatal("preflight copied assets before rejecting")
			}
		})
	}
}

func TestSPARejectsOutputSymlinksAndStalePrivateAssets(t *testing.T) {
	for _, name := range []string{"index.html", "public", "public/ordinary.txt", "gosx/bootstrap.js", "public/.env", "gosx/secrets.json"} {
		t.Run(name, func(t *testing.T) {
			deck := exportAssetTestDeck(t)
			out, outside := t.TempDir(), t.TempDir()
			writeCompositionFiles(t, outside, map[string]string{"authored.txt": "AUTHORED"})
			if strings.HasSuffix(name, ".env") || strings.HasSuffix(name, "secrets.json") {
				writeCompositionFiles(t, out, map[string]string{name: "PRIVATE STALE"})
			} else {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(out, name)), 0755); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(outside, "authored.txt")
				if name == "public" {
					target = outside
				}
				if err := os.Symlink(target, filepath.Join(out, name)); err != nil {
					t.Skip("symlink creation unavailable", err)
				}
			}
			if name != "index.html" {
				writeCompositionFiles(t, out, map[string]string{"index.html": "PREVIOUS"})
			}
			if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil {
				t.Fatal("unsafe existing output accepted")
			}
			if got, _ := os.ReadFile(filepath.Join(outside, "authored.txt")); string(got) != "AUTHORED" {
				t.Fatal("outside author file overwritten")
			}
			if name != "index.html" {
				if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "PREVIOUS" {
					t.Fatal("existing HTML changed on preflight failure")
				}
			}
		})
	}
}

func TestSPAPreflightsCompositionAndOverlappingDestinations(t *testing.T) {
	deck := exportAssetTestDeck(t)
	writeCompositionFiles(t, deck.Dir, map[string]string{
		"parts/plot.svg": "<svg></svg>", "parts/secrets.json": "PRIVATE",
		".slides/packs/brand/style.css": "body{color:navy}", ".slides/packs/brand/font.woff2": "FONT",
	})
	deck.compositionAssets = map[string]string{
		"/public/_slides/parts/plot.svg":                 "parts/plot.svg",
		"/public/_slides/.slides/packs/brand/style.css":  ".slides/packs/brand/style.css",
		"/public/_slides/.slides/packs/brand/font.woff2": ".slides/packs/brand/font.woff2",
	}
	out := t.TempDir()
	if err := exportSPA(deck.Dir, deck, "first", out, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(out, "public/_slides/parts/plot.svg")); err != nil || string(got) != "<svg></svg>" {
		t.Fatal("safe composition resource missing", err)
	}
	for _, name := range []string{"style.css", "font.woff2"} {
		if _, err := os.Stat(filepath.Join(out, "public/_slides/.slides/packs/brand", name)); err != nil {
			t.Fatal("legitimate managed pack resource denied", name, err)
		}
	}
	if _, err := inlineSnapshotAssets(deck, `<link rel="stylesheet" href="/public/_slides/.slides/packs/brand/style.css">`); err != nil {
		t.Fatal("legitimate managed pack stylesheet denied", err)
	}
	deck.compositionAssets["/public/_slides/parts/secrets.json"] = "parts/secrets.json"
	if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil {
		t.Fatal("private registered pack/include asset copied")
	}
	if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "first" {
		t.Fatal("composition preflight changed HTML")
	}
	deck.compositionAssets = nil
	for _, destination := range []string{deck.Dir, filepath.Dir(deck.Dir), filepath.Join(deck.Dir, "public/site"), filepath.Join(deck.Dir, "build/site")} {
		if err := exportSPA(deck.Dir, deck, "replacement", destination, false); err == nil {
			t.Fatal("overlapping destination accepted", destination)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(deck.Dir, "public/ordinary.txt")); string(got) != "PUBLIC" {
		t.Fatal("self-copy truncated authored asset")
	}
}

func TestSnapshotAssetsRespectNativePublicPrivacyPolicy(t *testing.T) {
	deck := exportAssetTestDeck(t)
	for _, name := range []string{".env", "credentials.json", "tls.key", "state.db"} {
		writeCompositionFiles(t, deck.Dir, map[string]string{"public/" + name: "PRIVATE"})
		if _, err := inlineSnapshotAssets(deck, `<img src="/public/`+name+`">`); err == nil || !strings.Contains(err.Error(), "public asset policy") {
			t.Fatal("snapshot inlined a live-policy denied private file", name, err)
		}
	}
	writeCompositionFiles(t, deck.Dir, map[string]string{"parts/secrets.json": "PRIVATE"})
	deck.compositionAssets = map[string]string{"/public/_slides/parts/secrets.json": "parts/secrets.json"}
	if _, err := inlineSnapshotAssets(deck, `<img src="/public/_slides/parts/secrets.json">`); err == nil {
		t.Fatal("snapshot inlined private include/pack resource")
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("PRIVATE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(deck.Dir, "public/leaked.txt")); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if _, err := inlineSnapshotAssets(deck, `<img src="/public/leaked.txt">`); err == nil {
		t.Fatal("snapshot followed public asset symlink")
	}
}

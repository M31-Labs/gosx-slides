package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticNotesRequireExplicitOptIn(t *testing.T) {
	deck := loadDeckFromSource(t, "# Audience\n\nPublic wording.\n\n<!-- Private presenter wording -->\n", nil)
	if err := os.Mkdir(filepath.Join(deck.Dir, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "site")
	for _, include := range []bool{false, true} {
		app, err := deck.NewServer(ServeOptions{Static: true, IncludeNotes: include})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		app.Build().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		doc := response.Body.String()
		if response.Code != 200 || strings.Contains(doc, "Private presenter wording") != include {
			t.Fatal("static presenter notes did not match explicit inclusion", include)
		}
		if err := exportSPA(deck.Dir, deck, doc, out, include); err != nil {
			t.Fatal(err)
		}
		notes, err := os.ReadFile(filepath.Join(out, "notes.html"))
		if include {
			if err != nil || !strings.Contains(string(notes), "Private presenter wording") {
				t.Fatal("explicit notes sidecar missing", err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("default SPA published a notes sidecar", err)
		}
	}
	previous, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil {
		t.Fatal("default export retained a previously published private sidecar")
	}
	current, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if string(current) != string(previous) {
		t.Fatal("rejected private-sidecar export changed existing output")
	}
}

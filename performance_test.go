package slides

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkServeDeck(b *testing.B) {
	dir := b.TempDir()
	source := "---\ntitle: Benchmark\n---\n\n# Title\n\nA slide with {2 + 3}.\n"
	for i := 0; i < 19; i++ {
		source += "\n---\n\n# Content\n\nA paragraph and a list.\n\n- One\n- Two\n\n<!-- notes -->\n"
	}
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte(source), 0644); err != nil {
		b.Fatal(err)
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		b.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		b.Fatal(err)
	}
	handler := app.Build()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}

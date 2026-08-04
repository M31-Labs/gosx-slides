package slides

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConferenceSafeAreaRenders(t *testing.T) {
	deck := loadDeckFromSource(t, "---\naspect-ratio: 16:9\ncaption-safe-bottom: 20%\n---\n\n# Safe\n", nil)
	app, err := deck.NewServer(ServeOptions{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, want := range []string{`data-aspect-ratio="16:9"`, `data-caption-safe-bottom="20"`, `data-conference-safe-area="true"`, `calc(20vh + 1.5rem)`} {
		if !strings.Contains(body, want) {
			t.Errorf("conference rendering missing %q", want)
		}
	}
}

func TestConferenceCaptionGuideIsOptIn(t *testing.T) {
	deck := loadDeckFromSource(t, "---\ncaption-safe-bottom: 20%\ncaption-guide: true\n---\n\n# Guide\n", nil)
	if !deckConferenceConfig(deck).CaptionGuide {
		t.Fatal("caption-guide: true was not enabled")
	}
}

package slides

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

// TestDevErrorOverlay: the build-error overlay renders ONLY in dev mode and only
// when something failed, and it names the deck error + each failed island. A
// production render (dev=false) and a healthy dev render both produce nothing, so
// the overlay never ships in a normal serve or a static export.
func TestDevErrorOverlay(t *testing.T) {
	failures := map[string]error{"Counter": errors.New("boom on line 3")}

	if h := gosx.RenderHTML(devErrorOverlay(false, errors.New("deckboom"), failures)); h != "" {
		t.Errorf("overlay must be empty when not in dev (never ships): %q", h)
	}
	if h := gosx.RenderHTML(devErrorOverlay(true, nil, nil)); h != "" {
		t.Errorf("overlay must be empty for a healthy deck: %q", h)
	}

	h := gosx.RenderHTML(devErrorOverlay(true, errors.New("deckboom"), failures))
	for _, want := range []string{"deck-dev-error", "deckboom", "Counter", "boom on line 3"} {
		if !strings.Contains(h, want) {
			t.Errorf("dev overlay missing %q:\n%s", want, h)
		}
	}
}

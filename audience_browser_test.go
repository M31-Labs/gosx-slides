package slides

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Optional real-browser integration exercises numbering, stable links and
// presenter notes without changing the CLI or requiring an island runtime.
func TestAudienceBrowserNavigationAndPresenter(t *testing.T) {
	if os.Getenv("SLIDES_CHROME") == "" {
		t.Skip("set SLIDES_CHROME for audience browser integration")
	}
	dir := t.TempDir()
	writeCompositionFiles(t, dir, map[string]string{"deck.md": audienceTestSource})
	deck, err := LoadIslandDeckAudience(dir, "leaders")
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Static: true, IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Build())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.close()
	if err := browser.call("Page.navigate", map[string]any{"url": server.URL + "#finish/questions"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.wait(`window.SlidesNav && SlidesNav.current()===3 && SlidesNav.step()===1`); err != nil {
		t.Fatal("stable ID/cue did not restore variant state", err)
	}
	var result struct {
		Count   int    `json:"count"`
		ID      string `json:"id"`
		Numeric string `json:"numeric"`
	}
	if err := browser.eval(`({count:document.querySelectorAll('.slide').length,id:document.querySelector('.deck-active').dataset.slideId,numeric:document.querySelector('a[href="#3/1"]').getAttribute('href')})`, &result); err != nil || result.Count != 3 || result.ID != "finish" || result.Numeric != "#3/1" {
		t.Fatal("wrong variant DOM", result, err)
	}
	if err := browser.eval(`document.querySelector('.deck-active a[href="#intro/ready"]').click()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.wait(`SlidesNav.current()===1 && SlidesNav.step()===1`); err != nil {
		t.Fatal("stable link failed", err)
	}
	if err := browser.eval(`document.querySelector('a[href="#3/1"]').click()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.wait(`SlidesNav.current()===3 && SlidesNav.step()===1`); err != nil {
		t.Fatal("remapped numeric link failed", err)
	}
	if err := browser.call("Page.navigate", map[string]any{"url": server.URL + "?present#3present"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.wait(`window.SlidesNav && SlidesNav.isPresenter() && document.querySelector('.pv-notes-body')?.textContent.includes('final notes')`); err != nil {
		t.Fatal("presenter did not show selected notes", err)
	}
	var counter string
	if err := browser.eval(`document.querySelector('.pv-counter').textContent`, &counter); err != nil || !strings.HasPrefix(counter, "3 / 3") {
		t.Fatal("presenter numbering failed", counter, err)
	}
}

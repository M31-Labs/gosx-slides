package slides

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestCaptureStillCompletesUncuedSplitText(t *testing.T) {
	if os.Getenv("SLIDES_CHROME") == "" {
		t.Skip("set SLIDES_CHROME for captured motion integration")
	}
	deck := loadDeckFromSource(t, "---\ntitle: Split capture\noffline-required: true\n---\n\n# Complete the entrance\n\n:::motion {preset=slide-up split=word duration=500 stagger=70 replay=slide}\nOne clear thought. One word at a time.\n:::\n", nil)
	app, err := deck.NewServer(ServeOptions{Static: true, StageRuntime: true})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Build())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.close()
	if err := browser.call("Page.navigate", map[string]any{"url": server.URL}, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.wait(`window.SlidesMotion && document.querySelectorAll('.deck-active .gosx-motion-unit').length===8`); err != nil {
		t.Fatal(err)
	}
	// Pause while the ordinary, uncued native entrance is still in progress.
	if err := browser.eval(`SlidesMotion.seek(0);true`, nil); err != nil {
		t.Fatal(err)
	}
	if err := browser.eval(captureStillPose, nil); err != nil {
		t.Fatal(err)
	}
	var pose struct {
		Duration float64 `json:"duration"`
		Visible  int     `json:"visible"`
	}
	if err := browser.eval(`({duration:SlidesMotion.duration(),visible:Array.from(document.querySelectorAll('.deck-active .gosx-motion-unit')).filter(el=>Number(getComputedStyle(el).opacity)>=0.999).length})`, &pose); err != nil {
		t.Fatal(err)
	}
	if pose.Duration < 990 || pose.Visible != 8 {
		t.Fatalf("staggered still is incomplete: %+v", pose)
	}
}

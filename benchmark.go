package slides

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"
)

// BrowserBenchmark measures cached server setup and fresh-profile browser
// startup separately. It never counts staging as browser time or asserts FPS.
type BrowserBenchmark struct {
	ServerMillis float64         `json:"serverMillis"`
	Slides       int             `json:"slides"`
	Runs         []BrowserSample `json:"runs"`
}
type BrowserSample struct {
	ReadyMillis     float64 `json:"readyMillis"`
	HeapBytes       int64   `json:"heapBytes"`
	DOMNodes        int64   `json:"domNodes"`
	TransferBytes   int64   `json:"transferBytes"`
	HydratedIslands int     `json:"hydratedIslands"`
}

func BenchmarkBrowser(dir string, runs int) (*BrowserBenchmark, error) {
	if runs < 1 || runs > 20 {
		return nil, fmt.Errorf("benchmark runs must be 1–20")
	}
	start := time.Now()
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		return nil, err
	}
	app, err := deck.NewServer(ServeOptions{StageRuntime: true, Static: true})
	if err != nil {
		return nil, err
	}
	report := &BrowserBenchmark{ServerMillis: float64(time.Since(start).Microseconds()) / 1000, Slides: len(deck.Slides)}
	server := httptest.NewServer(app.Build())
	defer server.Close()
	for i := 0; i < runs; i++ {
		sample, err := benchmarkSample(server.URL)
		if err != nil {
			return nil, fmt.Errorf("benchmark run %d: %w", i+1, err)
		}
		report.Runs = append(report.Runs, sample)
	}
	return report, nil
}
func benchmarkSample(url string) (BrowserSample, error) {
	sample := BrowserSample{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		return sample, err
	}
	defer browser.close()
	if err = browser.call("Performance.enable", map[string]any{}, nil); err != nil {
		return sample, err
	}
	if err = browser.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 720, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		return sample, err
	}
	start := time.Now()
	if err = browser.call("Page.navigate", map[string]any{"url": url}, nil); err != nil {
		return sample, err
	}
	if err = browser.wait(`document.readyState==='complete' && !!window.SlidesNav && (!document.getElementById('gosx-manifest') || !!(window.__gosx && window.__gosx.ready)) && !['pending','error'].includes(document.querySelector('.deck-active')?.dataset.slideHydration) && Array.from(document.querySelectorAll('.deck-active img')).every(img=>img.complete) && Array.from(document.querySelectorAll('.deck-active .slide-graphic,.deck-background-active[data-gosx-scene3d]')).every(g=>g.__gosxScene3DHandle?.__gosxScene3DCommandReady)`); err != nil {
		return sample, err
	}
	sample.ReadyMillis = float64(time.Since(start).Microseconds()) / 1000
	if err = browser.eval(`({transferBytes:performance.getEntriesByType('resource').reduce((n,e)=>n+e.transferSize,0)+performance.getEntriesByType('navigation').reduce((n,e)=>n+e.transferSize,0),hydratedIslands:window.SlidesRuntime ? SlidesRuntime.stats().hydrated : (window.__gosx?.islands?.size || 0)})`, &sample); err != nil {
		return sample, err
	}
	var metrics struct {
		Metrics []struct {
			Name  string
			Value float64
		}
	}
	if err = browser.call("Performance.getMetrics", map[string]any{}, &metrics); err != nil {
		return sample, err
	}
	for _, m := range metrics.Metrics {
		switch m.Name {
		case "JSHeapUsedSize":
			sample.HeapBytes = int64(m.Value)
		case "Nodes":
			sample.DOMNodes = int64(m.Value)
		}
	}
	return sample, nil
}

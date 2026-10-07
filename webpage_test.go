package slides

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func webTestDeck(t *testing.T, settings, props string) *IslandDeck {
	t.Helper()
	d := loadDeckFromSource(t, "---\ntheme: paper\n"+settings+"\n---\n\n# A web page\n\n<WebPage "+props+"/>\n", nil)
	return d
}

func webTestSnapshot(t *testing.T, d *IslandDeck) WebSnapshot {
	t.Helper()
	var p WebPageProps
	for _, p = range d.web.pages {
		break
	}
	picture := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			picture.Set(x, y, color.RGBA{30, 100, 190, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, picture); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(buf.Bytes())
	name := "web-" + hex.EncodeToString(hash[:]) + ".png"
	s := WebSnapshot{URL: p.Src, FinalURL: p.Src, CapturedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), Image: name, SHA256: hex.EncodeToString(hash[:]), Width: p.Width, Height: p.Height, Scroll: p.Scroll, AllowedHosts: d.web.hosts}
	d.web.manifest.Snapshots[webKey(p)] = s
	if err := writeWebAsset(d.Dir, name, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(d.web.manifest)
	if err := writeWebAsset(d.Dir, "manifest.json", data); err != nil {
		t.Fatal(err)
	}
	return s
}

func webTestResponse(t *testing.T, d *IslandDeck, opts ServeOptions) *httptest.ResponseRecorder {
	t.Helper()
	app, err := d.NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://deck.example/", nil))
	if rec.Code != 200 {
		t.Fatalf("response %d: %s", rec.Code, rec.Body.String())
	}
	return rec
}

func TestWebPagePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, settings, src string
		live                bool
		csp                 string
	}{
		{"allowed", "web-allow: [example.com]", "https://example.com/demo", true, "frame-src https://example.com;"},
		{"unlisted", "web-allow: [example.com]", "https://other.example/", false, "frame-src https://example.com;"},
		{"subdomain", "web-allow: [example.com]", "https://sub.example.com/", false, "frame-src https://example.com;"},
		{"suffix", "web-allow: [example.com]", "https://example.com.evil.example/", false, "frame-src https://example.com;"},
		{"port", "web-allow: [example.com]", "https://example.com:8443/", false, "frame-src https://example.com;"},
		{"offline", "web-allow: [example.com]\noffline-required: true", "https://example.com/", false, "frame-src 'none';"},
		{"empty", "", "https://example.com/", false, "frame-src 'none';"},
		{"sorted", "web-allow:\n  - z.example\n  - example.com\n  - example.com", "https://example.com/", true, "frame-src https://example.com https://z.example;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := webTestDeck(t, tc.settings, fmt.Sprintf(`src=%q`, tc.src))
			s := webTestSnapshot(t, d)
			rec := webTestResponse(t, d, ServeOptions{})
			body := rec.Body.String()
			if strings.Contains(body, `data-web-src="`) != tc.live {
				t.Fatalf("live candidate mismatch")
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != tc.csp {
				t.Fatalf("CSP %q; want %q", got, tc.csp)
			}
			if !strings.Contains(body, s.Image) || !strings.Contains(body, s.CapturedAt) {
				t.Fatal("snapshot or caption missing")
			}
			if strings.Contains(renderSlidesHTML(t, d), "<iframe") {
				t.Fatal("frame must only mount on the active audience slide")
			}
			if _, failures := d.compileComponents(); len(failures) != 0 {
				t.Fatal(failures)
			}
		})
	}
}

func TestWebPageRejectsUnsafeInput(t *testing.T) {
	for _, src := range []string{"http://example.com/", "javascript:alert(1)", "data:text/html,hi", "//example.com/", "https://user:password@example.com/", "https://example.com:0/", "https://example.com:65536/", "https://example.com/\nfoo", "https://*.example.com/", "https://example.com:/", "https://a..example/"} {
		if _, err := webProps(fmt.Sprintf(`src=%q`, src)); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
	for _, props := range []string{`src="https://example.com" Scripts="true"`, `src="https://example.com" Width={20}`, `src="https://example.com" allow="*"`, `src="https://example.com" Src="https://other.example"`} {
		if _, err := webProps(props); err == nil {
			t.Errorf("accepted %s", props)
		}
	}
	for _, host := range []string{"https://example.com", "*.example.com", "example.com/path", "example.com; script-src *", "user@example.com"} {
		src := fmt.Sprintf("---\nweb-allow: [%q]\n---\n\n<WebPage src=\"https://example.com\"/>\n", host)
		if _, err := parseIslandDeck(t.TempDir(), []byte(src)); err == nil {
			t.Errorf("accepted allowlist %q", host)
		}
	}
}

func TestWebPageFramingPolicies(t *testing.T) {
	for _, tc := range []struct {
		xfo, csp string
		blocked  bool
	}{
		{"", "", false}, {"DENY", "", true}, {"SAMEORIGIN", "", true},
		{"", "default-src 'self'", false}, {"", "frame-ancestors 'none'", true}, {"", "frame-ancestors 'self'", true},
		{"", "frame-ancestors https://deck.example", true}, {"DENY", "frame-ancestors *", false},
		{"", "frame-ancestors *\nframe-ancestors 'none'", true}, {"", "frame-ancestors *, frame-ancestors 'self'", true},
	} {
		if got := webFramingBlocked(tc.xfo, tc.csp); got != tc.blocked {
			t.Errorf("%q %q: blocked=%v", tc.xfo, tc.csp, got)
		}
	}
	d := webTestDeck(t, "web-allow: [example.com]", `Src="https://example.com" Scripts={true} SameOrigin={true} Popups={true}`)
	s := webTestSnapshot(t, d)
	if body := renderSlidesHTML(t, d); !strings.Contains(body, `data-web-sandbox="allow-scripts allow-same-origin allow-popups"`) {
		t.Fatal("typed capability props were not applied")
	}
	for key, value := range d.web.manifest.Snapshots {
		value.XFrameOptions = "DENY"
		d.web.manifest.Snapshots[key] = value
	}
	if body := renderSlidesHTML(t, d); strings.Contains(body, `data-web-src="`) || !strings.Contains(body, s.Image) {
		t.Fatal("DENY must use snapshot")
	}
	for key, value := range d.web.manifest.Snapshots {
		value.XFrameOptions = ""
		value.FinalURL = "https://other.example/"
		d.web.manifest.Snapshots[key] = value
	}
	if body := renderSlidesHTML(t, d); strings.Contains(body, `data-web-src="`) {
		t.Fatal("unlisted final redirect origin must use snapshot")
	}
	if csp := d.webCSP("example.com"); csp != "frame-src 'none';" {
		t.Fatal("own origin must not be frameable", csp)
	}
}

func TestWebPageAssetPathsAndPresenterControls(t *testing.T) {
	d := webTestDeck(t, "web-allow: [example.com]", `src="https://example.com/"`)
	s := webTestSnapshot(t, d)
	for _, key := range []string{"manifest.json", s.Image} {
		path := filepath.Join(d.Dir, "public", "webpages", key)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadIslandDeck(d.Dir); err == nil {
			t.Fatal("nonregular asset accepted")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		webTestSnapshot(t, d)
	}
	b := newPresenterBroker()
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"index":0,"web":{"page":0,"action":"zoom","value":"100"}}`, 204},
		{`{"index":0,"web":{"page":0,"action":"lock","value":"false"}}`, 204},
		{`{"index":0,"web":{"page":0,"action":"reload","value":""}}`, 204},
		{`{"index":0,"web":{"page":64,"action":"reload","value":""}}`, 400},
		{`{"index":0,"web":{"page":0,"action":"navigate","value":"https://other.example"}}`, 400},
		{`{"index":0,"web":{"page":0,"action":"zoom","value":"script"}}`, 400},
	} {
		req := httptest.NewRequest(http.MethodPost, "/presenter/state", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		b.handleState(rec, req)
		if rec.Code != tc.status {
			t.Errorf("control status %d want %d", rec.Code, tc.status)
		}
	}
}

func TestWebPageRawIframeStillDropped(t *testing.T) {
	d := loadDeckFromSource(t, "# Raw\n\n<iframe src=\"https://example.com\">IFRAME_SECRET</iframe>\n\n<script>SCRIPT_SECRET</script>\n\n<form>FORM_SECRET</form>\n", nil)
	body := renderSlidesHTML(t, d)
	for _, bad := range []string{"<iframe", "IFRAME_SECRET", "<script", "SCRIPT_SECRET", "<form", "FORM_SECRET"} {
		if strings.Contains(body, bad) {
			t.Errorf("sanitizer retained %s", bad)
		}
	}
}

func TestWebPageStaticExportsReuseSnapshot(t *testing.T) {
	d := webTestDeck(t, "web-allow: [example.com]\noffline-required: true", `src="https://example.com/"`)
	s := webTestSnapshot(t, d)
	for _, format := range []string{"single", "handout", "spa"} {
		t.Run(format, func(t *testing.T) {
			out := t.TempDir()
			if err := ExportStatic(d.Dir, ExportOptions{Format: format, OutDir: out}); err != nil {
				t.Fatal(err)
			}
			name := "deck.html"
			if format == "handout" {
				name = "handout.html"
			}
			if format == "spa" {
				name = "index.html"
			}
			data, err := os.ReadFile(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			body := string(data)
			if strings.Contains(body, `data-web-src="`) || strings.Contains(body, `data-slides-webpage`) || !strings.Contains(body, s.CapturedAt) {
				t.Fatal("export must retain caption and disable live controller")
			}
			if format == "spa" {
				if !strings.Contains(body, "public/webpages/"+s.Image) {
					t.Fatal("SPA snapshot missing")
				}
				if _, err := os.Stat(filepath.Join(out, "public", "webpages", s.Image)); err != nil {
					t.Fatal(err)
				}
			} else if !strings.Contains(body, "data:image/png;base64,") {
				t.Fatal("snapshot pixels must be embedded")
			}
		})
	}
	first := webTestResponse(t, d, ServeOptions{Static: true})
	if strings.Contains(first.Body.String(), `data-web-src="`) {
		t.Fatal("static server live")
	}
	if len(DeckComponents(d)) != 0 {
		t.Fatal("built-in must not require a .gsx source")
	}
	var p WebPageProps
	for _, p = range d.web.pages {
		break
	}
	if err := os.WriteFile(filepath.Join(d.Dir, "public", "webpages", s.Image), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIslandDeck(d.Dir); err == nil {
		t.Fatal("tampered pixels accepted")
	}
	delete(d.web.manifest.Snapshots, webKey(p))
	if err := d.captureWebSnapshots(false, true); err == nil {
		t.Fatal("missing offline snapshot accepted")
	}
}

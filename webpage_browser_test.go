package slides

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This test uses only local HTTPS fixtures. The certificate exception is set
// on this test browser through CDP; production capture always verifies TLS.
func TestWebPageBrowser(t *testing.T) {
	if _, err := findChrome(); err != nil {
		if os.Getenv("SLIDES_WEB_EVIDENCE") != "" {
			t.Fatal("web page browser gate requires Chrome", err)
		}
		t.Skip("Chrome is optional")
	}
	var requests, scriptRuns atomic.Int64
	var slow atomic.Bool
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ran" {
			scriptRuns.Add(1)
			return
		}
		// Full Chrome requests favicons after top-level capture navigation.
		// Page-owned resources can carry their own referrer and arrive after
		// navigation; only document requests measure frame privacy/lifetime.
		if r.URL.Path != "/allows" && r.URL.Path != "/deny" && r.URL.Path != "/ancestors" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		requests.Add(1)
		if slow.Load() {
			time.Sleep(500 * time.Millisecond)
		}
		if r.Header.Get("Referer") != "" {
			t.Errorf("document %s leaked referrer", r.URL.Path)
		}
		switch r.URL.Path {
		case "/deny":
			w.Header().Set("X-Frame-Options", "DENY")
		case "/ancestors":
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><style>body{margin:0;padding:40px;background:#eef4ff;color:#183153;font:24px system-ui}h1{color:#245bb0}.box{padding:30px;border:2px solid #245bb0;border-radius:16px}footer{margin-top:1200px}</style></head><body><h1>Embedded web page</h1><div class="box">A local HTTPS fixture with a recorded snapshot.</div><footer>Scrolling capture</footer><script>fetch('/ran').catch(()=>{});</script></body></html>`)
	}))
	defer remote.Close()
	host, _ := url.Parse(remote.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	b, err := startCaptureBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	for _, method := range []string{"Page.enable", "Network.enable"} {
		if err := b.call(method, map[string]any{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.call("Security.setIgnoreCertificateErrors", map[string]any{"ignore": true}, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := fmt.Sprintf("---\ntitle: Web page fixtures\ntheme: paper\ntransition: none\nweb-allow: [%q]\n---\n\n# Live web page\n\n<WebPage src=%q Width={960} Height={540}/>\n\n<!-- Live controls -->\n\n---\n\n# Framing refused\n\n<WebPage src=%q Width={960} Height={540}/>\n", host.Host, remote.URL+"/allows", remote.URL+"/deny")
	if err := os.WriteFile(filepath.Join(dir, "deck.md"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.web.pages {
		s, pixels, err := captureWebPage(b, p)
		if err != nil {
			t.Fatal(err)
		}
		if s.FramingBlocked != strings.HasSuffix(p.Src, "/deny") {
			t.Fatalf("framing metadata: %+v", s)
		}
		s.AllowedHosts = d.web.hosts
		d.web.manifest.Snapshots[webKey(p)] = s
		if err := writeWebAsset(dir, s.Image, pixels); err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(pixels))
		if err != nil || config.Width != 960 || config.Height != 540 {
			t.Fatal("viewport screenshot dimensions", config, err)
		}
	}
	// CSP denial and bounded scrolling are recorded without bypassing headers.
	p := WebPageProps{Src: remote.URL + "/ancestors", Width: 960, Height: 540, Scroll: true}
	s, pixels, err := captureWebPage(b, p)
	if err != nil {
		t.Fatal(err)
	}
	if !s.FramingBlocked || s.CSP != "frame-ancestors 'none'" {
		t.Fatal("CSP ancestor denial not recorded")
	}
	config, err := png.DecodeConfig(bytes.NewReader(pixels))
	if err != nil || config.Height <= 540 || config.Height > 1620 {
		t.Fatal("bounded scrolling screenshot", config, err)
	}
	manifest, _ := json.Marshal(d.web.manifest)
	if err := writeWebAsset(dir, "manifest.json", manifest); err != nil {
		t.Fatal(err)
	}
	app, err := d.NewServer(ServeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Build())
	defer func() { b.close(); server.Close() }()
	// Remove external font requests from this isolated test; no internet needed.
	if err := b.call("Network.setBlockedURLs", map[string]any{"urls": []string{"*fonts.googleapis.com*", "*fonts.gstatic.com*"}}, nil); err != nil {
		t.Fatal(err)
	}
	// Retain the real streams so a delayed control can be dispatched directly
	// into their handlers, independently of network timing.
	if err := b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `window.__webTestSources=[];window.EventSource=class extends EventSource{constructor(...args){super(...args);window.__webTestSources.push(this);}};`}, nil); err != nil {
		t.Fatal(err)
	}
	navigate := func(target, ready string) {
		t.Helper()
		if err := b.call("Page.navigate", map[string]any{"url": target}, nil); err != nil {
			t.Fatal(err)
		}
		if err := b.wait(ready); err != nil {
			t.Fatal(err)
		}
	}
	eval := func(expression string) {
		t.Helper()
		var ok bool
		if err := b.eval(expression, &ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("browser assertion failed:", expression)
		}
	}
	requests.Store(0)
	scriptRuns.Store(0)
	slow.Store(true)
	navigate(server.URL, `!!window.SlidesNav && !!document.querySelector('.deck-active iframe') && !document.querySelector('.deck-active [data-web-loaded="true"]')`)
	eval(`getComputedStyle(document.querySelector('.deck-active .webpage-snapshot')).visibility==='visible'`)
	if err := b.wait(`!!document.querySelector('.deck-active [data-web-loaded="true"]')`); err != nil {
		t.Fatal(err)
	}
	slow.Store(false)
	eval(`document.querySelectorAll('iframe').length===1 && document.querySelector('iframe').getAttribute('sandbox')==='' && document.querySelector('iframe').referrerPolicy==='no-referrer' && document.querySelector('iframe').loading==='lazy' && document.querySelector('iframe').inert && document.querySelector('iframe').tabIndex===-1 && !document.querySelector('iframe').hasAttribute('allowfullscreen')`)
	if scriptRuns.Load() != 0 {
		t.Fatal("default sandbox ran scripts")
	}
	eval(`(()=>{document.querySelector('[data-web-action="zoom"]').click();document.querySelector('[data-web-action="lock"]').click();return document.querySelector('[data-web-page]').dataset.webZoom==='100' && document.querySelector('[data-web-page]').dataset.webUnlocked==='true';})()`)
	eval(`!document.querySelector('iframe').inert && document.querySelector('iframe').tabIndex===0`)
	before := requests.Load()
	eval(`(()=>{document.querySelector('[data-web-action="reload"]').click();return true;})()`)
	if err := b.wait(`!!document.querySelector('[data-web-loaded="true"]')`); err != nil {
		t.Fatal(err)
	}
	if requests.Load() <= before {
		t.Fatal("reload did not request the page")
	}
	// Capture representative delivered states; CI retains these as artifacts.
	evidence := os.Getenv("SLIDES_WEB_EVIDENCE")
	save := func(name string) {
		t.Helper()
		if evidence == "" {
			return
		}
		if err := os.MkdirAll(evidence, 0755); err != nil {
			t.Fatal(err)
		}
		pixels, err := b.png()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, name+".png"), pixels, 0644); err != nil {
			t.Fatal(err)
		}
	}
	eval(`(()=>{document.querySelector('[data-web-action="zoom"]').click();return true;})()`)
	save("live-desktop")
	eval(`(()=>{SlidesNav.show(1,0,false);return document.querySelectorAll('iframe').length===0 && !!document.querySelector('.deck-active .webpage-snapshot');})()`)
	eval(`(()=>{const data=JSON.stringify({index:0,step:0,source:'delayed-web-control',sequence:1,web:{page:0,action:'reload',value:''}});window.__webTestSources.forEach(stream=>stream.dispatchEvent(new MessageEvent('state',{data})));return SlidesNav.current()===2 && document.querySelectorAll('iframe').length===0;})()`)
	save("framing-denied")
	before = requests.Load()
	navigate(server.URL+"?present#1", `!!window.SlidesNav && SlidesNav.isPresenter() && !!document.querySelector('.pv-current .webpage-snapshot')`)
	eval(`document.querySelectorAll('iframe').length===0 && !!document.querySelector('.pv-next .webpage-snapshot')`)
	eval(`document.querySelectorAll('.webpage-presenter-controls [data-web-action]').length===3 && getComputedStyle(document.querySelector('.webpage-presenter-controls')).fontSize==='14px'`)
	save("presenter")
	if requests.Load() != before {
		t.Fatal("presenter must not request remote pages")
	}
	navigate(server.URL, `!!document.querySelector('[data-web-loaded="true"]')`)
	if err := b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.eval(`(async()=>{dispatchEvent(new Event('resize'));await new Promise(r=>setTimeout(r,200));return true;})()`, nil); err != nil {
		t.Fatal(err)
	}
	save("live-mobile")
	eval(`(()=>{SlidesNav.openOverview();return true;})()`)
	if err := b.wait(`document.querySelectorAll('iframe').length===0`); err != nil {
		t.Fatal(err)
	}
	// A separate browser profile exercises server synchronization rather than
	// relying on a same-machine BroadcastChannel shared by the two windows.
	speaker, err := startCaptureBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer speaker.close()
	if err := speaker.call("Page.navigate", map[string]any{"url": server.URL + "?present#1"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := speaker.wait(`!!window.SlidesNav && SlidesNav.isPresenter() && !!document.querySelector('.pv-current [data-web-action="zoom"]')`); err != nil {
		t.Fatal(err)
	}
	eval(`(()=>{SlidesNav.closeOverview();return true;})()`)
	if err := b.wait(`!!document.querySelector('[data-web-loaded="true"]')`); err != nil {
		t.Fatal(err)
	}
	if err := speaker.eval(`(()=>{const page=document.querySelector('.pv-current [data-web-page]');page.querySelector('[data-web-action="zoom"]').click();return page.dataset.webZoom;})()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.wait(`document.querySelector('.deck-active [data-web-page]').dataset.webZoom==='100'`); err != nil {
		t.Fatal(err)
	}
	if err := speaker.eval(`(()=>{document.querySelector('.pv-current [data-web-action="lock"]').click();return true;})()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.wait(`document.querySelector('.deck-active [data-web-page]').dataset.webUnlocked==='true'`); err != nil {
		t.Fatal(err)
	}
	if err := speaker.eval(`(()=>{SlidesNav.show(1,0,true);return true;})()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.wait(`SlidesNav.current()===2 && document.querySelectorAll('iframe').length===0`); err != nil {
		t.Fatal(err)
	}
	var frames int
	if err := speaker.eval(`document.querySelectorAll('iframe').length`, &frames); err != nil || frames != 0 {
		t.Fatal("speaker created a live frame", err)
	}
	speaker.close()
	// Every export uses recorded pixels, even when the page could frame live.
	before = requests.Load()
	if err := d.captureWebSnapshots(false, false); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"pdf", "pptx", "pptx-editable"} {
		editable := format == "pptx-editable"
		suffix := format
		if editable {
			suffix = "pptx"
		}
		out := filepath.Join(t.TempDir(), "deck."+format)
		if err := ExportStatic(dir, ExportOptions{Format: suffix, OutDir: out, Editable: editable}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if format == "pdf" {
			if !bytes.HasPrefix(data, []byte("%PDF")) || !bytes.Contains(data, []byte("/Subtype /Image")) {
				t.Fatal("PDF snapshot image missing")
			}
		} else {
			archive, err := zip.OpenReader(out)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, f := range archive.File {
				if strings.HasPrefix(f.Name, "ppt/media/") && strings.HasSuffix(f.Name, ".png") {
					count++
					stream, _ := f.Open()
					img, _ := io.ReadAll(stream)
					stream.Close()
					if _, err := png.DecodeConfig(bytes.NewReader(img)); err != nil {
						t.Fatal(err)
					}
				}
			}
			archive.Close()
			if count < 2 {
				t.Fatal("PPTX snapshot backgrounds missing")
			}
		}
	}
	if requests.Load() != before {
		t.Fatal("PDF/PPTX export made a remote request")
	}
}

package slides

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// RefreshWebSnapshots captures missing pages, or every referenced page when
// refresh is true. Capture opens each HTTPS URL directly in an isolated, muted
// Chrome profile. It never proxies a page or modifies its framing headers.
func RefreshWebSnapshots(dir string, refresh bool) error {
	d, err := LoadIslandDeck(dir)
	if err != nil {
		return err
	}
	return d.captureWebSnapshots(refresh, false)
}

func (d *IslandDeck) captureWebSnapshots(refresh, reuseOnly bool) error {
	if d.web == nil || len(d.web.pages) == 0 {
		return nil
	}
	pages := map[string]WebPageProps{}
	for _, p := range d.web.pages {
		key := webKey(p)
		if _, ok := d.web.manifest.Snapshots[key]; !ok || refresh {
			pages[key] = p
		}
	}
	if len(pages) == 0 {
		return nil
	}
	if reuseOnly {
		return fmt.Errorf("WebPage snapshots are missing; run slides web refresh before an offline or author-tool export")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	b, err := startCaptureBrowser(ctx)
	if err != nil {
		return err
	}
	defer b.close()
	if err := b.call("Page.enable", map[string]any{}, nil); err != nil {
		return err
	}
	if err := b.call("Network.enable", map[string]any{}, nil); err != nil {
		return err
	}
	if err := b.call("Network.setBlockedURLs", map[string]any{"urls": []string{"http://*", "file://*", "ftp://*"}}, nil); err != nil {
		return err
	}
	keys := make([]string, 0, len(pages))
	for key := range pages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Publish the manifest only after every requested page succeeds. Existing
	// captures remain authoritative if refresh fails; new unreferenced PNGs are safe.
	manifest := webManifest{Version: 1, Snapshots: map[string]WebSnapshot{}}
	for key, s := range d.web.manifest.Snapshots {
		manifest.Snapshots[key] = s
	}
	for _, key := range keys {
		p := pages[key]
		s, pixels, err := captureWebPage(b, p)
		if err != nil {
			return fmt.Errorf("capture WebPage %s: %w", p.Src, err)
		}
		s.AllowedHosts = append([]string{}, d.web.hosts...)
		if err := writeWebAsset(d.Dir, s.Image, pixels); err != nil {
			return err
		}
		manifest.Snapshots[key] = s
	}
	if len(manifest.Snapshots) > 256 {
		return fmt.Errorf("WebPage manifest supports at most 256 captures; remove unused entries before refreshing")
	}
	root, err := os.OpenRoot(d.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var total int64
	seen := map[string]bool{}
	for _, p := range d.web.pages {
		s := manifest.Snapshots[webKey(p)]
		if seen[s.Image] {
			continue
		}
		seen[s.Image] = true
		info, err := root.Stat("public/webpages/" + s.Image)
		if err != nil {
			return err
		}
		total += info.Size()
		if total > snapshotTotalLimit {
			return fmt.Errorf("WebPage snapshots exceed the 96 MiB asset budget")
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > 1<<20 {
		return fmt.Errorf("WebPage manifest exceeds 1 MiB")
	}
	if err := writeWebAsset(d.Dir, "manifest.json", append(data, '\n')); err != nil {
		return err
	}
	d.web.manifest = manifest
	return nil
}

func writeWebAsset(dir, name string, data []byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, path := range []string{"public", "public/webpages"} {
		if err := root.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("WebPage asset directory must be a regular directory")
		}
	}
	if name != "manifest.json" && !webImagePattern.MatchString(name) {
		return fmt.Errorf("invalid WebPage asset name")
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	tmp := "public/webpages/.capture-" + hex.EncodeToString(random[:]) + ".tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return root.Rename(tmp, "public/webpages/"+name)
}

func captureWebPage(b *captureBrowser, p WebPageProps) (WebSnapshot, []byte, error) {
	s := WebSnapshot{URL: p.Src, Width: p.Width, Height: p.Height, Scroll: p.Scroll}
	var response struct {
		URL     string         `json:"url"`
		Status  int            `json:"status"`
		Headers map[string]any `json:"headers"`
	}
	responses := map[string]json.RawMessage{}
	b.onEvent = func(method string, params json.RawMessage) {
		if method != "Network.responseReceived" {
			return
		}
		var event struct {
			Type     string          `json:"type"`
			FrameID  string          `json:"frameId"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(params, &event) == nil && event.Type == "Document" {
			responses[event.FrameID] = event.Response
		}
	}
	defer func() { b.onEvent = nil }()
	if err := b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": p.Width, "height": p.Height, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		return s, nil, err
	}
	// Clear the previous document so readiness can never refer to an old page.
	if err := b.call("Page.navigate", map[string]any{"url": "about:blank"}, nil); err != nil {
		return s, nil, err
	}
	if err := b.wait(`location.href==='about:blank'`); err != nil {
		return s, nil, err
	}
	var nav struct {
		FrameID   string `json:"frameId"`
		ErrorText string `json:"errorText"`
	}
	if err := b.call("Page.navigate", map[string]any{"url": p.Src}, &nav); err != nil {
		return s, nil, err
	}
	if nav.ErrorText != "" {
		return s, nil, fmt.Errorf("page navigation failed: %s", nav.ErrorText)
	}
	if err := b.wait(`location.protocol==='https:' && document.readyState==='complete'`); err != nil {
		return s, nil, err
	}
	if err := b.eval(`(async()=>{await Promise.race([document.fonts.ready,new Promise(r=>setTimeout(r,2000))]);await new Promise(r=>setTimeout(r,300));return true;})()`, nil); err != nil {
		return s, nil, err
	}
	if err := json.Unmarshal(responses[nav.FrameID], &response); err != nil {
		return s, nil, fmt.Errorf("Chrome did not record the page response")
	}
	if response.Status < 200 || response.Status >= 300 {
		return s, nil, fmt.Errorf("page returned HTTP %d", response.Status)
	}
	u, err := webURL(response.URL)
	if err != nil {
		return s, nil, err
	}
	s.FinalURL = u.String()
	for name, value := range response.Headers {
		if text, ok := value.(string); ok {
			switch strings.ToLower(name) {
			case "x-frame-options":
				s.XFrameOptions = text
			case "content-security-policy":
				s.CSP = text
			}
		}
	}
	s.FramingBlocked = webFramingBlocked(s.XFrameOptions, s.CSP)
	height := p.Height
	if p.Scroll {
		if err := b.eval(fmt.Sprintf(`(async()=>{const end=Math.min(document.documentElement.scrollHeight,%d);for(let y=0;y<end;y+=%d){scrollTo(0,y);await new Promise(r=>setTimeout(r,100));}scrollTo(0,0);return Math.min(end,Math.max(innerHeight,document.documentElement.scrollHeight));})()`, p.Height*3, p.Height), &height); err != nil {
			return s, nil, err
		}
	}
	var shot struct {
		Data string `json:"data"`
	}
	params := map[string]any{"format": "png", "captureBeyondViewport": p.Scroll}
	if p.Scroll {
		params["clip"] = map[string]any{"x": 0, "y": 0, "width": p.Width, "height": height, "scale": 1}
	}
	if err := b.call("Page.captureScreenshot", params, &shot); err != nil {
		return s, nil, err
	}
	pixels, err := base64.StdEncoding.DecodeString(shot.Data)
	if err != nil {
		return s, nil, err
	}
	if len(pixels) > webImageLimit {
		return s, nil, fmt.Errorf("page screenshot exceeds 16 MiB")
	}
	hash := sha256.Sum256(pixels)
	s.SHA256 = hex.EncodeToString(hash[:])
	s.CapturedAt = time.Now().UTC().Format(time.RFC3339Nano)
	identity := sha256.Sum256([]byte(p.Src + s.CapturedAt + s.SHA256))
	s.Image = "web-" + hex.EncodeToString(identity[:]) + ".png"
	return s, pixels, nil
}

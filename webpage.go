package slides

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image/png"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"m31labs.dev/gosx"
)

const webNamespace = "__slidesWeb"
const webManifestPath = "public/webpages/manifest.json"
const webImageLimit = 16 << 20

// WebPageProps is the built-in WebPage component's literal prop contract.
// Src also accepts the HTML-style spelling src. Capabilities default to false.
type WebPageProps struct {
	Src, Title                          string
	Width, Height                       int
	Scripts, SameOrigin, Popups, Scroll bool
}

// WebSnapshot records the pixels and response policy observed by Chrome.
type WebSnapshot struct {
	URL            string   `json:"url"`
	FinalURL       string   `json:"finalURL"`
	CapturedAt     string   `json:"capturedAt"`
	Image          string   `json:"image"`
	SHA256         string   `json:"sha256"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Scroll         bool     `json:"scroll"`
	AllowedHosts   []string `json:"allowedHosts"`
	XFrameOptions  string   `json:"xFrameOptions,omitempty"`
	CSP            string   `json:"contentSecurityPolicy,omitempty"`
	FramingBlocked bool     `json:"framingBlocked"`
}

type webManifest struct {
	Version   int                    `json:"version"`
	Snapshots map[string]WebSnapshot `json:"snapshots"`
}

type deckWebPages struct {
	hosts    []string
	pages    map[string]WebPageProps
	manifest webManifest
	static   bool
}

var webHostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
var webImagePattern = regexp.MustCompile(`^web-[a-f0-9]{64}\.png$`)

func webURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || strings.ContainsAny(raw, "\\\r\n\t ") {
		return nil, fmt.Errorf("WebPage Src must be an absolute HTTPS URL without credentials")
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		for _, label := range strings.Split(host, ".") {
			if len(label) > 63 || !webHostPattern.MatchString(label) {
				return nil, fmt.Errorf("WebPage Src has an invalid host")
			}
		}
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("WebPage Src has an invalid port")
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("WebPage Src has an invalid port")
		}
		port = strconv.Itoa(n)
	}
	u.Host = host
	if port != "" && port != "443" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

func webProps(raw string) (WebPageProps, error) {
	p := WebPageProps{Width: 1280, Height: 720}
	seen := map[string]bool{}
	for _, token := range splitPropTokens(raw) {
		name, _, _ := strings.Cut(token, "=")
		key := strings.ToLower(name)
		if seen[key] {
			return p, fmt.Errorf("WebPage repeats prop %s", name)
		}
		seen[key] = true
	}
	for name, value := range parseProps(raw) {
		key := strings.ToLower(name)
		switch key {
		case "src", "title":
			s, ok := value.(string)
			if !ok {
				return p, fmt.Errorf("WebPage %s must be a string literal", name)
			}
			if key == "src" {
				p.Src = s
			} else {
				p.Title = s
			}
		case "width", "height":
			n, ok := value.(int)
			if !ok {
				return p, fmt.Errorf("WebPage %s must be an integer literal", name)
			}
			if key == "width" {
				p.Width = n
			} else {
				p.Height = n
			}
		case "scripts", "sameorigin", "popups", "scroll":
			b, ok := value.(bool)
			if !ok {
				return p, fmt.Errorf("WebPage %s must be a boolean literal", name)
			}
			switch key {
			case "scripts":
				p.Scripts = b
			case "sameorigin":
				p.SameOrigin = b
			case "popups":
				p.Popups = b
			case "scroll":
				p.Scroll = b
			}
		default:
			return p, fmt.Errorf("unknown WebPage prop %s", name)
		}
	}
	u, err := webURL(p.Src)
	if err != nil {
		return p, err
	}
	p.Src = u.String()
	if p.Width < 320 || p.Width > 1920 || p.Height < 240 || p.Height > 1080 {
		return p, fmt.Errorf("WebPage viewport must be 320–1920 by 240–1080 pixels")
	}
	if p.Title == "" {
		p.Title = "Web page: " + u.Host
	}
	return p, nil
}

func webKey(p WebPageProps) string {
	data, _ := json.Marshal([]any{p.Src, p.Width, p.Height, p.Scroll})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (d *IslandDeck) loadWebPages() error {
	w := &deckWebPages{pages: map[string]WebPageProps{}, manifest: webManifest{Version: 1, Snapshots: map[string]WebSnapshot{}}}
	for _, slide := range d.Slides {
		for _, ref := range slide.Components {
			if ref.Name != "WebPage" {
				continue
			}
			p, err := webProps(ref.Props)
			if err != nil {
				return fmt.Errorf("slide %d: %w", slide.Index+1, err)
			}
			w.pages[ref.Props] = p
		}
	}
	head, _, _ := splitHeadmatter(string(d.Source))
	if len(w.pages) == 0 && !strings.Contains(head, "web-allow:") {
		return nil
	}
	var settings struct {
		Hosts []string `yaml:"web-allow"`
	}
	if err := yaml.Unmarshal([]byte(head), &settings); err != nil {
		return fmt.Errorf("WebPage deck settings: %w", err)
	}
	for _, host := range settings.Hosts {
		u, err := webURL("https://" + host)
		if err != nil || u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(host, "/?#") {
			return fmt.Errorf("web-allow requires exact HTTPS hosts, optionally with a port; wildcards and URLs are not allowed")
		}
		w.hosts = append(w.hosts, u.Host)
	}
	sort.Strings(w.hosts)
	w.hosts = uniqueStrings(w.hosts)
	if len(w.hosts) > 64 || len(w.pages) > 64 {
		return fmt.Errorf("WebPage supports at most 64 hosts and pages per deck")
	}
	d.web = w
	if err := d.readWebManifest(); err != nil {
		return err
	}
	return nil
}

func (d *IslandDeck) readWebManifest() error {
	data, err := readWebAsset(d.Dir, webManifestPath, 1<<20)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var manifest webManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("WebPage manifest: %w", err)
	}
	if manifest.Version != 1 || len(manifest.Snapshots) > 256 {
		return fmt.Errorf("unsupported or oversized WebPage manifest")
	}
	var total int
	checked := map[string]bool{}
	for _, p := range d.web.pages {
		key := webKey(p)
		s, ok := manifest.Snapshots[key]
		if !ok || checked[key] {
			continue
		}
		checked[key] = true
		if s.URL != p.Src || s.Width != p.Width || s.Height != p.Height || s.Scroll != p.Scroll || !webImagePattern.MatchString(s.Image) {
			return fmt.Errorf("WebPage snapshot metadata does not match its URL and viewport")
		}
		if _, err := time.Parse(time.RFC3339Nano, s.CapturedAt); err != nil {
			return fmt.Errorf("WebPage snapshot has an invalid capture date")
		}
		if _, err := webURL(s.FinalURL); err != nil {
			return err
		}
		pixels, err := readWebAsset(d.Dir, "public/webpages/"+s.Image, webImageLimit)
		if err != nil {
			return err
		}
		total += len(pixels)
		if len(pixels) > webImageLimit || total > snapshotTotalLimit {
			return fmt.Errorf("WebPage snapshots exceed the asset budget")
		}
		hash := sha256.Sum256(pixels)
		if hex.EncodeToString(hash[:]) != s.SHA256 || len(pixels) < 8 || string(pixels[:8]) != "\x89PNG\r\n\x1a\n" {
			return fmt.Errorf("WebPage snapshot image failed SHA-256 or PNG validation")
		}
		config, err := png.DecodeConfig(bytes.NewReader(pixels))
		maxHeight := p.Height
		if p.Scroll {
			maxHeight *= 3
		}
		if err != nil || config.Width != p.Width || config.Height < p.Height || config.Height > maxHeight {
			return fmt.Errorf("WebPage snapshot dimensions do not match its viewport")
		}
	}
	if manifest.Snapshots == nil {
		manifest.Snapshots = map[string]WebSnapshot{}
	}
	d.web.manifest = manifest
	return nil
}

func readWebAsset(dir, name string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("WebPage assets must be regular files")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("WebPage asset exceeds its size budget")
	}
	return data, nil
}

func (w *deckWebPages) allowed(raw string) bool {
	u, err := webURL(raw)
	if err != nil {
		return false
	}
	for _, host := range w.hosts {
		if host == u.Host {
			return true
		}
	}
	return false
}

// Restrictive ancestor policies use pixels conservatively: a portable deck has
// no fixed deployment origin against which 'self' or a named ancestor can match.
func webFramingBlocked(xfo, csp string) bool {
	ancestors := false
	for _, policy := range strings.FieldsFunc(csp, func(r rune) bool { return r == '\n' || r == ',' }) {
		for _, directive := range strings.Split(policy, ";") {
			fields := strings.Fields(directive)
			if len(fields) > 0 && strings.EqualFold(fields[0], "frame-ancestors") {
				ancestors = true
				if len(fields) != 2 || fields[1] != "*" {
					return true
				}
			}
		}
	}
	// Enforced frame-ancestors overrides X-Frame-Options. All policies apply.
	return !ancestors && strings.TrimSpace(xfo) != ""
}

func (d *IslandDeck) webCSP(ownHost string) string {
	if own, err := webURL("https://" + ownHost); err == nil {
		ownHost = own.Host
	}
	var sources []string
	if d.web != nil && !d.web.static && !deckConferenceConfig(d).OfflineRequired {
		for _, host := range d.web.hosts {
			if host != ownHost {
				sources = append(sources, "https://"+host)
			}
		}
	}
	if len(sources) == 0 {
		sources = []string{"'none'"}
	}
	return "frame-src " + strings.Join(sources, " ") + ";"
}

func (d *IslandDeck) renderWebPage(raw string) gosx.Node {
	if d.web == nil {
		return gosx.Text("")
	}
	p, ok := d.web.pages[raw]
	if !ok {
		return gosx.Text("")
	}
	s, captured := d.web.manifest.Snapshots[webKey(p)]
	live := !d.web.static && !deckConferenceConfig(d).OfflineRequired && d.web.allowed(p.Src)
	if captured && (webFramingBlocked(s.XFrameOptions, s.CSP) || s.FramingBlocked || !d.web.allowed(s.FinalURL)) {
		live = false
	}
	sandbox := []string{}
	if p.Scripts {
		sandbox = append(sandbox, "allow-scripts")
	}
	if p.SameOrigin {
		sandbox = append(sandbox, "allow-same-origin")
	}
	if p.Popups {
		sandbox = append(sandbox, "allow-popups")
	}
	caption := "Snapshot not captured"
	image := `<div class="webpage-placeholder">Snapshot not captured</div>`
	if captured {
		caption = "Captured " + s.CapturedAt
		image = `<img class="webpage-snapshot" src="/public/webpages/` + s.Image + `" alt="` + html.EscapeString(p.Title) + `" decoding="async">`
	}
	attrs := ""
	if live {
		attrs = ` data-web-src="` + html.EscapeString(p.Src) + `" data-web-sandbox="` + strings.Join(sandbox, " ") + `"`
	}
	controls := ""
	if !d.web.static {
		controls = `<div class="webpage-controls">`
		if live {
			controls += `<button type="button" data-web-action="reload">Reload</button><button type="button" data-web-action="zoom" aria-pressed="false">100%</button><button type="button" data-web-action="lock" aria-pressed="true">Unlock scroll</button>`
		}
		controls += `<a href="` + html.EscapeString(p.Src) + `" target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">Open in new tab</a></div>`
	}
	return gosx.RawHTML(`<figure class="webpage" style="--web-aspect:` + strconv.Itoa(p.Width) + `/` + strconv.Itoa(p.Height) + `" data-web-page="true" data-web-width="` + strconv.Itoa(p.Width) + `" data-web-height="` + strconv.Itoa(p.Height) + `" data-web-title="` + html.EscapeString(p.Title) + `"` + attrs + `><div class="webpage-viewport">` + image + `<div class="webpage-live"></div><div class="webpage-lock" aria-hidden="true"></div></div><figcaption><span class="webpage-url">` + html.EscapeString(p.Src) + `</span><span class="webpage-date">` + html.EscapeString(caption) + `</span></figcaption>` + controls + `</figure>`)
}

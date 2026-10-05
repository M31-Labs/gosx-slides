package slides

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	htmlParser "golang.org/x/net/html"
)

// export_island.go is the REAL-lane static exporter. It renders the deck through
// the same gosx server.App that `serve` uses — in-process via an httptest
// recorder, no listener — then writes a hostable static site. Islands stay live:
// the staged runtime.wasm + island JSON are copied alongside and the absolute
// /gosx/ asset paths are rewritten relative so hydration works from any static
// host (or file://). This replaces the fallback render-based export.go.

// ExportOptions configures a static export.
type ExportOptions struct {
	Format       string  // "spa" (default), "single", or "pdf"
	Editable     bool    // native text and supported SVG objects in PPTX
	Notes        bool    // include speaker notes in a reading handout (explicit opt-in)
	Narration    string  // deck-relative narration audio for video (short audio pads with silence)
	Captions     string  // deck-relative authored WebVTT file for video
	Capture      bool    // capture live graphics through Chrome for single/PDF
	Steps        bool    // include every reveal/cue state in captured output
	Seconds      float64 // video hold time per state (default 2)
	FPS          int     // video sampling rate (default 15)
	OutDir       string  // output directory (default "dist"); for pdf, may be a .pdf path
	Aspect       string  // capture/PPTX aspect; defaults to deck aspect-ratio, then 16:9
	Width        int     // custom CSS-pixel viewport, paired with Height (320–4096)
	Height       int     // custom CSS-pixel viewport, paired with Width; at most 8MP
	PPTXTemplate string  // optional PPTX from which only its theme is reused
}

// ExportStatic renders the real-lane deck at dir to a static bundle.
//
//	spa    — a hostable folder: index.html + gosx/ assets (+ public/, notes.html).
//	         Islands hydrate; the whole deck works offline from the folder.
//	single — one self-contained deck.html: theme + slide navigation work, islands
//	         show their server-rendered initial state (a static snapshot — the
//	         island runtime is stripped, since a 30MB wasm cannot live in one file).
//	pdf    — a one-slide-per-page PDF handout printed through a system
//	         Chrome/Chromium (optional dependency; see exportPDF).
func ExportStatic(dir string, opts ExportOptions) error {
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		return err
	}
	// StageRuntime builds/caches the wasm + bootstrap JS into <dir>/build and points
	// the App's runtime root there; StageIslandPrograms writes each island's JSON to
	// <dir>/build/islands so the export can copy real files (not just the in-process
	// mounts).
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if err := validateVideoNarrationOptions(opts, format); err != nil {
		return err
	}
	if opts.Editable && format != "pptx" {
		return fmt.Errorf("--editable requires --format pptx")
	}
	if format != "" && format != "spa" && format != "single" && format != "handout" && format != "pdf" && format != "frames" && format != "video" && format != "pptx" {
		return fmt.Errorf("unknown export format %q (use spa, single, handout, pdf, frames, video, or pptx)", opts.Format)
	}
	if opts.Notes && format != "handout" {
		return fmt.Errorf("--notes requires --format handout")
	}
	if _, _, err := exportSize(deck, opts); err != nil {
		return err
	}
	if opts.PPTXTemplate != "" && format != "pptx" {
		return fmt.Errorf("PPTX template requires --format pptx")
	}
	if opts.Seconds == 0 {
		opts.Seconds = 2
	}
	if opts.FPS == 0 {
		opts.FPS = 15
	}
	if opts.Seconds < 0.1 || opts.Seconds > 60 || math.IsNaN(opts.Seconds) || math.IsInf(opts.Seconds, 0) || opts.FPS < 1 || opts.FPS > 60 {
		return fmt.Errorf("video seconds must be 0.1–60 and fps 1–60")
	}
	if (opts.Capture || opts.Steps) && format == "handout" {
		return fmt.Errorf("handout is a reading document; --capture and --steps require single, pdf, frames, video, or pptx")
	}
	if opts.Capture || opts.Steps || format == "frames" || format == "video" || format == "pptx" {
		if format == "" || format == "spa" {
			return fmt.Errorf("--capture and --steps require single, pdf, frames, video, or pptx")
		}
		opts.Format = format
		return exportCaptured(deck, opts)
	}
	app, err := deck.NewServer(ServeOptions{StageRuntime: format == "" || format == "spa", Static: true})
	if err != nil {
		return fmt.Errorf("build deck app: %w", err)
	}
	if format == "" || format == "spa" {
		if err := StageIslandPrograms(dir); err != nil {
			return fmt.Errorf("stage island programs: %w", err)
		}
	}

	rec := httptest.NewRecorder()
	app.Build().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		return fmt.Errorf("render deck: server returned status %d", rec.Code)
	}
	// The gosx server stamps a per-request requestID (gosx-<nanos>-<counter>) into
	// the document-contract JSON, which would make every `slides build` produce a
	// different index.html. Normalize it to a fixed sentinel so static builds are
	// byte-identical run-to-run (CI cache keys, diff-only deploys). The slidegen
	// layer is already deterministic; this is the only non-reproducible field.
	doc := requestIDRe.ReplaceAllString(rec.Body.String(), `"requestID":"gosx-static"`)

	out := opts.OutDir
	if out == "" {
		out = "dist"
	}
	switch strings.ToLower(strings.TrimSpace(opts.Format)) {
	case "", "spa":
		return exportSPA(dir, deck, doc, out)
	case "single":
		return exportSingleSnapshot(deck, doc, out)
	case "handout":
		return exportHandout(deck, doc, out, opts.Notes)
	case "pdf":
		return exportPDF(deck, doc, out)
	default:
		return fmt.Errorf("unknown export format %q (use spa, single, or pdf)", opts.Format)
	}
}

// pdfChromeCandidates are the browser binaries exportPDF looks for, in order.
// SLIDES_CHROME overrides the search entirely.
var pdfChromeCandidates = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome",
}

// pdfPageStyle sizes the printed page to one 16:9 slide with no margins; the
// print stylesheet (nav.go) already lays slides out one per page, so each PDF
// page is exactly one slide.
const pdfPageStyle = `<style>@page { size: 1920px 1080px; margin: 0; }</style>`

// exportPDF prints the deck to a PDF through a system Chrome/Chromium in
// headless mode — the same single-snapshot page `--format single` writes, so
// the PDF needs no server and no wasm. out may be a .pdf file path or a
// directory (then <out>/deck.pdf). Chrome is an OPTIONAL dependency: when no
// binary is found the error says exactly what to install or set.
func exportPDF(deck *IslandDeck, doc, out string) error {
	chrome := os.Getenv("SLIDES_CHROME")
	if chrome == "" {
		for _, candidate := range pdfChromeCandidates {
			if found, err := exec.LookPath(candidate); err == nil {
				chrome = found
				break
			}
		}
	}
	if chrome == "" {
		return fmt.Errorf("pdf export needs Chrome or Chromium on PATH (or SLIDES_CHROME=/path/to/chrome); none found")
	}

	pdfPath := out
	if strings.EqualFold(filepath.Ext(out), ".pdf") {
		if dir := filepath.Dir(out); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
	} else {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
		pdfPath = filepath.Join(out, "deck.pdf")
	}
	page, err := inlineSnapshotAssets(deck, stripIslandRuntime(doc))
	if err != nil {
		return err
	}
	page = strings.Replace(page, "</head>", pdfPageStyle+"</head>", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		return fmt.Errorf("chrome pdf print: %w", err)
	}
	defer browser.close()
	if err = browser.call("Page.navigate", map[string]any{"url": server.URL}, nil); err != nil {
		return err
	}
	if err = browser.wait(`document.readyState === 'complete' && !!window.SlidesNav`); err != nil {
		return err
	}
	if err = browser.eval(`(async()=>{if(document.fonts)await document.fonts.ready;await Promise.all(Array.from(document.images,image=>image.decode().catch(()=>{})));return true})()`, nil); err != nil {
		return err
	}
	var result struct {
		Data string `json:"data"`
	}
	if err = browser.call("Page.printToPDF", map[string]any{"printBackground": true, "preferCSSPageSize": true, "displayHeaderFooter": false}, &result); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("chrome returned an invalid PDF")
	}
	return os.WriteFile(pdfPath, data, 0o644)
}

// gosxAbsRefRe matches a quoted absolute /gosx/ asset reference (attribute value
// or JSON string). A leading quote can only precede /gosx/ in machine-generated
// markup (attrs, the manifest/document-contract JSON) — never in rendered prose —
// so rewriting these is safe.
var (
	gosxAbsRefRe   = regexp.MustCompile(`(["'])/gosx/`)
	publicAbsRefRe = regexp.MustCompile(`(["'])/public/`)
)

// requestIDRe matches the per-request requestID gosx stamps into the document
// contract; normalized at export so static builds are reproducible.
var requestIDRe = regexp.MustCompile(`"requestID":"gosx-[0-9-]+"`)

// relativizeGosxPaths rewrites absolute /gosx/... asset refs to relative gosx/...
// so the exported page hydrates from any static host or file://, not just origin
// root. Covers <script src>, <link href> preload hints, and the JSON bodies of
// the gosx-manifest and gosx-document <script> blocks (runtime.path, programRef).
func relativizeGosxPaths(doc string) string {
	return gosxAbsRefRe.ReplaceAllString(doc, `${1}gosx/`)
}

// relativizePublicPaths rewrites deck asset references for file:// exports.
// The exporter copies <deck>/public to the same directory as the generated
// HTML, so a relative public/... URL works in SPA and PDF output.
func relativizePublicPaths(doc string) string {
	return publicAbsRefRe.ReplaceAllString(doc, `${1}public/`)
}

func exportSPA(dir string, deck *IslandDeck, doc, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	staticDoc := relativizePublicPaths(relativizeGosxPaths(doc))
	// GoSX's lazy text-layout loader discovers preload hints before using its
	// origin-root fallback. Keep that runtime relative to this static bundle.
	if _, err := os.Stat(filepath.Join(dir, "build", "bootstrap-feature-textlayout.js")); err == nil {
		staticDoc = strings.Replace(staticDoc, "</head>", `<link rel="preload" as="script" href="gosx/bootstrap-feature-textlayout.js"></head>`, 1)
	}
	if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(staticDoc), 0o644); err != nil {
		return err
	}
	// Copy the staged client runtime + island JSON into <out>/gosx, mapping the
	// build filenames to the URL names the page references.
	if err := copyBuildToGosx(filepath.Join(dir, "build"), filepath.Join(out, "gosx")); err != nil {
		return fmt.Errorf("copy runtime assets: %w", err)
	}
	// Carry the deck's static assets (images, fonts) if any.
	if src := filepath.Join(dir, "public"); isDir(src) {
		if err := copyTree(src, filepath.Join(out, "public")); err != nil {
			return fmt.Errorf("copy public: %w", err)
		}
	}
	if err := copyCompositionAssets(deck, out); err != nil {
		return fmt.Errorf("copy included/pack assets: %w", err)
	}
	// A speaker-notes sidecar, derived from the real deck.
	if err := os.WriteFile(filepath.Join(out, "notes.html"), []byte(notesHTML(deck)), 0o644); err != nil {
		return err
	}
	return nil
}

func exportSingleSnapshot(deck *IslandDeck, doc, out string) error {
	doc, err := inlineSnapshotAssets(deck, stripIslandRuntime(doc))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "deck.html"), []byte(doc), 0o644)
}

func exportHandout(deck *IslandDeck, doc, out string, withNotes bool) error {
	doc = stripIslandRuntime(doc)
	root, err := htmlParser.Parse(strings.NewReader(doc))
	if err != nil {
		return err
	}
	var walk func(*htmlParser.Node)
	walk = func(node *htmlParser.Node) {
		if node.Type == htmlParser.ElementNode && node.Data == "aside" {
			for _, attr := range node.Attr {
				if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " slide-notes ") {
					node.Parent.RemoveChild(node)
					return
				}
			}
		}
		if node.Type == htmlParser.ElementNode && node.Data == "main" {
			for _, attr := range node.Attr {
				if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " deck ") {
					node.Attr = append(node.Attr, htmlParser.Attribute{Key: "data-reading", Val: "1"})
				}
			}
		}
		if node.Type == htmlParser.ElementNode && node.Data == "section" && withNotes {
			for _, attr := range node.Attr {
				if attr.Key != "data-slide" {
					continue
				}
				index, parseErr := strconv.Atoi(attr.Val)
				if parseErr != nil || index < 0 || index >= len(deck.Slides) {
					continue
				}
				if note := extractSlideNotes(deck.Slides[index]); note != "" {
					aside := &htmlParser.Node{Type: htmlParser.ElementNode, Data: "aside", Attr: []htmlParser.Attribute{{Key: "class", Val: "handout-notes"}, {Key: "aria-label", Val: "Speaker notes"}}}
					aside.AppendChild(&htmlParser.Node{Type: htmlParser.TextNode, Data: note})
					node.AppendChild(aside)
				}
			}
		}
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			walk(child)
			child = next
		}
	}
	walk(root)
	var rendered strings.Builder
	if err = htmlParser.Render(&rendered, root); err != nil {
		return err
	}
	result, err := inlineSnapshotAssets(deck, rendered.String())
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "handout.html"), []byte(result), 0o644)
}

var (
	gosxScriptRe   = regexp.MustCompile(`(?s)<script[^>]*\ssrc=["']/gosx/[^>]*></script>`)
	gosxLinkRe     = regexp.MustCompile(`<link[^>]*\shref=["']/gosx/[^>]*>`)
	gosxManifestRe = regexp.MustCompile(`(?s)<script[^>]*id="gosx-manifest"[^>]*>.*?</script>`)
	gosxDocumentRe = regexp.MustCompile(`(?s)<script[^>]*id="gosx-document"[^>]*>.*?</script>`)
)

// stripIslandRuntime removes every /gosx/ external reference and the island
// runtime contract from the page, leaving a self-contained single HTML file. The
// inline theme CSS and the self-contained nav/presenter scripts (no island
// dependency) survive, so a `single` export still themes and navigates — only
// island hydration is dropped (the wasm cannot be embedded).
func stripIslandRuntime(doc string) string {
	doc = gosxManifestRe.ReplaceAllString(doc, "")
	doc = gosxDocumentRe.ReplaceAllString(doc, "")
	doc = gosxScriptRe.ReplaceAllString(doc, "")
	doc = gosxLinkRe.ReplaceAllString(doc, "")
	return doc
}

// copyBuildToGosx copies the staged build/ dir to destGosx, mapping the runtime
// wasm's build filename (gosx-runtime.wasm) to the URL name the page references
// (runtime.wasm); every other file keeps its relative path (wasm_exec.js,
// bootstrap*.js, patch.js, islands/<Name>.json).
func copyBuildToGosx(buildDir, destGosx string) error {
	return filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(buildDir, path)
		if err != nil {
			return err
		}
		if rel == "gosx-runtime.wasm" {
			rel = "runtime.wasm"
		}
		return copyFile(filepath.Join(destGosx, rel), path)
	})
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		return copyFile(filepath.Join(dst, rel), path)
	})
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// notesHTML renders a simple speaker-notes handout from the real deck: one
// section per slide that has notes. Opaque author prose, HTML-escaped.
func notesHTML(deck *IslandDeck) string {
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=utf-8><title>")
	b.WriteString(html.EscapeString(deck.title()))
	b.WriteString(" — notes</title>\n")
	b.WriteString("<body style=\"font:16px/1.6 system-ui,sans-serif;max-width:48rem;margin:2rem auto;padding:0 1rem\">\n")
	b.WriteString("<h1>")
	b.WriteString(html.EscapeString(deck.title()))
	b.WriteString(" — speaker notes</h1>\n")
	for _, slide := range deck.Slides {
		note := extractSlideNotes(slide)
		if note == "" {
			continue
		}
		b.WriteString("<section><h2>")
		b.WriteString(html.EscapeString(fmt.Sprintf("%02d. %s", slide.Index+1, slideTitle(slide))))
		b.WriteString("</h2><p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(note), "\n", "<br>"))
		b.WriteString("</p></section>\n")
	}
	return b.String()
}

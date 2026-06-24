package slides

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// export_island.go is the REAL-lane static exporter. It renders the deck through
// the same gosx server.App that `serve` uses — in-process via an httptest
// recorder, no listener — then writes a hostable static site. Islands stay live:
// the staged runtime.wasm + island JSON are copied alongside and the absolute
// /gosx/ asset paths are rewritten relative so hydration works from any static
// host (or file://). This replaces the fallback render-based export.go.

// ExportOptions configures a static export.
type ExportOptions struct {
	Format string // "spa" (default) or "single"
	OutDir string // output directory (default "dist")
}

// ExportStatic renders the real-lane deck at dir to a static bundle.
//
//	spa    — a hostable folder: index.html + gosx/ assets (+ public/, notes.html).
//	         Islands hydrate; the whole deck works offline from the folder.
//	single — one self-contained deck.html: theme + slide navigation work, islands
//	         show their server-rendered initial state (a static snapshot — the
//	         island runtime is stripped, since a 30MB wasm cannot live in one file).
func ExportStatic(dir string, opts ExportOptions) error {
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		return err
	}
	// StageRuntime builds/caches the wasm + bootstrap JS into <dir>/build and points
	// the App's runtime root there; StageIslandPrograms writes each island's JSON to
	// <dir>/build/islands so the export can copy real files (not just the in-process
	// mounts).
	app, err := deck.NewServer(ServeOptions{StageRuntime: true})
	if err != nil {
		return fmt.Errorf("build deck app: %w", err)
	}
	if err := StageIslandPrograms(dir); err != nil {
		return fmt.Errorf("stage island programs: %w", err)
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
	default:
		return fmt.Errorf("unknown export format %q (use spa or single)", opts.Format)
	}
}

// gosxAbsRefRe matches a quoted absolute /gosx/ asset reference (attribute value
// or JSON string). A leading quote can only precede /gosx/ in machine-generated
// markup (attrs, the manifest/document-contract JSON) — never in rendered prose —
// so rewriting these is safe.
var gosxAbsRefRe = regexp.MustCompile(`(["'])/gosx/`)

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

func exportSPA(dir string, deck *IslandDeck, doc, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(relativizeGosxPaths(doc)), 0o644); err != nil {
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
	// A speaker-notes sidecar, derived from the real deck.
	if err := os.WriteFile(filepath.Join(out, "notes.html"), []byte(notesHTML(deck)), 0o644); err != nil {
		return err
	}
	return nil
}

func exportSingleSnapshot(deck *IslandDeck, doc, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "deck.html"), []byte(stripIslandRuntime(doc)), 0o644)
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

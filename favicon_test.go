package slides

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var iconHrefRe = regexp.MustCompile(`<link rel="icon" type="([^"]+)" href="data:[^;]+;base64,([^"]+)">`)

// faviconFromPage pulls the single icon link out of a served page and decodes it.
func faviconFromPage(t *testing.T, page string) (mime, body string) {
	t.Helper()
	m := iconHrefRe.FindAllStringSubmatch(page, -1)
	if len(m) != 1 {
		t.Fatalf("want exactly one icon link, got %d", len(m))
	}
	raw, err := base64.StdEncoding.DecodeString(m[0][2])
	if err != nil {
		t.Fatalf("icon data not base64: %v", err)
	}
	return m[0][1], string(raw)
}

const faviconDeckHead = "---\ntitle: T\ntheme: aurora\n%s---\n\n# Hi\n"

func faviconDeck(line string) string {
	if line != "" {
		line += "\n"
	}
	return strings.Replace(faviconDeckHead, "%s", line, 1)
}

func TestFaviconDefault(t *testing.T) {
	mime, body := faviconFromPage(t, serveBodyWithFiles(t, faviconDeck(""), nil))
	if mime != "image/svg+xml" || body != defaultFaviconSVG {
		t.Fatalf("default icon not emitted: %s %s", mime, body)
	}
}

func TestFaviconEmoji(t *testing.T) {
	mime, body := faviconFromPage(t, serveBodyWithFiles(t, faviconDeck(`favicon: "🎤"`), nil))
	if mime != "image/svg+xml" || !strings.Contains(body, ">🎤</text>") {
		t.Fatalf("emoji icon wrong: %s %s", mime, body)
	}
}

func TestFaviconMonogram(t *testing.T) {
	page := serveBodyWithFiles(t, faviconDeck(`favicon: {text: "GT", color: "#10b981"}`), nil)
	mime, body := faviconFromPage(t, page)
	if mime != "image/svg+xml" || !strings.Contains(body, `fill="#10b981"`) || !strings.Contains(body, ">GT</text>") {
		t.Fatalf("monogram wrong: %s %s", mime, body)
	}
	// Colour defaults to the theme accent (aurora amber).
	_, body = faviconFromPage(t, serveBodyWithFiles(t, faviconDeck(`favicon: {text: "G"}`), nil))
	if !strings.Contains(body, `fill="#f6b352"`) {
		t.Fatalf("monogram did not default to theme accent: %s", body)
	}
}

func TestFaviconFile(t *testing.T) {
	icon := `<svg xmlns="http://www.w3.org/2000/svg"><circle r="3"/></svg>`
	mime, body := faviconFromPage(t, serveBodyWithFiles(t, faviconDeck("favicon: art/icon.svg"), map[string]string{"art/icon.svg": icon}))
	if mime != "image/svg+xml" || body != icon {
		t.Fatalf("file icon wrong: %s %s", mime, body)
	}
	mime, _ = faviconFromPage(t, serveBodyWithFiles(t, faviconDeck("favicon: i.png"), map[string]string{"i.png": "\x89PNG"}))
	if mime != "image/png" {
		t.Fatalf("png mime = %s", mime)
	}
}

func TestFaviconEscaping(t *testing.T) {
	svg, err := monogramSVG(`<&`, "#123456")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "&lt;&amp;") || strings.Contains(svg, "<&") {
		t.Fatalf("text not escaped: %s", svg)
	}
	svg, err = monogramSVG(`"'`, "#123456")
	if err != nil || strings.Contains(svg, `>"'<`) {
		t.Fatalf("quotes not escaped: %v %s", err, svg)
	}
	if got := emojiSVG(`<script>`); strings.Contains(got, "<script>") {
		t.Fatalf("emoji text not escaped: %s", got)
	}
	if _, err := monogramSVG("GT", `red" onload="x`); err == nil {
		t.Fatal("hostile colour accepted")
	}
}

func TestFaviconErrors(t *testing.T) {
	for name, line := range map[string]string{
		"missing file": "favicon: nope.svg",
		"escapes deck": "favicon: ../x.png",
		"bad word":     "favicon: banana",
		"bad key":      `favicon: {txt: "GT"}`,
		"too long":     `favicon: {text: "GTX"}`,
	} {
		dir := newDeckDirUnderModule(t, faviconDeck(line), nil)
		deck, err := LoadIslandDeck(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deck.NewServer(ServeOptions{}); err == nil || !strings.Contains(err.Error(), "favicon") {
			t.Errorf("%s: want favicon error from NewServer, got %v", name, err)
		}
		report, _ := Doctor(dir)
		failed := false
		for _, it := range report.Items {
			if it.Name == "favicon" && it.Status == "fail" {
				failed = true
			}
		}
		if !failed {
			t.Errorf("%s: doctor did not report a favicon failure", name)
		}
	}
}

func TestFaviconRejectsSymlinkEscapeAndOversize(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link.png")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := fileFaviconLink(dir, "link.png"); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink escape accepted: %v", err)
	}
	big := filepath.Join(dir, "big.png")
	if err := os.WriteFile(big, make([]byte, maxFaviconBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := fileFaviconLink(dir, "big.png"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversize accepted: %v", err)
	}
}

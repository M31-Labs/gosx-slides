package slides

import (
	"strings"
	"testing"
)

// TestRelativizeGosxPaths guards the static-host path rewrite: absolute /gosx/
// refs (in attributes AND the manifest/document-contract JSON) must become
// relative so islands hydrate from any host, while prose is untouched.
func TestRelativizeGosxPaths(t *testing.T) {
	in := `<link rel="preload" href="/gosx/runtime.wasm">` +
		`<script defer src="/gosx/bootstrap.js"></script>` +
		`<script id="gosx-manifest" type="application/json">{"runtime":{"path":"/gosx/runtime.wasm"},"islands":[{"programRef":"/gosx/islands/Counter.json"}]}</script>` +
		`<p>visit /gosx/ in your browser</p>` // prose: no quote before /gosx/, must NOT be rewritten
	got := relativizeGosxPaths(in)

	for _, want := range []string{`href="gosx/runtime.wasm"`, `src="gosx/bootstrap.js"`, `"path":"gosx/runtime.wasm"`, `"programRef":"gosx/islands/Counter.json"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing relativized ref %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"/gosx/`) || strings.Contains(got, `'/gosx/`) {
		t.Errorf("a quoted absolute /gosx/ ref survived:\n%s", got)
	}
	if !strings.Contains(got, "visit /gosx/ in your browser") {
		t.Errorf("prose mention of /gosx/ was wrongly rewritten")
	}
}

// TestStripIslandRuntime guards the single-file snapshot: every /gosx/ external
// reference and the island runtime contract are removed, leaving a self-contained
// file (theme + nav survive, island hydration does not).
func TestStripIslandRuntime(t *testing.T) {
	in := `<head>` +
		`<link rel="preload" href="/gosx/runtime.wasm">` +
		`<style>main.deck{}</style>` +
		`<script id="gosx-manifest" type="application/json">{"runtime":{"path":"/gosx/runtime.wasm"}}</script>` +
		`<script id="gosx-document" type="application/json">{"runtimePath":"/gosx/runtime.wasm"}</script>` +
		`<script defer src="/gosx/bootstrap.js"></script>` +
		`</head><body><section data-slide="0">hi</section>` +
		`<script>/* inline nav, self-contained */</script></body>`
	got := stripIslandRuntime(in)

	if strings.Contains(got, "/gosx/") {
		t.Errorf("a /gosx/ reference survived the snapshot strip:\n%s", got)
	}
	if strings.Contains(got, "gosx-manifest") || strings.Contains(got, "gosx-document") {
		t.Errorf("the island runtime contract survived:\n%s", got)
	}
	// Self-contained survivors: theme CSS, slide markup, inline nav script.
	for _, want := range []string{"main.deck{}", `data-slide="0"`, "inline nav, self-contained"} {
		if !strings.Contains(got, want) {
			t.Errorf("snapshot dropped self-contained content %q", want)
		}
	}
}

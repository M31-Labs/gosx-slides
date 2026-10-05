package slides

import (
	"os"
	"strings"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
)

// The optional fixture exercises the same native lowering and offline assets
// before a caller integrates simulationAssets into its server page shell.
func TestSimulationBrowserFixture(t *testing.T) {
	path := os.Getenv("SLIDES_SIMULATION_FIXTURE")
	if path == "" {
		t.Skip("optional browser fixture")
	}
	deck, err := LoadIslandDeck("examples/simulation-lab")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compileDeckProgram(deck)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	out.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>` + navStyle() + baseContentStyle() + themeCSS("paper") + baseLayoutStyle() + `</style></head><body><main class="deck" data-theme="paper" data-exported="true" data-transition="none">`)
	for _, node := range renderProgramSlides(island.NewRenderer("simulation-browser"), deck, compiled, nil) {
		out.WriteString(gosx.RenderHTML(node))
	}
	out.WriteString(`</main><script>` + navScript() + motionTimelineScript + `</script>` + simulationAssets(deck) + `</body></html>`)
	if err = os.WriteFile(path, []byte(out.String()), 0600); err != nil {
		t.Fatal(err)
	}
}

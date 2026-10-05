package slides

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/mdpp"
)

//go:embed assets/simulation.js
var simulationScript string

//go:embed assets/simulation.css
var simulationStyle string

const simulationNamespace = "__slidesSimulation"

// simulationAssets is feature-gated and entirely offline. State frames come
// from the authoritative GoSX model; JavaScript never integrates physics.
func simulationAssets(deck *IslandDeck) string {
	if deck.Simulations == nil || len(deck.Simulations.Simulations) == 0 {
		return ""
	}
	data, _ := json.Marshal(deck.Simulations)
	return `<script type="application/json" id="slides-simulations">` + string(data) + `</script><style>` + simulationStyle + `</style><script>` + simulationScript + `</script>`
}

type simulationMountData struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Seed     string        `json:"seed"`
	TickRate int           `json:"tickRate"`
	Ticks    int           `json:"ticks"`
	Hash     string        `json:"hash"`
	State    particleState `json:"state"`
	Branches []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"branches"`
}

func prepareSimulationMounts(deck *IslandDeck) error {
	if deck.Simulations == nil {
		return nil
	}
	byID := map[string]CompiledSimulation{}
	for _, entry := range deck.Simulations.Simulations {
		byID[entry.ID] = entry
	}
	for index, slide := range deck.Slides {
		var walk func(*mdpp.Node) error
		walk = func(n *mdpp.Node) error {
			if n.Type == mdpp.NodeContainerDirective && n.Attr("name") == "simulation" {
				id := strings.TrimSpace(n.Attr("title"))
				entry, ok := byID[id]
				if !ok {
					return fmt.Errorf("missing simulation mount %s", id)
				}
				tick := 0
				for _, pose := range entry.Playhead {
					if pose.SlideIndex == index && pose.Step == 0 {
						tick = pose.Tick
					}
				}
				frame := entry.Branches[0].Frames[tick]
				data := simulationMountData{ID: id, Label: entry.Label, Seed: entry.Seed, TickRate: entry.TickRate, Ticks: entry.Ticks, Hash: frame.Hash}
				if err := json.Unmarshal(frame.State, &data.State); err != nil {
					return err
				}
				for _, branch := range entry.Branches {
					data.Branches = append(data.Branches, struct {
						ID    string `json:"id"`
						Label string `json:"label"`
					}{branch.ID, branch.Label})
				}
				encoded, _ := json.Marshal(data)
				if n.Attrs == nil {
					n.Attrs = map[string]string{}
				}
				n.Attrs["__slides_simulation_mount"] = string(encoded)
			}
			for _, child := range n.Children {
				if err := walk(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(slide.Node); err != nil {
			return err
		}
	}
	return nil
}

// Render accepts bounded typed mount data and only emits fixed markup with
// escaped labels and numeric coordinates. Authors cannot inject executable HTML
// through the expression namespace.
func simulationMountNode(source string) gosx.Node {
	var data simulationMountData
	if len(source) > 64<<10 || json.Unmarshal([]byte(source), &data) != nil || !slideClassTokenRe.MatchString(data.ID) || len(data.State.Particles) > 32 {
		return gosx.El("p", gosx.Text("Simulation unavailable"))
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<figure class="slides-simulation" data-simulation="%s" data-simulation-tick="%d" data-simulation-branch="baseline" data-simulation-hash="%s"><figcaption>%s · seed %s · %d ticks/s</figcaption><svg viewBox="0 0 600 300" role="img" aria-label="%s"><rect width="600" height="300" rx="8" class="simulation-background"/>`, html.EscapeString(data.ID), data.State.Tick, html.EscapeString(data.Hash), html.EscapeString(data.Label), html.EscapeString(data.Seed), data.TickRate, html.EscapeString(data.Label))
	for i, particle := range data.State.Particles {
		fmt.Fprintf(&out, `<circle data-simulation-particle="%d" cx="%.3f" cy="%.3f" r="8" class="simulation-particle simulation-color-%d"/>`, i, float64(particle.X)/1000, float64(particle.Y)/1000, i%4)
	}
	out.WriteString(`</svg><div class="simulation-controls">`)
	out.WriteString(`<label>Branch <select class="simulation-branch" aria-label="Simulation branch">`)
	for _, branch := range data.Branches {
		fmt.Fprintf(&out, `<option value="%s">%s</option>`, html.EscapeString(branch.ID), html.EscapeString(branch.Label))
	}
	out.WriteString(`</select></label>`)
	fmt.Fprintf(&out, `<label>Tick <input class="simulation-tick" type="range" aria-label="Simulation tick" min="0" max="%d" value="%d" step="1"></label><button type="button" class="simulation-reset">Follow presentation</button><output class="simulation-status">Tick %d / %d</output></div></figure>`, data.Ticks, data.State.Tick, data.State.Tick, data.Ticks)
	return gosx.RawHTML(out.String())
}

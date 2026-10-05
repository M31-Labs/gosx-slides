package slides

import (
	_ "embed"
	"encoding/json"
)

//go:embed assets/semantic-story.js
var semanticStoryScript string

func semanticStoryAssets(deck *IslandDeck) string {
	if deck.Story == nil {
		return ""
	}
	// Runtime effects need semantic identities, not authored source paths/ranges.
	// Keep provenance in inspect/report APIs rather than audience page metadata.
	type runtimeBeat struct {
		StoryBeat
		SlideIndex       int               `json:"slideIndex"`
		Step             int               `json:"step"`
		GraphKey         string            `json:"graphKey"`
		SurfaceGraphKeys map[string]string `json:"surfaceGraphKeys,omitempty"`
	}
	type runtimeActor struct {
		Name  string `json:"name"`
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	type runtimeGraph struct {
		Actors      []runtimeActor `json:"actors"`
		Surface     string         `json:"surface,omitempty"`
		ContainerID string         `json:"containerId,omitempty"`
	}
	public := struct {
		Version int                     `json:"version"`
		Beats   []runtimeBeat           `json:"beats"`
		Graphs  map[string]runtimeGraph `json:"graphs"`
	}{Version: deck.Story.Version, Beats: []runtimeBeat{}, Graphs: map[string]runtimeGraph{}}
	for _, beat := range deck.Story.Beats {
		public.Beats = append(public.Beats, runtimeBeat{beat.StoryBeat, beat.SlideIndex, beat.Step, beat.GraphKey, beat.SurfaceGraphKeys})
	}
	for key, graph := range deck.Story.Graphs {
		entry := runtimeGraph{Actors: []runtimeActor{}, Surface: graph.Surface, ContainerID: graph.ContainerID}
		for _, actor := range graph.Actors {
			entry.Actors = append(entry.Actors, runtimeActor{actor.Name, actor.ID, actor.Label})
		}
		public.Graphs[key] = entry
	}
	data, _ := json.Marshal(public)
	return `<script type="application/json" id="slides-story">` + string(data) + `</script><style>pre.code-block .ts-line[data-story-code=false]{opacity:.25!important}
main.deck .slides-story-caption{position:fixed;bottom:2.7rem;left:50%;transform:translateX(-50%);width:min(90vw,70ch);max-width:none;z-index:30;box-sizing:border-box;max-height:min(28vh,12rem);overflow-y:auto;margin:0;text-align:center;font:500 clamp(.9rem,1.6vw,1.15rem)/1.5 system-ui;color:var(--fg,#fff);background:var(--surface,#151923);padding:.4rem .8rem;border-radius:.5rem}
.slides-story-caption:focus-visible{outline:2px solid var(--accent,#ffd27d);outline-offset:3px}
.slides-story-caption[hidden],.deck-overview .slides-story-caption,.deck-blank .slides-story-caption,.deck-reading .slides-story-caption{display:none}
@media print{.slides-story-caption{display:none}}</style><script>` + semanticStoryScript + `</script>`
}

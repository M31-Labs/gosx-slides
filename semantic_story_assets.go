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
	data, _ := json.Marshal(deck.Story)
	return `<script type="application/json" id="slides-story">` + string(data) + `</script><style>pre.code-block .ts-line[data-story-code=false]{opacity:.25!important}.slides-story-caption{position:fixed;bottom:2.7rem;left:5%;right:5%;z-index:30;margin:0;text-align:center;font:500 clamp(.9rem,1.6vw,1.15rem)/1.5 system-ui;color:var(--fg,#fff);background:var(--surface,#151923);padding:.4rem .8rem;border-radius:.5rem;pointer-events:none}.slides-story-caption[hidden],.deck-overview .slides-story-caption,.deck-blank .slides-story-caption,.deck-reading .slides-story-caption{display:none}@media print{.slides-story-caption{display:none}}</style><script>` + semanticStoryScript + `</script>`
}

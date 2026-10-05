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
	return `<script type="application/json" id="slides-story">` + string(data) + `</script><style>pre.code-block .ts-line[data-story-code=false]{opacity:.25!important}
main.deck .slides-story-caption{position:fixed;bottom:2.7rem;left:50%;transform:translateX(-50%);width:min(90vw,70ch);max-width:none;z-index:30;box-sizing:border-box;max-height:min(28vh,12rem);overflow-y:auto;margin:0;text-align:center;font:500 clamp(.9rem,1.6vw,1.15rem)/1.5 system-ui;color:var(--fg,#fff);background:var(--surface,#151923);padding:.4rem .8rem;border-radius:.5rem}
.slides-story-caption:focus-visible{outline:2px solid var(--accent,#ffd27d);outline-offset:3px}
.slides-story-caption[hidden],.deck-overview .slides-story-caption,.deck-blank .slides-story-caption,.deck-reading .slides-story-caption{display:none}
@media print{.slides-story-caption{display:none}}</style><script>` + semanticStoryScript + `</script>`
}

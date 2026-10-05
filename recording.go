package slides

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"m31labs.dev/gosx"
)

//go:embed assets/recording.css
var recordingStyle string

//go:embed assets/recording.js
var recordingScript string

type recordingSlide struct {
	ID      string         `json:"id"`
	Caption string         `json:"caption"`
	Beats   map[int]string `json:"beats,omitempty"`
}

// Captions are explicit author metadata, never inferred from notes or titles.
func slideRecordingCaption(slide IslandSlide) string {
	value, _ := slideFrontmatterValues(slide)["caption"].(string)
	value = strings.TrimSpace(value)
	if len(value) > 4096 {
		value = value[:4096]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func recordingMetadata(deck *IslandDeck) gosx.Node {
	data := make([]recordingSlide, 0, len(deck.Slides))
	for _, slide := range deck.Slides {
		id, _ := slideFrontmatterValues(slide)["id"].(string)
		metadata := recordingSlide{ID: id, Caption: slideRecordingCaption(slide)}
		if deck.Story != nil {
			for _, beat := range deck.Story.Beats {
				if beat.SlideIndex == slide.Index {
					if metadata.Beats == nil {
						metadata.Beats = map[int]string{}
					}
					metadata.Beats[beat.Step] = beat.Caption
				}
			}
		}
		data = append(data, metadata)
	}
	encoded, _ := json.Marshal(data) // encoding/json escapes script delimiters.
	return gosx.RawHTML(`<script type="application/json" id="slides-recording-data">` + string(encoded) + `</script>`)
}

// Exact beat addresses win, including intentionally empty captions. Other
// steps use explicit slide metadata. Notes and generated labels never become
// narration captions.
func deckRecordingCaption(deck *IslandDeck, slide IslandSlide, step int) string {
	if deck.Story != nil {
		for _, beat := range deck.Story.Beats {
			if beat.SlideIndex == slide.Index && beat.Step == step {
				return beat.Caption
			}
		}
	}
	return slideRecordingCaption(slide)
}

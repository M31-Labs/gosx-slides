package slides

import (
	"strconv"
	"strings"
)

const (
	conferenceAspectRatio      = "16:9"
	conferenceCaptionSafeFloor = 20
)

// ConferenceConfig is the presentation-room contract declared in deck
// headmatter. It is shared by rendering, validation, export, and diagnostics.
type ConferenceConfig struct {
	AspectRatio       string `json:"aspectRatio"`
	CaptionSafeBottom int    `json:"captionSafeBottom"`
	CaptionGuide      bool   `json:"captionGuide"`
	DurationMinutes   int    `json:"durationMinutes"`
	OfflineRequired   bool   `json:"offlineRequired"`
}

func deckConferenceConfig(deck *IslandDeck) ConferenceConfig {
	values := deckFrontmatterValues(deck)
	return ConferenceConfig{
		AspectRatio:       normalizedAspect(frontmatterText(values, "aspect-ratio")),
		CaptionSafeBottom: frontmatterPercent(values, "caption-safe-bottom"),
		CaptionGuide:      frontmatterBool(values, "caption-guide"),
		DurationMinutes:   frontmatterInt(values, "duration-minutes"),
		OfflineRequired:   frontmatterBool(values, "offline-required"),
	}
}

func normalizedAspect(value string) string {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	value = strings.ReplaceAll(value, "/", ":")
	return value
}

func frontmatterText(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func frontmatterInt(values map[string]any, key string) int {
	raw := strings.TrimSpace(strings.TrimSuffix(frontmatterText(values, key), "%"))
	n, _ := strconv.Atoi(raw)
	return n
}

func frontmatterPercent(values map[string]any, key string) int {
	n := frontmatterInt(values, key)
	if n < 0 || n > 40 {
		return 0
	}
	return n
}

func frontmatterBool(values map[string]any, key string) bool {
	switch strings.ToLower(frontmatterText(values, key)) {
	case "1", "true", "yes", "on", "required":
		return true
	default:
		return false
	}
}

func slideFallback(slide IslandSlide) string {
	value, _ := slideFrontmatterValues(slide)["fallback"].(string)
	return strings.TrimSpace(value)
}

func conferenceStyle(config ConferenceConfig) string {
	if config.CaptionSafeBottom == 0 {
		return ""
	}
	bottom := strconv.Itoa(config.CaptionSafeBottom)
	return `main.deck[data-caption-safe-bottom="` + bottom + `"] {
  font-size: clamp(1.35rem, 2vw, 1.8rem);
  line-height: 1.45;
}
main.deck[data-caption-safe-bottom="` + bottom + `"] > .slide {
  box-sizing: border-box;
  padding-bottom: max(clamp(2.5rem, 7vw, 7rem), calc(` + bottom + `vh + 1.5rem));
}
main.deck[data-caption-guide="1"][data-caption-safe-bottom="` + bottom + `"]::after {
  content: "caption-safe area · bottom ` + bottom + `%";
  position: fixed; inset: auto 0 0; height: ` + bottom + `vh; z-index: 39;
  pointer-events: none; box-sizing: border-box;
  border-top: 2px dashed rgba(255, 170, 80, .85);
  background: rgba(255, 170, 80, .12);
  color: rgba(255,255,255,.82); font: 700 12px/1 system-ui,sans-serif;
  padding: .6rem 1rem; text-align: right;
}
@media print {
  main.deck[data-caption-guide="1"][data-caption-safe-bottom]::after { display: none !important; }
}`
}

func conferenceInteractiveComponents() []string {
	return []string{"Benchmark", "Canvas", "CorpusRun", "ParseTree", "ParityMatrix", "Poll", "ProfileBuckets", "QueryDemo", "Scene3D"}
}

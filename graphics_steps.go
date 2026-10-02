package slides

import (
	"encoding/json"
	"fmt"

	"m31labs.dev/gosx/scene"
)

type graphicTimeline struct {
	Version int            `json:"version"`
	Frames  []graphicFrame `json:"frames"`
}
type graphicFrame struct {
	Label      string          `json:"label"`
	DurationMS *int            `json:"durationMs,omitempty"`
	Easing     string          `json:"easing,omitempty"`
	Commands   []scene.Command `json:"commands"`
}

// Keyframes are authored absolute command batches. Keep explanatory metadata
// outside the renderer's props and transport it on the owning slide surface.
func graphicStepAttrs(payload map[string]any) (map[string]any, error) {
	raw, exists := payload["slideSteps"]
	if !exists {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if len(data) > 4<<20 {
		return nil, fmt.Errorf("graphic keyframes exceed 4 MiB")
	}
	var timeline graphicTimeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("graphic keyframes: %w", err)
	}
	if timeline.Version != 1 || len(timeline.Frames) == 0 || len(timeline.Frames) > 128 {
		return nil, fmt.Errorf("graphic keyframes need version 1 and 1–128 frames")
	}
	scenePayload, _ := payload["scene"].(map[string]any)
	ids := map[string]bool{}
	for _, kind := range []string{"objects", "labels", "lights", "sprites", "htmlOverlays"} {
		items, _ := scenePayload[kind].([]any)
		for _, item := range items {
			if obj, ok := item.(map[string]any); ok {
				if id, ok := obj["id"].(string); ok {
					ids[id] = true
				}
			}
		}
	}
	for index, frame := range timeline.Frames {
		if frame.DurationMS != nil && (*frame.DurationMS < 0 || *frame.DurationMS > 600000) {
			return nil, fmt.Errorf("graphic durationMs must be between 0 and 600000")
		}
		switch frame.Easing {
		case "", "linear", "ease", "ease-in", "ease-out", "ease-in-out":
		default:
			return nil, fmt.Errorf("unknown graphic easing %q", frame.Easing)
		}
		if frame.Commands == nil {
			timeline.Frames[index].Commands = []scene.Command{}
		}
		if len(frame.Commands) > 10000 {
			return nil, fmt.Errorf("graphic frame has too many commands")
		}
		for _, command := range frame.Commands {
			if command.Kind < scene.CommandCreateObject || command.Kind > scene.CommandSetPostUniforms {
				return nil, fmt.Errorf("graphic frame has unknown command kind %d", command.Kind)
			}
			if command.Kind <= scene.CommandSetLight && !ids[command.ObjectID] {
				return nil, fmt.Errorf("graphic frame targets unknown object %q", command.ObjectID)
			}
			if command.Kind != scene.CommandRemoveObject && command.Data == nil {
				return nil, fmt.Errorf("graphic frame command requires data")
			}
		}
	}
	data, err = json.Marshal(timeline)
	if err != nil {
		return nil, err
	}
	delete(payload, "slideSteps")
	return map[string]any{"data-slide-steps": string(data), "data-steps": len(timeline.Frames) - 1}, nil
}

func graphicsStepScript() string { return graphicsMotionScript }

func graphicsClickBudgets(graphics []DeckGraphicInfo) map[string]int {
	budgets := map[string]int{}
	for _, graphic := range graphics {
		budgets[graphic.Source] = max(budgets[graphic.Source], graphic.Steps)
	}
	return budgets
}

func slideGraphicClicks(slide IslandSlide, budgets map[string]int) int {
	steps := 0
	for _, ref := range slide.Components {
		if isGraphicsComponent(ref.Name) {
			steps = max(steps, budgets[graphicString(parseProps(ref.Props), "Src", "")])
		}
	}
	return steps
}

package slides

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGraphicKeyframesTransportAndInventory(t *testing.T) {
	deck, err := LoadIslandDeck("examples/sirena-scene")
	if err != nil {
		t.Fatal(err)
	}
	body := graphicsBody(t, deck)
	if !strings.Contains(body, `data-steps="3"`) || !strings.Contains(body, "data-slide-steps=") {
		t.Fatal("keyframes are missing from the owning graphic mount")
	}
	if !strings.Contains(body, `data-gosx-text-layout="true"`) {
		t.Fatal("native labels must activate the typography feature")
	}
	manifest := graphicsManifest(t, body)
	props := manifest["engines"].([]any)[0].(map[string]any)["props"].(map[string]any)
	if _, exists := props["slideSteps"]; exists {
		t.Fatal("presentation metadata leaked into renderer props")
	}
	analysis := Analyze(deck)
	if analysis.TotalClicks != 3 || analysis.Slides[0].Clicks != 3 || analysis.Graphics[0].Steps != 3 {
		t.Fatalf("incorrect keyframe inventory: %+v", analysis)
	}
	summary, err := Check("examples/sirena-scene")
	if err != nil || summary.TotalClicks != 3 {
		t.Fatalf("incorrect click summary: %+v, %v", summary, err)
	}
}

func TestGraphicKeyframesValidation(t *testing.T) {
	data, err := os.ReadFile("examples/sirena-scene/request.scene.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, timeline string }{
		{"version", `{"version":2,"frames":[{}]}`},
		{"empty", `{"version":1,"frames":[]}`},
		{"target", `{"version":1,"frames":[{"commands":[{"kind":2,"objectId":"missing","data":{"x":1}}]}]}`},
		{"kind", `{"version":1,"frames":[{"commands":[{"kind":99,"data":{}}]}]}`},
		{"data", `{"version":1,"frames":[{"commands":[{"kind":5}]}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var payload map[string]any
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			var timeline any
			if err := json.Unmarshal([]byte(test.timeline), &timeline); err != nil {
				t.Fatal(err)
			}
			payload["slideSteps"] = timeline
			if _, err := graphicStepAttrs(payload); err == nil {
				t.Fatal("invalid keyframes accepted")
			}
		})
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	payload["slideSteps"] = map[string]any{"version": 1, "frames": []any{map[string]any{"label": "Start"}}}
	attrs, err := graphicStepAttrs(payload)
	if err != nil || !strings.Contains(attrs["data-slide-steps"].(string), `"commands":[]`) {
		t.Fatalf("empty frame must carry an empty command array: %v, %v", attrs, err)
	}
}

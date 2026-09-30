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
	Label    string          `json:"label"`
	Commands []scene.Command `json:"commands"`
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

func graphicsStepScript() string {
	return `(function () {
  function start() {
    var deck = document.querySelector('main.deck');
    if (!deck || !document.querySelector('#gosx-manifest')) return;
    var controllers = Array.prototype.map.call(deck.querySelectorAll('.slide-graphic[data-slide-steps]'), function (mount) {
      var frames = JSON.parse(mount.getAttribute('data-slide-steps')).frames;
      var slide = mount.closest('[data-slide]');
      var desired = 0, applied = -1, busy = false, timer = null, attempts = 0;
      function active() { return slide && slide.classList.contains('deck-active'); }
      function pump() {
        timer = null;
        if (!active() || busy || desired === applied) return;
        var handle = mount.__gosxScene3DHandle;
        if (!handle || !handle.__gosxScene3DCommandReady) {
          if (++attempts <= 100) timer = setTimeout(pump, 100);
          return;
        }
        attempts = 0;
        var target = desired;
        busy = true;
        Promise.resolve().then(function () { return handle.applyCommands(frames[target].commands); }).then(function () {
          applied = target;
          mount.setAttribute('data-applied-step', String(target));
          mount.setAttribute('aria-label', frames[target].label || 'Slide graphic');
          mount.removeAttribute('data-slide-step-error');
        }).catch(function (error) {
          mount.setAttribute('data-slide-step-error', String(error.message || error));
          applied = target;
        }).finally(function () { busy = false; pump(); });
      }
      return function () {
        if (timer) { clearTimeout(timer); timer = null; }
        desired = Math.min(frames.length - 1, parseInt(slide && slide.getAttribute('data-active-step'), 10) || 0);
        attempts = 0;
        pump();
      };
    });
    function update() { controllers.forEach(function (sync) { sync(); }); }
    deck.addEventListener('slides:change', update);
    update();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
})();`
}

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

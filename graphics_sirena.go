package slides

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"m31labs.dev/sirena"
	sirenascene "m31labs.dev/sirena/render/scene3d"
)

// Compile authored diagrams and absolute steps through Sirena's native adapter.
// Its attachment model moves labels, routes and arrowheads with their actors.
func compileSirenaGraphic(dir string, props map[string]any, source, steps []byte) (map[string]any, error) {
	if len(source) > maxSourceBytes || len(steps) > maxSourceBytes {
		return nil, fmt.Errorf("Sirena source and steps must each fit the 1 MiB authoring limit")
	}
	ws, err := sirena.NewFenceWorkspace(source, sirena.FenceOptions{})
	if err != nil {
		return nil, err
	}
	doc := ws.Files[0].Document
	for _, diagnostic := range append(doc.Diagnostics(), ws.ResolveDiagnostics()...) {
		if diagnostic.Severity == sirena.SeverityError {
			return nil, fmt.Errorf("Sirena: %s", diagnostic.Message)
		}
	}
	viewName := graphicString(props, "View", "")
	rv := sirena.AllElementsView(doc)
	if len(doc.Views) > 0 || viewName != "" {
		var selected *sirena.ViewDecl
		for _, view := range doc.Views {
			if viewName == "" || view.Name == viewName {
				selected = view
				break
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("Sirena view %q not found", viewName)
		}
		rv, err = sirena.EvaluateView(ws, selected)
		if err != nil {
			return nil, err
		}
	}
	layout, _, err := sirena.Render(rv, sirena.RenderOptions{})
	if err != nil {
		return nil, err
	}
	opts := sirenascene.Options{}
	if steps != nil {
		decoder := json.NewDecoder(bytes.NewReader(steps))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&opts.Steps); err != nil {
			return nil, fmt.Errorf("scene steps: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF || len(opts.Steps) < 1 || len(opts.Steps) > 128 {
			return nil, fmt.Errorf("scene steps require one JSON array with 1–128 cues")
		}
	}
	if shader := graphicString(props, "Shader", ""); shader != "" {
		opts.Shader, err = readGraphicFile(dir, shader)
		if err != nil {
			return nil, err
		}
		opts.Material = graphicString(props, "Material", "")
		for _, target := range strings.Split(graphicString(props, "Targets", ""), ",") {
			if target = strings.TrimSpace(target); target != "" {
				opts.Targets = append(opts.Targets, target)
			}
		}
	}
	raw, err := sirenascene.Build(layout, opts)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	err = json.Unmarshal(raw, &payload)
	// Sirena's typed adapter omits zero-valued durations. Keep explicitly authored
	// zero so an immediate cue remains immediate in the shared transport.
	if err == nil && steps != nil {
		var authored []map[string]json.RawMessage
		json.Unmarshal(steps, &authored)
		if timeline, ok := payload["slideSteps"].(map[string]any); ok {
			frames, _ := timeline["frames"].([]any)
			for i, step := range authored {
				if value, exists := step["durationMs"]; exists && i < len(frames) {
					var duration int
					json.Unmarshal(value, &duration)
					frames[i].(map[string]any)["durationMs"] = duration
				}
			}
		}
	}
	return payload, err
}

func sceneGraphicID(ref ComponentRef) string {
	return sourceRevision([]byte(graphicsKey(ref.Name, ref.Props)))
}

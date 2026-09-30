package slides

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/engine"
	"m31labs.dev/gosx/scene"
	"m31labs.dev/gosx/scene/schema"
)

// Graphics use GoSX's native engine rather than an island VM or a second renderer.
// Shader programs and scene payloads are prepared once alongside the deck program.
const graphicsNamespace = "__slidesGraphics"

type engineMounter interface {
	RenderEngine(engine.Config, gosx.Node) gosx.Node
}
type deckGraphic struct {
	config engine.Config
	err    error
}

func isGraphicsComponent(name string) bool  { return name == "Shader" || name == "Scene3D" }
func graphicsKey(name, props string) string { return name + ":" + strings.TrimSpace(props) }
func graphicsSceneSource(value string) bool {
	switch strings.ToLower(filepath.Ext(value)) {
	case ".sel", ".json":
		return true
	}
	return false
}
func backgroundGraphicRef(source string) ComponentRef {
	name := "Scene3D"
	if strings.EqualFold(filepath.Ext(source), ".sel") {
		name = "Shader"
	}
	return ComponentRef{Name: name, Props: "Src=" + strconv.Quote(source) + " Background={true}"}
}
func deckGraphicRefs(deck *IslandDeck) []ComponentRef {
	var refs []ComponentRef
	for _, slide := range deck.Slides {
		for _, ref := range slide.Components {
			if isGraphicsComponent(ref.Name) {
				refs = append(refs, ref)
			}
		}
		source := resolveSlideLayer(slide, "scene", deckFrontmatterString(deck, "scene"))
		if graphicsSceneSource(source) {
			refs = append(refs, backgroundGraphicRef(source))
		}
	}
	return refs
}
func compileDeckGraphics(deck *IslandDeck) map[string]deckGraphic {
	graphics := map[string]deckGraphic{}
	for _, ref := range deckGraphicRefs(deck) {
		key := graphicsKey(ref.Name, ref.Props)
		if _, exists := graphics[key]; exists {
			continue
		}
		cfg, err := compileGraphic(deck.Dir, ref)
		graphics[key] = deckGraphic{config: cfg, err: err}
	}
	return graphics
}

// Read deck-local sources, rejecting both traversal and symlinks leaving the deck.
func readGraphicFile(dir, name string) ([]byte, error) {
	if strings.TrimSpace(name) == "" || !safeDeckRelPath(name) {
		return nil, fmt.Errorf("graphics source must be a deck-relative path: %q", name)
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || !safeDeckRelPath(rel) {
		return nil, fmt.Errorf("graphics source escapes deck: %q", name)
	}
	return os.ReadFile(path)
}
func graphicString(props map[string]any, name, fallback string) string {
	if value, ok := props[name].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
func compileGraphic(dir string, ref ComponentRef) (engine.Config, error) {
	props := parseProps(ref.Props)
	src := graphicString(props, "Src", "")
	data, err := readGraphicFile(dir, src)
	if err != nil {
		return engine.Config{}, err
	}
	var payload map[string]any
	if ref.Name == "Shader" {
		material, _, err := scene.CompileSelenaMaterial(data, scene.SelenaMaterialOptions{Material: graphicString(props, "Material", ""), Standard: scene.StandardMaterial{Color: "#ffffff"}})
		if err != nil {
			return engine.Config{}, fmt.Errorf("shader %s: %w", src, err)
		}
		if uniforms := graphicString(props, "Uniforms", ""); uniforms != "" {
			raw, err := readGraphicFile(dir, uniforms)
			if err != nil {
				return engine.Config{}, err
			}
			var values map[string]any
			if err := json.Unmarshal(raw, &values); err != nil {
				return engine.Config{}, fmt.Errorf("shader uniforms: %w", err)
			}
			for key, value := range values {
				material.Uniforms[key] = value
			}
		}
		var geometry scene.Geometry
		rotation := scene.Euler{X: math.Pi / 2}
		shape := graphicString(props, "Shape", "plane")
		switch shape {
		case "plane":
			geometry = scene.PlaneGeometry{Width: 18, Height: 12}
		case "sphere":
			geometry = scene.SphereGeometry{Radius: 2.5, Segments: 48}
		case "box":
			geometry = scene.BoxGeometry{Width: 4, Height: 4, Depth: 4}
		case "torus":
			geometry = scene.TorusGeometry{Radius: 2.2, Tube: .7, RadialSegments: 24, TubularSegments: 64}
		default:
			return engine.Config{}, fmt.Errorf("unknown shader Shape %q (plane, sphere, box, torus)", shape)
		}
		if shape != "plane" {
			rotation = scene.Euler{X: .9, Y: .25}
		}
		config := scene.Props{
			Background: "transparent", CanvasAlpha: scene.Bool(true), Responsive: scene.Bool(true), FillHeight: scene.Bool(true),
			MaxFrameRate: 30, MaxDevicePixelRatio: 1.5, MaxPixels: 2_000_000, AdaptiveQuality: scene.Bool(true),
			AutoRotate: scene.Bool(shape != "plane"), DragToRotate: scene.Bool(shape != "plane"),
			Camera: scene.PerspectiveCamera{Position: scene.Vec3(0, 0, 10), FOV: 50, Near: .1, Far: 100},
			Graph:  scene.NewGraph(scene.Mesh{ID: "shader-shape", Geometry: geometry, Material: material, Rotation: rotation}),
		}.EngineConfig()
		if err := json.Unmarshal(config.Props, &payload); err != nil {
			return engine.Config{}, err
		}
	} else {
		if err := json.Unmarshal(data, &payload); err != nil {
			return engine.Config{}, fmt.Errorf("scene %s: %w", src, err)
		}
		// Accept a full GoSX Scene3D props document or a bare SceneIR scene.
		if _, ok := payload["scene"]; !ok {
			payload = map[string]any{"scene": payload}
		}
		if _, ok := payload["scene"].(map[string]any); !ok {
			return engine.Config{}, fmt.Errorf("scene %s: expected a scene object", src)
		}
		if shader := graphicString(props, "Shader", ""); shader != "" {
			raw, err := readGraphicFile(dir, shader)
			if err != nil {
				return engine.Config{}, err
			}
			material, _, err := scene.CompileSelenaMaterial(raw, scene.SelenaMaterialOptions{Material: graphicString(props, "Material", ""), Standard: scene.StandardMaterial{Color: "#ffffff"}})
			if err != nil {
				return engine.Config{}, err
			}
			sample := scene.Props{Graph: scene.NewGraph(scene.Mesh{ID: "sample", Geometry: scene.CubeGeometry{Size: 1}, Material: material})}.EngineConfig()
			var samplePayload map[string]any
			if err := json.Unmarshal(sample.Props, &samplePayload); err != nil {
				return engine.Config{}, err
			}
			if _, exists := payload["scene"].(map[string]any)["backendCaps"]; !exists {
				payload["scene"].(map[string]any)["backendCaps"] = samplePayload["scene"].(map[string]any)["backendCaps"]
			}
			sampleObject := samplePayload["scene"].(map[string]any)["objects"].([]any)[0].(map[string]any)
			targets := strings.Split(graphicString(props, "Targets", ""), ",")
			objects, _ := payload["scene"].(map[string]any)["objects"].([]any)
			for _, rawObject := range objects {
				object, ok := rawObject.(map[string]any)
				if !ok {
					continue
				}
				selected := targets[0] == ""
				for _, id := range targets {
					if strings.TrimSpace(id) == object["id"] {
						selected = true
					}
				}
				if !selected {
					continue
				}
				for key, value := range sampleObject {
					if strings.HasPrefix(key, "custom") || strings.HasPrefix(key, "shader") || key == "materialKind" || key == "wireframe" {
						object[key] = value
					}
				}
			}
		}
	}
	for key, value := range map[string]any{"responsive": true, "fillHeight": true, "maxFrameRate": 30, "maxDevicePixelRatio": 1.5, "maxPixels": 2_000_000, "adaptiveQuality": true} {
		if _, exists := payload[key]; !exists {
			payload[key] = value
		}
	}
	label := graphicString(props, "Label", "Slide graphic")
	payload["ariaLabel"] = label
	sceneJSON, err := json.Marshal(payload["scene"])
	if err != nil {
		return engine.Config{}, err
	}
	if report := schema.ValidateJSON(sceneJSON, schema.Options{}); !report.Valid {
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Severity == schema.Error || diagnostic.Severity == schema.Fatal {
				return engine.Config{}, fmt.Errorf("scene %s: %s", src, diagnostic.Message)
			}
		}
	}
	scene.ApplyShaderLib(payload["scene"].(map[string]any))
	raw, err := json.Marshal(payload)
	if err != nil {
		return engine.Config{}, err
	}
	cfg := engine.Config{Name: scene.DefaultEngineName, Kind: engine.KindSurface, Props: raw,
		MountAttrs: map[string]any{"class": "slide-graphic", "data-gosx-scene3d": true, "role": "img", "aria-label": label}}
	if background, _ := props["Background"].(bool); background {
		cfg.MountAttrs["class"] = "deck-graphics-background"
		cfg.MountAttrs["aria-hidden"] = "true"
		cfg.MountAttrs["data-scene-source"] = src
	}
	return cfg, nil
}
func renderDeckGraphic(r islandMounter, graphics map[string]deckGraphic, key string) gosx.Node {
	graphic, exists := graphics[key]
	if !exists {
		return gosx.Text("")
	}
	if graphic.err != nil {
		return gosx.El("pre", gosx.Attrs(gosx.Attr("class", "graphic-error")), gosx.Text("graphic error: "+graphic.err.Error()))
	}
	fallback := gosx.El("div", gosx.Attrs(gosx.Attr("class", "graphic-fallback")), gosx.Text(graphic.config.MountAttrs["aria-label"].(string)))
	if mounter, ok := r.(engineMounter); ok {
		return mounter.RenderEngine(graphic.config, fallback)
	}
	return fallback
}
func renderGraphicsBackgrounds(r islandMounter, deck *IslandDeck, cd *compiledDeck) gosx.Node {
	if cd == nil {
		return gosx.Text("")
	}
	seen := map[string]bool{}
	var nodes []gosx.Node
	for _, slide := range deck.Slides {
		src := resolveSlideLayer(slide, "scene", deckFrontmatterString(deck, "scene"))
		if !graphicsSceneSource(src) || seen[src] {
			continue
		}
		seen[src] = true
		ref := backgroundGraphicRef(src)
		nodes = append(nodes, renderDeckGraphic(r, cd.graphics, graphicsKey(ref.Name, ref.Props)))
	}
	return gosx.Fragment(nodes...)
}
func graphicsStyle() string {
	return `
main.deck .slide-graphic { width: 100%; height: min(54vh, 32rem); min-height: 12rem; border-radius: 1rem; overflow: hidden; background: var(--surface, #10141e); }
main.deck .graphic-fallback { display: grid; place-items: center; height: 100%; min-height: 12rem; color: var(--muted, #aaa); }
main.deck .graphic-error { color: #ff7979; white-space: pre-wrap; font-size: .8rem; }
main.deck > .deck-graphics-background { position: fixed !important; inset: 0; width: 100vw !important; height: 100vh !important; z-index: 0; pointer-events: none; display: none; }
main.deck > .deck-graphics-background.deck-background-active { display: block; }
main.deck:has(> .deck-graphics-background.deck-background-active) > .slide { background: transparent; z-index: 1; }
@media (prefers-reduced-motion: reduce) { main.deck > .deck-graphics-background { display: none !important; } }
@media print { main.deck > .deck-graphics-background { display: none !important; } }
`
}

type DeckGraphicInfo struct {
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Compiles bool   `json:"compiles"`
	Error    string `json:"error,omitempty"`
}

func DeckGraphics(deck *IslandDeck) []DeckGraphicInfo {
	graphics := compileDeckGraphics(deck)
	seen := map[string]bool{}
	var info []DeckGraphicInfo
	for _, ref := range deckGraphicRefs(deck) {
		key := graphicsKey(ref.Name, ref.Props)
		if seen[key] {
			continue
		}
		seen[key] = true
		entry := DeckGraphicInfo{Kind: ref.Name, Source: graphicString(parseProps(ref.Props), "Src", ""), Compiles: graphics[key].err == nil}
		if graphics[key].err != nil {
			entry.Error = graphics[key].err.Error()
		}
		info = append(info, entry)
	}
	return info
}

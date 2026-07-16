package slides

// scene.go is the decorative/illustrative SCENE layer: a deck (or a single
// slide) names an island that renders FULL-BLEED BEHIND the slide's content —
// living backgrounds and visual mental models, not just wallpaper images.
//
//	---
//	scene: parse-forest        # deck-wide: a built-in preset
//	---
//
//	```yaml
//	scene: GLRForks            # this slide: an author island (GLRForks.gsx)
//	```
//
//	```yaml
//	scene: false               # this slide: no scene
//	```
//
// The scene value is either a COMPONENT NAME (uppercase initial — resolved to
// <Name>.gsx in the deck dir like any island, so a scene can be a real gosx
// program: animated SVG today, <Scene3D> the day the island grammar grows it)
// or a PRESET KEY (lowercase — one of the embedded decorative presets below).
// A deck file named like a preset's component SHADOWS the preset, so presets
// are eject-and-edit: copy the source out of this file, drop it next to
// deck.md, and own it.
//
// Layering: the lowering emits `<div class="slide-scene" aria-hidden="true">`
// as the slide's FIRST child; baseContentStyle pins it absolute inset-0 at
// z-index -1 inside the slide's isolated stacking context, pointer-events
// none. Scenes are decorative by contract: prefers-reduced-motion hides the
// layer entirely, and aria-hidden keeps it out of the accessibility tree.

import (
	"fmt"
	"regexp"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/ir"
	"m31labs.dev/gosx/island/program"
)

// sceneComponentNameRe is the shape of a scene value that names an author
// island directly (same uppercase-initial rule as component tags).
var sceneComponentNameRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)

// presetSceneComponents maps a preset key (the lowercase scene: value) to the
// embedded island component that implements it.
var presetSceneComponents = map[string]string{
	"parse-forest": "SlidesSceneParseForest",
}

// presetSceneSources holds the embedded .gsx source for each preset island,
// keyed by COMPONENT name. The styling lives in baseContentStyle (framework
// CSS for framework presets); the island itself is just the layer structure,
// so it compiles through the exact pipeline an author island does.
var presetSceneSources = map[string]string{
	"SlidesSceneParseForest": `package main

//gosx:island
func SlidesSceneParseForest(props any) Node {
	return <div class="scene-parse-forest">
		<div class="spf spf-1"> </div>
		<div class="spf spf-2"> </div>
		<div class="spf spf-3"> </div>
	</div>
}
`,
}

// sceneComponentName resolves a scene: frontmatter value to the island
// component that renders it: "" for empty/disabled values, the value itself
// when it names a component, the preset's component for a known preset key,
// and "" (fail-soft; doctor warns) for anything else.
func sceneComponentName(value string) string {
	v := strings.TrimSpace(value)
	switch strings.ToLower(v) {
	case "", "false", "none", "off":
		return ""
	}
	if sceneComponentNameRe.MatchString(v) {
		return v
	}
	return presetSceneComponents[strings.ToLower(v)]
}

// slideSceneComponent resolves the scene island for one slide: its own
// `scene:` frontmatter when present (false/none/off disables), else the
// deck-level default.
func slideSceneComponent(slide IslandSlide, deckScene string) string {
	return sceneComponentName(resolveSlideLayer(slide, "scene", deckScene))
}

// deckSceneComponents returns the distinct scene island names the deck uses
// across all slides (deck default + per-slide overrides), in first-use order.
func deckSceneComponents(deck *IslandDeck) []string {
	if deck == nil {
		return nil
	}
	deckScene := strings.TrimSpace(deckFrontmatterString(deck, "scene"))
	seen := map[string]bool{}
	var out []string
	for _, slide := range deck.Slides {
		name := slideSceneComponent(slide, deckScene)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// sceneIslandSource reads the source for a scene island: the deck's own
// <Name>.gsx first (an author island, or an ejected preset), then the
// embedded preset registry.
func sceneIslandSource(deck *IslandDeck, name string) (string, error) {
	if source, err := deck.readComponentSource(name); err == nil {
		return source, nil
	}
	if source, ok := presetSceneSources[name]; ok {
		return source, nil
	}
	return "", fmt.Errorf("scene island %q: no %s.gsx in deck dir and no such preset", name, name)
}

// compileSceneComponent compiles a scene island to its program + JSON wire
// form: the deck's own file through the standard component pipeline, an
// embedded preset through the identical Compile → LowerIsland → EncodeJSON
// steps on the embedded source.
func compileSceneComponent(deck *IslandDeck, name string) (*program.Program, []byte, error) {
	if _, err := deck.readComponentSource(name); err == nil {
		return deck.CompileComponent(name)
	}
	source, ok := presetSceneSources[name]
	if !ok {
		return nil, nil, fmt.Errorf("scene preset %q not found", name)
	}
	irProg, err := gosx.Compile([]byte(source))
	if err != nil {
		return nil, nil, fmt.Errorf("compile scene preset %s: %w", name, err)
	}
	for i, comp := range irProg.Components {
		if !comp.IsIsland || comp.Name != name {
			continue
		}
		isl, err := ir.LowerIsland(irProg, i)
		if err != nil {
			return nil, nil, fmt.Errorf("lower scene preset %s: %w", name, err)
		}
		data, err := program.EncodeJSON(isl)
		if err != nil {
			return nil, nil, fmt.Errorf("encode scene preset %s: %w", name, err)
		}
		return isl, data, nil
	}
	return nil, nil, fmt.Errorf("scene preset %q: island not found in embedded source", name)
}

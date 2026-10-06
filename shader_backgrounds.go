package slides

import (
	"embed"
	"fmt"
	"math"
	"strconv"
	"strings"
)

//go:embed assets/backgrounds/*.sel
var backgroundShaderSources embed.FS

// BackgroundOptions are shared by built-in Selena backgrounds and the wizard.
// Zero speed gives an authored still, independent of the motion playhead.
type BackgroundOptions struct {
	Preset   string  `json:"preset"`
	Ink      string  `json:"ink"`
	Glow     string  `json:"glow"`
	Speed    float64 `json:"speed"`
	Strength float64 `json:"strength"`
	Scale    float64 `json:"scale"`
}

type BackgroundPreset struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Defaults    BackgroundOptions `json:"defaults"`
	Source      string            `json:"source"`
}

// BackgroundPresets returns copyable sources as well as zero-file scene names.
func BackgroundPresets() []BackgroundPreset {
	rows := []BackgroundPreset{
		{Name: "aurora", Title: "Aurora", Description: "Slow ribbons of light for an opening or closing slide.", Defaults: BackgroundOptions{Ink: "#08121f", Glow: "#51d6b0", Speed: .18, Strength: .38, Scale: 1}},
		{Name: "silk", Title: "Silk", Description: "Soft flowing folds with a warm, quiet palette.", Defaults: BackgroundOptions{Ink: "#17101c", Glow: "#e8b68b", Speed: .12, Strength: .32, Scale: 1}},
		{Name: "contours", Title: "Contours", Description: "Topographic lines for architecture and research talks.", Defaults: BackgroundOptions{Ink: "#101b20", Glow: "#79bfbb", Speed: .08, Strength: .3, Scale: 1}},
		{Name: "waves", Title: "Waves", Description: "Layered ocean bands that drift at the edge of the slide.", Defaults: BackgroundOptions{Ink: "#0a1428", Glow: "#79a8ed", Speed: .15, Strength: .36, Scale: 1}},
		{Name: "grid", Title: "Grid", Description: "A restrained technical grid with a passing pool of light.", Defaults: BackgroundOptions{Ink: "#101821", Glow: "#91b6de", Speed: .1, Strength: .3, Scale: 1}},
		{Name: "spotlight", Title: "Spotlight", Description: "A soft off-center glow for text-heavy slides.", Defaults: BackgroundOptions{Ink: "#17151c", Glow: "#d5a9c8", Speed: .08, Strength: .28, Scale: 1}},
	}
	for i := range rows {
		rows[i].Defaults.Preset = rows[i].Name
		raw, _ := backgroundShaderSources.ReadFile("assets/backgrounds/" + rows[i].Name + ".sel")
		rows[i].Source = string(raw)
	}
	return rows
}

func backgroundPreset(name string) (BackgroundPreset, error) {
	for _, row := range BackgroundPresets() {
		if row.Name == name {
			return row, nil
		}
	}
	return BackgroundPreset{}, fmt.Errorf("unknown Selena background %q; use slides backgrounds to list presets", name)
}

func (o BackgroundOptions) validate() error {
	if _, err := backgroundPreset(o.Preset); err != nil {
		return err
	}
	for _, color := range []string{o.Ink, o.Glow} {
		if _, err := backgroundColor(color); err != nil {
			return err
		}
	}
	for _, p := range []struct {
		name      string
		v, lo, hi float64
	}{{"speed", o.Speed, 0, 2}, {"strength", o.Strength, 0, 1}, {"scale", o.Scale, .5, 4}} {
		if math.IsNaN(p.v) || math.IsInf(p.v, 0) || p.v < p.lo || p.v > p.hi {
			return fmt.Errorf("background %s must be between %g and %g", p.name, p.lo, p.hi)
		}
	}
	return nil
}

func backgroundColor(value string) ([]float64, error) {
	if len(value) != 7 || value[0] != '#' {
		return nil, fmt.Errorf("background colors must use #rrggbb")
	}
	color := make([]float64, 3)
	for i := range color {
		n, err := strconv.ParseUint(value[1+i*2:3+i*2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("background colors must use #rrggbb")
		}
		color[i] = float64(n) / 255
	}
	return color, nil
}

// BackgroundShaderSource bakes the controls into a standalone editable .sel.
func BackgroundShaderSource(o BackgroundOptions) (string, error) {
	if err := o.validate(); err != nil {
		return "", err
	}
	preset, _ := backgroundPreset(o.Preset)
	lines := strings.Split(preset.Source, "\n")
	for i, line := range lines {
		for _, name := range []string{"ink", "glow", "speed", "strength", "scale"} {
			if !strings.HasPrefix(strings.TrimSpace(line), "param "+name+" :") {
				continue
			}
			var value string
			switch name {
			case "ink", "glow":
				color := o.Ink
				if name == "glow" {
					color = o.Glow
				}
				rgb, _ := backgroundColor(color)
				value = fmt.Sprintf("rgb(%.6f, %.6f, %.6f)", rgb[0], rgb[1], rgb[2])
			default:
				v := map[string]float64{"speed": o.Speed, "strength": o.Strength, "scale": o.Scale}[name]
				value = strconv.FormatFloat(v, 'f', -1, 64)
				if !strings.Contains(value, ".") {
					value += ".0"
				}
			}
			lines[i] = strings.SplitN(line, "=", 2)[0] + "= " + value
		}
	}
	return strings.Join(lines, "\n"), nil
}

var backgroundKeys = []string{"scene", "shader-ink", "shader-glow", "shader-speed", "shader-strength", "shader-scale"}

func deckShaderValues(deck *IslandDeck) map[string]string {
	values := map[string]string{}
	frontmatter := deckFrontmatterValues(deck)
	for _, key := range backgroundKeys[1:] {
		if value, exists := frontmatter[key]; exists {
			values[key] = fmt.Sprint(value)
		}
	}
	return values
}

func backgroundValues(o BackgroundOptions) map[string]string {
	return map[string]string{"scene": "shader:" + o.Preset, "shader-ink": o.Ink, "shader-glow": o.Glow,
		"shader-speed": strconv.FormatFloat(o.Speed, 'f', -1, 64), "shader-strength": strconv.FormatFloat(o.Strength, 'f', -1, 64), "shader-scale": strconv.FormatFloat(o.Scale, 'f', -1, 64)}
}

func shaderBackgroundRef(source string, values map[string]string) ComponentRef {
	ref := backgroundGraphicRef(source)
	if strings.HasPrefix(source, "shader:") {
		for _, key := range backgroundKeys[1:] {
			if value, exists := values[key]; exists {
				name := strings.TrimPrefix(key, "shader-")
				ref.Props += " " + strings.ToUpper(name[:1]) + name[1:] + "=" + strconv.Quote(value)
			}
		}
		// Defaults, equivalent numeric spellings and color case share a surface.
		if options, err := shaderBackgroundOptions(source, parseProps(ref.Props)); err == nil {
			options.Ink, options.Glow = strings.ToLower(options.Ink), strings.ToLower(options.Glow)
			canonical := backgroundValues(options)
			ref = backgroundGraphicRef(source)
			for _, key := range backgroundKeys[1:] {
				name := strings.TrimPrefix(key, "shader-")
				ref.Props += " " + strings.ToUpper(name[:1]) + name[1:] + "=" + strconv.Quote(canonical[key])
			}
		}
	}
	return ref
}

func slideBackgroundRef(slide IslandSlide, layers slideLayers) ComponentRef {
	values := make(map[string]string)
	for key, value := range layers.Shader {
		values[key] = value
	}
	if slide.Node != nil {
		for key, value := range parseFrontmatter(slide.Node.Attr("frontmatter")) {
			values[key] = value
		}
	}
	return shaderBackgroundRef(resolveSlideLayer(slide, "scene", layers.Scene), values)
}

func backgroundRefID(ref ComponentRef) string {
	source := graphicString(parseProps(ref.Props), "Src", "")
	if strings.HasPrefix(source, "shader:") {
		return graphicsKey(ref.Name, ref.Props)
	}
	return source
}

func shaderBackgroundOptions(source string, props map[string]any) (BackgroundOptions, error) {
	preset, err := backgroundPreset(strings.TrimPrefix(source, "shader:"))
	if err != nil {
		return BackgroundOptions{}, err
	}
	o := preset.Defaults
	for _, p := range []struct {
		name  string
		field *string
	}{{"Ink", &o.Ink}, {"Glow", &o.Glow}} {
		if value, exists := props[p.name]; exists {
			*p.field = fmt.Sprint(value)
		}
	}
	for _, p := range []struct {
		name  string
		field *float64
	}{{"Speed", &o.Speed}, {"Strength", &o.Strength}, {"Scale", &o.Scale}} {
		if value, exists := props[p.name]; exists {
			v, err := strconv.ParseFloat(fmt.Sprint(value), 64)
			if err != nil {
				return o, fmt.Errorf("background %s must be a number", strings.ToLower(p.name))
			}
			*p.field = v
		}
	}
	return o, o.validate()
}

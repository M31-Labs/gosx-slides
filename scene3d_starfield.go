package slides

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/scene"
	"m31labs.dev/gosx/server"
)

const (
	m31StarfieldPreset        = "m31-starfield"
	m31StarfieldSkyCount      = 5400
	m31StarfieldNearCount     = 380
	m31StarfieldTotalCount    = m31StarfieldSkyCount + m31StarfieldNearCount
	m31StarfieldCameraZ       = 520.0
	m31StarfieldFOV           = 52.0
	m31StarfieldColor         = "#eff8ff"
	m31StarfieldSkyShimmer    = 0.25
	m31StarfieldSkyPulseRate  = 0.75
	m31StarfieldNearShimmer   = 0.38
	m31StarfieldNearPulseRate = 1.20

	// Frustum placement. The earlier layers scattered stars through a cube
	// (spread 2000, group pushed to z -1300). From this camera that cube only
	// reaches about two thirds of the viewport width, so the projected sky
	// read as a bright column down the middle with bare edges — visible on a
	// 16:9 projector far more than on a laptop. Placing each star on the
	// frustum slice at its own depth makes screen density even by
	// construction, at any aspect.
	m31StarfieldAspect  = 2.18
	m31StarfieldMarginX = 1.12
	m31StarfieldMarginY = 1.38

	// Depth drift is what makes a star field read as a star field rather than
	// a photograph: each star advances toward the near plane, then wraps to
	// the far plane. Per-band rates live in m31StarfieldBands; they run from
	// about 40s at the near band to 80s at the far sky.
	m31StarfieldTau = 6.28318530718
)

// m31StarfieldFrustumPoint places one star on the camera frustum slice at its
// own depth, so projected density is even from edge to edge.
func m31StarfieldFrustumPoint(seed uint64, index, count int, salt uint64, distMin, distMax float64) scene.Vector3 {
	dist := m31StarfieldDepthDistance(seed, index, count, salt, distMin, distMax)
	halfH := math.Tan(m31StarfieldFOV*math.Pi/360.0) * dist
	u, v := m31StarfieldScreenSample(seed, index, count, salt)
	x := (u - 0.5) * 2 * halfH * m31StarfieldAspect * m31StarfieldMarginX
	y := (v - 0.5) * 2 * halfH * m31StarfieldMarginY
	return scene.Vec3(x, y, m31StarfieldCameraZ-dist)
}

func m31StarfieldDepthDistance(seed uint64, index, count int, salt uint64, distMin, distMax float64) float64 {
	if count <= 0 {
		return distMin
	}
	t := (float64(index) + m31StarRand(seed, uint64(index)*13+salt+2)) / float64(count)
	t = math.Mod(t+m31StarRand(seed, salt)*0.37, 1)
	return distMin + t*(distMax-distMin)
}

// m31StarfieldScreenSample stratifies stars over a grid before jittering, so
// no run of the generator leaves a visible bald patch on a projector.
func m31StarfieldScreenSample(seed uint64, index, count int, salt uint64) (float64, float64) {
	if count <= 0 {
		return 0.5, 0.5
	}
	cols := int(math.Ceil(math.Sqrt(float64(count) * m31StarfieldAspect)))
	if cols < 1 {
		cols = 1
	}
	rows := int(math.Ceil(float64(count) / float64(cols)))
	perm := (index*73 + int(salt)*19) % count
	col := perm % cols
	row := (perm / cols) % rows
	u := (float64(col) + m31StarRand(seed, uint64(index)*17+salt+5)) / float64(cols)
	v := (float64(row) + m31StarRand(seed, uint64(index)*17+salt+6)) / float64(rows)
	return m31Clamp(u, 0.002, 0.998), m31Clamp(v, 0.002, 0.998)
}

func m31Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// One process seed keeps presenter and audience views visually consistent
// while giving each server launch or static export a genuinely new sky.
var m31StarfieldSeed = m31NewStarfieldSeed()

// deckScene3DBackground mounts one persistent Scene3D surface behind the deck.
// It is deliberately a page-level engine rather than one engine per slide:
// navigation does not tear down the GPU context, and the same local scene is
// present in served and offline SPA builds.
func deckScene3DBackground(rt *server.PageRuntime, deck *IslandDeck) gosx.Node {
	if rt == nil || deck == nil || !strings.EqualFold(strings.TrimSpace(deckFrontmatterString(deck, "scene")), m31StarfieldPreset) {
		return gosx.Text("")
	}
	starfield := m31StarfieldScene().EngineConfig()
	starfield.MountAttrs["class"] = "deck-scene3d starfield-surface"
	starfield.MountAttrs["aria-hidden"] = "true"
	starfield.MountAttrs["data-starfield"] = "true"

	galaxy := m31ClosingGalaxyScene().EngineConfig()
	galaxy.MountAttrs["class"] = "deck-scene3d deck-galaxy3d"
	galaxy.MountAttrs["aria-hidden"] = "true"
	galaxy.MountAttrs["data-closing-galaxy"] = "true"

	return gosx.Fragment(
		rt.Engine(starfield, gosx.El("div", gosx.Attrs(gosx.Attr("class", "deck-starfield-fallback")))),
		rt.Engine(galaxy, gosx.Text("")),
	)
}

// m31ClosingGalaxyScene mounts the deterministic homepage galaxy GLB as a
// second, real Scene3D surface. CSS reveals it only while the closing slide is
// active; the PDF path uses the local OG still instead.
func m31ClosingGalaxyScene() scene.Props {
	return scene.Props{
		Label:               "M31 galaxy",
		AriaLabel:           "Animated M31 galaxy",
		Background:          "transparent",
		Responsive:          scene.Bool(true),
		FillHeight:          scene.Bool(true),
		CanvasAlpha:         scene.Bool(true),
		AutoRotate:          scene.Bool(true),
		PreferWebGL:         scene.Bool(true),
		ForceWebGL:          scene.Bool(true),
		MaxDevicePixelRatio: 1.5,
		Camera: scene.PerspectiveCamera{
			Position: scene.Vec3(0, 0, 500),
			FOV:      60,
			Near:     1,
			Far:      3000,
		},
		Environment: scene.Environment{
			AmbientColor:     "#0b0818",
			AmbientIntensity: 0,
			FogColor:         "#010106",
			FogDensity:       0,
			Exposure:         1,
			ToneMapping:      "aces",
		},
		Graph: scene.NewGraph(scene.Model{
			ID:     "m31-homepage-galaxy",
			Src:    "/public/m31-galaxy.glb",
			Static: scene.Bool(true),
		}),
	}
}

// m31StarfieldScene is the deck's ambient sky: six independent depth bands
// (VISUAL_SYSTEM.md) placed across the camera frustum and drifting toward the
// viewer, each band wrapping at its own rate so the field has parallax rather
// than moving as one sheet. It follows the m31labs.dev content-route star
// field, which replaced an earlier cubic scatter after that cube was found to
// cover only the middle of a wide frame. Total point budget and the 30fps cap
// are unchanged, so venue hardware sees the same load.
func m31StarfieldScene() scene.Props {
	return scene.Props{
		Label:               "M31 ambient star field",
		AriaLabel:           "Decorative star field background",
		Background:          "#010106",
		Responsive:          scene.Bool(true),
		FillHeight:          scene.Bool(true),
		CanvasAlpha:         scene.Bool(false),
		AutoRotate:          scene.Bool(false),
		PreferWebGL:         scene.Bool(true),
		ForceWebGL:          scene.Bool(true),
		DeferPostFX:         scene.Bool(false),
		MaxDevicePixelRatio: 1.75,
		MinDevicePixelRatio: 1,
		MaxPixels:           3_200_000,
		MaxFrameRate:        30,
		Environment: scene.Environment{
			AmbientColor:     "#0b142a",
			AmbientIntensity: 0,
			FogColor:         "#010106",
			FogDensity:       0,
			Exposure:         1,
			ToneMapping:      "aces",
		},
		Camera: scene.PerspectiveCamera{
			Position: scene.Vec3(0, 0, m31StarfieldCameraZ),
			FOV:      m31StarfieldFOV,
			Near:     1,
			Far:      2400,
		},
		Graph: scene.NewGraph(m31StarfieldDepthBands(m31StarfieldSeed)...),
	}
}

// m31StarfieldBand is one independent depth band. VISUAL_SYSTEM.md specifies
// six of them; the parallax between bands is what gives the projected sky its
// sense of volume, and it only reads if each band wraps at its own rate.
type m31StarfieldBand struct {
	ID        string
	Count     int
	Salt      uint64
	DistMin   float64
	DistMax   float64
	SizeMin   float64
	SizeSpan  float64
	SizeCurve float64
	MaxPixel  float64
	Shimmer   float64
	PulseRate float64
	// WrapSpeed is authored in world units per second; the shader clock is
	// derived from it so a band's drift stays proportional to its own depth
	// slab rather than to the scene as a whole.
	WrapSpeed float64
	// SpinY is a slow rotation layered on top of the drift.
	//
	// The two do different perceptual jobs and the field needs both. Drift
	// moves stars along the view axis, which reads strongly near the camera
	// and almost not at all in the far bands — so drift alone leaves the bulk
	// of the sky (the far bands hold most of the count) visually frozen.
	// Rotation moves every star tangentially, including distant ones, so it is
	// what makes the whole field read as alive from the back of a room.
	// Keeping it differential per band means the layers shear against each
	// other instead of turning as one rigid sheet.
	SpinY float64
	SpinX float64
}

// m31StarfieldBands runs near-to-far. Nearer bands hold fewer, larger, faster
// stars; the far sky holds the bulk of the count at low speed. Total points
// stay at the historical budget so the 30fps cap still holds on venue hardware.
func m31StarfieldBands() []m31StarfieldBand {
	return []m31StarfieldBand{
		{"starfield-near", 380, 97, 430, 960, 1.72, 3.52, 2.40, 5.0, m31StarfieldNearShimmer, m31StarfieldNearPulseRate, 83, 0.020, 0.007},
		{"starfield-inner", 520, 31, 900, 1400, 1.32, 3.12, 2.20, 5.1, 0.29, 1.06, 63, 0.016, 0.005},
		{"starfield-mid", 900, 53, 1300, 1750, 1.08, 2.92, 2.05, 5.3, 0.27, 0.96, 49, 0.012, 0.004},
		{"starfield-outer", 1200, 71, 1650, 2000, 1.00, 2.72, 1.90, 5.4, 0.25, 0.88, 34, 0.009, 0.003},
		{"starfield-deep", 1400, 113, 1900, 2200, 0.96, 2.58, 1.80, 5.5, 0.24, 0.80, 26, 0.007, 0.0015},
		{"starfield-stars", 1380, 11, 2100, 2300, 0.94, 3.28, 1.55, 5.6, m31StarfieldSkyShimmer, m31StarfieldSkyPulseRate, 15.7, 0.005, 0.0007},
	}
}

func m31StarfieldDepthBands(seed uint64) []scene.Node {
	bands := m31StarfieldBands()
	nodes := make([]scene.Node, 0, len(bands))
	for i, band := range bands {
		nodes = append(nodes, m31StarfieldBandLayer(seed+m31Mix64(uint64(i)*31+7), band))
	}
	return nodes
}

func m31StarfieldBandLayer(seed uint64, band m31StarfieldBand) scene.Points {
	positions := make([]scene.Vector3, band.Count)
	sizes := make([]float64, band.Count)
	for i := range positions {
		positions[i] = m31StarfieldFrustumPoint(seed, i, band.Count, band.Salt, band.DistMin, band.DistMax)
		sizes[i] = band.SizeMin + math.Pow(m31StarRand(seed, uint64(i)*7+3), band.SizeCurve)*band.SizeSpan
	}
	span := band.DistMax - band.DistMin
	if span <= 0 {
		span = 1
	}
	return scene.Points{
		ID:           band.ID,
		Count:        band.Count,
		Positions:    positions,
		Sizes:        sizes,
		Color:        m31StarfieldColor,
		Style:        scene.PointStyleGlow,
		Size:         1.7,
		MinPixelSize: 1.3,
		MaxPixelSize: band.MaxPixel,
		Opacity:      1,
		BlendMode:    scene.BlendAdditive,
		DepthWrite:   false,
		Attenuation:  false,
		// Drift (shader) carries depth; spin carries the whole field. No group
		// offset — stars sit at their true depth.
		Spin: scene.Euler{Y: band.SpinY, X: band.SpinX},
		Material: m31StarfieldTwinkleMaterial(band.Shimmer, band.PulseRate,
			(band.WrapSpeed/span)/m31StarfieldTau, band.DistMin, band.DistMax),
	}
}

// m31StarfieldTwinkleMaterial is the July 19 authored M31 glow envelope in
// direct Scene3D GLSL form. Scene3D supplies the reserved time uniform; the
// point size supplies stable per-star phase, and the slow pulse only removes
// light before returning each star to full brightness. That produces the
// site's crisp, irregular scintillation without making the whole sky breathe.
// driftRate is in cycles/second; distMin/distMax bound the depth slab a layer
// wraps through. The shader recovers each star's normalized screen position
// from its authored placement, then re-projects it at the drifted depth — so
// stars travel straight toward the viewer and the field stays evenly covered
// at every moment of the cycle.
func m31StarfieldTwinkleMaterial(amplitude, rate, driftRate, distMin, distMax float64) *scene.CustomMaterial {
	span := distMax - distMin
	if span <= 0 {
		span = 1
	}
	tanHalfFOV := math.Tan(m31StarfieldFOV * math.Pi / 360.0)
	material := &scene.CustomMaterial{
		ShaderBackend: "custom",
		ShaderLayout: map[string]any{
			"material": "M31StarfieldTwinkle",
			"uniformBlock": map[string]any{
				"size": 32,
				"fields": []map[string]any{
					{"name": "time", "type": "float", "offset": 0, "size": 4},
					{"name": "_pad", "type": "vec3", "offset": 16, "size": 12},
				},
			},
			"wgsl": map[string]any{"group": 1, "binding": 0},
		},
		Uniforms: map[string]any{"time": float32(0), "_pad": []float32{0, 0, 0}},
	}
	material.VertexGLSL = fmt.Sprintf(`#version 300 es
precision highp float;
precision highp int;

in vec3 a_position;
in float a_size;
in vec4 a_color;

uniform mat4 u_modelMatrix;
uniform mat4 u_viewMatrix;
uniform mat4 u_projectionMatrix;
uniform float u_defaultSize;
uniform vec4 u_defaultColor;
uniform bool u_hasPerVertexColor;
uniform bool u_hasPerVertexSize;
uniform bool u_sizeAttenuation;
uniform float u_viewportHeight;
uniform float u_minPixelSize;
uniform float u_maxPixelSize;
uniform int u_hasFog;
uniform float u_fogDensity;
uniform float time;

out vec4 v_color;
out float v_fogFactor;
out float v_pointSize;
out float v_pulse;

const float starfieldPulseAmp = %.6f;
const float starfieldPulseRate = %.6f;
const float starfieldDepthRate = %.9f;
const float starfieldDistMin = %.4f;
const float starfieldSpan = %.4f;
const float starfieldCameraZ = %.4f;
const float starfieldTanHalfFOV = %.6f;
const float starfieldAspect = %.4f;
const float starfieldMarginX = %.4f;
const float starfieldMarginY = %.4f;

float starfieldFract(float v) {
	return v - floor(v);
}

void main() {
	float baseDist = clamp(starfieldCameraZ - a_position.z, starfieldDistMin, starfieldDistMin + starfieldSpan);
	float baseHalfH = max(starfieldTanHalfFOV * baseDist, 0.001);
	float nx = a_position.x / max(baseHalfH * starfieldAspect * starfieldMarginX, 0.001);
	float ny = a_position.y / max(baseHalfH * starfieldMarginY, 0.001);
	float phase = starfieldFract((baseDist - starfieldDistMin) / starfieldSpan - starfieldFract(time * starfieldDepthRate));
	float dist = starfieldDistMin + phase * starfieldSpan;
	float halfH = starfieldTanHalfFOV * dist;
	vec3 drifted = vec3(
		nx * halfH * starfieldAspect * starfieldMarginX,
		ny * halfH * starfieldMarginY,
		starfieldCameraZ - dist);

	vec4 world = u_modelMatrix * vec4(drifted, 1.0);
	vec4 viewPos = u_viewMatrix * world;
	gl_Position = u_projectionMatrix * viewPos;

	float size = u_hasPerVertexSize ? a_size : u_defaultSize;
	float nearBoost = 1.0 + pow(1.0 - phase, 2.0) * 0.36;
	size = size * nearBoost;
	// Mix two slow, position-seeded pulses. Using only point size as the phase
	// made nearby stars breathe in loose cohorts; the spatial seed gives the
	// projector a crisp, irregular twinkle without making the whole sky flash.
	float phaseSeed = dot(a_position.xy, vec2(0.0067, 0.0091)) + a_position.z * 0.0023 + size * 1.7;
	float primaryPulse = sin(time * starfieldPulseRate + phaseSeed);
	float secondaryPulse = sin(time * starfieldPulseRate * 1.71 + phaseSeed * 1.37) * 0.22;
	v_pulse = 1.0 + starfieldPulseAmp * (0.82 * primaryPulse + secondaryPulse);
	float pixelSize;
	if (u_sizeAttenuation) {
		pixelSize = max(size * (u_viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);
	} else {
		pixelSize = max(size, 1.0);
	}
	pixelSize = max(pixelSize, max(u_minPixelSize, 0.0));
	if (u_maxPixelSize > 0.0) {
		pixelSize = min(pixelSize, u_maxPixelSize);
	}
	gl_PointSize = pixelSize;
	v_pointSize = pixelSize;
	v_color = u_hasPerVertexColor ? a_color : u_defaultColor;
	if (u_hasFog != 0) {
		float d = length(viewPos.xyz);
		v_fogFactor = clamp(exp(-u_fogDensity * u_fogDensity * d * d), 0.0, 1.0);
	} else {
		v_fogFactor = 1.0;
	}
}`, amplitude, rate, driftRate, distMin, span, m31StarfieldCameraZ, tanHalfFOV,
		m31StarfieldAspect, m31StarfieldMarginX, m31StarfieldMarginY)
	material.FragmentGLSL = `#version 300 es
precision highp float;
precision highp int;

in vec4 v_color;
in float v_fogFactor;
in float v_pointSize;
in float v_pulse;

uniform float u_opacity;
uniform vec3 u_fogColor;

out vec4 fragColor;

void main() {
	vec2 centered = gl_PointCoord - vec2(0.5);
	float radial = length(centered) * 2.0;
	float sizeFocus = clamp((v_pointSize - 4.0) / 48.0, 0.0, 1.0);
	float falloff = mix(5.2, 4.0, sizeFocus);
	float core = exp(-(radial * radial * falloff));
	float edge = 1.0 - smoothstep(0.78, 1.0, radial);
	float alpha = core * edge * v_color.a * u_opacity * v_pulse;
	vec3 color = mix(u_fogColor, v_color.rgb, v_fogFactor);
	fragColor = vec4(color, alpha);
}`
	return material
}

func m31NewStarfieldSeed() uint64 {
	var raw [8]byte
	if _, err := cryptorand.Read(raw[:]); err == nil {
		if seed := binary.LittleEndian.Uint64(raw[:]); seed != 0 {
			return seed
		}
	}
	return 0x6d33316c61627321
}

func m31StarRand(seed, lane uint64) float64 {
	value := m31Mix64(seed + lane*0x9e3779b97f4a7c15)
	return float64(value>>11) * (1.0 / (1 << 53))
}

func m31Mix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

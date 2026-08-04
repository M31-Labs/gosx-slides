package slides

import (
	"math"
	"strings"
	"testing"

	"m31labs.dev/gosx/scene"
)

func TestM31StarfieldMatchesHistoricalM31ContentScene(t *testing.T) {
	props := m31StarfieldScene()
	if props.MaxFrameRate != 30 {
		t.Fatalf("max frame rate = %.0f, want the active M31 content-scene budget of 30", props.MaxFrameRate)
	}
	if props.MaxPixels != 3_200_000 {
		t.Fatalf("max pixels = %d, want the historical M31 density-preserving render target", props.MaxPixels)
	}
	if props.Camera.Position.Z != 520 || props.Camera.FOV != 52 || props.Camera.Far != 2400 {
		t.Fatalf("camera = %+v, want the July 19 M31 content camera", props.Camera)
	}
	// VISUAL_SYSTEM.md specifies six independent depth bands.
	if got, want := len(props.Graph.Nodes), len(m31StarfieldBands()); got != want {
		t.Fatalf("graph nodes = %d, want %d independent depth bands", got, want)
	}

	const coverageColumns = 12
	const coverageRows = 7
	const aspect = 16.0 / 9.0
	coverage := make([]int, coverageColumns*coverageRows)
	layers := make([]scene.Points, 0, len(props.Graph.Nodes))
	total := 0
	minDepth := math.Inf(1)
	maxDepth := math.Inf(-1)
	tanHalfFOV := math.Tan(m31StarfieldFOV * math.Pi / 360)

	for i, node := range props.Graph.Nodes {
		points, ok := node.(scene.Points)
		if !ok {
			t.Fatalf("graph node %d = %T, want scene.Points", i, node)
		}
		layers = append(layers, points)
		if points.Count != len(points.Positions) || points.Count != len(points.Sizes) {
			t.Fatalf("%s count=%d positions=%d sizes=%d", points.ID, points.Count, len(points.Positions), len(points.Sizes))
		}
		if points.Attenuation {
			t.Fatalf("%s must keep screen-space point sizes like the M31 content scene", points.ID)
		}
		if points.Opacity != 1 {
			t.Fatalf("%s opacity = %.2f, want the historical luminous-look clamp of 1", points.ID, points.Opacity)
		}
		if points.Color != m31StarfieldColor || points.BlendMode != scene.BlendAdditive || points.DepthWrite {
			t.Fatalf("%s visual envelope drifted: color=%s blend=%s depthWrite=%v", points.ID, points.Color, points.BlendMode, points.DepthWrite)
		}
		if points.Material == nil {
			t.Fatalf("%s has no authored M31 glow material", points.ID)
		}
		if points.Material.ShaderBackend != "custom" {
			t.Fatalf("%s shader backend = %q, want custom", points.ID, points.Material.ShaderBackend)
		}
		if !strings.Contains(points.Material.VertexGLSL, "uniform float time;") ||
			!strings.Contains(points.Material.VertexGLSL, "size * 1.7") ||
			!strings.Contains(points.Material.FragmentGLSL, "v_pulse") ||
			!strings.Contains(points.Material.FragmentGLSL, "smoothstep(0.78, 1.0, radial)") {
			t.Fatalf("%s material is missing the July 19 authored twinkle/glow path", points.ID)
		}
		if _, ok := points.Material.Uniforms["time"]; !ok {
			t.Fatalf("%s material does not expose Scene3D's reserved time uniform", points.ID)
		}
		total += points.Count
		for _, local := range points.Positions {
			position := scene.Vec3(local.X+points.Position.X, local.Y+points.Position.Y, local.Z+points.Position.Z)
			minDepth = math.Min(minDepth, position.Z)
			maxDepth = math.Max(maxDepth, position.Z)
			distance := m31StarfieldCameraZ - position.Z
			halfHeight := distance * tanHalfFOV
			halfWidth := halfHeight * aspect
			x := position.X / halfWidth
			y := position.Y / halfHeight
			if x < -1 || x >= 1 || y < -1 || y >= 1 {
				continue
			}
			column := int((x + 1) * 0.5 * coverageColumns)
			row := int((y + 1) * 0.5 * coverageRows)
			coverage[row*coverageColumns+column]++
		}
	}

	if got, want := total, m31StarfieldTotalCount; got != want {
		t.Fatalf("total points = %d, want %d", got, want)
	}
	// The layers now span the frustum slab (sky 700..2300, near 430..960)
	// rather than a cube, so the depth range is bounded by those bands.
	if got := maxDepth - minDepth; got < 1700 {
		t.Fatalf("depth span = %.1f, want the full sky-to-near frustum slab", got)
	}
	// Coverage is the projector contract. A cube of stars viewed through this
	// camera only fills the middle of a 16:9 frame, which read as a bright
	// column with bare edges on anything wider than a laptop. Frustum
	// placement makes every bucket populated by construction — this assertion
	// is what stops a regression back to the cube.
	for i, count := range coverage {
		if count < 3 {
			t.Fatalf("visible coverage bucket %d has %d stars, want an evenly covered frame", i, count)
		}
	}

	// Every band must carry BOTH kinds of motion. Drift alone leaves the far
	// bands — which hold most of the stars — visually frozen, because distant
	// stars have almost no screen-space parallax along the view axis. Rotation
	// alone was the old behaviour and gave no depth. The field only reads as
	// alive from the back of a room when both are present.
	for _, layer := range layers {
		if layer.Spin == (scene.Euler{}) {
			t.Fatalf("%s has no rotation; drift alone leaves the far sky static", layer.ID)
		}
		if layer.Spin.Y > 0.05 {
			t.Fatalf("%s spinY = %.4f, too fast — the sky would read as a turning sheet", layer.ID, layer.Spin.Y)
		}
		if layer.Position.Z != 0 {
			t.Fatalf("%s group offset z = %.1f, want frustum-placed stars at true depth", layer.ID, layer.Position.Z)
		}
		for _, want := range []string{"starfieldDepthRate", "starfieldFract(time * starfieldDepthRate)", "nearBoost"} {
			if !strings.Contains(layer.Material.VertexGLSL, want) {
				t.Fatalf("%s is frozen: missing depth-drift term %q", layer.ID, want)
			}
		}
		if strings.Contains(layer.Material.VertexGLSL, "starfieldDepthRate = 0.000000000") {
			t.Fatalf("%s depth rate is zero — the band would render as a still image", layer.ID)
		}
	}

	// Parallax: bands must wrap at monotonically slower rates going outward.
	// If they converge the sky moves as one sheet and the volume collapses.
	prev := 0.0
	for _, band := range m31StarfieldBands() {
		span := band.DistMax - band.DistMin
		period := span * m31StarfieldTau / band.WrapSpeed
		if period <= prev {
			t.Fatalf("%s wraps every %.0fs, want slower than the band inside it (%.0fs)", band.ID, period, prev)
		}
		if period > 120 {
			t.Fatalf("%s wraps every %.0fs, too slow to read as motion on a projector", band.ID, period)
		}
		prev = period
	}

	// Rotation must also be differential and decrease outward, so the bands
	// shear against each other rather than turning as one rigid sheet.
	prevSpin := math.Inf(1)
	for _, band := range m31StarfieldBands() {
		if band.SpinY <= 0 {
			t.Fatalf("%s has no rotation rate", band.ID)
		}
		if band.SpinY >= prevSpin {
			t.Fatalf("%s spinY = %.4f, want slower than the band inside it (%.4f)", band.ID, band.SpinY, prevSpin)
		}
		prevSpin = band.SpinY
	}

}

func TestM31ClosingGalaxySceneUsesOfflineModelAndAnimation(t *testing.T) {
	props := m31ClosingGalaxyScene()
	if props.AutoRotate == nil || !*props.AutoRotate {
		t.Fatal("closing galaxy must animate")
	}
	if props.CanvasAlpha == nil || !*props.CanvasAlpha || props.Background != "transparent" {
		t.Fatal("closing galaxy must composite over the local first-paint underlay")
	}
	if len(props.Graph.Nodes) != 1 {
		t.Fatalf("graph nodes = %d, want 1 offline model", len(props.Graph.Nodes))
	}
	model, ok := props.Graph.Nodes[0].(scene.Model)
	if !ok {
		t.Fatalf("graph node = %T, want scene.Model", props.Graph.Nodes[0])
	}
	if model.Src != "/public/m31-galaxy.glb" {
		t.Fatalf("model src = %q, want offline galaxy GLB", model.Src)
	}
}

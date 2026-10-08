package main

import (
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
	"m31labs.dev/selena"
)

func TestBackgroundSourceCustomSettingsCompile(t *testing.T) {
	for _, preset := range slides.BackgroundPresets() {
		t.Run(preset.Name, func(t *testing.T) {
			original, err := backgroundSource([]string{preset.Name})
			if err != nil || original != preset.Source {
				t.Fatalf("default source changed: %v", err)
			}
			source, err := backgroundSource([]string{preset.Name, "--ink", "#112233", "--glow=#abcdef", "--speed", "0", "--strength", "1", "--scale", "4"})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"rgb(0.066667, 0.133333, 0.200000)", "rgb(0.670588, 0.803922, 0.937255)", "param speed : float = 0.0", "param strength : float = 1.0", "param scale : float = 4.0"} {
				if !strings.Contains(source, want) {
					t.Errorf("export is missing %s", want)
				}
			}
			if _, err := selena.Compile([]byte(source), selena.CompileOptions{Targets: []selena.Target{selena.TargetGLSL, selena.TargetWGSL}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBackgroundSourceRejectsInvalidSettings(t *testing.T) {
	for _, args := range [][]string{
		nil, {"unknown"}, {"aurora", "extra"}, {"aurora", "--speed"},
		{"aurora", "--speeed", "1"}, {"aurora", "--speed", "fast"},
		{"aurora", "--ink", "red"}, {"aurora", "--glow", "#gggggg"},
		{"aurora", "--speed", "-1"}, {"aurora", "--speed", "2.1"},
		{"aurora", "--speed", "NaN"}, {"aurora", "--strength", "Inf"},
		{"aurora", "--strength", "1.1"}, {"aurora", "--scale", "0"},
		{"aurora", "--scale", "4.1"},
	} {
		if source, err := backgroundSource(args); err == nil || source != "" {
			t.Errorf("invalid args %q produced shader output or no error: %v", args, err)
		}
	}
}

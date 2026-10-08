package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	slides "m31labs.dev/gosx-slides"
)

func backgroundsCommand(args []string) error {
	if len(args) > 0 && args[0] == "source" {
		source, err := backgroundSource(args[1:])
		if err != nil {
			return err
		}
		fmt.Print(source)
		return nil
	}
	jsonOut, rest := takeBoolFlag(args, "json")
	if len(rest) != 0 {
		return fmt.Errorf("usage: slides backgrounds [--json] | slides backgrounds source <preset>")
	}
	presets := slides.BackgroundPresets()
	if jsonOut {
		raw, err := json.MarshalIndent(presets, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	for _, preset := range presets {
		fmt.Printf("%s — %s\n  %s\n", preset.Name, preset.Title, preset.Description)
	}
	fmt.Println("Use scene: shader:<preset> in deck headmatter or slide YAML.\nOpen Backgrounds while serving with --edit to preview, tune and apply.\nCopy a standalone shader: slides backgrounds source <preset> > background.sel")
	return nil
}

// Export the same validated controls as the wizard, with preset defaults for
// omitted flags. Keep stdout exclusively shader source for shell redirection.
func backgroundSource(args []string) (string, error) {
	const usage = "usage: slides backgrounds source <preset> [--ink #rrggbb] [--glow #rrggbb] [--speed 0..2] [--strength 0..1] [--scale 0.5..4]"
	if len(args) == 0 {
		return "", fmt.Errorf("%s", usage)
	}
	for _, preset := range slides.BackgroundPresets() {
		if preset.Name != args[0] {
			continue
		}
		if len(args) == 1 {
			return preset.Source, nil
		}
		opts := preset.Defaults
		flags := flag.NewFlagSet("background source", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.StringVar(&opts.Ink, "ink", opts.Ink, "base color (#rrggbb)")
		flags.StringVar(&opts.Glow, "glow", opts.Glow, "highlight color (#rrggbb)")
		flags.Float64Var(&opts.Speed, "speed", opts.Speed, "motion speed (0–2)")
		flags.Float64Var(&opts.Strength, "strength", opts.Strength, "highlight strength (0–1)")
		flags.Float64Var(&opts.Scale, "scale", opts.Scale, "pattern scale (0.5–4)")
		if err := flags.Parse(args[1:]); err != nil {
			return "", fmt.Errorf("%w; %s", err, usage)
		}
		if flags.NArg() != 0 {
			return "", fmt.Errorf("unexpected argument %q; %s", flags.Arg(0), usage)
		}
		return slides.BackgroundShaderSource(opts)
	}
	return "", fmt.Errorf("unknown background %q; use slides backgrounds to list presets", args[0])
}

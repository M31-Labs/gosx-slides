package main

import (
	"encoding/json"
	"fmt"

	slides "m31labs.dev/gosx-slides"
)

func backgroundsCommand(args []string) error {
	if len(args) == 2 && args[0] == "source" {
		for _, preset := range slides.BackgroundPresets() {
			if preset.Name == args[1] {
				fmt.Print(preset.Source)
				return nil
			}
		}
		return fmt.Errorf("unknown background %q; use slides backgrounds to list presets", args[1])
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

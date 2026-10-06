package main

import (
	"fmt"

	slides "m31labs.dev/gosx-slides"
)

func runtimeCommand(args []string) error {
	if len(args) == 0 || args[0] != "pack" {
		return fmt.Errorf("usage: slides runtime pack [deck-dir] --out <fresh-runtime-dir> [--json]")
	}
	jsonOut, rest := takeBoolFlag(args[1:], "json")
	out, rest, err := takeStringFlag(rest, "out", "runtime")
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return fmt.Errorf("runtime pack accepts one deck directory")
	}
	bundle, err := slides.PackageRuntime(deckDir(rest), out)
	if err != nil {
		return err
	}
	if jsonOut {
		return printSemanticJSON(bundle)
	}
	fmt.Printf("packaged %d runtime assets for GoSX %s in %s\n", len(bundle.Assets), bundle.GoSXVersion, out)
	return nil
}

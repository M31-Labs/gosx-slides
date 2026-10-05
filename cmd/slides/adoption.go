package main

import (
	"encoding/json"
	"fmt"

	slides "m31labs.dev/gosx-slides"
)

func templatesCommand(args []string) error {
	jsonOut, rest := takeBoolFlag(args, "json")
	if len(rest) != 0 {
		return fmt.Errorf("usage: slides templates [--json]")
	}
	templates := slides.DeckTemplates()
	if jsonOut {
		payload, err := json.MarshalIndent(templates, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	for _, template := range templates {
		fmt.Printf("%s (%s, %d slides)\n  %s\n", template.Name, template.Theme, template.Slides, template.Description)
	}
	fmt.Println("Create one: slides init <new-directory> --template <name>")
	return nil
}

func migrateCommand(args []string) error {
	jsonOut, rest := takeBoolFlag(args, "json")
	format, rest, err := takeStringFlag(rest, "from", "")
	if err != nil {
		return err
	}
	output, rest, err := takeStringFlag(rest, "out", "")
	if err != nil {
		return err
	}
	if len(rest) != 1 || format == "" || output == "" {
		return fmt.Errorf("usage: slides migrate <source.md> --from slidev|marp|quarto --out <new-directory> [--json]")
	}
	report, err := slides.MigrateMarkdown(rest[0], output, slides.MigrationOptions{Format: format})
	if err != nil {
		return err
	}
	if jsonOut {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Printf("migrated %d slides and %d local assets to %s\n", report.Slides, report.Assets, report.Destination)
	for _, diagnostic := range report.Diagnostics {
		fmt.Printf("%s:%d:%d: %s: %s\n", diagnostic.File, diagnostic.Range.StartLine, diagnostic.Range.StartCol, diagnostic.Code, diagnostic.Message)
	}
	fmt.Println("Review migration/report.json and migration/source.md before presenting.")
	return nil
}

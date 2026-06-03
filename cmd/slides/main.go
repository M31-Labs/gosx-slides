package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	slides "m31labs.dev/gosx-slides"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "slides:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "new":
		theme, rest, err := takeStringFlag(args[1:], "theme", "m31")
		if err != nil {
			return err
		}
		template, rest, err := takeStringFlag(rest, "template", "default")
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return fmt.Errorf("usage: slides new <name> [--theme m31] [--template default|gotreesitter]")
		}
		return slides.ScaffoldWithOptions(rest[0], slides.ScaffoldOptions{Theme: theme, Template: template})
	case "check":
		path := deckPath(args[1:])
		summary, err := slides.Check(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", summary.Title)
		fmt.Printf("slides: %d\n", summary.SlideCount)
		fmt.Printf("clicks: %d\n", summary.TotalClicks)
		fmt.Printf("notes: %d\n", summary.Notes)
		var layouts []string
		for layout := range summary.Layouts {
			layouts = append(layouts, layout)
		}
		sort.Strings(layouts)
		for _, layout := range layouts {
			fmt.Printf("layout %s: %d\n", layout, summary.Layouts[layout])
		}
		return nil
	case "fmt":
		path := deckPath(args[1:])
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted := slides.FormatSource(string(src))
		if _, err := slides.Parse(formatted, slides.ParseOptions{SourcePath: path}); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(formatted), 0o644)
	case "inspect":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		path := deckPath(rest)
		deck, err := slides.ParseFile(path)
		if err != nil {
			return err
		}
		analysis := slides.Analyze(deck)
		if jsonOut {
			payload, err := json.MarshalIndent(analysis, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
			return nil
		}
		printAnalysis(analysis)
		return nil
	case "validate":
		strict, rest := takeBoolFlag(args[1:], "strict")
		profile, rest, err := takeStringFlag(rest, "profile", "standard")
		if err != nil {
			return err
		}
		path := deckPath(rest)
		deck, err := slides.ParseFile(path)
		if err != nil {
			return err
		}
		report := slides.Validate(deck, slides.ValidateOptions{Profile: profile})
		if len(report.Errors) == 0 && len(report.Warnings) == 0 {
			fmt.Printf("%s is valid (%s)\n", path, report.Profile)
			return nil
		}
		fmt.Printf("%s validation profile %s:\n", path, report.Profile)
		for _, errText := range report.Errors {
			fmt.Printf("error: %s\n", errText)
		}
		for _, warning := range report.Warnings {
			fmt.Printf("warning: %s\n", warning)
		}
		if !report.Passed(strict) {
			if strict {
				return fmt.Errorf("strict validation failed")
			}
			return fmt.Errorf("validation failed")
		}
		return nil
	case "rehearse":
		path := deckPath(args[1:])
		deck, err := slides.ParseFile(path)
		if err != nil {
			return err
		}
		fmt.Print(slides.RehearsalScript(deck))
		return nil
	case "split":
		out, rest, err := takeStringFlag(args[1:], "out", "")
		if err != nil {
			return err
		}
		if len(rest) != 1 || out == "" {
			return fmt.Errorf("usage: slides split <deck.md> --out <deck-dir>")
		}
		return slides.SplitDeck(rest[0], out)
	case "merge":
		out, rest, err := takeStringFlag(args[1:], "out", "")
		if err != nil {
			return err
		}
		if len(rest) != 1 || out == "" {
			return fmt.Errorf("usage: slides merge <deck-dir> --out <deck.md>")
		}
		return slides.MergeDeck(rest[0], out)
	case "components":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		if len(rest) != 0 {
			return fmt.Errorf("usage: slides components [--json]")
		}
		if jsonOut {
			payload, err := json.MarshalIndent(slides.BuiltInComponents(), "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
			return nil
		}
		printComponents(slides.BuiltInComponents())
		return nil
	case "themes":
		// List the real-lane themes selectable via deck headmatter `theme: <name>`.
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		if len(rest) != 0 {
			return fmt.Errorf("usage: slides themes [--json]")
		}
		if jsonOut {
			payload, err := json.MarshalIndent(slides.Themes(), "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
			return nil
		}
		for _, theme := range slides.Themes() {
			fmt.Println(theme)
		}
		return nil
	case "doctor":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		report, err := slides.Doctor(deckPath(rest))
		if err != nil {
			return err
		}
		if jsonOut {
			payload, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
		} else {
			printDoctor(report)
		}
		if report.HasFailures() {
			return fmt.Errorf("doctor found failures")
		}
		return nil
	case "serve":
		// Real lane: serve a deck whose slides host live GoSX islands, staging
		// the client WASM runtime so they hydrate in the browser. Distinct from
		// the fallback `dev`/`present`, which run the HTML presenter.
		port, rest, err := takeIntFlag(args[1:], "port", 8080)
		if err != nil {
			return err
		}
		// --rebuild forces a fresh GOOS=js runtime.wasm build; the wasm is
		// existence-cached, so this is how a gosx runtime change is picked up.
		rebuild, rest := takeBoolFlag(rest, "rebuild")
		// --watch turns the real lane into the hot-swap dev loop: editing a
		// component .gsx hot-swaps the live island in place (no reload, state
		// preserved); editing deck.md full-reloads with the new content. It fronts
		// the in-process deck server with the gosx dev proxy.
		watch, rest := takeBoolFlag(rest, "watch")
		dir := deckDir(rest)
		if watch {
			fmt.Printf("gosx-slides real lane (hot-swap) serving %s at http://%s\n", dir, addr(port))
			return slides.DevDeck(dir, slides.DevOptions{Addr: addr(port), RebuildRuntime: rebuild})
		}
		fmt.Printf("gosx-slides real lane serving %s at http://%s\n", dir, addr(port))
		return slides.ServeDeck(dir, slides.ServeOptions{Addr: addr(port), StageRuntime: true, RebuildRuntime: rebuild})
	case "dev":
		port, rest, err := takeIntFlag(args[1:], "port", 8080)
		if err != nil {
			return err
		}
		return slides.Serve(deckPath(rest), slides.ServerOptions{Mode: "dev", Addr: addr(port)})
	case "present":
		port, rest, err := takeIntFlag(args[1:], "port", 8080)
		if err != nil {
			return err
		}
		return slides.Serve(deckPath(rest), slides.ServerOptions{Mode: "present", Addr: addr(port)})
	case "build":
		out, rest, err := takeStringFlag(args[1:], "out", "dist")
		if err != nil {
			return err
		}
		return slides.Export(deckPath(rest), slides.ExportOptions{Format: "spa", OutDir: out})
	case "export":
		format, rest, err := takeStringFlag(args[1:], "format", "spa")
		if err != nil {
			return err
		}
		out, rest, err := takeStringFlag(rest, "out", "dist")
		if err != nil {
			return err
		}
		return slides.Export(deckPath(rest), slides.ExportOptions{Format: format, OutDir: out})
	case "version":
		fmt.Println("gosx-slides v0.1.0")
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func deckPath(args []string) string {
	if len(args) == 0 {
		return "deck.md"
	}
	return filepath.Clean(args[0])
}

// deckDir resolves a deck DIRECTORY argument for the real lane (which reads
// <dir>/deck.md). It accepts either a directory or a path to a deck.md and
// returns the containing directory; with no argument it defaults to ".".
func deckDir(args []string) string {
	if len(args) == 0 {
		return "."
	}
	p := filepath.Clean(args[0])
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		return filepath.Dir(p)
	}
	if strings.HasSuffix(p, ".md") {
		return filepath.Dir(p)
	}
	return p
}

func takeStringFlag(args []string, name, fallback string) (string, []string, error) {
	value := fallback
	var rest []string
	long := "--" + name
	short := "-" + name
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == long || arg == short {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s requires a value", long)
			}
			value = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, long+"=") {
			value = strings.TrimPrefix(arg, long+"=")
			continue
		}
		rest = append(rest, arg)
	}
	return value, rest, nil
}

func takeIntFlag(args []string, name string, fallback int) (int, []string, error) {
	raw, rest, err := takeStringFlag(args, name, strconv.Itoa(fallback))
	if err != nil {
		return 0, nil, err
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil, fmt.Errorf("--%s must be an integer", name)
	}
	return value, rest, nil
}

func takeBoolFlag(args []string, name string) (bool, []string) {
	value := false
	var rest []string
	long := "--" + name
	short := "-" + name
	for _, arg := range args {
		if arg == long || arg == short {
			value = true
			continue
		}
		rest = append(rest, arg)
	}
	return value, rest
}

func printAnalysis(analysis slides.DeckAnalysis) {
	fmt.Printf("%s\n", analysis.Title)
	fmt.Printf("slides: %d\n", analysis.SlideCount)
	fmt.Printf("clicks: %d\n", analysis.TotalClicks)
	fmt.Printf("words: %d\n", analysis.WordCount)
	fmt.Printf("estimated: %s\n", formatDuration(analysis.EstimatedSeconds))
	var components []string
	for component := range analysis.Components {
		components = append(components, component)
	}
	sort.Strings(components)
	for _, component := range components {
		fmt.Printf("component %s: %d\n", component, analysis.Components[component])
	}
	if len(analysis.Warnings) > 0 {
		fmt.Println("warnings:")
		for _, warning := range analysis.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}
}

func formatDuration(seconds int) string {
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}

func addr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func usage() {
	fmt.Println(strings.TrimSpace(`
slides is the gosx-slides command.

Commands:
  new <name> [--theme m31] [--template default|gotreesitter]
  check [deck.md]
  fmt [deck.md]
  inspect [deck.md] [--json]
  validate [deck.md] [--strict] [--profile standard|conference|demo|lecture]
  rehearse [deck.md]
  split <deck.md> --out <deck-dir>
  merge <deck-dir> --out <deck.md>
  components [--json]
  themes [--json]                                       (real-lane themes selectable via deck headmatter "theme: <name>")
  doctor [deck.md] [--json]
  serve [deck-dir] [--port 8080] [--rebuild] [--watch]   (real lane: live GoSX islands, hydrated; --watch = hot-swap dev loop: .gsx hot-swaps in place, deck.md reloads; --rebuild forces a fresh runtime.wasm)
  dev [deck.md] [--port 8080]                            (fallback HTML presenter; for the real-lane hot-swap loop use serve --watch)
  present [deck.md] [--port 8080]
  build [deck.md] [--out dist]
  export [deck.md] --format spa|single|pdf|png [--out dist]
  version
`))
}

func printComponents(components []slides.ComponentInfo) {
	currentPack := ""
	for _, component := range components {
		if component.Pack != currentPack {
			currentPack = component.Pack
			fmt.Printf("%s:\n", currentPack)
		}
		fmt.Printf("  %-15s %s\n", component.Name, component.Description)
	}
}

func printDoctor(report slides.DoctorReport) {
	for _, item := range report.Items {
		fmt.Printf("%-10s %-4s %s\n", item.Name, item.Status, item.Detail)
	}
}

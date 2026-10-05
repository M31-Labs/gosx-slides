package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	slides "m31labs.dev/gosx-slides"
)

var version = "v0.9.0-dev"

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
	case "story":
		return storyCommand(args[1:])
	case "tour":
		return tourCommand(args[1:])
	case "audiences":
		return audiencesCommand(args[1:])
	case "init":
		// Scaffold a deck you can `slides serve` immediately (live
		// islands + evaluated {expr} + highlighted code + theme).
		theme, rest, err := takeStringFlag(args[1:], "theme", "aurora")
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return fmt.Errorf("usage: slides init <name> [--theme aurora|paper|neon|swiss]")
		}
		name := rest[0]
		if err := slides.ScaffoldRealLane(name, slides.ScaffoldRealOptions{Theme: theme}); err != nil {
			return err
		}
		fmt.Printf("created deck %q (theme %s)\n", name, theme)
		fmt.Printf("run it:  slides serve %s\n", name)
		return nil
	case "check":
		summary, err := slides.Check(deckDir(args[1:]))
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
	case "inspect":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		deck, err := slides.LoadIslandDeck(deckDir(rest))
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
		dir := deckDir(rest)
		deck, err := slides.LoadIslandDeck(dir)
		if err != nil {
			return err
		}
		report := slides.Validate(deck, slides.ValidateOptions{Profile: profile})
		if len(report.Errors) == 0 && len(report.Warnings) == 0 {
			fmt.Printf("%s is valid (%s)\n", dir, report.Profile)
			return nil
		}
		fmt.Printf("%s validation profile %s:\n", dir, report.Profile)
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
		deck, err := slides.LoadIslandDeck(deckDir(args[1:]))
		if err != nil {
			return err
		}
		fmt.Print(slides.RehearsalScript(deck))
		return nil
	case "components":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		deck, err := slides.LoadIslandDeck(deckDir(rest))
		if err != nil {
			return err
		}
		components := slides.DeckComponents(deck)
		if jsonOut {
			payload, err := json.MarshalIndent(components, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
			return nil
		}
		printDeckComponents(components)
		return nil
	case "themes":
		// List the themes selectable via deck headmatter `theme: <name>`.
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
	case "packs":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		if len(rest) > 1 {
			return fmt.Errorf("usage: slides packs [deck-dir] [--json]")
		}
		deck, err := slides.LoadIslandDeck(deckDir(rest))
		if err != nil {
			return err
		}
		packs := slides.DeckPacks(deck)
		if jsonOut {
			payload, err := json.MarshalIndent(packs, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(payload))
		} else {
			if len(packs) == 0 {
				fmt.Println("no enabled packs")
			}
			for _, pack := range packs {
				fmt.Printf("%s@%s  %s\n", pack.Name, pack.Version, pack.Path)
			}
		}
		return nil
	case "pack":
		if len(args) < 3 || len(args) > 4 || args[1] != "install" {
			return fmt.Errorf("usage: slides pack install <source-dir> [deck-dir]")
		}
		manifest, err := slides.InstallDeckPack(deckDir(args[3:]), args[2])
		if err != nil {
			return err
		}
		fmt.Printf("installed %s@%s; enable in deck headmatter: packs: %s@%s\n", manifest.Name, manifest.Version, manifest.Name, manifest.Version)
		return nil
	case "doctor":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		report, err := slides.Doctor(deckDir(rest))
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
		// Serve a deck whose slides host live GoSX islands, staging the client WASM
		// runtime so they hydrate in the browser, with the presenter SSE endpoints
		// mounted alongside.
		port, rest, err := takeIntFlag(args[1:], "port", 8080)
		if err != nil {
			return err
		}
		// --rebuild forces a fresh GOOS=js runtime.wasm build; the wasm is
		// existence-cached, so this is how a gosx runtime change is picked up.
		rebuild, rest := takeBoolFlag(rest, "rebuild")
		// --watch is the hot-swap dev loop: editing a component .gsx hot-swaps the
		// live island in place (no reload, state preserved); editing deck.md
		// full-reloads. It fronts the in-process deck server with the gosx dev proxy.
		watch, rest := takeBoolFlag(rest, "watch")
		edit, rest := takeBoolFlag(rest, "edit")
		collaborate, rest := takeBoolFlag(rest, "collab")
		if collaborate {
			edit = true
		}
		sessionHTTP, rest := takeBoolFlag(rest, "session-http")
		flags := map[string]string{}
		for _, flag := range []string{"host", "editor-token-file", "audience-token-file", "session-secret-file", "tls-cert", "tls-key", "audience"} {
			value, next, err := takeStringFlag(rest, flag, "")
			if err != nil {
				return err
			}
			flags[flag], rest = value, next
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		host := flags["host"]
		if host == "" {
			host = "127.0.0.1"
		}
		address := net.JoinHostPort(host, strconv.Itoa(port))
		if flags["audience"] != "" && (watch || edit || collaborate) {
			return fmt.Errorf("--audience cannot be combined with --watch, --edit or --collab")
		}
		var sessions *slides.SessionOptions
		if flags["editor-token-file"] != "" {
			sessions = &slides.SessionOptions{AllowInsecure: sessionHTTP}
			for flag, dest := range map[string]*string{"editor-token-file": &sessions.EditorToken, "audience-token-file": &sessions.AudienceToken, "session-secret-file": &sessions.Secret} {
				if flags[flag] == "" {
					continue
				}
				value, err := readSessionSecret(flags[flag])
				if err != nil {
					return fmt.Errorf("%s: %w", flag, err)
				}
				*dest = value
			}
		} else if sessionHTTP || flags["audience-token-file"] != "" || flags["session-secret-file"] != "" {
			return fmt.Errorf("session options require --editor-token-file")
		}
		if watch && edit {
			return fmt.Errorf("use serve --edit for browser authoring, or --watch for filesystem hot-swap")
		}
		dir := deckDir(rest)
		if watch {
			if host != "127.0.0.1" || sessions != nil || flags["tls-cert"] != "" || flags["tls-key"] != "" {
				return fmt.Errorf("watch requires the local default host without sessions or TLS")
			}
			fmt.Printf("gosx-slides (hot-swap) serving %s at http://%s\n", dir, address)
			return slides.DevDeck(dir, slides.DevOptions{Addr: address, RebuildRuntime: rebuild})
		}
		scheme := "http"
		if flags["tls-cert"] != "" {
			scheme = "https"
		}
		fmt.Printf("gosx-slides serving %s at %s://%s\n", dir, scheme, address)
		return slides.ServeDeck(dir, slides.ServeOptions{Addr: address, Audience: flags["audience"], StageRuntime: true, RebuildRuntime: rebuild, Edit: edit, Collaborate: collaborate, Sessions: sessions, TLSCertFile: flags["tls-cert"], TLSKeyFile: flags["tls-key"]})
	case "bench":
		runs, rest, err := takeIntFlag(args[1:], "runs", 3)
		if err != nil {
			return err
		}
		budgetPath, rest, err := takeStringFlag(rest, "budget", "")
		if err != nil {
			return err
		}
		var budget slides.BrowserBudget
		if budgetPath != "" {
			data, readErr := os.ReadFile(budgetPath)
			if readErr != nil {
				return readErr
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&budget); err != nil {
				return err
			}
			if err = decoder.Decode(new(any)); err != io.EOF {
				return fmt.Errorf("budget must contain one JSON object")
			}
			if err = budget.Validate(); err != nil {
				return err
			}
		}
		report, err := slides.BenchmarkBrowser(deckDir(rest), runs)
		if err != nil {
			return err
		}
		if err = json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return err
		}
		return report.CheckBudget(budget)
	case "build":
		out, rest, err := takeStringFlag(args[1:], "out", "dist")
		if err != nil {
			return err
		}
		audience, rest, err := takeStringFlag(rest, "audience", "")
		if err != nil {
			return err
		}
		return slides.ExportStatic(deckDir(rest), slides.ExportOptions{Format: "spa", Audience: audience, OutDir: out})
	case "import":
		jsonOut, rest := takeBoolFlag(args[1:], "json")
		out, rest, err := takeStringFlag(rest, "out", "imported-deck")
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return fmt.Errorf("usage: slides import <source.pptx> [--out new-deck-dir] [--json]")
		}
		report, err := slides.ImportPPTX(rest[0], out)
		if err != nil {
			return err
		}
		if jsonOut {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("imported %d slides and %d images into %s\n", report.Slides, report.Images, report.Destination)
			for _, warning := range report.Warnings {
				fmt.Printf("warning (slide %d, %s): %s\n", warning.Slide, warning.Code, warning.Message)
			}
		}
		return nil
	case "export":
		format, rest, err := takeStringFlag(args[1:], "format", "spa")
		if err != nil {
			return err
		}
		audience, rest, err := takeStringFlag(rest, "audience", "")
		if err != nil {
			return err
		}
		out, rest, err := takeStringFlag(rest, "out", "dist")
		if err != nil {
			return err
		}
		capture, rest := takeBoolFlag(rest, "capture")
		steps, rest := takeBoolFlag(rest, "steps")
		editable, rest := takeBoolFlag(rest, "editable")
		notes, rest := takeBoolFlag(rest, "notes")
		narration, rest, err := takeStringFlag(rest, "narration", "")
		if err != nil {
			return err
		}
		captions, rest, err := takeStringFlag(rest, "captions", "")
		if err != nil {
			return err
		}
		aspect, rest, err := takeStringFlag(rest, "aspect", "")
		if err != nil {
			return err
		}
		width, rest, err := takeIntFlag(rest, "width", 0)
		if err != nil {
			return err
		}
		height, rest, err := takeIntFlag(rest, "height", 0)
		if err != nil {
			return err
		}
		template, rest, err := takeStringFlag(rest, "template", "")
		if err != nil {
			return err
		}
		secondsText, rest, err := takeStringFlag(rest, "seconds", "2")
		if err != nil {
			return err
		}
		seconds, err := strconv.ParseFloat(secondsText, 64)
		if err != nil {
			return fmt.Errorf("invalid --seconds: %w", err)
		}
		fpsText, rest, err := takeStringFlag(rest, "fps", "15")
		if err != nil {
			return err
		}
		fps, err := strconv.Atoi(fpsText)
		if err != nil {
			return fmt.Errorf("invalid --fps: %w", err)
		}
		return slides.ExportStatic(deckDir(rest), slides.ExportOptions{Format: format, Audience: audience, OutDir: out, Capture: capture, Steps: steps, Editable: editable, Notes: notes, Narration: narration, Captions: captions, Aspect: aspect, Width: width, Height: height, PPTXTemplate: template, Seconds: seconds, FPS: fps})
	case "version":
		fmt.Println("gosx-slides " + version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// deckDir resolves a deck DIRECTORY argument (the server reads <dir>/deck.md). It
// accepts either a directory or a path to a deck.md and returns the containing
// directory; with no argument it defaults to ".".
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

func readSessionSecret(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", fmt.Errorf("secret file exceeds 4096 bytes")
	}
	value := strings.TrimSpace(string(data))
	if len(value) < 32 {
		return "", fmt.Errorf("secret file must contain at least 32 bytes")
	}
	return value, nil
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
slides is the gosx-slides command. One lane: a deck is a directory with deck.md +
<Name>.gsx islands, compiled to live GoSX and served (or exported static).

Commands:
  init <name> [--theme aurora|paper|neon|swiss]          scaffold a portable deck you can serve immediately
  serve [deck-dir] [--edit] [--collab] [--host 127.0.0.1] [--port 8080] [--rebuild] [--watch]
      [--editor-token-file file] [--audience-token-file file] [--session-secret-file file]
      [--tls-cert file --tls-key file | --session-http]   serve a local or authenticated shared deck
                                                         (.gsx swaps in place, deck.md reloads); --rebuild = fresh runtime.wasm.
                                                         Presenter: open with ?present or the 'p' key; phone remote at /remote
                                                         (audience screens follow over SSE, across machines).
  bench [deck-dir] [--runs 3] [--budget file.json]         measure browser readiness, transfer, heap, DOM and frame intervals
  build [deck-dir] [--out dist]                          static SPA: index.html + gosx/ assets; islands stay live
  export [deck-dir] --format spa|single|handout|pdf|frames|video|pptx [--out dist]
      [--aspect 16:9|4:3 | --width N --height N] [--template theme.pptx]
      [--narration audio.wav] [--captions authored.vtt]
  import <source.pptx> [--out new-deck-dir] [--json]   migrate supported Office content
    --capture    Render live graphics in single/PDF snapshots (needs Chrome)
    --editable   Export native text and supported SVG shapes in PPTX
    --notes      Publish speaker notes in SPA, handout or editable PPTX (opt-in)
    --steps      Capture every click state (implies capture)
    --seconds 2  Seconds per video state; --fps 15 (video needs ffmpeg)
  check [deck-dir]                                       title / slide / click / notes / layout counts
  inspect [deck-dir] [--json]                            full authoring analysis (words, estimate, components, warnings)
  validate [deck-dir] [--strict] [--profile standard|conference|demo|lecture]
  rehearse [deck-dir]                                    speaker run sheet with per-slide notes
  components [deck-dir] [--json]                          the deck's own .gsx islands + compile status
  doctor [deck-dir] [--json]                             deck health + serve prerequisites
  themes [--json]                                        themes selectable via deck headmatter "theme: <name>"
  packs [deck-dir] [--json]                              enabled local theme/component packs
  pack install <source-dir> [deck-dir]                   vendor a validated pack; enable its exact pin in headmatter
  Reading view: append ?read or press V; export --format handout for a static reading document
  version
`))
}

func printDeckComponents(components []slides.DeckComponentInfo) {
	if len(components) == 0 {
		fmt.Println("no island components referenced by this deck")
		return
	}
	for _, component := range components {
		status := "ok"
		if !component.Compiles {
			status = "FAILS: " + component.Error
		}
		fmt.Printf("  %-18s %-10s %s\n", component.Name, status, component.Path)
	}
}

func printDoctor(report slides.DoctorReport) {
	for _, item := range report.Items {
		fmt.Printf("%-10s %-4s %s\n", item.Name, item.Status, item.Detail)
	}
}

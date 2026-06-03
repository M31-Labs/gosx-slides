package slides

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldOptions configures a new deck.
type ScaffoldOptions struct {
	Theme    string
	Template string
}

// Scaffold creates a new deck directory.
func Scaffold(name, theme string) error {
	return ScaffoldWithOptions(name, ScaffoldOptions{Theme: theme})
}

// ScaffoldWithOptions creates a new deck directory from a template.
func ScaffoldWithOptions(name string, opts ScaffoldOptions) error {
	if name == "" {
		return fmt.Errorf("deck name is required")
	}
	if opts.Theme == "" {
		opts.Theme = "m31"
	}
	if opts.Template == "" {
		opts.Template = "default"
	}
	if err := os.MkdirAll(filepath.Join(name, "public"), 0o755); err != nil {
		return err
	}
	deckPath := filepath.Join(name, "deck.md")
	if _, err := os.Stat(deckPath); err == nil {
		return fmt.Errorf("%s already exists", deckPath)
	}
	switch opts.Template {
	case "default":
		return os.WriteFile(deckPath, []byte(sampleDeck(opts.Theme)), 0o644)
	case "gotreesitter":
		return os.WriteFile(deckPath, []byte(gotreesitterDeck(opts.Theme)), 0o644)
	default:
		return fmt.Errorf("unknown template %q", opts.Template)
	}
}

func sampleDeck(theme string) string {
	return `---
title: GoSX Slides
theme: ` + theme + `
transition: slide
---

# GoSX Slides

Decks are markdown-shaped data with live presentation state.

<Step n={1}>Clicks are a signal, not a CSS afterthought. Current click: {$step}</Step>

<Metrics>
<Metric label="Authoring" value="mdpp" delta="deck-as-data"/>
<Metric label="Runtime" value="Go" delta="presenter-ready"/>
</Metrics>

<Notes>
Open with the authoring loop: edit deck.md, keep the room in sync.
</Notes>

---
layout: center
---

## Agenda

<Agenda/>

<Notes>
This slide is generated from the deck structure.
</Notes>

---
layout: two-cols
clicks: 2
---

## Authoring

- Frontmatter config
- Built-in layouts
- Presenter notes

<Step n={1}>Step reveals work inline.</Step>

::right::

` + "```go {1-2|4|all}\npackage main\n\nfunc main() {\n    println(\"hello slides\")\n}\n```" + `

---
layout: center
clicks: 2
---

## Showcase

<Scene3D/>

<Poll question="What should this deck prove?" options="Live embeds|Presenter sync|Export"/>

---
layout: quote
---

> Presenter sync should be a primitive, not a plugin.

<!-- Keep this closing slide short. -->
`
}

func gotreesitterDeck(theme string) string {
	return `---
title: GoTreeSitter Without CGo
theme: ` + theme + `
transition: slide
---

# GoTreeSitter Without CGo

Pure-Go parsing, GLR pressure, and portable language tooling.

<Metrics>
<Metric label="Runtime" value="pure Go" delta="cross-compilable"/>
<Metric label="Target" value="C parity" delta="byte-for-byte trees"/>
</Metrics>

<Notes>
Open with the portability constraint and the parity bar.
</Notes>

---
layout: center
---

## Agenda

<Agenda/>

<Notes>
The generated agenda keeps the talk structure synchronized with the deck.
</Notes>

---
layout: two-cols
---

## Parser Pipeline

<Pipeline steps="Grammar blob: extracted tables|Pure-Go parser: LR plus GLR|Forest: ambiguity control|Tree: compatibility normalization|Queries: captures and tags"/>

::right::

<ParseTree root="source_file" tree="package_clause>package,identifier;function_declaration>func,identifier,parameters,block"/>

<Notes>
Explain that the grammar ecosystem remains tree-sitter; the runtime becomes Go.
</Notes>

---
layout: center
---

## Performance Story

<Benchmark title="Fork reduction ratio" values="Before 5.95x|After 4.41x|C baseline 1.00x"/>

<Takeaway>Fork count is the first-order lever in GLR performance work.</Takeaway>

<Notes>
Use real project numbers here and keep them tied to parity.
</Notes>

---
layout: quote
---

> Make parity boring, then build language tooling on top.

<Notes>
Close on portability as infrastructure.
</Notes>
`
}

package slides

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"m31labs.dev/gosx"
	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

type backgroundTarget struct {
	document   ProjectDocument
	start, end int
	metadata   bool
	options    BackgroundOptions
	title      string
}

// Locate only parsed deck headmatter or a slide's leading metadata fence.
// The location is mapped back to the original include, never the expanded deck.
func findBackgroundTarget(project *AuthorProject, scope string, index int) (backgroundTarget, error) {
	var target backgroundTarget
	root, err := project.Read(DeckFileName)
	if err != nil {
		return target, err
	}
	deck, err := parseIslandDeck(project.dir, []byte(root.Source))
	if err != nil {
		return target, err
	}
	if index < 0 || index >= len(deck.Slides) {
		return target, projectFailure(400, "slide is out of range")
	}
	flat, err := mdpp.Parse(deck.ExpandedSource)
	if err != nil {
		return target, err
	}
	repairDeckHeadings(flat)
	where := SourceLocation{File: DeckFileName}
	var node *mdpp.Node
	switch scope {
	case "deck":
		for _, n := range flat.Root.Children {
			if n.Type == mdpp.NodeFrontmatter {
				node = n
				target.metadata = true
				break
			}
		}
		target.title = "Deck default"
	case "slide":
		current := 0
		for _, n := range flat.Root.Children {
			if n.Type == mdpp.NodeFrontmatter {
				continue
			}
			if n.Type == mdpp.NodeThematicBreak {
				if current == index {
					node = n
					break
				}
				current++
				continue
			}
			if current == index {
				node = n
				target.metadata = n.Type == mdpp.NodeCodeBlock && (strings.EqualFold(n.Attr("language"), "yaml") || strings.EqualFold(n.Attr("language"), "yml"))
				break
			}
		}
		target.title = slideTitle(deck.Slides[index])
		if node == nil {
			// An empty trailing slide still has an original author insertion point.
			if len(deck.ExpandedSource) > 0 {
				where, _ = deck.SourceLocation(len(flat.Source)-1, len(flat.Source))
				where.StartByte = where.EndByte
			} else {
				where = SourceLocation{File: DeckFileName}
			}
		}
	default:
		return target, projectFailure(400, "scope must be slide or deck")
	}
	if node != nil {
		end := node.Range.StartByte
		if target.metadata {
			end = node.Range.EndByte
		}
		var mapped bool
		where, mapped = deck.SourceLocation(node.Range.StartByte, end)
		if !mapped {
			return target, projectFailure(422, "background metadata crosses included files; edit the source directly")
		}
	}
	if where.File == "" {
		return target, projectFailure(422, "background has no editable source location")
	}
	target.document, err = project.Read(where.File)
	if err != nil {
		return target, err
	}
	if target.document.ContextRevision != root.ContextRevision {
		return target, projectFailure(409, "project changed; reload background settings")
	}
	if strings.ContainsRune(target.document.Source, '\r') {
		return target, projectFailure(422, "background saves require LF line endings; convert this file in the source editor first")
	}
	target.start, target.end = where.StartByte, where.EndByte
	if target.start < 0 || target.end < target.start || target.end > len(target.document.Source) {
		return target, projectFailure(422, "background source range is invalid")
	}
	ref := slideBackgroundRef(deck.Slides[index], deckSlideLayers(deck))
	if scope == "deck" {
		ref = shaderBackgroundRef(deckFrontmatterString(deck, "scene"), deckShaderValues(deck))
	}
	source := graphicString(parseProps(ref.Props), "Src", "")
	if strings.HasPrefix(source, "shader:") {
		target.options, err = shaderBackgroundOptions(source, parseProps(ref.Props))
	} else {
		target.options = BackgroundPresets()[0].Defaults
	}
	return target, err
}

// Preserve unrelated YAML bytes, comments and ordering. Complex values for our
// six scalar keys are refused instead of rewriting the author's YAML tree.
func updateBackgroundYAML(raw string, values map[string]string) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return "", fmt.Errorf("invalid background metadata: %w", err)
	}
	lines := strings.Split(raw, "\n")
	seen := map[string]bool{}
	if len(doc.Content) > 0 {
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode || mapping.Style&yaml.FlowStyle != 0 {
			return "", fmt.Errorf("use block YAML metadata before applying a background")
		}
		for i := 0; i < len(mapping.Content); i += 2 {
			key, value := mapping.Content[i], mapping.Content[i+1]
			if seen[key.Value] {
				return "", fmt.Errorf("duplicate metadata key %q", key.Value)
			}
			seen[key.Value] = true
			replacement, owned := values[key.Value]
			if !owned {
				continue
			}
			line := key.Line - 1
			var oneLine map[string]any
			if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode || value.Line != key.Line || value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 || line < 0 || line >= len(lines) || yaml.Unmarshal([]byte(lines[line]), &oneLine) != nil || len(oneLine) != 1 {
				return "", fmt.Errorf("metadata %q needs a single-line scalar; edit it in the source editor", key.Value)
			}
			lineValue := oneLine[key.Value]
			if strings.Contains(value.Value, "\n") || lineValue == nil && value.Tag != "!!null" {
				return "", fmt.Errorf("metadata %q needs a single-line scalar", key.Value)
			}
			comment := value.LineComment
			if comment == "" {
				comment = key.LineComment
			}
			lines[line] = backgroundYAMLLine(key.Value, replacement)
			if comment != "" {
				lines[line] += " " + comment
			}
		}
	}
	output := strings.Join(lines, "\n")
	if output != "" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	for _, key := range backgroundKeys {
		if !seen[key] {
			output += backgroundYAMLLine(key, values[key]) + "\n"
		}
	}
	return output, nil
}

func backgroundYAMLLine(key, value string) string {
	if key == "scene" || key == "shader-ink" || key == "shader-glow" {
		value = strconv.Quote(value)
	}
	return key + ": " + value
}

func backgroundEditedSource(target backgroundTarget, options BackgroundOptions, scope string) (string, error) {
	if err := options.validate(); err != nil {
		return "", err
	}
	src := target.document.Source
	var replacement string
	if target.metadata {
		block := src[target.start:target.end]
		first := strings.IndexByte(block, '\n')
		last := strings.LastIndexByte(strings.TrimRight(block, "\n"), '\n')
		if first < 0 || last <= first {
			return "", fmt.Errorf("background metadata fence is incomplete")
		}
		body, err := updateBackgroundYAML(block[first+1:last+1], backgroundValues(options))
		if err != nil {
			return "", err
		}
		replacement = block[:first+1] + body + block[last+1:]
	} else {
		body, err := updateBackgroundYAML("", backgroundValues(options))
		if err != nil {
			return "", err
		}
		opening := "```yaml\n"
		closing := "```\n\n"
		if scope == "deck" {
			opening, closing = "---\n", "---\n\n"
		}
		replacement = opening + body + closing
		if target.start > 0 && src[target.start-1] != '\n' {
			replacement = "\n\n" + replacement
		}
	}
	result := src[:target.start] + replacement + src[target.end:]
	if len(result) > maxSourceBytes {
		return "", projectFailure(413, "background edit exceeds the 1 MiB authoring limit")
	}
	return result, nil
}

func mountBackgroundWizard(app *server.App, deck *IslandDeck, token string, mu *sync.Mutex) error {
	project, err := NewAuthorProject(deck.Dir)
	if err != nil {
		return err
	}
	project.mu = mu
	app.Mount("/_slides/background", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(err error) {
			status := 422
			var p *ProjectError
			if errors.As(err, &p) {
				status = p.Status
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		if r.Method != "GET" && r.Method != "PUT" {
			w.Header().Set("Allow", "GET, PUT")
			fail(projectFailure(405, "method not allowed"))
			return
		}
		if err := authorizeSourceRequest(r, token, r.Method == "PUT"); err != nil {
			fail(projectFailure(403, err.Error()))
			return
		}
		if r.Method == "GET" {
			index, err := strconv.Atoi(r.URL.Query().Get("slide"))
			if err != nil {
				fail(projectFailure(400, "slide must be an index"))
				return
			}
			target, err := findBackgroundTarget(project, r.URL.Query().Get("scope"), index)
			if err != nil {
				fail(err)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"presets": BackgroundPresets(), "options": target.options, "file": target.document.Path, "revision": target.document.Revision, "contextRevision": target.document.ContextRevision, "token": token, "title": target.title})
			return
		}
		var input struct {
			Scope           string            `json:"scope"`
			Slide           int               `json:"slide"`
			File            string            `json:"file"`
			Revision        string            `json:"revision"`
			ContextRevision string            `json:"contextRevision"`
			Options         BackgroundOptions `json:"options"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF {
			fail(projectFailure(400, "invalid background settings"))
			return
		}
		target, err := findBackgroundTarget(project, input.Scope, input.Slide)
		if err != nil {
			fail(err)
			return
		}
		if target.document.Path != input.File || target.document.Revision != input.Revision || target.document.ContextRevision != input.ContextRevision {
			fail(projectFailure(409, "project changed; reload background settings"))
			return
		}
		source, err := backgroundEditedSource(target, input.Options, input.Scope)
		if err != nil {
			fail(err)
			return
		}
		result, err := project.Write(ProjectEdit{File: input.File, Source: source, Revision: input.Revision, ContextRevision: input.ContextRevision})
		if err != nil {
			fail(err)
			return
		}
		json.NewEncoder(w).Encode(result)
	}))
	app.Page("/_slides/background/preview", func(ctx *server.Context) gosx.Node {
		ctx.Header().Set("Cache-Control", "private, no-store")
		ctx.Header().Set("X-Frame-Options", "SAMEORIGIN")
		if err := authorizeSourceRequest(ctx.Request, token, false); err != nil {
			return gosx.Text("Editor access required")
		}
		query := ctx.Request.URL.Query()
		preset, err := backgroundPreset(query.Get("preset"))
		if err != nil {
			return gosx.Text(err.Error())
		}
		props := map[string]any{}
		for _, name := range []string{"Ink", "Glow", "Speed", "Strength", "Scale"} {
			if value := query.Get(strings.ToLower(name)); value != "" {
				props[name] = value
			}
		}
		options, err := shaderBackgroundOptions("shader:"+preset.Name, props)
		if err != nil {
			return gosx.Text(err.Error())
		}
		ref := shaderBackgroundRef("shader:"+preset.Name, backgroundValues(options))
		config, err := compileGraphic(deck.Dir, ref)
		if err != nil {
			return gosx.Text(err.Error())
		}
		config.MountAttrs["class"] = "deck-graphics-background deck-background-active"
		ctx.AddHead(gosx.RawHTML(`<style>` + graphicsStyle() + backgroundPreviewStyle + `</style>`))
		return gosx.El("main", gosx.Attrs(gosx.Attr("class", "deck")),
			ctx.Runtime().Engine(config, gosx.Text("Background preview")),
			gosx.El("section", gosx.Attrs(gosx.Attr("class", "background-preview-copy")), gosx.El("p", gosx.Text("YOUR NEXT GREAT TALK")), gosx.El("h1", gosx.Text("Give your ideas a little atmosphere.")), gosx.El("p", gosx.Text("Keep the texture quiet. Let the words do the work."))),
			gosx.RawHTML(`<script>const surface=document.querySelector('[data-gosx-scene3d]');const reduced=matchMedia('(prefers-reduced-motion: reduce)');function pause(){if(reduced.matches)surface?.__gosxScene3DHandle?.pauseAnimations?.()}document.addEventListener('gosx:ready',pause);reduced.addEventListener('change',pause);</script>`))
	})
	return nil
}

const backgroundPreviewStyle = `html,body{margin:0;height:100%;background:#0d1420;color:#f3f5fa;font-family:system-ui,sans-serif}main.deck{height:100%;isolation:isolate}.background-preview-copy{position:relative;z-index:1;box-sizing:border-box;height:100vh;display:flex;flex-direction:column;justify-content:center;padding:9%;max-width:52rem}.background-preview-copy h1{font-size:clamp(26px,5.6vw,56px);font-weight:650;line-height:1.08;letter-spacing:-.045em;margin:.25em 0}.background-preview-copy p{line-height:1.5;font-size:clamp(12px,2vw,18px)}.background-preview-copy p:first-child{font-size:11px;letter-spacing:.18em;color:#ced7e2} @media(prefers-reduced-motion:reduce){main.deck .deck-graphics-background{display:none!important}}`

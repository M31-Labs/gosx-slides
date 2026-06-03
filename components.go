package slides

import "sort"

// ComponentInfo describes a built-in deck component.
type ComponentInfo struct {
	Name        string `json:"name"`
	Pack        string `json:"pack"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// BuiltInComponents returns the component registry exposed by the CLI and manifest.
func BuiltInComponents() []ComponentInfo {
	components := []ComponentInfo{
		{Name: "Agenda", Pack: "core", Description: "Generated table of contents from slide titles.", Example: "<Agenda/>"},
		{Name: "Step", Pack: "core", Description: "Inline click reveal tied to the current slide step.", Example: `<Step n={1}>Reveal this.</Step>`},
		{Name: "Steps", Pack: "core", Description: "Block click reveals, one line per step.", Example: "<Steps>\nFirst\nSecond\n</Steps>"},
		{Name: "Code", Pack: "core", Description: "Fenced code with optional stepped range metadata.", Example: "```go {1|2|all}\nfunc main() {}\n```"},
		{Name: "Checkpoint", Pack: "presenter", Description: "Named presenter jump target for demos and branches.", Example: `<Checkpoint id="demo" label="Live demo"/>`},
		{Name: "Metric", Pack: "product", Description: "Single statistic with optional delta text.", Example: `<Metric label="Latency" value="24ms" delta="-8ms"/>`},
		{Name: "Metrics", Pack: "product", Description: "Responsive grid of Metric components.", Example: "<Metrics>\n<Metric label=\"Users\" value=\"120\"/>\n</Metrics>"},
		{Name: "Timeline", Pack: "product", Description: "Compact ordered sequence.", Example: `<Timeline items="Plan|Build|Ship"/>`},
		{Name: "Poll", Pack: "product", Description: "Local interactive audience poll.", Example: `<Poll question="Pick one" options="A|B"/>`},
		{Name: "Takeaway", Pack: "product", Description: "Reusable thesis callout.", Example: `<Takeaway>Remember this.</Takeaway>`},
		{Name: "Callout", Pack: "architecture", Description: "Emphasized note with tone support.", Example: `<Callout tone="gold">Important.</Callout>`},
		{Name: "Diagram", Pack: "architecture", Description: "Lightweight diagram placeholder from fenced or inline text.", Example: "<Diagram/>"},
		{Name: "Scene3D", Pack: "showcase", Description: "Built-in animated scene canvas.", Example: "<Scene3D/>"},
		{Name: "Canvas", Pack: "showcase", Description: "Built-in drawing/demo canvas.", Example: "<Canvas/>"},
		{Name: "Pipeline", Pack: "parser", Description: "Phase diagram for runtimes, systems, and workflows.", Example: `<Pipeline steps="Source|Parse|Render"/>`},
		{Name: "ParseTree", Pack: "parser", Description: "Compact syntax tree visualization.", Example: `<ParseTree root="source_file" tree="decl>name,body"/>`},
		{Name: "Benchmark", Pack: "parser", Description: "Ratio or measurement bar chart.", Example: `<Benchmark title="Speed" values="Before 5.95x|After 4.41x"/>`},
		{Name: "Citation", Pack: "evidence", Description: "Source reference for slides and handouts.", Example: `<Citation href="hypha://m31labs/gotreesitter/object/concept.glr-fork-reduction"/>`},
		{Name: "QueryDemo", Pack: "parser", Description: "Source, query, tree, and capture walkthrough.", Example: "<QueryDemo lang=\"go\">\n```go\nfunc main() {}\n```\n```query\n(function_declaration name: (identifier) @fn)\n```\n</QueryDemo>"},
		{Name: "ProfileBuckets", Pack: "parser", Description: "Parser/runtime attribution buckets.", Example: `<ProfileBuckets buckets="scan 20|reduce 42|materialize 18"/>`},
		{Name: "ParityMatrix", Pack: "parser", Description: "Compatibility grid across languages or runtimes.", Example: `<ParityMatrix rows="Go pass|JavaScript pass|Python watch"/>`},
		{Name: "CorpusRun", Pack: "parser", Description: "Corpus benchmark or compatibility run table.", Example: `<CorpusRun rows="JavaScript corpus 4.41x pass|Go stdlib 1.02x pass"/>`},
		{Name: "GrammarBlob", Pack: "parser", Description: "Grammar artifact build pipeline.", Example: `<GrammarBlob steps="parser.c|tables|blob|registry"/>`},
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Pack == components[j].Pack {
			return components[i].Name < components[j].Name
		}
		return components[i].Pack < components[j].Pack
	})
	return components
}

func componentRegistryByName() map[string]ComponentInfo {
	out := map[string]ComponentInfo{}
	for _, component := range BuiltInComponents() {
		out[component.Name] = component
	}
	return out
}

func componentNames(counts map[string]int) []string {
	var names []string
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

package main

// ParseTree is a live, collapsible syntax tree — the concrete object a parser
// talk needs on screen early. It renders the tree-sitter node shape for:
//
//	package main
//	func main() { fmt.Println("hello") }
//
// Each interior node is a button backed by a bool signal. Clicking it toggles
// the node: the caret flips ▸/▾ via a ternary, and the children subtree is
// conditionally rendered with `{open.Get() && <div>…</div>}` — real
// mount/unmount (requires gosx >= v0.25.8).
//
// Built from nested divs, not ul/li, so theme list markers never collide with
// the island's own carets. Styling is theme-neutral: only CSS custom
// properties every gosx-slides theme defines (--surface, --line, --accent,
// --fg-muted, --radius, --font-mono).
//
//gosx:island
func ParseTree(props any) Node {
	pkgOpen := signal.New(true)
	fnOpen := signal.New(true)
	callOpen := signal.New(false)
	togglePkg := func() { pkgOpen.Set(!pkgOpen.Get()) }
	toggleFn := func() { fnOpen.Set(!fnOpen.Get()) }
	toggleCall := func() { callOpen.Set(!callOpen.Get()) }

	return <div class="ptree" style="font-family:var(--font-mono);font-size:0.98rem;line-height:1.55;width:min(620px,100%);background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:1.1rem 1.4rem">
		<div style="color:var(--accent);font-weight:700;margin-bottom:0.35rem">source_file</div>
		<div style="padding-left:1.2rem;border-left:1px solid var(--line)">
			<div>
				<button onClick={togglePkg} style="all:unset;cursor:pointer;color:var(--fg)">{pkgOpen.Get() ? "▾ " : "▸ "}package_clause</button>
				{pkgOpen.Get() && <div style="padding-left:1.2rem;border-left:1px solid var(--line);color:var(--fg-muted)">
					<div>package</div>
					<div>package_identifier <span style="color:var(--accent)">main</span></div>
				</div>}
			</div>
			<div>
				<button onClick={toggleFn} style="all:unset;cursor:pointer;color:var(--fg)">{fnOpen.Get() ? "▾ " : "▸ "}function_declaration</button>
				{fnOpen.Get() && <div style="padding-left:1.2rem;border-left:1px solid var(--line);color:var(--fg-muted)">
					<div>func</div>
					<div>identifier <span style="color:var(--accent)">main</span></div>
					<div>parameter_list</div>
					<div>
						block
						<button onClick={toggleCall} style="all:unset;cursor:pointer;color:var(--fg);margin-left:0.6rem">{callOpen.Get() ? "▾ " : "▸ "}call_expression</button>
						{callOpen.Get() && <div style="padding-left:1.2rem;border-left:1px solid var(--line);color:var(--fg-muted)">
							<div>selector_expression <span style="color:var(--accent)">fmt.Println</span></div>
							<div>argument_list <span style="color:var(--accent)">"hello"</span></div>
						</div>}
					</div>
				</div>}
			</div>
		</div>
	</div>
}

package main

// ParseTree is a live, collapsible syntax tree — the concrete object a parser
// talk needs on screen early. It renders the tree-sitter node shape for:
//
//	package main
//	func main() { fmt.Println("hello") }
//
// Each interior node is a button backed by a bool signal. Clicking it toggles
// the node: the caret flips ▸/▾ via a ternary, and the children subtree is
// conditionally rendered with `{open.Get() && <ul>…</ul>}` — real mount/unmount.
// This is "agent-visible syntax trees" made literal — the same node names a query
// would capture, expandable as genuine GoSX island state (requires gosx
// >= v0.25.8 for reactive conditional mount/unmount).
//
//gosx:island
func ParseTree(props any) Node {
	pkgOpen := signal.New(true)
	fnOpen := signal.New(true)
	callOpen := signal.New(false)
	togglePkg := func() { pkgOpen.Set(!pkgOpen.Get()) }
	toggleFn := func() { fnOpen.Set(!fnOpen.Get()) }
	toggleCall := func() { callOpen.Set(!callOpen.Get()) }

	return <div class="ptree" style="font-family:var(--font-mono);font-size:1.05rem;line-height:1.85;width:min(560px,100%);background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:1.3rem 1.6rem;box-shadow:0 0 28px rgba(47,242,214,0.1)">
		<div style="color:var(--accent);font-weight:700;margin-bottom:0.4rem">source_file</div>
		<ul style="list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line)">
			<li>
				<button onClick={togglePkg} style="all:unset;cursor:pointer;color:var(--accent-2)">{pkgOpen.Get() ? "▾ " : "▸ "}package_clause</button>
				{pkgOpen.Get() && <ul style="list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted)">
					<li>package</li>
					<li>package_identifier <span style="color:var(--accent)">main</span></li>
				</ul>}
			</li>
			<li>
				<button onClick={toggleFn} style="all:unset;cursor:pointer;color:var(--accent-2)">{fnOpen.Get() ? "▾ " : "▸ "}function_declaration</button>
				{fnOpen.Get() && <ul style="list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted)">
					<li>func</li>
					<li>identifier <span style="color:var(--accent)">main</span></li>
					<li>parameter_list</li>
					<li>
						block
						<button onClick={toggleCall} style="all:unset;cursor:pointer;color:var(--accent-2);margin-left:0.6rem">{callOpen.Get() ? "▾ " : "▸ "}call_expression</button>
						{callOpen.Get() && <ul style="list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted)">
							<li>selector_expression <span style="color:var(--accent)">fmt.Println</span></li>
							<li>argument_list <span style="color:var(--accent)">"hello"</span></li>
						</ul>}
					</li>
				</ul>}
			</li>
		</ul>
	</div>
}

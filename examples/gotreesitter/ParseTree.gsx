package main

// ParseTree is a live, drill-in syntax tree — the concrete object a parser talk
// needs on screen early. It renders the tree-sitter node shape for:
//
//	package main
//	func main() { fmt.Println("hello") }
//
// source_file's two children show immediately; clicking an interior node
// (package_clause, function_declaration, call_expression) reveals its children
// and flips its caret ▸ -> ▾. "Collapse" resets the whole tree. This is
// "agent-visible syntax trees" made literal — the same node names a query would
// capture, expandable in the browser as genuine GoSX island state.
//
// Per-node visibility + caret are string signals (display:none/block, ▸/▾)
// bound via {s.Get()} and style={"display:" + s.Get()}; the reveal buttons set
// them on click.
//
//gosx:island
func ParseTree(props any) Node {
	pkgDisp := signal.New("none")
	pkgCaret := signal.New("▸")
	fnDisp := signal.New("none")
	fnCaret := signal.New("▸")
	callDisp := signal.New("none")
	callCaret := signal.New("▸")

	revealPkg := func() { pkgDisp.Set("block"); pkgCaret.Set("▾") }
	revealFn := func() { fnDisp.Set("block"); fnCaret.Set("▾") }
	revealCall := func() { callDisp.Set("block"); callCaret.Set("▾") }
	collapseAll := func() {
		pkgDisp.Set("none")
		pkgCaret.Set("▸")
		fnDisp.Set("none")
		fnCaret.Set("▸")
		callDisp.Set("none")
		callCaret.Set("▸")
	}

	return <div class="ptree" style="font-family:var(--font-mono);font-size:1.05rem;line-height:1.85;width:min(560px,100%);background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:1.3rem 1.6rem;box-shadow:0 0 28px rgba(47,242,214,0.1)">
		<div style="display:flex;align-items:center;justify-content:space-between;gap:1rem;margin-bottom:0.4rem">
			<span style="color:var(--accent);font-weight:700">source_file</span>
			<button onClick={collapseAll} style="font-family:var(--font-mono);font-size:0.75rem;cursor:pointer;color:var(--fg-muted);background:transparent;border:1px solid var(--line);border-radius:6px;padding:0.25rem 0.6rem">collapse</button>
		</div>
		<ul style="list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line)">
			<li>
				<button onClick={revealPkg} style="all:unset;cursor:pointer;color:var(--accent-2)">{pkgCaret.Get()} package_clause</button>
				<ul style={"list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted);display:" + pkgDisp.Get()}>
					<li>package</li>
					<li>package_identifier <span style="color:var(--accent)">main</span></li>
				</ul>
			</li>
			<li>
				<button onClick={revealFn} style="all:unset;cursor:pointer;color:var(--accent-2)">{fnCaret.Get()} function_declaration</button>
				<ul style={"list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted);display:" + fnDisp.Get()}>
					<li>func</li>
					<li>identifier <span style="color:var(--accent)">main</span></li>
					<li>parameter_list</li>
					<li>
						block
						<button onClick={revealCall} style="all:unset;cursor:pointer;color:var(--accent-2);margin-left:0.6rem">{callCaret.Get()} call_expression</button>
						<ul style={"list-style:none;margin:0;padding-left:1.3rem;border-left:1px solid var(--line);color:var(--fg-muted);display:" + callDisp.Get()}>
							<li>selector_expression <span style="color:var(--accent)">fmt.Println</span></li>
							<li>argument_list <span style="color:var(--accent)">"hello"</span></li>
						</ul>
					</li>
				</ul>
			</li>
		</ul>
	</div>
}

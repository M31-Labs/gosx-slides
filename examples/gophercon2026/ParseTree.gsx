package main

// ParseTree is a live, collapsible view of the Go CST for the exact expression
// used in “Inside a Pure-Go Tree-sitter Runtime”:
//
//	total := price * (count + 1)
//
// The node labels were verified against the current GoTreeSitter Go grammar:
// source_file → function_declaration → block → statement_list →
// short_var_declaration → expression_list / binary_expression /
// parenthesized_expression. Anonymous operators and grouping punctuation are
// shown beside the named nodes because the article's CST beat explicitly
// distinguishes both. The component does not claim to parse on stage; it is a
// faithful interactive rendering of a verified tree.
//
//gosx:island
func ParseTree(props any) Node {
	declOpen := signal.New(true)
	rhsOpen := signal.New(true)
	groupOpen := signal.New(false)
	toggleDecl := func() { declOpen.Set(!declOpen.Get()) }
	toggleRHS := func() { rhsOpen.Set(!rhsOpen.Get()) }
	toggleGroup := func() { groupOpen.Set(!groupOpen.Get()) }

	return <div class="ptree" style="font-family:var(--font-mono);font-size:0.9rem;line-height:1.45;width:min(760px,100%);background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:1rem 1.3rem">
		<div style="display:flex;justify-content:space-between;gap:1rem;margin-bottom:0.45rem">
			<strong style="color:var(--accent)">total := price * (count + 1)</strong>
			<span style="color:var(--fg-muted)">bytes 0–28</span>
		</div>
		<div style="color:var(--accent);font-weight:700">source_file</div>
		<div style="padding-left:1.1rem;border-left:1px solid var(--line)">
			<button onClick={toggleDecl} style="background:transparent;border:0;padding:0;font:inherit;cursor:pointer">{declOpen.Get() ? "▾ " : "▸ "}function_declaration</button>
			{declOpen.Get() && <div style="padding-left:1.1rem;border-left:1px solid var(--line);color:var(--fg-muted)">
				<div>block → statement_list</div>
				<div>short_var_declaration</div>
				<div style="padding-left:1.1rem;border-left:1px solid var(--line)">
					<div>expression_list → identifier <span style="color:var(--accent)">total</span></div>
					<div>anonymous token <span style="color:var(--fg)">:=</span></div>
					<button onClick={toggleRHS} style="background:transparent;border:0;padding:0;font:inherit;cursor:pointer">{rhsOpen.Get() ? "▾ " : "▸ "}expression_list → binary_expression</button>
					{rhsOpen.Get() && <div style="padding-left:1.1rem;border-left:1px solid var(--line)">
						<div>identifier <span style="color:var(--accent)">price</span> · anonymous <span style="color:var(--fg)">*</span></div>
						<button onClick={toggleGroup} style="background:transparent;border:0;padding:0;font:inherit;cursor:pointer">{groupOpen.Get() ? "▾ " : "▸ "}parenthesized_expression</button>
						{groupOpen.Get() && <div style="padding-left:1.1rem;border-left:1px solid var(--line)">
							<div>anonymous <span style="color:var(--fg)">(</span></div>
							<div>binary_expression → identifier <span style="color:var(--accent)">count</span> · <span style="color:var(--fg)">+</span> · int_literal <span style="color:var(--accent)">1</span></div>
							<div>anonymous <span style="color:var(--fg)">)</span></div>
						</div>}
					</div>}
				</div>
			</div>}
		</div>
	</div>
}

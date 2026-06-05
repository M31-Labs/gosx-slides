package main

// Benchmark is the headline island: an interactive bar chart of the Go-vs-C
// wall-time ratio for JavaScript parsing, before and after the GLR
// fork-reduction work (Hyphae: concept.glr-fork-reduction, PR #90).
//
//	Before  5.95x  — gotreesitter forks where tree-sitter C does not
//	After   4.41x  — after collapsing those C-deterministic forks
//	C       1.00x  — the tree-sitter C baseline (the floor)
//
// One bool signal + a toggle; render-side ternaries derive the JS bar's height,
// color, label, and the caption from it. Click "Collapse forks" and every
// `collapsed.Get() ? … : …` binding re-renders — the bar drops 5.95x -> 4.41x
// and recolors pink -> lime. Click again to restore. Same reactive path as the
// showcase Counter (requires gosx >= v0.25.5 for ternaries in text children).
//
//gosx:island
func Benchmark(props any) Node {
	collapsed := signal.New(false)
	toggle := func() { collapsed.Set(!collapsed.Get()) }

	return <div class="bench" style="display:flex;flex-direction:column;gap:1.25rem;width:min(640px,100%);font-family:var(--font-body)">
		<div class="bench-plot" style="display:flex;align-items:flex-end;gap:3rem;height:300px;padding:1.25rem 1.75rem 0;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);box-shadow:0 0 28px rgba(47,242,214,0.12)">
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.6rem">
				<span class="bench-ratio" style={"font-family:var(--font-mono);font-weight:700;font-size:1.7rem;text-shadow:0 0 16px currentColor;color:" + (collapsed.Get() ? "var(--accent-2)" : "#ff8ad8")}>{collapsed.Get() ? "4.41x" : "5.95x"}</span>
				<div class="bench-bar" style={"width:78px;border-radius:8px 8px 0 0;transition:height 700ms cubic-bezier(0.34,1.56,0.64,1),background 400ms ease;" + (collapsed.Get() ? "height:74%;background:linear-gradient(180deg,var(--accent-2),rgba(198,255,74,0.18))" : "height:100%;background:linear-gradient(180deg,#ff8ad8,rgba(255,138,216,0.18))")}></div>
				<span style="font-family:var(--font-mono);font-size:0.85rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · Go</span>
			</div>
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.6rem">
				<span class="bench-ratio" style="font-family:var(--font-mono);font-weight:700;font-size:1.7rem;text-shadow:0 0 16px currentColor;color:var(--accent)">1.00x</span>
				<div class="bench-bar" style="width:78px;border-radius:8px 8px 0 0;background:linear-gradient(180deg,var(--accent),rgba(47,242,214,0.18));height:17%"></div>
				<span style="font-family:var(--font-mono);font-size:0.85rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · C</span>
			</div>
		</div>
		<div class="bench-foot" style="display:flex;align-items:center;justify-content:space-between;gap:1.5rem;flex-wrap:wrap">
			<span class="bench-caption" style={"font-family:var(--font-mono);font-size:0.95rem;color:" + (collapsed.Get() ? "var(--accent-2)" : "var(--fg-muted)")}>{collapsed.Get() ? "forks collapsed · −26% wall · byte-for-byte parity" : "forking where tree-sitter C stays deterministic"}</span>
			<button class="bench-btn" onClick={toggle} style="font:700 1rem/1 var(--font-display);text-transform:uppercase;letter-spacing:0.04em;cursor:pointer;padding:0.7rem 1.4rem;border-radius:999px;border:1px solid var(--accent);background:var(--accent);color:var(--bg);box-shadow:0 0 18px rgba(47,242,214,0.35)">{collapsed.Get() ? "Restore forks" : "Collapse forks"}</button>
		</div>
	</div>
}

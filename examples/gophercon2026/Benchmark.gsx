package main

// ⚠ STAGE HAZARD — DO NOT PRESENT WITHOUT RE-DERIVING THE NUMBERS.
// The 5.95x / 4.41x pair below is not published in the current BENCH.md or
// README.md (only the concept survives, CHANGELOG.md:3191). BENCH.md opens with
// "anything not on this page is not a claim" and has already withdrawn two C
// headlines as oracle-mismatched. Before this island goes on a slide, re-derive
// against the frozen tag, or retarget it at a published figure — the sealed
// equal-fixture geomean is 5.526x C (BENCH.md:201).
//
// Benchmark is the headline island: an interactive bar chart of the Go-vs-C
// wall-time ratio for JavaScript parsing, before and after the GLR
// fork-reduction work (Hyphae: concept.glr-fork-reduction, PR #90).
//
//	Before  5.95x  — gotreesitter forks where tree-sitter C does not
//	After   4.41x  — after collapsing those C-deterministic forks
//	C       1.00x  — the tree-sitter C baseline (the floor)
//
// One bool signal + a toggle; render-side ternaries derive the Go bar's
// height, color, label, and the caption. Click "Collapse forks" and the bar
// drops 5.95x -> 4.41x, rose -> green. Theme-neutral: theme custom properties
// for chrome, two literal data colors (rose = pre-fix, green = post-fix) that
// hold up on any dark or light surface; the C baseline reads var(--accent).
//
//gosx:island
func Benchmark(props any) Node {
	collapsed := signal.New(false)
	toggle := func() { collapsed.Set(!collapsed.Get()) }

	return <div class="bench" style="display:flex;flex-direction:column;gap:0.9rem;width:min(680px,100%);font-family:var(--font-body)">
		<div class="bench-plot" style="display:flex;align-items:flex-end;gap:3.5rem;height:230px;padding:1.1rem 2rem 0;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius)">
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.5rem">
				<span class="bench-ratio" style={"font-family:var(--font-mono);font-weight:700;font-size:1.5rem;color:" + (collapsed.Get() ? "#7bd88f" : "#e2557b")}>{collapsed.Get() ? "4.41x" : "5.95x"}</span>
				<div class="bench-bar" style={"width:84px;border-radius:8px 8px 0 0;transition:height 700ms cubic-bezier(0.34,1.56,0.64,1),background 400ms ease;" + (collapsed.Get() ? "height:74%;background:linear-gradient(180deg,#7bd88f,rgba(123,216,143,0.15))" : "height:100%;background:linear-gradient(180deg,#e2557b,rgba(226,85,123,0.15))")}></div>
				<span style="font-family:var(--font-mono);font-size:0.8rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · Go</span>
			</div>
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.5rem">
				<span class="bench-ratio" style="font-family:var(--font-mono);font-weight:700;font-size:1.5rem;color:var(--accent)">1.00x</span>
				<div class="bench-bar" style="width:84px;border-radius:8px 8px 0 0;background:linear-gradient(180deg,var(--accent),transparent);height:17%"></div>
				<span style="font-family:var(--font-mono);font-size:0.8rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · C</span>
			</div>
		</div>
		<div class="bench-foot" style="display:flex;align-items:center;justify-content:space-between;gap:1.25rem;flex-wrap:wrap">
			<span class="bench-caption" style={"font-family:var(--font-mono);font-size:0.9rem;color:" + (collapsed.Get() ? "#7bd88f" : "var(--fg-muted)")}>{collapsed.Get() ? "forks collapsed · −26% wall · byte-for-byte parity" : "forking where tree-sitter C stays deterministic"}</span>
			<button class="bench-btn" onClick={toggle} style="font:700 0.95rem/1 var(--font-display);text-transform:uppercase;letter-spacing:0.04em;cursor:pointer;padding:0.65rem 1.3rem;border-radius:999px;border:1px solid var(--accent);background:var(--accent);color:var(--bg)">{collapsed.Get() ? "Restore forks" : "Collapse forks"}</button>
		</div>
	</div>
}

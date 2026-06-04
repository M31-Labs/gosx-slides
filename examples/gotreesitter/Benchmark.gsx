package main

// Benchmark is the headline island: an interactive bar chart of the Go-vs-C
// wall-time ratio for JavaScript parsing, before and after the GLR
// fork-reduction work (Hyphae: concept.glr-fork-reduction, PR #90).
//
//	Before  5.95x  — gotreesitter forks where tree-sitter C does not
//	After   4.41x  — after collapsing those C-deterministic forks
//	C       1.00x  — the tree-sitter C baseline (the floor)
//
// "Collapse forks" drops the JS bar from 5.95x to 4.41x (height + color +
// caption all re-render); "Reset" puts it back. Genuinely reactive — the same
// signal/Get/Set binding path as the showcase Counter. State is held in string
// signals bound via {s.Get()} in text and inside style={"..." + s.Get()}.
//
//gosx:island
func Benchmark(props any) Node {
	ratio := signal.New("5.95x")
	barH := signal.New("100%")
	barColor := signal.New("#ff8ad8")
	barGlow := signal.New("rgba(255,138,216,0.18)")
	caption := signal.New("forking where tree-sitter C stays deterministic")
	captionColor := signal.New("var(--fg-muted)")

	collapse := func() {
		ratio.Set("4.41x")
		barH.Set("74%")
		barColor.Set("var(--accent-2)")
		barGlow.Set("rgba(198,255,74,0.18)")
		caption.Set("forks collapsed · −26% wall · byte-for-byte parity")
		captionColor.Set("var(--accent-2)")
	}
	reset := func() {
		ratio.Set("5.95x")
		barH.Set("100%")
		barColor.Set("#ff8ad8")
		barGlow.Set("rgba(255,138,216,0.18)")
		caption.Set("forking where tree-sitter C stays deterministic")
		captionColor.Set("var(--fg-muted)")
	}

	return <div class="bench" style="display:flex;flex-direction:column;gap:1.25rem;width:min(640px,100%);font-family:var(--font-body)">
		<div class="bench-plot" style="display:flex;align-items:flex-end;gap:3rem;height:300px;padding:1.25rem 1.75rem 0;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);box-shadow:0 0 28px rgba(47,242,214,0.12)">
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.6rem">
				<span class="bench-ratio" style={"font-family:var(--font-mono);font-weight:700;font-size:1.7rem;text-shadow:0 0 16px currentColor;color:" + barColor.Get()}>{ratio.Get()}</span>
				<div class="bench-bar" style={"width:78px;border-radius:8px 8px 0 0;transition:height 700ms cubic-bezier(0.34,1.56,0.64,1),background 400ms ease;background:linear-gradient(180deg," + barColor.Get() + "," + barGlow.Get() + ");height:" + barH.Get()}></div>
				<span style="font-family:var(--font-mono);font-size:0.85rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · Go</span>
			</div>
			<div class="bench-col" style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%;gap:0.6rem">
				<span class="bench-ratio" style="font-family:var(--font-mono);font-weight:700;font-size:1.7rem;text-shadow:0 0 16px currentColor;color:var(--accent)">1.00x</span>
				<div class="bench-bar" style="width:78px;border-radius:8px 8px 0 0;background:linear-gradient(180deg,var(--accent),rgba(47,242,214,0.18));height:17%"></div>
				<span style="font-family:var(--font-mono);font-size:0.85rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">JS · C</span>
			</div>
		</div>
		<div class="bench-foot" style="display:flex;align-items:center;justify-content:space-between;gap:1.25rem;flex-wrap:wrap">
			<span class="bench-caption" style={"font-family:var(--font-mono);font-size:0.95rem;color:" + captionColor.Get()}>{caption.Get()}</span>
			<div style="display:flex;gap:0.6rem">
				<button class="bench-btn" onClick={collapse} style="font:700 1rem/1 var(--font-display);text-transform:uppercase;letter-spacing:0.04em;cursor:pointer;padding:0.7rem 1.4rem;border-radius:999px;border:1px solid var(--accent);background:var(--accent);color:var(--bg);box-shadow:0 0 18px rgba(47,242,214,0.35)">Collapse forks</button>
				<button class="bench-reset" onClick={reset} style="font:700 1rem/1 var(--font-display);text-transform:uppercase;letter-spacing:0.04em;cursor:pointer;padding:0.7rem 1.2rem;border-radius:999px;border:1px solid var(--line);background:transparent;color:var(--fg-muted)">Reset</button>
			</div>
		</div>
	</div>
}

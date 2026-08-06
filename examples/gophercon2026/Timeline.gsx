package main

// Timeline is the static roadmap island for the evolution narrative: five
// eras, each a date chip + what changed + what became trustworthy. Dates are
// real git history (first commits of the runtime, the cgo harness, and
// grammargen). No signals — purely presentational, theme-neutral (only CSS
// custom properties every gosx-slides theme defines).
//
//gosx:island
func Timeline(props any) Node {
	return <div class="tl" style="display:flex;flex-direction:column;gap:0.65rem;width:min(860px,100%);font-family:var(--font-body)">
		<div style="display:flex;align-items:baseline;gap:1rem"><span style="flex:0 0 5.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--accent);border:1px solid var(--line);border-radius:999px;padding:0.2rem 0.6rem;text-align:center">Feb 19</span><span style="color:var(--fg)"><b>Day one</b> — lexer, LR parser, trees, ts2go: it parses hello world</span></div>
		<div style="display:flex;align-items:baseline;gap:1rem"><span style="flex:0 0 5.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--accent);border:1px solid var(--line);border-radius:999px;padding:0.2rem 0.6rem;text-align:center">Feb 21</span><span style="color:var(--fg)"><b>The oracle</b> — cgo quarantined; the C runtime becomes the judge</span></div>
		<div style="display:flex;align-items:baseline;gap:1rem"><span style="flex:0 0 5.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--accent);border:1px solid var(--line);border-radius:999px;padding:0.2rem 0.6rem;text-align:center">Mar 9</span><span style="color:var(--fg)"><b>Monotone</b> — grammargen born strapped to parity floors</span></div>
		<div style="display:flex;align-items:baseline;gap:1rem"><span style="flex:0 0 5.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--accent);border:1px solid var(--line);border-radius:999px;padding:0.2rem 0.6rem;text-align:center">May</span><span style="color:var(--fg)"><b>Bench gates</b> — speed claims need receipts; no fake wins</span></div>
		<div style="display:flex;align-items:baseline;gap:1rem"><span style="flex:0 0 5.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--accent);border:1px solid var(--line);border-radius:999px;padding:0.2rem 0.6rem;text-align:center">Jul</span><span style="color:var(--fg)"><b>Self-hosted</b> — 206 grammars · 119 scanners · v0.47.0</span></div>
	</div>
}

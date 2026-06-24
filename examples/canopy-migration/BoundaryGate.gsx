package main

// BoundaryGate is the GOVERNANCE island: a live `canopy analyze boundaries`
// CI gate for the NEW Go service. The target architecture is layered —
// api → service → store, allowed downward only. The button toggles the
// "tempting shortcut": letting `store` reach back up and import `api`. Flip it
// on and the gate goes red, lists the violation, and exits non-zero — the same
// signal canopy gives CI (add `--format sarif` for inline GitHub annotations).
//
// One bool signal drives everything: the banner color/label (attribute +
// text-child ternaries) and the violation row (`{bad.Get() && <div/>}` mount).
// The point on stage: the Go rewrite only stays clean if a gate keeps it clean —
// canopy turns "intended architecture" into an enforced, diff-aware check.
//
// Requires gosx >= v0.25.8. Illustrative rules; mirror your real .canopyboundaries.
//
//gosx:island
func BoundaryGate(props any) Node {
	bad := signal.New(false)
	toggle := func() { bad.Set(!bad.Get()) }

	return <div class="bgate" style="display:flex;flex-direction:column;gap:1.1rem;width:min(660px,100%);font-family:var(--font-body)">
		<div class="bgate-banner" style={"display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:0.9rem 1.25rem;border-radius:var(--radius);font-family:var(--font-mono);font-weight:700;transition:all 350ms ease;" + (bad.Get() ? "background:rgba(255,107,107,0.12);box-shadow:inset 0 0 0 1px #ff6b6b,0 0 26px rgba(255,107,107,0.2);color:#ff6b6b" : "background:rgba(127,212,168,0.1);box-shadow:inset 0 0 0 1px #7fd4a8,0 0 26px rgba(127,212,168,0.18);color:#7fd4a8")}>
			<span>{bad.Get() ? "✗ boundaries: FAIL (1 violation)" : "✓ boundaries: PASS"}</span>
			<span style="font-size:0.85rem;opacity:0.85">{bad.Get() ? "exit 1" : "exit 0"}</span>
		</div>

		<div class="bgate-rules" style="display:flex;flex-direction:column;gap:0.55rem;padding:1rem 1.25rem;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius)">
			<div style="font-family:var(--font-mono);font-size:0.8rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em;margin-bottom:0.15rem">.canopyboundaries · target Go service</div>
			<div style="font-family:var(--font-mono);color:var(--fg)">pkg/api <span style="color:var(--accent)">→</span> pkg/service <span style="color:#7fd4a8">allow</span></div>
			<div style="font-family:var(--font-mono);color:var(--fg)">pkg/service <span style="color:var(--accent)">→</span> pkg/store <span style="color:#7fd4a8">allow</span></div>
			<div style={"font-family:var(--font-mono);transition:opacity 250ms;" + (bad.Get() ? "color:#ff6b6b;opacity:1" : "color:var(--fg-muted);opacity:0.45")}>pkg/store <span>→</span> pkg/api <span>{bad.Get() ? "VIOLATION" : "(not allowed)"}</span></div>
		</div>

		{bad.Get() && <div class="bgate-detail" style="padding:0.85rem 1.1rem;background:rgba(255,107,107,0.08);border-left:3px solid #ff6b6b;border-radius:0 var(--radius) var(--radius) 0;font-family:var(--font-mono);font-size:0.92rem;color:#ffb3b3">
			pkg/store imports pkg/api, not allowed by module pkg/store<br/>
			→ the store layer is reaching back into the web layer · CI blocks the merge
		</div>}

		<button class="bgate-btn" onClick={toggle} style={"align-self:flex-start;font:700 1rem/1 var(--font-display);text-transform:uppercase;letter-spacing:0.04em;cursor:pointer;padding:0.75rem 1.5rem;border-radius:999px;transition:all 300ms;" + (bad.Get() ? "border:1px solid #7fd4a8;background:transparent;color:#7fd4a8" : "border:1px solid #ff6b6b;background:transparent;color:#ff6b6b")}>{bad.Get() ? "Revert the shortcut" : "Add the tempting shortcut"}</button>
	</div>
}

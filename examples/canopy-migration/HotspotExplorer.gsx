package main

// HotspotExplorer is the TRIAGE island: a live, ranked view of the legacy Java
// service's riskiest methods to port — exactly what `canopy analyze hotspot`
// and `canopy analyze complexity --sort cognitive` surface, made clickable.
//
// Each row is a method with its cognitive-complexity score and a proportional
// bar. One int signal holds the selected row; clicking a method highlights it
// (attribute-ternary restyle) and mounts that method's "why it's risky" note
// (`{sel.Get() == N && <div/>}` real mount/unmount). The data is illustrative
// and generic on purpose — swap in your own `analyze complexity --json` numbers.
//
// Only the proven island patterns are used (signals, per-row closure handlers,
// text-child + attribute ternaries, &&-mount), so it hot-swaps under
// `slides serve --watch` like the showcase Counter. Requires gosx >= v0.25.8.
//
//gosx:island
func HotspotExplorer(props any) Node {
	sel := signal.New(0)
	pick0 := func() { sel.Set(0) }
	pick1 := func() { sel.Set(1) }
	pick2 := func() { sel.Set(2) }
	pick3 := func() { sel.Set(3) }
	pick4 := func() { sel.Set(4) }

	return <div class="hotspot" style="display:flex;flex-direction:column;gap:0.55rem;width:min(680px,100%);font-family:var(--font-body)">
		<div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.2rem">
			<span style="font-family:var(--font-mono);color:var(--accent);font-weight:700;letter-spacing:0.04em">analyze hotspot · OrderService</span>
			<span style="font-family:var(--font-mono);font-size:0.8rem;color:var(--fg-muted);text-transform:uppercase;letter-spacing:0.08em">cognitive complexity</span>
		</div>

		<button onClick={pick0} style={"all:unset;cursor:pointer;display:flex;align-items:center;gap:0.9rem;padding:0.6rem 0.85rem;border-radius:10px;transition:background 200ms,box-shadow 200ms;" + (sel.Get() == 0 ? "background:var(--accent-soft);box-shadow:inset 0 0 0 1px var(--accent)" : "background:var(--surface)")}>
			<span style="font-family:var(--font-mono);min-width:16rem;color:var(--fg)">OrderService.checkout()</span>
			<span style="flex:1;height:12px;border-radius:6px;background:linear-gradient(90deg,#ff6b6b,rgba(255,107,107,0.18));width:100%"></span>
			<span style="font-family:var(--font-mono);font-weight:700;color:#ff6b6b;min-width:2.5rem;text-align:right">47</span>
		</button>

		<button onClick={pick1} style={"all:unset;cursor:pointer;display:flex;align-items:center;gap:0.9rem;padding:0.6rem 0.85rem;border-radius:10px;transition:background 200ms,box-shadow 200ms;" + (sel.Get() == 1 ? "background:var(--accent-soft);box-shadow:inset 0 0 0 1px var(--accent)" : "background:var(--surface)")}>
			<span style="font-family:var(--font-mono);min-width:16rem;color:var(--fg)">PricingEngine.applyDiscounts()</span>
			<span style="flex:1;height:12px;border-radius:6px;background:linear-gradient(90deg,#ff9f45,rgba(255,159,69,0.18));width:81%"></span>
			<span style="font-family:var(--font-mono);font-weight:700;color:#ff9f45;min-width:2.5rem;text-align:right">38</span>
		</button>

		<button onClick={pick2} style={"all:unset;cursor:pointer;display:flex;align-items:center;gap:0.9rem;padding:0.6rem 0.85rem;border-radius:10px;transition:background 200ms,box-shadow 200ms;" + (sel.Get() == 2 ? "background:var(--accent-soft);box-shadow:inset 0 0 0 1px var(--accent)" : "background:var(--surface)")}>
			<span style="font-family:var(--font-mono);min-width:16rem;color:var(--fg)">InventoryClient.reserve()</span>
			<span style="flex:1;height:12px;border-radius:6px;background:linear-gradient(90deg,var(--accent),var(--accent-soft));width:62%"></span>
			<span style="font-family:var(--font-mono);font-weight:700;color:var(--accent);min-width:2.5rem;text-align:right">29</span>
		</button>

		<button onClick={pick3} style={"all:unset;cursor:pointer;display:flex;align-items:center;gap:0.9rem;padding:0.6rem 0.85rem;border-radius:10px;transition:background 200ms,box-shadow 200ms;" + (sel.Get() == 3 ? "background:var(--accent-soft);box-shadow:inset 0 0 0 1px var(--accent)" : "background:var(--surface)")}>
			<span style="font-family:var(--font-mono);min-width:16rem;color:var(--fg-muted)">OrderRepository.findPending()</span>
			<span style="flex:1;height:12px;border-radius:6px;background:linear-gradient(90deg,#7fd4a8,rgba(127,212,168,0.18));width:30%"></span>
			<span style="font-family:var(--font-mono);font-weight:700;color:#7fd4a8;min-width:2.5rem;text-align:right">14</span>
		</button>

		<button onClick={pick4} style={"all:unset;cursor:pointer;display:flex;align-items:center;gap:0.9rem;padding:0.6rem 0.85rem;border-radius:10px;transition:background 200ms,box-shadow 200ms;" + (sel.Get() == 4 ? "background:var(--accent-soft);box-shadow:inset 0 0 0 1px var(--accent)" : "background:var(--surface)")}>
			<span style="font-family:var(--font-mono);min-width:16rem;color:var(--fg-muted)">OrderMapper.toDto()</span>
			<span style="flex:1;height:12px;border-radius:6px;background:linear-gradient(90deg,#7fd4a8,rgba(127,212,168,0.18));width:13%"></span>
			<span style="font-family:var(--font-mono);font-weight:700;color:#7fd4a8;min-width:2.5rem;text-align:right">6</span>
		</button>

		<div class="hotspot-note" style="margin-top:0.5rem;padding:0.85rem 1.1rem;background:var(--surface);border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:0 var(--radius) var(--radius) 0;color:var(--fg);min-height:3.2rem">
			{sel.Get() == 0 && <span>Port <strong>last</strong>, with a characterization-test harness first. 47 cognitive points, 9 nested branches, 14 fan-in callers — the heart of the service and the highest-risk cut.</span>}
			{sel.Get() == 1 && <span>Port <strong>behind a feature flag</strong>. Dense rule branches; pair the Go port with a golden-file test of real pricing inputs before you switch traffic.</span>}
			{sel.Get() == 2 && <span>A clean <strong>strangler-fig seam</strong>: an external call boundary. Reimplement in Go behind the same interface and route through it first.</span>}
			{sel.Get() == 3 && <span>Low risk. Mostly a query mapping — safe to port <strong>early</strong> to build momentum and prove the data layer.</span>}
			{sel.Get() == 4 && <span>Trivial. A pure DTO mapping — port <strong>first</strong> as a warm-up; near-zero behavioral risk.</span>}
		</div>
	</div>
}

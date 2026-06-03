package main

// Counter is a live GoSX island that SEEDS its state from a typed prop.
//
// <Counter initial={3}/> in deck.md starts the count at 3 — the same gosx
// compiler that type-checks this embed evaluates the prop, so a wrong-typed
// initial (e.g. initial={"x"}) is a compile error, not a silent runtime bug.
// This is the spec's "type-checked embeds" in action.
//
// It is also the hot-swap demo: run `slides serve --watch` in this directory,
// bump the count, then edit below and save — the running island swaps in place
// (the count you clicked up stays put) without a page reload. Try:
//   - change "count is" to "clicks:" (static text swap)
//   - change the step in increment/decrement (handler swap)
//   - add a class to the <div> (attribute swap)
//
//gosx:island
func Counter(props any) Node {
	count := signal.New(props.Initial)
	increment := func() { count.Set(count.Get() + 1) }
	decrement := func() { count.Set(count.Get() - 1) }
	return <div class="counter">
		<button class="counter-btn" onClick={decrement}>-</button>
		<span class="counter-label">count is {count.Get()}</span>
		<button class="counter-btn" onClick={increment}>+</button>
	</div>
}

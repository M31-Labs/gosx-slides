package main

//gosx:island
func SectionCounter(props any) Node {
    count := signal.New(props.Initial)
    increment := func() { count.Set(count.Get() + 1) }
    return <div class="pack-counter">
        <span>Shared section count: {count.Get()}</span>
        <button class="pack-increment" onClick={increment}>Add one</button>
    </div>
}

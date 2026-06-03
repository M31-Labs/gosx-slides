# Live Counter

This deck runs in the **real lane**: the prose you are reading is static
server HTML, while the component below is a genuine GoSX island compiled to
bytecode and hydrated in your browser.

Click the buttons — the count is real reactive state, and editing `Counter.gsx`
under `gosx dev` hot-swaps the running island without losing it.

<Counter initial={3}/>

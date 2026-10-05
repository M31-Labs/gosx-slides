package slides

func presentationControlsStyle() string {
	return `
main.deck .deck-controls { position: fixed; z-index: 40; bottom: 1.25rem; left: 50%; transform: translateX(-50%); display: flex; flex-wrap: wrap; justify-content: center; width: max-content; max-width: calc(100vw - 1rem); box-sizing: border-box; gap: .3rem; padding: .4rem; border: 1px solid color-mix(in srgb, var(--fg, white) 16%, transparent); border-radius: 1.75rem; background: color-mix(in srgb, var(--bg, #10141e) 88%, transparent); box-shadow: 0 6px 24px #0003; opacity: 0; transition: opacity 180ms; }
main.deck .deck-controls-visible, main.deck .deck-controls:has(:focus-visible) { opacity: 1; }
main.deck .deck-controls button { display: grid; place-items: center; flex: 0 0 auto; min-width: 2.75rem; width: auto; height: 2.75rem; padding: 0 .65rem; border: 0; border-radius: 999px; background: transparent; color: var(--fg, #eee); cursor: pointer; font: .95rem/1 system-ui; }
main.deck .deck-controls .deck-icon-control, main.deck .deck-controls button[aria-label="Record presentation"] { font-size: 1.3rem; }
main.deck .deck-controls button:hover { background: color-mix(in srgb, var(--accent, #eeb86b) 20%, transparent); }
main.deck .deck-controls button:focus-visible { outline: 2px solid var(--accent, #eeb86b); outline-offset: 2px; }
main.deck .deck-curtain { display: none; position: fixed; inset: 0; z-index: 1000; background: #000; cursor: pointer; }
main.deck.deck-blank .deck-curtain { display: block; }
main.deck.deck-overview .deck-controls, main.deck.deck-presenter .deck-controls { display: none; }
@media (hover: none) { main.deck .deck-controls { bottom: .75rem; } }
@media (prefers-reduced-motion: reduce) { main.deck .deck-controls { transition: none; } }
@media print { main.deck .deck-controls, main.deck .deck-curtain { display: none !important; } }
`
}

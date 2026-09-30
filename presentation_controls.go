package slides

func presentationControlsStyle() string {
	return `
main.deck .deck-controls { position: fixed; z-index: 40; bottom: 1.25rem; left: 50%; transform: translateX(-50%); display: flex; gap: .3rem; padding: .4rem; border: 1px solid color-mix(in srgb, var(--fg, white) 16%, transparent); border-radius: 999px; background: color-mix(in srgb, var(--bg, #10141e) 88%, transparent); box-shadow: 0 6px 24px #0003; opacity: 0; transition: opacity 180ms; }
main.deck:hover .deck-controls, main.deck .deck-controls:focus-within { opacity: 1; }
main.deck .deck-controls button { display: grid; place-items: center; width: 2.6rem; height: 2.6rem; border: 0; border-radius: 50%; background: transparent; color: var(--fg, #eee); cursor: pointer; font: 1.3rem/1 system-ui; }
main.deck .deck-controls button:hover { background: color-mix(in srgb, var(--accent, #eeb86b) 20%, transparent); }
main.deck .deck-controls button:focus-visible { outline: 2px solid var(--accent, #eeb86b); outline-offset: 2px; }
main.deck .deck-curtain { display: none; position: fixed; inset: 0; z-index: 1000; background: #000; cursor: pointer; }
main.deck.deck-blank .deck-curtain { display: block; }
main.deck.deck-overview .deck-controls, main.deck.deck-presenter .deck-controls { display: none; }
@media (hover: none) { main.deck .deck-controls { opacity: 1; bottom: .75rem; } }
@media (prefers-reduced-motion: reduce) { main.deck .deck-controls { transition: none; } }
@media print { main.deck .deck-controls, main.deck .deck-curtain { display: none !important; } }
`
}

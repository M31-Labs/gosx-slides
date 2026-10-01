package slides

// Pointer activity extends one pending timer rather than creating a timer on
// every move. Keyboard focus remains visible through :focus-visible in CSS.
func controlsActivityScript() string {
	return `
  var controlsTimer = null, controlsHideAt = 0;
  function hideIdleControls() {
    var remaining = controlsHideAt - Date.now();
    if (remaining > 0) { controlsTimer = setTimeout(hideIdleControls, remaining); return; }
    controlsTimer = null;
    controls.classList.remove('deck-controls-visible');
  }
  function revealControls() {
    controlsHideAt = Date.now() + 2200;
    controls.classList.add('deck-controls-visible');
    if (controlsTimer == null) controlsTimer = setTimeout(hideIdleControls, 2200);
  }
  deck.addEventListener('pointermove', revealControls, { passive: true });
  deck.addEventListener('pointerdown', revealControls, { passive: true });
  deck.addEventListener('pointerleave', function (event) { if (event.pointerType === 'mouse') controls.classList.remove('deck-controls-visible'); }, { passive: true });
  revealControls();
`
}

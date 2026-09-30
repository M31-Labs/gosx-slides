package slides

// A slide/step fragment addresses the same absolute state as presenter sync.
// Keep the legacy presenter hash suffix and slide-only links readable.
func navLinkScript() string {
	return `
  function readPosition(hash, count) {
    var match = /^#(\d+)(?:\/(\d+))?(?:present)?$/.exec(hash || '');
    if (!match) return { index: 0, step: 0 };
    var slide = Number(match[1]), targetStep = match[2] ? Number(match[2]) : 0;
    if (!Number.isSafeInteger(slide) || slide < 1) return { index: 0, step: 0 };
    if (!Number.isSafeInteger(targetStep)) targetStep = 0;
    return { index: Math.min(slide - 1, count - 1), step: targetStep };
  }
  function positionHash(i, s, presenter) {
    return '#' + (i + 1) + (s > 0 ? '/' + s : '') + (presenter ? 'present' : '');
  }
`
}

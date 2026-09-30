package slides

// A slide/step fragment addresses the same absolute state as presenter sync.
// Keep the legacy presenter hash suffix and slide-only links readable.
func navLinkScript() string {
	return `
  function readPosition(hash, count) {
    var match = /^#(\d+)(?:\/(\d+))?(?:present)?$/.exec(hash || '');
    if (!match) {
      var named = /^#([A-Za-z][A-Za-z0-9_-]{0,63})(?:\/([A-Za-z][A-Za-z0-9_-]{0,63}|\d+))?$/.exec(hash || '');
      if (!named || typeof slides === 'undefined') return { index: 0, step: 0 };
      var at = slides.findIndex(function (slide) { return slide.getAttribute('data-slide-id') === named[1]; });
      if (at < 0) return { index: 0, step: 0 };
      var names = JSON.parse(slides[at].getAttribute('data-slide-cues') || '[]');
      var cue = named[2] ? (/^\d+$/.test(named[2]) ? Number(named[2]) : names.indexOf(named[2])) : 0;
      return { index: at, step: Number.isSafeInteger(cue) && cue >= 0 ? cue : 0 };
    }
    var slide = Number(match[1]), targetStep = match[2] ? Number(match[2]) : 0;
    if (!Number.isSafeInteger(slide) || slide < 1) return { index: 0, step: 0 };
    if (!Number.isSafeInteger(targetStep)) targetStep = 0;
    return { index: Math.min(slide - 1, count - 1), step: targetStep };
  }
  function positionHash(i, s, presenter) {
    var slide = typeof slides === 'undefined' ? null : slides[i];
    var id = slide && slide.getAttribute('data-slide-id');
    if (id && !presenter) {
      var cues = JSON.parse(slide.getAttribute('data-slide-cues') || '[]');
      return '#' + id + (s > 0 ? '/' + (cues[s] || s) : '');
    }
    return '#' + (i + 1) + (s > 0 ? '/' + s : '') + (presenter ? 'present' : '');
  }
`
}

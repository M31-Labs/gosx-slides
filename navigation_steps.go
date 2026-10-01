package slides

// Cache only visited slides, and update the outgoing and incoming step state.
func navStepScript() string {
	return `
  var stepElements = [];
  function applyStep(i, activeStep) {
    if (!stepElements[i]) {
      stepElements[i] = {
        lines: Array.prototype.map.call(slides[i].querySelectorAll('pre[data-steps] .ts-line[data-step]'), function (line) {
          return { node: line, steps: line.getAttribute('data-step').split(/\s+/) };
        }),
        fragments: Array.prototype.map.call(slides[i].querySelectorAll('[data-fragment]'), function (fragment) {
          return { node: fragment, index: Number(fragment.getAttribute('data-fragment')), aria: fragment.getAttribute('aria-hidden'), inert: fragment.hasAttribute('inert') };
        })
      };
    }
    stepElements[i].lines.forEach(function (line) { line.node.classList.toggle('slides-step-active', activeStep > 0 && line.steps.indexOf(String(activeStep)) >= 0); });
    stepElements[i].fragments.forEach(function (fragment) {
      var visible = fragment.index <= activeStep;
      fragment.node.classList.toggle('slides-fragment-visible', visible);
      if (!visible) { fragment.node.setAttribute('aria-hidden', 'true'); fragment.node.setAttribute('inert', ''); }
      else {
        if (fragment.aria == null) fragment.node.removeAttribute('aria-hidden'); else fragment.node.setAttribute('aria-hidden', fragment.aria);
        if (!fragment.inert) fragment.node.removeAttribute('inert');
      }
    });
  }

`
}

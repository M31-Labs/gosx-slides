package slides

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNavigationStepsBeyondFormerLimits(t *testing.T) {
	fixture := `
const assert = require('node:assert/strict');
function element(attributes) {
  const attrs = new Map(Object.entries(attributes));
  const classes = new Set();
  return {
    getAttribute: name => attrs.has(name) ? attrs.get(name) : null,
    setAttribute: (name, value) => attrs.set(name, value),
    removeAttribute: name => attrs.delete(name),
    hasAttribute: name => attrs.has(name),
    classList: { toggle: (name, on) => on ? classes.add(name) : classes.delete(name), contains: name => classes.has(name) }
  };
}
const lines = [element({'data-step': '1 17 30'}), element({'data-step': '2 29'})];
const fragments = Array.from({length: 31}, (_, i) => element({'data-fragment': String(i)}));
// An author's own visibility and inert settings must survive the reveal cycle.
fragments[3].setAttribute('aria-hidden', 'false');
fragments[4].setAttribute('aria-hidden', 'true'); fragments[4].setAttribute('inert', '');
let queries = 0;
const slides = [{querySelectorAll(selector) { queries++; return selector.includes('ts-line') ? lines : fragments; }}];
`
	checks := `
applyStep(0, 0);
assert.equal(fragments[0].classList.contains('slides-fragment-visible'), true);
assert.equal(fragments[1].getAttribute('aria-hidden'), 'true');
assert.equal(fragments[1].hasAttribute('inert'), true);
applyStep(0, 17);
assert.equal(lines[0].classList.contains('slides-step-active'), true);
assert.equal(lines[1].classList.contains('slides-step-active'), false);
applyStep(0, 30);
assert.equal(lines[0].classList.contains('slides-step-active'), true);
assert.equal(fragments[30].classList.contains('slides-fragment-visible'), true);
assert.equal(fragments[30].hasAttribute('inert'), false);
assert.equal(fragments[3].getAttribute('aria-hidden'), 'false');
assert.equal(fragments[4].getAttribute('aria-hidden'), 'true');
assert.equal(fragments[4].hasAttribute('inert'), true);
// Direct and reverse seeks restore the exact reveal state, including focusability.
applyStep(0, 29);
assert.equal(lines[0].classList.contains('slides-step-active'), false);
assert.equal(lines[1].classList.contains('slides-step-active'), true);
assert.equal(fragments[30].classList.contains('slides-fragment-visible'), false);
assert.equal(fragments[30].hasAttribute('inert'), true);
applyStep(0, 0);
assert.equal(lines[0].classList.contains('slides-step-active'), false);
assert.equal(fragments[3].getAttribute('aria-hidden'), 'true');
assert.equal(fragments[0].getAttribute('aria-hidden'), null);
assert.equal(queries, 2, 'step metadata must be cached rather than rescanned');
`
	runNavigationJS(t, fixture+navStepScript()+checks)
}

func TestOverviewSearchPolicy(t *testing.T) {
	runNavigationJS(t, overviewSearchScript()+`
const assert = require('node:assert/strict');
const records = ['Live state, number 2', 'Crème de la crème, diagram motion', 'Diagram labels', 'Motion details'].map(searchText);
assert.deepEqual(matchingSlideIndices(records, ''), [0, 1, 2, 3]);
assert.deepEqual(matchingSlideIndices(records, '  CRÈME  '), [1]);
assert.deepEqual(matchingSlideIndices(records, 'creme'), [1]);
assert.deepEqual(matchingSlideIndices(records, 'diagram motion'), [1]);
assert.deepEqual(matchingSlideIndices(records, 'diagram'), [1, 2]);
assert.deepEqual(matchingSlideIndices(records, '2'), [1], 'numeric query means slide ordinal, not body text');
assert.deepEqual(matchingSlideIndices(records, '02'), [1]);
assert.deepEqual(matchingSlideIndices(records, '0'), []);
assert.deepEqual(matchingSlideIndices(records, '9999'), []);
assert.deepEqual(matchingSlideIndices(records, 'absent'), []);
assert.deepEqual(matchingSlideIndices([], 'motion'), []);
`)
}

func runNavigationJS(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	path := filepath.Join(t.TempDir(), "navigation.cjs")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, path).CombinedOutput(); err != nil {
		t.Fatalf("navigation regression: %v\n%s", err, out)
	}

}

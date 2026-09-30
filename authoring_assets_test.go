package slides

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeferredIslandsResumeAfterStartupPollingExpires(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for runtime lifecycle checks")
	}
	source, err := json.Marshal(lazyIslandScript)
	if err != nil {
		t.Fatal(err)
	}
	script := `const assert = require('node:assert/strict');
const vm = require('node:vm');
const timers = new Map(), deckEvents = new Map(), documentEvents = new Map();
let nextTimer = 0, calls = 0;
const opening = { nextElementSibling: null, contains: () => false };
const widgetSlide = { dataset: {slide: '1'}, nextElementSibling: null,
  contains: element => element === root, classList: {contains: () => true} };
const root = { closest: () => widgetSlide };
let active = opening;
const manifest = {textContent: JSON.stringify({islands: [{id: 'counter'}]})};
const deck = {dataset: {}, querySelector: () => active,
  addEventListener: (name, fn) => deckEvents.set(name, fn), dispatchEvent: () => {}};
const window = {addEventListener: () => {}};
const document = {querySelector: () => deck,
  getElementById: id => id === 'gosx-manifest' ? manifest : root,
  addEventListener: (name, fn) => documentEvents.set(name, fn),
  dispatchEvent: event => documentEvents.get(event.type)?.()};
const context = {window, document, console, CustomEvent: function(name, options) {},
  setTimeout: fn => {const id = ++nextTimer; timers.set(id, fn); return id},
  clearTimeout: id => timers.delete(id)};
const flush = async () => {for (let i = 0; i < 8; i++) await Promise.resolve()};
(async () => {
  vm.runInNewContext(` + string(source) + `, context);
  await flush();
  active = widgetSlide; deckEvents.get('slides:change')(); await flush();
  let polls = 0;
  while (timers.size && polls++ < 400) {
    const [id, run] = timers.entries().next().value;
    timers.delete(id); run(); await flush();
  }
  assert.equal(timers.size, 0, 'startup polling must remain bounded');
  assert.ok(polls >= 299, 'exercise the entire 30-second startup allowance');
  assert.equal(window.SlidesRuntime.stats().deferred, 1);
  window.__gosx_hydrate = () => {};
  window.__gosx = {islands: new Map(), host: {hydration: {
    hydrateIsland: async entry => {calls++; window.__gosx.islands.set(entry.id, {root, count: 0})}
  }}};
  document.dispatchEvent({type: 'gosx:ready'}); await flush();
  assert.equal(widgetSlide.dataset.slideHydration, 'ready');
  assert.equal(window.SlidesRuntime.stats().deferred, 0);
  assert.equal(window.__gosx.islands.size, 1);
  window.__gosx.islands.get('counter').count = 7;
  document.dispatchEvent({type: 'gosx:ready'}); deckEvents.get('slides:change')(); await flush();
  assert.equal(calls, 1, 'readiness and revisits must not rehydrate a live island');
  assert.equal(window.__gosx.islands.get('counter').count, 7);
})().catch(error => {console.error(error); process.exitCode = 1});
`
	path := filepath.Join(t.TempDir(), "late-runtime.cjs")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, path).CombinedOutput(); err != nil {
		t.Fatalf("late runtime lifecycle: %v\n%s", err, out)
	}
}

(function () {
  'use strict';
  const deck = document.querySelector('main.deck'), script = document.getElementById('gosx-manifest');
  if (!deck || !script || deck.dataset.hydration === 'eager') return;
  let manifest;
  try { manifest = JSON.parse(script.textContent); } catch (_) { return; }
  const all = Array.isArray(manifest.islands) ? manifest.islands : [];
  const pending = new Map();
  const roots = new Map(all.map(entry => [entry.id, document.getElementById(entry.id)]));
  const current = deck.querySelector('.slide.deck-active');
  manifest.islands = all.map(entry => {
    const root = roots.get(entry.id), slide = root && root.closest('.slide');
    if (!slide || slide === current || entry.static) return entry;
    pending.set(entry.id, entry); return { ...entry, static: true };
  });
  // This runs before deferred GoSX scripts read the manifest. The document's
  // runtime capabilities remain unchanged; only DOM island work is deferred. Static entries keep WASM activation
  // available even when the opening slide has no interactive widgets.
  script.textContent = JSON.stringify(manifest);
  let timer = null, stopped = false;
  async function hydrate(slide) {
    const host = window.__gosx && window.__gosx.host;
    const ready = window.__gosx && window.__gosx.islands;
    if (!host || !host.hydration || typeof window.__gosx_hydrate !== 'function' || !ready) return false;
    const entries = all.filter(entry => roots.get(entry.id) && slide.contains(roots.get(entry.id)) && pending.has(entry.id));
    if (!entries.length) return true;
    slide.dataset.slideHydration = 'pending';
    await Promise.all(entries.map(async entry => {
      // Remove before awaiting, so fast navigation cannot hydrate twice.
      pending.delete(entry.id);
      try { await host.hydration.hydrateIsland(entry); }
      catch (error) { console.error('[slides] island hydration failed', entry.id, error); }
      if (!ready.has(entry.id)) pending.set(entry.id, entry);
    }));
    slide.dataset.slideHydration = entries.every(entry => ready.has(entry.id)) ? 'ready' : 'error';
    deck.dispatchEvent(new CustomEvent('slides:hydrated', { detail: { index: Number(slide.dataset.slide) } }));
    return true;
  }
  function schedule() {
    if (stopped) return;
    clearTimeout(timer);
    const slide = deck.querySelector('.slide.deck-active');
    if (!slide) return;
    hydrate(slide).then(ready => {
      if (!ready) { timer = setTimeout(schedule, 100); return; }
      // Warm only the adjacent slide during idle; visited instances stay live.
      const warm = () => { const next = slide.nextElementSibling; if (next && next.matches('.slide') && slide.classList.contains('deck-active')) hydrate(next); };
      if (window.requestIdleCallback) requestIdleCallback(warm, { timeout: 1000 }); else timer = setTimeout(warm, 250);
    });
  }
  deck.addEventListener('slides:change', schedule);
  window.addEventListener('pagehide', () => { stopped = true; clearTimeout(timer); });
  window.SlidesRuntime = { stats: () => ({ total: all.length, deferred: pending.size,
    hydrated: window.__gosx && window.__gosx.islands ? window.__gosx.islands.size : 0 }) };
  if (pending.size) schedule();
})();

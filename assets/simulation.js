(function () {
  'use strict';
  const deck = document.querySelector('main.deck'), node = document.querySelector('#slides-simulations');
  if (!deck || !node || !window.SlidesNav) return;
  const manifest = JSON.parse(node.textContent), entries = new Map(manifest.simulations.map(entry => [entry.id, entry]));
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const selected = new Map(), manual = new Set();
  const mounts = Array.from(deck.querySelectorAll('figure.slides-simulation[data-simulation]')).filter(mount =>
    entries.has(mount.dataset.simulation) && mount.querySelector('svg') &&
    ['.simulation-tick','.simulation-branch','.simulation-reset','.simulation-status'].every(selector => mount.querySelector(selector)));
  const active = () => mounts.filter(mount => mount.closest('.slide')?.classList.contains('deck-active'));
  const pose = entry => entry.playhead.find(pose => pose.slideIndex === SlidesNav.current() - 1 && pose.step === SlidesNav.step());
  const branch = entry => entry.branches.find(branch => branch.id === (selected.get(entry.id) || 'baseline'));
  function tickNumber(entry, tick) {
    if (!Number.isFinite(tick)) throw new RangeError('Simulation tick must be finite');
    return Math.max(0, Math.min(entry.ticks, Math.floor(tick)));
  }
  function render(mount, entry, tick) {
    const currentBranch = branch(entry), frame = currentBranch.frames[tick];
    const dots = mount.querySelectorAll('[data-simulation-particle]');
    frame.state.particles.forEach((particle, index) => {
      const dot = dots[index]; if (!dot) return;
      dot.setAttribute('cx', String(particle.x / 1000)); dot.setAttribute('cy', String(particle.y / 1000));
    });
    mount.dataset.simulationTick = String(tick); mount.dataset.simulationBranch = currentBranch.id; mount.dataset.simulationHash = frame.hash;
    mount.querySelector('.simulation-tick').value = String(tick);
    mount.querySelector('.simulation-branch').value = currentBranch.id;
    mount.querySelector('.simulation-status').textContent = 'Tick ' + tick + ' / ' + entry.ticks;
    mount.querySelector('svg').setAttribute('aria-label', entry.label + ', ' + currentBranch.label + ', tick ' + tick);
  }
  function sample(ms) {
    for (const mount of active()) {
      const entry = entries.get(mount.dataset.simulation); if (!entry || manual.has(entry.id)) continue;
      const current = pose(entry), start = current?.tick || 0, end = current?.endTick ?? start;
      const elapsed = reduced.matches ? Infinity : Math.max(0, Number(ms) || 0);
      render(mount, entry, tickNumber(entry, Math.min(end, start + Math.floor(elapsed * entry.tickRate / 1000))));
    }
  }
  function duration() {
    return active().reduce((longest, mount) => {
      const entry = entries.get(mount.dataset.simulation), current = entry && pose(entry);
      return Math.max(longest, current ? ((current.endTick ?? current.tick) - current.tick) * 1000 / entry.tickRate : 0);
    }, 0);
  }
  function seek(id, tick) {
    const entry = entries.get(id); if (!entry) throw new RangeError('Unknown simulation');
    const frame = tickNumber(entry, tick); manual.add(id);
    for (const mount of mounts) if (mount.dataset.simulation === id) render(mount, entry, frame);
    return { tick: frame, branch: branch(entry).id, hash: branch(entry).frames[frame].hash };
  }
  function chooseBranch(id, value) {
    const entry = entries.get(id); if (!entry || !entry.branches.some(branch => branch.id === value)) throw new RangeError('Unknown simulation branch');
    selected.set(id, value);
    const mount = active().find(mount => mount.dataset.simulation === id);
    if (manual.has(id)) return seek(id, Number(mount?.dataset.simulationTick || 0));
    sample(window.SlidesMotion?.state().time || 0);
    return state(id);
  }
  function follow(id) {
    manual.delete(id); sample(window.SlidesMotion?.state().time || 0); return state(id);
  }
  function state(id) {
    const mount = active().find(mount => mount.dataset.simulation === id);
    return mount ? { tick: Number(mount.dataset.simulationTick), branch: mount.dataset.simulationBranch, hash: mount.dataset.simulationHash, manual: manual.has(id) } : null;
  }
  mounts.forEach(mount => {
    const id = mount.dataset.simulation;
    mount.querySelector('.simulation-tick').addEventListener('input', event => { window.SlidesMotion?.pause(); seek(id, Number(event.target.value)); });
    mount.querySelector('.simulation-branch').addEventListener('change', event => chooseBranch(id, event.target.value));
    mount.querySelector('.simulation-reset').addEventListener('click', () => follow(id));
  });
  window.SlidesSimulation = { manifest, sample, duration, seek, branch: chooseBranch, follow, state };
  deck.addEventListener('slides:change', () => { manual.clear(); sample(window.SlidesMotion?.state().time || 0); });
  reduced.addEventListener('change', () => sample(window.SlidesMotion?.state().time || 0));
  sample(window.SlidesMotion?.state().time || 0);
})();

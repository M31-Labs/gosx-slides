package slides

import "strconv"

func renderToolbar(deck *Deck) string {
	last := "1"
	if len(deck.Slides) > 0 {
		last = strconv.Itoa(len(deck.Slides))
	}
	return `<nav class="toolbar" aria-label="Slide controls">
  <button type="button" data-action="prev" title="Previous slide" aria-label="Previous slide">&lsaquo;</button>
  <button type="button" data-action="next" title="Next slide" aria-label="Next slide">&rsaquo;</button>
  <button type="button" data-action="overview" title="Overview" aria-label="Overview">&#9638;</button>
  <button type="button" data-action="fullscreen" title="Fullscreen" aria-label="Fullscreen">&#9974;</button>
  <span class="counter" id="counter">1 / ` + last + `</span>
  <span class="progress-meter" aria-hidden="true"><span id="progress-bar"></span></span>
</nav>
`
}

func runtimeJS(opts RenderOptions) string {
	live := "false"
	if opts.LiveReload {
		live = "true"
	}
	return `
(function () {
  const deck = window.__SLIDES_DECK__ || { slides: [] };
  const slides = Array.from(document.querySelectorAll('.slide'));
  const overview = document.getElementById('overview');
  const counter = document.getElementById('counter');
  const progressBar = document.getElementById('progress-bar');
  let index = initialSlide();
  let step = 0;
  let remoteLocked = document.body.classList.contains('mode-audience');

  function initialSlide() {
    const query = new URLSearchParams(location.search);
    const fromQuery = parseInt(query.get('slide') || '', 10);
    if (!Number.isNaN(fromQuery) && fromQuery > 0) return Math.min(fromQuery - 1, slides.length - 1);
    const fromHash = parseInt((location.hash || '').replace('#', ''), 10);
    if (!Number.isNaN(fromHash) && fromHash > 0) return Math.min(fromHash - 1, slides.length - 1);
    return 0;
  }

  function maxStep() {
    const active = slides[index];
    return active ? parseInt(active.dataset.clicks || '0', 10) : 0;
  }

  function show(nextIndex, nextStep, push) {
    if (!slides.length) return;
    index = Math.max(0, Math.min(slides.length - 1, nextIndex));
    step = Math.max(0, Math.min(maxStep(), nextStep));
    slides.forEach((slide, i) => slide.classList.toggle('is-active', i === index));
    applySteps();
    if (counter) counter.textContent = (index + 1) + ' / ' + slides.length + (maxStep() ? ' +' + step : '');
    if (progressBar) {
      const clickFraction = maxStep() > 0 ? step / (maxStep() + 1) : 0;
      const progress = ((index + clickFraction) / Math.max(1, slides.length - 1)) * 100;
      progressBar.style.width = Math.max(0, Math.min(100, progress)) + '%';
    }
    if (push && !remoteLocked) history.replaceState(null, '', '#' + (index + 1));
  }

  function applySteps() {
    const active = slides[index];
    document.querySelectorAll('.bind-step').forEach(el => {
      el.textContent = String(el.closest('.slide') === active ? step : 0);
    });
    document.querySelectorAll('.step').forEach(el => {
      const n = parseInt(el.dataset.step || '0', 10);
      const visible = el.closest('.slide') !== active || n <= step;
      el.classList.toggle('is-visible', visible);
    });
    document.querySelectorAll('.code-frame').forEach(frame => {
      const activeFrame = frame.closest('.slide') === active;
      const hasSpec = frame.querySelector('[data-steps]');
      frame.classList.toggle('no-step', !hasSpec);
      frame.querySelectorAll('.code-line').forEach(line => {
        const list = (line.dataset.steps || '').split(',').filter(Boolean).map(Number);
        const focus = !activeFrame || !hasSpec || list.includes(step) || (step === 0 && list.length === 0);
        line.classList.toggle('is-focus', focus);
      });
    });
    drawCanvases();
  }

  function next() {
    if (step < maxStep()) show(index, step + 1, true);
    else show(index + 1, 0, true);
  }
  function prev() {
    if (step > 0) show(index, step - 1, true);
    else show(index - 1, 0, true);
  }
  function gotoSlide(n) { show(n, 0, true); }

  document.addEventListener('keydown', event => {
    if (event.target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(event.target.tagName)) return;
    if (event.key === 'ArrowRight' || event.key === ' ') { event.preventDefault(); next(); }
    if (event.key === 'ArrowLeft') { event.preventDefault(); prev(); }
    if (event.key === 'f') toggleFullscreen();
    if (event.key === 'o') toggleOverview();
  });

  document.querySelectorAll('[data-action]').forEach(button => {
    button.addEventListener('click', () => {
      const action = button.dataset.action;
      if (action === 'next') next();
      if (action === 'prev') prev();
      if (action === 'overview') toggleOverview();
      if (action === 'fullscreen') toggleFullscreen();
    });
  });
  document.querySelectorAll('[data-goto]').forEach(button => {
    button.addEventListener('click', () => {
      if (overview) overview.classList.remove('is-open');
      gotoSlide(parseInt(button.dataset.goto || '0', 10));
    });
  });

  document.querySelectorAll('[data-poll]').forEach(poll => {
    const key = 'gosx-slides:' + poll.dataset.poll;
    let state = {};
    try { state = JSON.parse(localStorage.getItem(key) || '{}'); } catch (_) {}
    function renderPoll() {
      poll.querySelectorAll('[data-poll-option]').forEach(button => {
        const option = button.dataset.pollOption;
        const count = state[option] || 0;
        const label = button.querySelector('[data-poll-count]');
        if (label) label.textContent = String(count);
        button.classList.toggle('is-selected', button.dataset.selected === 'true');
      });
    }
    poll.querySelectorAll('[data-poll-option]').forEach(button => {
      button.addEventListener('click', () => {
        const option = button.dataset.pollOption;
        const already = button.dataset.selected === 'true';
        poll.querySelectorAll('[data-poll-option]').forEach(other => { other.dataset.selected = 'false'; });
        if (!already) {
          state[option] = (state[option] || 0) + 1;
          button.dataset.selected = 'true';
        }
        localStorage.setItem(key, JSON.stringify(state));
        renderPoll();
      });
    });
    renderPoll();
  });

  function toggleFullscreen() {
    if (!document.fullscreenElement && document.documentElement.requestFullscreen) document.documentElement.requestFullscreen();
    else if (document.exitFullscreen) document.exitFullscreen();
  }
  function toggleOverview() {
    if (!overview) return;
    overview.classList.toggle('is-open');
    overview.setAttribute('aria-hidden', overview.classList.contains('is-open') ? 'false' : 'true');
  }

  function drawCanvases() {
    document.querySelectorAll('.scene3d canvas').forEach(canvas => drawScene(canvas));
    document.querySelectorAll('.canvas-board canvas').forEach(canvas => drawBoard(canvas));
  }
  function fitCanvas(canvas) {
    const rect = canvas.getBoundingClientRect();
    const scale = window.devicePixelRatio || 1;
    const width = Math.max(1, Math.floor(rect.width * scale));
    const height = Math.max(1, Math.floor(rect.height * scale));
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }
    return { width, height };
  }
  function drawScene(canvas) {
    const size = fitCanvas(canvas);
    const ctx = canvas.getContext('2d');
    const t = (Date.now() / 900) + step * 0.6;
    ctx.clearRect(0, 0, size.width, size.height);
    const grad = ctx.createLinearGradient(0, 0, size.width, size.height);
    grad.addColorStop(0, '#f7f4ed');
    grad.addColorStop(1, '#dfeee9');
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, size.width, size.height);
    ctx.strokeStyle = 'rgba(11,117,111,0.18)';
    ctx.lineWidth = 2;
    for (let x = 0; x < size.width; x += 42) {
      ctx.beginPath(); ctx.moveTo(x, size.height * 0.72); ctx.lineTo(size.width / 2, size.height * 0.18); ctx.stroke();
    }
    const cx = size.width * 0.52;
    const cy = size.height * 0.48;
    for (let i = 0; i < 3; i++) {
      const a = t + i * 2.1;
      const r = size.width * (0.09 + i * 0.035);
      ctx.fillStyle = ['#0b756f', '#c94f3d', '#c79a2b'][i];
      ctx.beginPath();
      ctx.ellipse(cx + Math.cos(a) * r, cy + Math.sin(a) * r * 0.5, r * 0.55, r * 0.35, a, 0, Math.PI * 2);
      ctx.fill();
    }
  }
  function drawBoard(canvas) {
    const size = fitCanvas(canvas);
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, size.width, size.height);
    ctx.fillStyle = '#fffdf8';
    ctx.fillRect(0, 0, size.width, size.height);
    ctx.strokeStyle = '#d7d1c4';
    ctx.lineWidth = 1;
    for (let x = 0; x < size.width; x += 32) { ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, size.height); ctx.stroke(); }
    for (let y = 0; y < size.height; y += 32) { ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(size.width, y); ctx.stroke(); }
    ctx.strokeStyle = '#c94f3d';
    ctx.lineWidth = 8;
    ctx.lineCap = 'round';
    ctx.beginPath();
    ctx.moveTo(size.width * 0.18, size.height * 0.64);
    ctx.bezierCurveTo(size.width * 0.32, size.height * 0.24, size.width * 0.62, size.height * 0.82, size.width * 0.82, size.height * 0.34);
    ctx.stroke();
  }
  window.addEventListener('resize', drawCanvases);
  setInterval(() => {
    if (document.querySelector('.slide.is-active .scene3d')) drawCanvases();
  }, 80);

  if (` + live + ` && window.EventSource) {
    const events = new EventSource('/events');
    events.addEventListener('reload', () => location.reload());
    events.addEventListener('state', event => {
      try {
        const state = JSON.parse(event.data);
        remoteLocked = true;
        show(state.slideIndex || 0, state.clickStep || 0, false);
      } catch (_) {}
    });
  }

  window.Slides = { show, next, prev, goto: gotoSlide };
  show(index, step, false);
})();
`
}

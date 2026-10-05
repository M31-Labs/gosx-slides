(function () {
  'use strict';
  const deck = document.querySelector('main.deck');
  const api = window.SlidesNav;
  if (!deck || !api || deck.classList.contains('deck-presenter')) return;
  const slides = Array.from(deck.querySelectorAll(':scope > .slide'));
  const toc = document.createElement('nav');
  toc.className = 'reading-toc';
  toc.setAttribute('aria-label', 'Table of contents');
  const heading = document.createElement('h2');
  heading.textContent = 'Contents';
  const list = document.createElement('ol');
  slides.forEach((slide, index) => {
    const item = document.createElement('li');
    const link = document.createElement('a');
    link.href = '#' + (slide.dataset.slideId || String(index + 1));
    link.textContent = slide.querySelector('h1,h2,h3,h4,h5,h6')?.textContent || 'Slide ' + (index + 1);
    link.addEventListener('click', event => {
      if (!enabled) return;
      event.preventDefault();
      if (api.preview) api.preview(index, 0);
      history.replaceState(null, '', link.getAttribute('href'));
      slide.scrollIntoView({ block: 'start', behavior: 'instant' });
      const title = slide.querySelector('h1,h2,h3,h4,h5,h6');
      if (title) { title.setAttribute('tabindex', '-1'); title.focus({ preventScroll: true }); }
    });
    item.append(link); list.append(item);
  });
  toc.append(heading, list); deck.prepend(toc);
  const button = document.createElement('button');
  button.className = 'reading-toggle';
  button.type = 'button'; button.textContent = 'Read';
  button.setAttribute('aria-label', 'Toggle reading view');
  button.setAttribute('aria-pressed', 'false');
  deck.querySelector('.deck-controls')?.append(button);
  let enabled = false;
  let savedFragments = [];
  function reveal() {
    if (!enabled) return;
    deck.querySelectorAll('.slide [data-fragment]').forEach(fragment => {
      fragment.inert = false; fragment.removeAttribute('aria-hidden'); fragment.hidden = false;
    });
  }
  const observer = new MutationObserver(reveal);
  function setReading(value, updateURL = true) {
    if (value === enabled) return;
    enabled = value;
    if (value) {
      savedFragments = Array.from(deck.querySelectorAll('.slide [data-fragment]'), element => ({
        element, inert: element.inert, hidden: element.hidden, aria: element.getAttribute('aria-hidden')
      }));
      window.SlidesMotion?.pause();
      reveal();
      observer.observe(deck, { subtree: true, attributes: true, attributeFilter: ['inert', 'aria-hidden', 'hidden'] });
    } else {
      observer.disconnect();
      savedFragments.forEach(({ element, inert, hidden, aria }) => {
        element.inert = inert; element.hidden = hidden;
        if (aria === null) element.removeAttribute('aria-hidden'); else element.setAttribute('aria-hidden', aria);
      });
      savedFragments = [];
    }
    document.documentElement.classList.toggle('slides-reading', value);
    deck.classList.toggle('deck-reading', value);
    button.setAttribute('aria-pressed', String(value));
    button.textContent = value ? 'Present' : 'Read';
    if (updateURL) {
      const url = new URL(location.href);
      if (value) url.searchParams.set('read', ''); else url.searchParams.delete('read');
      history.replaceState(null, '', url);
    }
    window.dispatchEvent(new Event('resize'));
    if (!value) slides[api.current() - 1]?.scrollIntoView({ block: 'start', behavior: 'instant' });
    document.dispatchEvent(new CustomEvent('slides:reading', { detail: { enabled: value } }));
  }
  button.addEventListener('click', () => setReading(!enabled));
  const inputSelector = 'input,textarea,select,[contenteditable],dialog,' + ['textbox','searchbox','combobox','slider','spinbutton','scrollbar','listbox','option','tablist','tab','checkbox','radio','radiogroup','switch','tree','treeitem','grid','treegrid','gridcell','menu','menubar','menuitem','menuitemcheckbox','menuitemradio'].map(role => '[role~="' + role + '"]').join(',');
  document.addEventListener('keydown', event => {
    if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.target.closest(inputSelector)) return;
    if (event.key.toLowerCase() === 'v') { event.preventDefault(); event.stopImmediatePropagation(); setReading(!enabled); }
    else if (enabled && ['ArrowUp','ArrowDown','ArrowLeft','ArrowRight','PageUp','PageDown','Home','End',' '].includes(event.key)) event.stopImmediatePropagation();
  }, true);
  window.SlidesReading = { set: setReading, enabled: () => enabled };
  if (new URLSearchParams(location.search).has('read') || deck.dataset.reading === '1') setReading(true, false);
})();

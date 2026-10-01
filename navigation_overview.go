package slides

// The picker contains text cards, never copies of live slide trees. Hiding the
// real surfaces lets GoSX pause rendering while their DOM and state stay intact.
func overviewStyle() string {
	return `
@media screen {
main.deck.deck-overview > .slide,
main.deck.deck-overview .pv-screen > .slide,
main.deck.deck-overview > .pv-stage,
main.deck.deck-overview > .pv-side,
main.deck.deck-overview > .pv-footer { display: none !important; }
}
main.deck .deck-overview-dialog[hidden] { display: none !important; }
main.deck .deck-overview-dialog {
  position: fixed; inset: 0; z-index: 100; display: flex; flex-direction: column;
  box-sizing: border-box; padding: clamp(1rem, 3vw, 2.5rem); gap: 1rem;
  background: var(--bg, #10141e); color: var(--fg, #eee);
  font: 400 1rem/1.5 var(--font-body, system-ui, sans-serif);
}
main.deck .deck-overview-head { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
main.deck .deck-overview-dialog .deck-overview-title { margin: 0; font: 700 clamp(1.5rem, 3vw, 2.25rem)/1.2 var(--font-heading, system-ui, sans-serif); letter-spacing: -.025em; }
main.deck .deck-overview-close { flex-shrink: 0; border: 1px solid var(--line, #8885); border-radius: .6rem; padding: .4rem .8rem; background: var(--surface, #8881); color: inherit; font: inherit; cursor: pointer; }
main.deck .deck-overview-search-label { display: block; font-size: .85rem; font-weight: 600; margin-bottom: .35rem; }
main.deck .deck-overview-search { display: block; box-sizing: border-box; width: 100%; border: 1px solid var(--line, #8885); border-radius: .7rem; padding: .7rem 1rem; background: var(--surface, #8881); color: inherit; font: inherit; }
main.deck .deck-overview-status { margin: 0; font-size: .85rem; color: var(--fg-muted, #888); }
main.deck .deck-overview-results { min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: .25rem; flex: 1; }
main.deck .deck-overview-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 260px), 1fr)); gap: .85rem; align-content: start; }
main.deck .deck-overview-card {
  display: flex; flex-direction: column; align-items: flex-start; gap: .55rem;
  min-width: 0; min-height: 11rem; padding: 1.1rem; border: 1px solid var(--line, #8885);
  border-radius: var(--radius, 12px); background: var(--surface, #8881); color: inherit;
  text-align: left; white-space: normal; overflow-wrap: anywhere; cursor: pointer; font: inherit;
}
main.deck .deck-overview-card[hidden] { display: none !important; }
main.deck .deck-overview-card[aria-current="true"] { border-color: var(--accent, #eeb86b); box-shadow: inset 0 0 0 1px var(--accent, #eeb86b); }
main.deck .deck-overview-card:hover { background: color-mix(in srgb, var(--accent, #eeb86b) 10%, var(--bg, #10141e)); }
main.deck .deck-overview-ordinal { font: 600 .75rem/1 var(--font-mono, ui-monospace, monospace); color: var(--accent, #eeb86b); }
main.deck .deck-overview-card-title { font-size: 1.1rem; font-weight: 700; line-height: 1.3; }
main.deck .deck-overview-excerpt { font-size: .85rem; color: var(--fg-muted, #aaa); line-height: 1.5; }
main.deck .deck-overview-meta { margin-top: auto; padding-top: .25rem; font-size: .75rem; color: var(--fg-muted, #aaa); }
main.deck .deck-overview-empty { padding: 2rem 0; text-align: center; color: var(--fg-muted, #aaa); }
main.deck .deck-overview-help { border-top: 1px solid var(--line, #8885); padding-top: .6rem; max-height: 35vh; overflow-y: auto; font-size: .85rem; }
main.deck .deck-overview-help summary { cursor: pointer; font-weight: 600; }
main.deck .deck-overview-shortcuts { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: .4rem 1.5rem; margin: .7rem 0 0; }
main.deck .deck-overview-shortcuts div { display: flex; justify-content: space-between; gap: .75rem; }
main.deck .deck-overview-shortcuts dd { margin: 0; color: var(--fg-muted, #aaa); }
main.deck .deck-overview-dialog :is(button, input, summary):focus-visible { outline: 2px solid var(--accent, #eeb86b); outline-offset: 3px; }
@media (max-width: 640px) {
  main.deck .deck-overview-dialog { gap: .65rem; padding: 1rem; }
  main.deck .deck-overview-dialog .deck-overview-title { font-size: 1.25rem; }
  main.deck .deck-overview-search { padding: .55rem .7rem; }
  main.deck .deck-overview-help { max-height: 25vh; }
}
@media print { main.deck .deck-overview-dialog { display: none !important; } }
`
}

// Inserted inside navScript's closure so it uses the same navigation state and
// remains available to presenter, live-server, and exported decks.
func overviewScript() string {
	return overviewSearchScript() + `
  var picker = null, search = null, resultGrid = null, resultStatus = null, emptyResult = null, help = null;
  var cards = [], visibleCards = [], searchRecords = [], pickerFocus = null, pausedMedia = [];

  function plainText(root) {
    var walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, {
      acceptNode: function (node) {
        if (node.nodeType === 1) return node.matches('script, style, template, .slide-notes, .slide-header, .slide-footer, .code-copy') ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_SKIP;
        return NodeFilter.FILTER_ACCEPT;
      }
    });
    var parts = [], node;
    while ((node = walker.nextNode())) parts.push(node.textContent);
    return parts.join(' ').replace(/\s+/g, ' ').trim();
  }
  function pickerElement(tag, cls, text) {
    var node = document.createElement(tag); node.className = cls;
    if (text != null) node.textContent = text;
    return node;
  }
  function buildPicker() {
    if (picker) return;
    picker = pickerElement('div', 'deck-overview-dialog'); picker.hidden = true;
    picker.setAttribute('role', 'dialog'); picker.setAttribute('aria-modal', 'true'); picker.setAttribute('aria-labelledby', 'slides-picker-title');
    var head = pickerElement('div', 'deck-overview-head');
    var title = pickerElement('h2', 'deck-overview-title', 'Find your next slide'); title.id = 'slides-picker-title'; head.appendChild(title);
    var close = pickerElement('button', 'deck-overview-close', 'Close · Esc'); close.type = 'button'; close.setAttribute('aria-label', 'Close slide overview'); close.addEventListener('click', closeOverview); head.appendChild(close); picker.appendChild(head);
    var field = pickerElement('div', 'deck-overview-field');
    var label = pickerElement('label', 'deck-overview-search-label', 'Search slides or enter a number'); label.htmlFor = 'slides-picker-search'; field.appendChild(label);
    search = pickerElement('input', 'deck-overview-search'); search.id = 'slides-picker-search'; search.type = 'search'; search.autocomplete = 'off'; search.placeholder = 'Title, topic, or slide number'; search.setAttribute('aria-controls', 'slides-picker-grid'); search.addEventListener('input', filterCards); field.appendChild(search); picker.appendChild(field);
    resultStatus = pickerElement('p', 'deck-overview-status'); resultStatus.setAttribute('role', 'status'); resultStatus.setAttribute('aria-live', 'polite'); picker.appendChild(resultStatus);
    var results = pickerElement('div', 'deck-overview-results');
    resultGrid = pickerElement('div', 'deck-overview-grid'); resultGrid.id = 'slides-picker-grid'; results.appendChild(resultGrid);
    emptyResult = pickerElement('p', 'deck-overview-empty', 'No matching slides. Try another topic or slide number.'); results.appendChild(emptyResult); picker.appendChild(results);
    help = pickerElement('details', 'deck-overview-help'); help.appendChild(pickerElement('summary', '', 'Keyboard shortcuts'));
    help.appendChild(pickerElement('p', '', 'In search, with a result focused: arrows and Home/End choose results; Enter or Space opens it. Tab moves focus; Esc closes search. While typing, search keeps its usual editing keys; Enter opens the first match.'));
    help.appendChild(pickerElement('p', '', 'Presentation shortcuts — close search with Esc to use these:'));
    var shortcuts = pickerElement('dl', 'deck-overview-shortcuts');
    [ ['→ / Space', 'Next step or slide'], ['←', 'Previous step or slide'], ['Home / End', 'First / last slide'], ['PageUp / PageDown', 'Previous / next step'], ['O / /', 'Search slides'], ['?', 'Keyboard shortcuts'], ['F', 'Fullscreen'], ['B / Esc', 'Blank / restore screen'], ['P', 'Open presenter'] ].forEach(function (pair) {
      var row = pickerElement('div', ''); row.appendChild(pickerElement('dt', '', pair[0])); row.appendChild(pickerElement('dd', '', pair[1])); shortcuts.appendChild(row);
    });
    help.appendChild(shortcuts); picker.appendChild(help);
    var batch = document.createDocumentFragment();
    slides.forEach(function (slide, i) {
      var heading = slide.querySelector('h1, h2, h3');
      var title = heading ? plainText(heading) : 'Slide ' + (i + 1);
      if (!title) title = 'Slide ' + (i + 1);
      var text = plainText(slide), excerpt = text.indexOf(title) === 0 ? text.slice(title.length).trim() : text;
      var clicks = stepCountFor(i);
      var card = pickerElement('button', 'deck-overview-card'); card.type = 'button'; card.setAttribute('data-picker-slide', String(i));
      card.setAttribute('aria-label', 'Slide ' + (i + 1) + ': ' + title);
      card.appendChild(pickerElement('span', 'deck-overview-ordinal', String(i + 1).padStart(2, '0')));
      card.appendChild(pickerElement('span', 'deck-overview-card-title', title));
      card.appendChild(pickerElement('span', 'deck-overview-excerpt', excerpt.length > 220 ? excerpt.slice(0, 220) + '…' : excerpt));
      if (clicks) card.appendChild(pickerElement('span', 'deck-overview-meta', clicks + ' step' + (clicks === 1 ? '' : 's')));
      cards.push(card); searchRecords.push(searchText(title + ' ' + text)); batch.appendChild(card);
    });
    resultGrid.appendChild(batch);
    resultGrid.addEventListener('click', function (event) {
      var card = event.target.closest('[data-picker-slide]');
      if (card) jumpTo(Number(card.getAttribute('data-picker-slide')));
    });
    deck.appendChild(picker);
  }
  function filterCards() {
    var query = searchText(search.value.trim());
    var matches = new Set(matchingSlideIndices(searchRecords, search.value));
    visibleCards = [];
    cards.forEach(function (card, i) {
      var matched = matches.has(i);
      card.hidden = !matched; card.tabIndex = -1;
      card.setAttribute('aria-current', i === index ? 'true' : 'false');
      if (matched) visibleCards.push(card);
    });
    var selected = visibleCards.indexOf(cards[index]);
    if (visibleCards.length) visibleCards[Math.max(0, selected)].tabIndex = 0;
    emptyResult.hidden = visibleCards.length > 0;
    resultStatus.textContent = visibleCards.length + ' of ' + slides.length + ' slides' + (query ? ' match' : ' · current slide ' + (index + 1));
  }
  function openOverview(showHelp) {
    if (overview) { if (showHelp === true) help.open = true; search.focus(); return; }
    buildPicker(); pickerFocus = document.activeElement; overview = true;
    if (blank) toggleBlank();
    deck.classList.add(OVERVIEW); picker.hidden = false; search.value = ''; help.open = showHelp === true;
    backgroundSurfaces.forEach(function (surface) { surface.classList.remove('deck-background-active'); });
    pausedMedia = Array.prototype.filter.call(slides[index].querySelectorAll('video, audio'), function (media) { return !media.paused; });
    pausedMedia.forEach(function (media) { media.pause(); });
    filterCards(); search.focus();
  }
  function dismissPicker() {
    overview = false; deck.classList.remove(OVERVIEW); if (picker) picker.hidden = true;
  }
  function focusPickerTarget(target) {
    if (!target || target === document.body || target === document.documentElement || !target.isConnected || !target.getClientRects().length || typeof target.focus !== 'function') return false;
    if (target.matches(':disabled') || target.closest('[inert], [aria-hidden="true"], [aria-disabled="true"]') || getComputedStyle(target).visibility !== 'visible') return false;
    target.focus({ preventScroll: true });
    return document.activeElement === target;
  }
  function restorePickerFocus() {
    if (focusPickerTarget(pickerFocus)) return;
    // Audience controls are hidden in presenter mode. Try every candidate so a
    // disabled/hidden control or a failed focus does not leave search focused.
    var candidates = deck.querySelectorAll('.deck-controls button[aria-label="Slide overview (O)"], .pv-controls button');
    for (var i = 0; i < candidates.length; i++) {
      if (focusPickerTarget(candidates[i])) return;
    }
    // Keep navigation reachable even without either set of chrome controls.
    if (!deck.hasAttribute('tabindex')) deck.setAttribute('tabindex', '-1');
    focusPickerTarget(deck);
  }
  function resumePickerMedia() {
    pausedMedia.forEach(function (media) { if (slides[index].contains(media)) { var promise = media.play(); if (promise && promise.catch) promise.catch(function () {}); } });
    pausedMedia = [];
  }
  function closeOverview() {
    if (!overview) return;
    dismissPicker(); show(index, step, false); resumePickerMedia(); restorePickerFocus();
  }
  function toggleOverview() { if (overview) closeOverview(); else openOverview(); }
  function jumpTo(i) {
    dismissPicker(); show(i, 0, true); resumePickerMedia(); restorePickerFocus();
  }
  function focusCard(i) {
    if (!visibleCards.length) return;
    i = Math.max(0, Math.min(visibleCards.length - 1, i));
    visibleCards.forEach(function (card, j) { card.tabIndex = j === i ? 0 : -1; });
    visibleCards[i].focus();
  }
  function columnCount() {
    var template = getComputedStyle(resultGrid).gridTemplateColumns;
    return template && template !== 'none' ? template.split(' ').length : 1;
  }
  function overviewKey(event) {
    if (event.key === 'Escape') { event.preventDefault(); closeOverview(); return; }
    if (event.key === 'Tab') {
      var stops = Array.prototype.filter.call(picker.querySelectorAll('button:not([disabled]), input, summary'), function (el) { return el.tabIndex >= 0 && el.getClientRects().length; });
      var first = stops[0], last = stops[stops.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      return;
    }
    if (event.target === search) {
      if (event.key === 'ArrowDown') { event.preventDefault(); focusCard(0); }
      else if (event.key === 'Enter' && visibleCards.length) { event.preventDefault(); jumpTo(Number(visibleCards[0].getAttribute('data-picker-slide'))); }
      return;
    }
    if (event.key === '?') { event.preventDefault(); help.open = true; return; }
    if (event.key === 'o' || event.key === 'O') { event.preventDefault(); closeOverview(); return; }
    var position = visibleCards.indexOf(document.activeElement);
    if (position >= 0) {
      var destination = position;
      if (event.key === 'ArrowRight') destination++;
      else if (event.key === 'ArrowLeft') destination--;
      else if (event.key === 'ArrowDown') destination += columnCount();
      else if (event.key === 'ArrowUp') destination -= columnCount();
      else if (event.key === 'Home') destination = 0;
      else if (event.key === 'End') destination = visibleCards.length - 1;
      else return; // Native button activation handles Enter and Space.
      event.preventDefault(); focusCard(destination);
    }
  }
`
}

// Search is pure so its number and text semantics can be checked independently
// of browser layout and hydration.
func overviewSearchScript() string {
	return `
  function searchText(text) { return text.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase(); }
  function matchingSlideIndices(records, input) {
    var query = searchText(input.trim()), tokens = query.split(/\s+/).filter(Boolean);
    var number = /^\d+$/.test(query) ? Number(query) : null;
    var matches = [];
    records.forEach(function (record, i) {
      if (number != null ? i + 1 === number : tokens.every(function (token) { return record.indexOf(token) >= 0; })) matches.push(i);
    });
    return matches;
  }
`
}

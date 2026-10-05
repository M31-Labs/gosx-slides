// Run against examples/navigation-lab. Playwright is an optional test dependency.
const { launchTestBrowser } = require('./test-browser.cjs');
const assert = require('node:assert/strict');

async function check() {
  const browser = await launchTestBrowser({
    ...(process.env.SLIDES_BROWSER ? { executablePath: process.env.SLIDES_BROWSER } : {}),
    args: ['--enable-unsafe-swiftshader'],
  });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const url = new URL(process.argv[2] || 'http://127.0.0.1:8080/');
    url.hash = '2';
    await page.goto(url.href, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.current() === 2);
    // Navigation is available before the asynchronous island bridge binds clicks.
    await page.waitForFunction(() => window.__gosx && __gosx.islands &&
      Array.from(__gosx.islands.values()).some(island => island.root.contains(document.querySelector('.counter-btn'))));
    await page.locator('.counter-btn').last().click();
    await page.waitForFunction(() => document.querySelector('.counter-label').textContent.includes('1'));
    await page.locator('.counter-btn').last().focus();
    await page.keyboard.press('Space');
    await page.waitForFunction(() => document.querySelector('.counter-label').textContent.includes('2'));
    assert.equal(await page.evaluate(() => SlidesNav.current()), 2);

    // A custom editor's focused child must retain both its keys and their
    // defaults. Composite controls and ARIA fallback role tokens count too.
    const widgetKeys = await page.evaluate(() => {
      const roles = ['textbox', 'searchbox', 'combobox', 'slider', 'spinbutton',
        'scrollbar', 'listbox', 'option', 'tablist', 'tab', 'checkbox', 'radio',
        'radiogroup', 'switch', 'tree', 'treeitem', 'grid', 'treegrid', 'gridcell',
        'menu', 'menubar', 'menuitem', 'menuitemcheckbox', 'menuitemradio',
        'unknown textbox', 'unknown searchbox', 'unknown combobox', 'unknown grid',
        'button', 'unknown button'];
      const results = [];
      const previousFocus = document.activeElement;
      for (const role of roles) {
        const widget = document.createElement('div');
        widget.setAttribute('role', role);
        const child = document.createElement('span');
        child.tabIndex = 0;
        child.textContent = 'Nested keyboard target';
        widget.append(child);
        document.querySelector('.deck-active').append(widget);
        child.focus();
        const keys = role === 'button' || role === 'unknown button' ? [' ', 'Enter'] :
          ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown', ' ', '?', 'o'];
        for (const key of keys) {
          const before = [SlidesNav.current(), SlidesNav.step(), location.hash];
          const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
          const accepted = child.dispatchEvent(event);
          results.push({ role, key, accepted, defaultPrevented: event.defaultPrevented,
            before, after: [SlidesNav.current(), SlidesNav.step(), location.hash] });
        }
        widget.remove();
      }
      previousFocus.focus();
      return results;
    });
    for (const result of widgetKeys) {
      assert.equal(result.accepted, true, `${result.role}: ${result.key} was canceled`);
      assert.equal(result.defaultPrevented, false, `${result.role}: ${result.key} lost its default`);
      assert.deepEqual(result.after, result.before, `${result.role}: ${result.key} navigated`);
    }
    // A standard button still permits arrow navigation; only activation keys
    // belong to the button (the Space activation and state are checked above).
    await page.keyboard.press('ArrowRight');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 3);
    await page.keyboard.press('ArrowLeft');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 2);
    await page.keyboard.press('o');
    assert.equal(await page.locator('.slide:visible').count(), 0);
    assert.equal(await page.locator('.deck-overview-dialog [data-gosx-island], .deck-overview-dialog canvas').count(), 0);
    await page.keyboard.press('?');
    assert.equal(await page.locator('.deck-overview-search').inputValue(), '?');
    assert.equal(await page.locator('.deck-overview-help').evaluate(node => node.open), false);
    await page.locator('.deck-overview-search').fill('');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('?');
    assert.equal(await page.locator('.deck-overview-help').evaluate(node => node.open), true);
    assert.ok((await page.locator('.deck-overview-help').textContent()).includes('close search with Esc to use these'));
    await page.locator('.deck-overview-search').fill('PRIVATE PRESENTER NOTE');
    assert.equal(await page.locator('.deck-overview-card:visible').count(), 0);
    await page.locator('.deck-overview-search').fill('creme brings');
    assert.equal(await page.locator('.deck-overview-card:visible').count(), 1);
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('.counter-label').textContent(), 'count is 2');

    url.hash = '3/1';
    await page.goto(url.href, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.step() === 1);
    await page.locator('a[href="#3/4"]').click();
    await page.waitForFunction(() => SlidesNav.step() === 4);
    assert.ok((await page.locator('.deck-active .slides-step-active').textContent()).includes('release'));
    await page.goBack();
    await page.waitForFunction(() => SlidesNav.step() === 1);
    await page.keyboard.press('ArrowRight');
    assert.ok(page.url().endsWith('#3/2'));
    await page.reload();
    await page.waitForFunction(() => window.SlidesNav && SlidesNav.step() === 2);

    // Reload leaves focus on the body. Closing search must restore a real focus
    // target, otherwise subsequent Home/arrow keys stay trapped in a hidden input.
    await page.keyboard.press('/');
    await page.locator('.deck-overview-search').fill('5');
    await page.keyboard.press('Enter');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 5);
    await page.keyboard.press('Home');
    assert.equal(await page.evaluate(() => SlidesNav.current()), 1);

    await page.mouse.click(120, 120);
    await page.mouse.move(130, 120);
    await page.waitForFunction(() => getComputedStyle(document.querySelector('.deck-controls')).opacity === '0', undefined, { timeout: 5000 });
    assert.equal(await page.locator('.deck-controls').evaluate(node => getComputedStyle(node).opacity), '0');
    await page.mouse.move(140, 120);
    await page.waitForFunction(() => getComputedStyle(document.querySelector('.deck-controls')).opacity === '1');
    await page.keyboard.press('Tab');
    await page.locator('.deck-controls button').first().focus();
    await page.waitForFunction(() => !document.querySelector('.deck-controls').classList.contains('deck-controls-visible'), undefined, { timeout: 5000 });
    assert.equal(await page.locator('.deck-controls').evaluate(node => getComputedStyle(node).opacity), '1');
    await page.close();

    // A touch lift emits pointerleave; retain controls until the idle timer
    // expires, rather than applying the immediate mouse-exit behavior.
    const touch = await browser.newPage({ viewport: { width: 390, height: 844 }, hasTouch: true });
    touch.on('pageerror', error => errors.push(error.message));
    url.hash = '1';
    await touch.goto(url.href, { waitUntil: 'domcontentloaded' });
    await touch.waitForFunction(() => window.SlidesNav);
    await touch.touchscreen.tap(120, 120);
    assert.equal(await touch.locator('.deck-controls').evaluate(node => node.classList.contains('deck-controls-visible')), true);
    await touch.waitForFunction(() => getComputedStyle(document.querySelector('.deck-controls')).opacity === '0', undefined, { timeout: 5000 });
    await touch.close();

    // Presenter chrome hides the audience toolbar. Restore a visible target
    // after both dismissal and selection, even when the prior target goes away.
    const presenterContext = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    try {
      const presenter = await presenterContext.newPage();
      presenter.on('pageerror', error => errors.push(error.message));
      const presenterURL = new URL(url.href);
      presenterURL.search = '?present';
      presenterURL.hash = '3/2';
      await presenter.goto(presenterURL.href, { waitUntil: 'domcontentloaded' });
      await presenter.waitForFunction(() => window.SlidesNav && SlidesNav.isPresenter() && document.querySelector('.pv-controls button'));
      for (const scenario of ['body-close', 'body-jump', 'hidden', 'disabled', 'failed-focus', 'no-controls']) {
        await presenter.evaluate(scenario => {
          SlidesNav.show(2, 2, true);
          document.activeElement.blur();
          if (scenario === 'hidden' || scenario === 'disabled') document.querySelector('.pv-controls button').focus();
          if (scenario === 'failed-focus') {
            const target = document.createElement('span');
            target.id = 'prior-picker-focus'; target.tabIndex = 0; target.textContent = 'Temporary focus';
            document.querySelector('.pv-controls').append(target); target.focus();
          }
        }, scenario);
        await presenter.keyboard.press('/');
        assert.equal(await presenter.locator('.deck-overview-search').evaluate(node => document.activeElement === node), true);
        await presenter.evaluate(scenario => {
          const button = document.querySelector('.pv-controls button');
          if (scenario === 'hidden') button.style.visibility = 'hidden';
          if (scenario === 'disabled') button.disabled = true;
          if (scenario === 'failed-focus') document.getElementById('prior-picker-focus').removeAttribute('tabindex');
          if (scenario === 'no-controls') document.querySelector('.pv-controls').remove();
        }, scenario);
        if (scenario === 'body-jump') {
          await presenter.locator('.deck-overview-search').fill('5');
          await presenter.keyboard.press('Enter');
          assert.equal(await presenter.evaluate(() => SlidesNav.current()), 5);
        } else {
          await presenter.keyboard.press('Escape');
          assert.equal(await presenter.evaluate(() => SlidesNav.current()), 3);
          assert.equal(await presenter.evaluate(() => SlidesNav.step()), 2);
        }
        assert.equal(await presenter.evaluate(() => {
          const target = document.activeElement;
          return !SlidesNav.isOverview() && target !== document.body &&
            !target.closest('.deck-overview-dialog') && target.getClientRects().length > 0 &&
            getComputedStyle(target).visibility === 'visible' && !target.matches(':disabled') &&
            !target.closest('[inert], [aria-hidden="true"], [aria-disabled="true"]');
        }), true, `${scenario}: focus did not leave hidden search for a visible enabled target`);
        await presenter.keyboard.press('Home');
        assert.equal(await presenter.evaluate(() => SlidesNav.current()), 1, `${scenario}: Home trapped`);
        await presenter.keyboard.press('ArrowDown');
        assert.equal(await presenter.evaluate(() => SlidesNav.current()), 1, `${scenario}: hidden picker handled ArrowDown`);
        await presenter.keyboard.press('ArrowRight');
        assert.equal(await presenter.evaluate(() => SlidesNav.current()), 2, `${scenario}: ArrowRight trapped`);
        await presenter.evaluate(() => {
          const button = document.querySelector('.pv-controls button');
          if (button) { button.style.visibility = ''; button.disabled = false; }
          document.getElementById('prior-picker-focus')?.remove();
        });
      }
    } finally {
      await presenterContext.close();
    }
    assert.deepEqual(errors, []);
    console.log('Navigation browser checks passed: widget state, nested input keys, search, step links, audience/presenter focus, idle controls.');
  } finally {
    await browser.close();
  }
}

check().catch(error => { console.error(error); process.exitCode = 1; });

const assert = require('node:assert/strict');
const fs = require('node:fs');
const { chromium } = require(process.env.SLIDES_PLAYWRIGHT_MODULE || 'playwright');
(async () => {
 const browser = await chromium.launch({...(process.env.SLIDES_BROWSER ? {executablePath:process.env.SLIDES_BROWSER} : {})});
 try {
  const page = await browser.newPage({viewport:{width:1280,height:720}}), errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  const url=process.argv[2], sourcePath=process.argv[3];
  await page.goto(url+'#1',{waitUntil:'domcontentloaded'});await page.waitForFunction(()=>window.SlidesInk);
  await page.keyboard.press('d');assert.equal(await page.locator('main.deck').getAttribute('data-ink-mode'),'pen');
  await page.mouse.move(220,260);await page.mouse.down();await page.mouse.move(420,360,{steps:8});await page.mouse.up();
  assert.equal(await page.evaluate(()=>SlidesInk.count()),1);assert.ok((await page.locator('.slides-ink polyline').getAttribute('points')).split(' ').length>=8);
  await page.getByRole('button',{name:'Done',exact:true}).click();
  await page.keyboard.press('ArrowRight');assert.equal(await page.evaluate(()=>SlidesInk.count()),0);
  await page.keyboard.press('ArrowLeft');assert.equal(await page.evaluate(()=>SlidesInk.count()),1);
  await page.keyboard.press('l');await page.mouse.move(350,300);assert.equal(await page.locator('main.deck').getAttribute('data-ink-mode'),'laser');await page.getByRole('button',{name:'Done',exact:true}).click();
  await page.keyboard.press('e');await page.getByRole('status').filter({hasText:'Ready to edit'}).waitFor();
  const textarea=page.locator('#slides-source'), original=await textarea.inputValue();
  await textarea.fill(original+'\n<Missing/>\n');await page.getByRole('button',{name:'Save and preview',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.slides-source-panel [data-status]').textContent.includes('validation failed'));
  assert.equal(fs.readFileSync(sourcePath,'utf8'),original);
  await textarea.fill(original.replace('Original text','Saved browser text'));await Promise.all([page.waitForNavigation(),page.getByRole('button',{name:'Save and preview',exact:true}).click()]);
  assert.ok(fs.readFileSync(sourcePath,'utf8').includes('Saved browser text'));await page.getByText('Saved browser text',{exact:true}).waitFor();
  await page.keyboard.press('e');await page.getByRole('status').filter({hasText:'Ready to edit'}).waitFor();fs.appendFileSync(sourcePath,'\n<!-- external edit -->\n');
  await page.getByRole('button',{name:'Save and preview',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.slides-source-panel [data-status]').textContent.includes('changed'));
  assert.ok(fs.readFileSync(sourcePath,'utf8').includes('external edit'));
  assert.deepEqual(errors,[]);console.log('Editing browser passed: save, validation, conflict, pen, laser, navigation.');
 } finally {await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});

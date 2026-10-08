const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { publishGallery } = require('./export-examples.cjs');

function fixture(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'slides-gallery-publish-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const output = path.join(dir, 'gallery');
  fs.mkdirSync(output);
  for (const name of ['deck.pdf', 'deck.png', 'manifest.json', 'keep.txt']) fs.writeFileSync(path.join(output, name), `old ${name}`);
  const files = {};
  for (const name of ['deck.pdf', 'deck.png']) {
    files[name] = path.join(dir, `new-${name}`);
    fs.writeFileSync(files[name], `new ${name}`);
  }
  const read = () => Object.fromEntries(fs.readdirSync(output).sort().map(name => [name, fs.readFileSync(path.join(output, name), 'utf8')]));
  return { dir, output, files, read, before: read(), manifest: { schema: 1, decks: [] } };
}

test('publishes assets and manifest together, retaining unrelated files', t => {
  const f = fixture(t);
  publishGallery(f.output, f.files, f.manifest, () => {
    assert.equal(f.read()['deck.pdf'], 'new deck.pdf');
    assert.equal(f.read()['deck.png'], 'new deck.png');
    assert.deepEqual(JSON.parse(f.read()['manifest.json']), f.manifest);
  });
  assert.equal(f.read()['keep.txt'], 'old keep.txt');
  assert.equal(fs.readdirSync(f.dir).some(name => name.startsWith('.gallery-publish-')), false);
});

for (const failure of ['copy', 'manifest', 'replacement', 'validation']) {
  test(`${failure} failure preserves the complete previous gallery`, t => {
    const f = fixture(t), io = { ...fs };
    const fail = () => { throw Object.assign(new Error(`injected ${failure} failure`), { code: 'ENOSPC' }); };
    if (failure === 'copy') io.copyFileSync = (source, target) => target.endsWith('.png') ? fail() : fs.copyFileSync(source, target);
    if (failure === 'manifest') io.writeFileSync = fail;
    if (failure === 'replacement') io.renameSync = (source, target) => path.basename(source) === 'next' ? fail() : fs.renameSync(source, target);
    assert.throws(() => publishGallery(f.output, f.files, f.manifest, failure === 'validation' ? fail : undefined, io), /injected/);
    assert.deepEqual(f.read(), f.before);
    assert.equal(fs.readdirSync(f.dir).some(name => name.startsWith('.gallery-publish-')), false);
  });
}

test('a failed rollback retains the complete previous gallery for recovery', t => {
  const f = fixture(t), io = { ...fs };
  io.renameSync = (source, target) => {
    if (['next', 'previous'].includes(path.basename(source))) throw new Error('injected rename failure');
    fs.renameSync(source, target);
  };
  assert.throws(() => publishGallery(f.output, f.files, f.manifest, undefined, io), /retained recovery files/);
  const work = fs.readdirSync(f.dir).find(name => name.startsWith('.gallery-publish-'));
  const previous = path.join(f.dir, work, 'previous');
  assert.deepEqual(Object.fromEntries(fs.readdirSync(previous).sort().map(name => [name, fs.readFileSync(path.join(previous, name), 'utf8')])), f.before);
});

// Regenerate the public gallery from real CLI captures, or verify retained
// source/output hashes without launching a browser: node ... --check.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');

const root = path.resolve(__dirname, '..');
const output = path.join(root, 'docs/examples');
const catalog = [
  { name: 'effects-cookbook', preview: 'slide-001-step-000.png', pages: 7 },
  { name: 'background-gallery', preview: 'slide-001-step-000.png', pages: 6 },
  { name: 'request-recovery', preview: 'slide-002-step-001.png', pages: 5 },
];
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const fingerprint = file => ({ bytes: fs.statSync(file).size, sha256: hash(file) });

function sources(name) {
  const files = {};
  function walk(dir) {
    for (const item of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const file = path.join(dir, item.name);
      if (item.isDirectory() && !item.name.startsWith('.') && !['build', 'dist', 'node_modules'].includes(item.name)) walk(file);
      else if (item.isFile() && /(?:^deck\.md$|\.css$|\.sel$|\.sir$|\.json$)/.test(item.name)) {
        files[path.relative(root, file).split(path.sep).join('/')] = hash(file);
      }
    }
  }
  walk(path.join(root, 'examples', name));
  return files;
}

function run(binary, args) {
  const result = spawnSync(binary, args, { cwd: root, encoding: 'utf8', timeout: 180000, maxBuffer: 8 * 1024 * 1024 });
  if (result.error || result.status !== 0) throw Error(`${args.join(' ')} failed: ${result.error || result.stderr || result.stdout}`);
  return result.stdout.trim();
}

function check() {
  const manifest = JSON.parse(fs.readFileSync(path.join(output, 'manifest.json'), 'utf8'));
  if (manifest.schema !== 1 || manifest.decks.length !== catalog.length) throw Error('Regenerate the gallery manifest.');
  for (const entry of catalog) {
    const saved = manifest.decks.find(deck => deck.name === entry.name);
    if (!saved || saved.pages !== entry.pages || saved.preview !== entry.preview || JSON.stringify(saved.sources) !== JSON.stringify(sources(entry.name))) {
      throw Error(`${entry.name}: sources changed; regenerate the gallery.`);
    }
    for (const ext of ['pdf', 'png']) {
      const file = `${entry.name}.${ext}`;
      if (JSON.stringify(saved.outputs[file]) !== JSON.stringify(fingerprint(path.join(output, file)))) throw Error(`${file}: output changed; regenerate the gallery.`);
    }
  }
  console.log('Gallery sources and all six published assets match their SHA-256 manifest.');
}

function publishGallery(destination, files, manifest, verify = () => {}, io = fs) {
  // Stage beside the destination so renames stay on the same filesystem.
  // Keep a complete prior directory until the new gallery passes validation.
  const work = io.mkdtempSync(path.join(path.dirname(destination), '.gallery-publish-'));
  const next = path.join(work, 'next'), previous = path.join(work, 'previous');
  let saved = false, installed = false, retainBackup = false;
  try {
    if (io.existsSync(destination)) io.cpSync(destination, next, { recursive: true });
    else io.mkdirSync(next);
    for (const [name, source] of Object.entries(files)) io.copyFileSync(source, path.join(next, name));
    io.writeFileSync(path.join(next, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
    if (io.existsSync(destination)) {
      io.renameSync(destination, previous);
      saved = true;
    }
    io.renameSync(next, destination);
    installed = true;
    verify();
  } catch (error) {
    try {
      if (installed) io.rmSync(destination, { recursive: true, force: true });
      if (saved) io.renameSync(previous, destination);
    } catch (rollbackError) {
      retainBackup = true;
      throw new AggregateError([error, rollbackError], `Gallery rollback failed; retained recovery files at ${work}: ${rollbackError.message} (original error: ${error.message})`);
    }
    throw error;
  } finally {
    if (!retainBackup) io.rmSync(work, { recursive: true, force: true });
  }
}

function generate(binary) {
  fs.mkdirSync(path.dirname(output), { recursive: true });
  const temp = fs.mkdtempSync(path.join(path.dirname(output), '.capture-'));
  try {
    const manifest = { schema: 1, cli: run(binary, ['version']), binarySHA256: hash(binary), viewport: { width: 1280, height: 720 }, flags: ['--capture', '--steps'], decks: [] };
    for (const entry of catalog) {
      const deck = path.join('examples', entry.name);
      const before = sources(entry.name);
      const pdf = path.join(temp, `${entry.name}.pdf`), frames = path.join(temp, entry.name);
      console.log(`Capturing ${deck}…`);
      run(binary, ['export', deck, '--format', 'pdf', '--capture', '--steps', '--width', '1280', '--height', '720', '--out', pdf]);
      run(binary, ['export', deck, '--format', 'frames', '--steps', '--width', '1280', '--height', '720', '--out', frames]);
      const count = fs.readdirSync(frames).filter(file => file.endsWith('.png')).length;
      if (count !== entry.pages) throw Error(`${entry.name}: expected ${entry.pages} states, got ${count}; review the deck and update the catalog.`);
      if (fs.readFileSync(pdf).subarray(0, 5).toString() !== '%PDF-') throw Error(`${entry.name}: invalid PDF output`);
      if (JSON.stringify(before) !== JSON.stringify(sources(entry.name))) throw Error(`${entry.name}: sources changed during capture; retry.`);
      fs.copyFileSync(path.join(frames, entry.preview), path.join(temp, `${entry.name}.png`));
      const outputs = {};
      for (const ext of ['pdf', 'png']) outputs[`${entry.name}.${ext}`] = fingerprint(path.join(temp, `${entry.name}.${ext}`));
      manifest.decks.push({ ...entry, sources: before, outputs });
    }
    // Capture and copy failures leave the old gallery untouched. A failed
    // directory replacement or validation restores the complete previous set.
    const files = {};
    for (const deck of catalog) for (const ext of ['pdf', 'png']) {
      const name = `${deck.name}.${ext}`;
      files[name] = path.join(temp, name);
    }
    publishGallery(output, files, manifest, check);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
}

if (require.main === module) {
  try {
    const args = process.argv.slice(2);
    if (args.length > 1 || args[0]?.startsWith('--') && args[0] !== '--check') throw Error('Usage: node scripts/export-examples.cjs [./slides | --check]');
    if (args[0] === '--check') check();
    else generate(path.resolve(args[0] || './slides'));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { publishGallery };

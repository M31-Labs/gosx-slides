// Reuse the same behavioral assertions across engines, including real WASM
// hydration, focus/search, mobile reading, local fonts and offline exports.
const {spawn, spawnSync} = require('node:child_process');
const path = require('node:path');
const http = require('node:http');
const assert = require('node:assert/strict');

(async () => {
  const binary = path.resolve(process.argv[2] || './slides');
  const reserve = http.createServer();
  await new Promise(resolve => reserve.listen(0, '127.0.0.1', resolve));
  const port = reserve.address().port;
  await new Promise(resolve => reserve.close(resolve));
  const server = spawn(binary, ['serve', 'examples/navigation-lab', '--port', String(port)], {stdio:['ignore','ignore','inherit']});
  try {
    const url = 'http://127.0.0.1:' + port + '/';
    for (let attempt = 0; ; attempt++) {
      try { if ((await fetch(url)).ok) break; } catch (_) {}
      if (attempt >= 300 || server.exitCode !== null) throw Error('Navigation server failed to start');
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    for (const [script, target] of [['navigation-browser.cjs',url], ['reading-browser.cjs',binary], ['math-browser.cjs',binary]]) {
      const result = spawnSync(process.execPath, [path.join(__dirname,script),target], {stdio:'inherit',env:process.env,timeout:240000});
      assert.equal(result.status, 0, script + ' failed: ' + (result.error?.message || ''));
    }
    console.log('Cross-browser checks passed for ' + (process.env.SLIDES_TEST_ENGINE || 'chromium'));
  } finally {
    server.kill('SIGTERM');
  }
})().catch(error => {console.error(error);process.exitCode = 1;});

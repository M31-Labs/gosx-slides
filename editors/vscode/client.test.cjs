const test = require('node:test');
const assert = require('node:assert/strict');
const { PassThrough, Writable } = require('node:stream');
const { EventEmitter } = require('node:events');
const { SlidesClient, rangeOffsets } = require('./client.cjs');
function fakeProcess(run) {
  const child = new EventEmitter(); child.stdout = new PassThrough(); child.stderr = new PassThrough();
  child.stdin = new Writable({ write(chunk, encoding, done) { const message = JSON.parse(chunk); queueMicrotask(() => run(message, child)); done(); } });
  child.kill = () => {}; return child;
}
test('MCP handshake, structured tools, errors and bounded protocol output', async () => {
  let notified = false;
  const child = fakeProcess((message, process) => {
    if (message.method === 'initialize') process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: message.id, result: { protocolVersion: '2025-11-25' } }) + '\n');
    else if (message.method === 'notifications/initialized') notified = true;
    else if (message.params.name === 'slides_project_list') {
      assert.equal(notified, true);
      process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: message.id, result: { structuredContent: { files: [{ path: 'deck.md' }] } } }) + '\n');
    } else process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: message.id, result: { isError: true, content: [{ type: 'text', text: 'project changed' }] } }) + '\n');
  });
  const client = new SlidesClient('slides binary', '/project', () => {}, (binary, args, options) => { assert.equal(binary, 'slides binary'); assert.deepEqual(args, ['mcp', '/project']); assert.equal(options.shell, false); return child; });
  assert.deepEqual(await client.call('slides_project_list'), { files: [{ path: 'deck.md' }] });
  await assert.rejects(client.call('slides_project_write'), /project changed/);
  await assert.rejects(client.request('tools/call', { huge: 'x'.repeat(7 * 1024 * 1024) }), /size limit/);
  child.stdout.write('ordinary log output\n');
  await assert.rejects(client.call('slides_project_list'), /closed/);
  client.dispose();
});
test('UTF-8 source ranges preserve Unicode and emoji in VS Code', () => {
  const source = 'Café 🦊\nmissing'; const start = Buffer.byteLength(source.split('missing')[0]);
  const [a, b] = rangeOffsets(source, { StartByte: start, EndByte: start + 7 });
  assert.equal(source.slice(a, b), 'missing');
});

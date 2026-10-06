const { spawn } = require('node:child_process');
const MAX_MESSAGE = 6 * 1024 * 1024 + 16384;
class SlidesClient {
  constructor(binary, directory, logger = () => {}, spawnProcess = spawn) {
    this.nextID = 1; this.pending = new Map(); this.buffer = ''; this.closed = false;
    this.process = spawnProcess(binary, ['mcp', directory], { stdio: ['pipe', 'pipe', 'pipe'], shell: false, windowsHide: true });
    this.process.stdout.setEncoding('utf8'); this.process.stderr.setEncoding('utf8');
    this.process.stdout.on('data', chunk => {
      this.buffer += chunk;
      while (this.buffer.includes('\n')) {
        const at = this.buffer.indexOf('\n'), line = this.buffer.slice(0, at); this.buffer = this.buffer.slice(at + 1);
        if (Buffer.byteLength(line) > MAX_MESSAGE) { this.fail(Error('slides MCP response exceeds size limit')); return; }
        let message; try { message = JSON.parse(line); } catch (_) { this.fail(Error('slides wrote invalid MCP output')); return; }
        if (message.jsonrpc !== '2.0') { this.fail(Error('slides wrote invalid JSON-RPC output')); return; }
        const waiting = this.pending.get(message.id); if (!waiting) continue;
        clearTimeout(waiting.timer); this.pending.delete(message.id);
        if (message.error) waiting.reject(Error(message.error.message)); else waiting.resolve(message.result);
      }
      if (Buffer.byteLength(this.buffer) > MAX_MESSAGE) this.fail(Error('slides MCP response exceeds size limit'));
    });
    this.process.stderr.on('data', chunk => logger(chunk.slice(0, 8192)));
    this.process.stdin.on('error', error => this.fail(error));
    this.process.on('error', error => this.fail(error));
    this.process.on('exit', () => this.fail(Error('slides MCP exited; reopen the project')));
    this.ready = this.request('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'gosx-slides-vscode', version: '0.1.0' } }).then(result => {
      if (!['2024-11-05', '2025-03-26', '2025-06-18', '2025-11-25'].includes(result.protocolVersion)) throw Error('Unsupported slides MCP protocol');
      this.process.stdin.write(JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' }) + '\n');
    });
  }
  fail(error) {
    if (this.closed) return; this.closed = true;
    for (const waiting of this.pending.values()) { clearTimeout(waiting.timer); waiting.reject(error); }
    this.pending.clear(); this.process.kill();
  }
  request(method, params) {
    if (this.closed) return Promise.reject(Error('slides MCP connection is closed'));
    if (this.pending.size >= 16) return Promise.reject(Error('Too many pending slides operations'));
    const id = this.nextID++, text = JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n';
    if (Buffer.byteLength(text) > MAX_MESSAGE) return Promise.reject(Error('slides MCP request exceeds size limit'));
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(Error('slides operation timed out; reload revisions before retrying a save')); }, 30000);
      this.pending.set(id, { resolve, reject, timer });
      this.process.stdin.write(text, error => { if (error) this.fail(error); });
    });
  }
  async call(name, argumentsValue = {}) {
    await this.ready;
    const result = await this.request('tools/call', { name, arguments: argumentsValue });
    if (result.isError) throw Error(result.content?.map(row => row.text || '').join('\n') || 'slides tool failed');
    return result.structuredContent || JSON.parse(result.content[0].text);
  }
  dispose() { this.fail(Error('slides project closed')); }
}
function rangeOffsets(text, range) {
  const bytes = Buffer.from(text, 'utf8');
  return [bytes.subarray(0, range.StartByte || 0).toString('utf8').length, bytes.subarray(0, range.EndByte || 0).toString('utf8').length];
}
module.exports = { SlidesClient, rangeOffsets };

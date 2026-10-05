const vscode = require('vscode');
const path = require('node:path');
const { SlidesClient, rangeOffsets } = require('./client.cjs');
const scheme = 'gosxslides';
function activate(context) {
  const output = vscode.window.createOutputChannel('GoSX Slides'), diagnostics = vscode.languages.createDiagnosticCollection('gosx-slides');
  const changes = new vscode.EventEmitter(), treeChanges = new vscode.EventEmitter(), cache = new Map();
  let client, directory, files = [], timer, diagnosisVersion = 0;
  const uri = name => vscode.Uri.from({ scheme, path: '/' + name, query: encodeURIComponent(directory) });
  const current = resource => {
    if (!client || decodeURIComponent(resource.query) !== directory) throw vscode.FileSystemError.Unavailable('Open this GoSX Slides project first');
    return resource.path.slice(1);
  };
  const provider = {
    onDidChangeFile: changes.event,
    watch: () => ({ dispose() {} }),
    stat: async resource => {
      const name = current(resource);
      if (!name || resource.path === '/') return { type: vscode.FileType.Directory, ctime: 0, mtime: 0, size: 0 };
      if (files.some(file => file.path.startsWith(name + '/'))) return { type: vscode.FileType.Directory, ctime: 0, mtime: 0, size: 0 };
      const row = files.find(file => file.path === name);
      if (!row) throw vscode.FileSystemError.FileNotFound(resource);
      return { type: vscode.FileType.File, ctime: 0, mtime: 0, size: row.bytes };
    },
    readFile: async resource => {
      const name = current(resource), doc = await client.call('slides_project_read', { file: name }); cache.set(name, doc);
      return Buffer.from(doc.source, 'utf8');
    },
    writeFile: async (resource, content) => {
      const name = current(resource), previous = cache.get(name);
      if (!previous) throw vscode.FileSystemError.Unavailable('Read the file before saving');
      try {
        const source = Buffer.from(content).toString('utf8');
        const previousContext = previous.contextRevision;
        const result = await client.call('slides_project_write', { file: name, source, revision: previous.revision, contextRevision: previous.contextRevision });
        for (const row of cache.values()) if (row.contextRevision === previousContext) row.contextRevision = result.contextRevision;
        cache.set(name, { ...previous, source, revision: result.revision, contextRevision: result.contextRevision });
        output.appendLine('Saved ' + name + '; previous source: ' + result.recovery);
        const row = files.find(file => file.path === name); if (row) row.bytes = content.length;
        changes.fire([{ type: vscode.FileChangeType.Changed, uri: resource }]);
      } catch (error) { throw vscode.FileSystemError.Unavailable(error.message); }
    },
    readDirectory: async resource => {
      const prefix = current(resource); const entries = new Map();
      for (const row of files) {
        if (prefix && !row.path.startsWith(prefix + '/')) continue;
        const tail = prefix ? row.path.slice(prefix.length + 1) : row.path, part = tail.split('/')[0];
        entries.set(part, tail.includes('/') ? vscode.FileType.Directory : vscode.FileType.File);
      }
      return [...entries];
    },
    createDirectory: () => { throw vscode.FileSystemError.NoPermissions('Create author files in the local project, then refresh'); },
    delete: () => { throw vscode.FileSystemError.NoPermissions('Project file deletion is not supported'); },
    rename: () => { throw vscode.FileSystemError.NoPermissions('Project file moves are not supported'); },
  };
  const tree = {
    onDidChangeTreeData: treeChanges.event,
    getChildren: () => files,
    getTreeItem: file => ({ label: file.path, description: file.kind, collapsibleState: vscode.TreeItemCollapsibleState.None, resourceUri: uri(file.path), command: { command: 'vscode.open', title: 'Open author source', arguments: [uri(file.path)] } }),
  };
  async function refresh() {
    if (!client) throw Error('Open a GoSX Slides project first');
    const result = await client.call('slides_project_list'); files = result.files; treeChanges.fire();
  }
  async function diagnose(document) {
    if (!client) return;
    const version = ++diagnosisVersion, documentVersion = document?.version, args = document?.uri.scheme === scheme ? { file: current(document.uri), source: document.getText() } : {};
    const report = await client.call('slides_project_diagnose', args);
    if (version !== diagnosisVersion || document?.version !== documentVersion) return;
    output.clear(); for (const row of report.diagnostics) output.appendLine(`${row.file}:${row.range.StartLine}:${row.range.StartCol} ${row.severity} ${row.code}: ${row.message}`);
    if (!report.diagnostics.length) output.appendLine('Project validates.');
    if (document && args.file) {
      const rows = report.diagnostics.filter(row => row.file === args.file).map(row => {
        const [start, end] = rangeOffsets(document.getText(), row.range);
        const severity = row.severity === 'error' ? vscode.DiagnosticSeverity.Error : row.severity === 'warning' ? vscode.DiagnosticSeverity.Warning : vscode.DiagnosticSeverity.Information;
        const finding = new vscode.Diagnostic(new vscode.Range(document.positionAt(start), document.positionAt(end)), row.message, severity); finding.code = row.code; finding.source = 'GoSX Slides'; return finding;
      });
      diagnostics.set(document.uri, rows);
    }
  }
  const command = (name, action) => context.subscriptions.push(vscode.commands.registerCommand(name, async () => {
    try { if (!vscode.workspace.isTrusted) throw Error('Workspace trust is required to start the slides binary'); await action(); } catch (error) { vscode.window.showErrorMessage(error.message); }
  }));
  command('gosxSlides.openProject', async () => {
    const entries = await vscode.workspace.findFiles('**/deck.md', '**/{.git,node_modules,build,dist,packs}/**', 64);
    const selected = await vscode.window.showQuickPick(entries.map(file => ({ label: vscode.workspace.asRelativePath(file), file })), { placeHolder: 'Choose a GoSX Slides deck.md' });
    if (!selected) return;
    if (vscode.workspace.textDocuments.some(doc => doc.uri.scheme === scheme && doc.isDirty)) throw Error('Save or close open project drafts before switching projects');
    client?.dispose(); directory = path.dirname(selected.file.fsPath); cache.clear(); diagnostics.clear();
    client = new SlidesClient(vscode.workspace.getConfiguration('gosxSlides').get('binary', 'slides'), directory, text => output.append(text));
    await client.ready; await refresh();
    await vscode.window.showTextDocument(await vscode.workspace.openTextDocument(uri('deck.md')), { preview: false });
  });
  command('gosxSlides.refresh', refresh);
  command('gosxSlides.diagnose', async () => { await diagnose(vscode.window.activeTextEditor?.document); output.show(true); });
  command('gosxSlides.assertStory', async () => { if (!client) throw Error('Open a project first'); output.appendLine(JSON.stringify(await client.call('slides_story_assert'), null, 2)); output.show(true); });
  command('gosxSlides.exportHandout', async () => { if (!client) throw Error('Open a project first'); const result = await client.call('slides_export_snapshot', { format: 'handout' }); output.appendLine('Handout: ' + path.join(directory, result.file)); output.show(true); });
  context.subscriptions.push(output, diagnostics, changes, treeChanges, vscode.workspace.registerFileSystemProvider(scheme, provider, { isCaseSensitive: true }), vscode.window.registerTreeDataProvider('gosxSlides.project', tree),
    vscode.workspace.onDidChangeTextDocument(event => { if (event.document.uri.scheme !== scheme) return; clearTimeout(timer); timer = setTimeout(() => diagnose(event.document).catch(error => output.appendLine(error.message)), 500); }),
    { dispose() { clearTimeout(timer); client?.dispose(); } });
}
module.exports = { activate };

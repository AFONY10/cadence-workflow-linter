import * as vscode from 'vscode';
import { initContext, extContext, outputChannel as _out, diagnosticCollection as _diag, lastIssues } from './context';
import * as fs from 'fs';
import { runLinter } from './diagnostics';
import { readReleasesManifest, artifactNameForPlatform, fetchJson, downloadArtifactTo, computeFileSha256 } from './cli';

export function activate(context: vscode.ExtensionContext) {
  // initialize shared context
  const diagnosticCollection = vscode.languages.createDiagnosticCollection('cadence-workflow-linter');
  const outputChannel = vscode.window.createOutputChannel('Cadence Workflow Linter');
  initContext(context, outputChannel, diagnosticCollection);
  outputChannel.appendLine('Cadence Workflow Linter extension activated');

  const runCmd = vscode.commands.registerCommand('cadenceLinter.run', () => runLinter());
  context.subscriptions.push(runCmd);

  // command: check for CLI updates and download if newer (uses cli helpers)
  const checkCmd = vscode.commands.registerCommand('cadenceLinter.checkForCliUpdate', async () => {
    if (!extContext) { vscode.window.showErrorMessage('Extension context not available'); return; }
    const ctx = extContext;
    const workspaceFolders = vscode.workspace.workspaceFolders;
    if (!workspaceFolders || workspaceFolders.length === 0) { vscode.window.showWarningMessage('No workspace folder open'); return; }
    const cwd = workspaceFolders[0].uri.fsPath;
    await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'Cadence Workflow Linter: checking for CLI update', cancellable: false }, async (progress) => {
      progress.report({ message: 'Checking release manifest...' });
      try {
        const manifest = readReleasesManifest();
        if (!manifest) throw new Error('No embedded releases manifest');
        const artifact = artifactNameForPlatform(manifest);
        if (!artifact) throw new Error('No artifact mapped for this platform in releases manifest');
        const owner = manifest.owner; const repo = manifest.repo; const tag = manifest.tag || 'latest';
        const manifestUrl = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/manifest.json`;
        outputChannel.appendLine(`Fetching manifest ${manifestUrl}`);
        const remoteManifest = await fetchJson(manifestUrl);
        const expectedSha = remoteManifest && remoteManifest.artifacts ? remoteManifest.artifacts[artifact] : null;
        const savedPath = ctx.globalState.get<string>('downloadedCliPath');
        const savedSha = ctx.globalState.get<string>('downloadedCliChecksum');
        const savedTag = ctx.globalState.get<string>('downloadedCliTag');

        if (savedPath && fs.existsSync(savedPath) && expectedSha && savedSha === expectedSha && savedTag === (remoteManifest.tag || tag)) {
          vscode.window.showInformationMessage('CLI is up-to-date (matching checksum)');
          outputChannel.appendLine('CLI is up-to-date; no action required');
          return;
        }

        progress.report({ message: 'Downloading new CLI binary...' });
        const url = `https://github.com/${owner}/${repo}/releases/${tag === 'latest' ? 'latest/download' : 'download/' + tag}/${artifact}`;
        outputChannel.appendLine(`Downloading CLI from ${url}`);
        const binPath = await downloadArtifactTo(ctx.globalStoragePath, url, artifact);
        if (expectedSha) {
          const realSha = await computeFileSha256(binPath);
          if (realSha !== expectedSha) {
            try { fs.unlinkSync(binPath); } catch (e) {}
            throw new Error(`Checksum mismatch: expected ${expectedSha} got ${realSha}`);
          }
        }
        await ctx.globalState.update('downloadedCliPath', binPath);
        await ctx.globalState.update('downloadedCliChecksum', expectedSha || '');
        await ctx.globalState.update('downloadedCliTag', remoteManifest.tag || tag);
        vscode.window.showInformationMessage('Downloaded and installed latest CLI binary');
        outputChannel.appendLine('Downloaded and verified CLI binary: ' + binPath);
      } catch (e) {
        const msg = String(e);
        vscode.window.showErrorMessage('Failed to update CLI: ' + msg);
        outputChannel.appendLine('Update failed: ' + msg);
      }
    });
  });
  context.subscriptions.push(checkCmd);

  const status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  status.text = 'Cadence Linter: Update CLI';
  status.command = 'cadenceLinter.checkForCliUpdate';
  status.tooltip = 'Check for latest cadence-workflow-linter CLI and download it';
  status.show();
  context.subscriptions.push(status);

  // Run on save if enabled (debounced)
  vscode.workspace.onDidSaveTextDocument((doc) => {
    const cfg = vscode.workspace.getConfiguration('cadenceLinter');
    if (cfg.get('runOnSave', true) && doc.languageId === 'go') {
      outputChannel.appendLine(`Document saved: ${doc.uri.fsPath} (language=${doc.languageId})`);
      outputChannel.appendLine('runOnSave enabled; scheduling linter run');
      scheduleRunLinter(doc.uri);
    }
  });

  // Run on activation
  scheduleRunLinter();

  // Hover provider
  const hoverProvider = vscode.languages.registerHoverProvider({ scheme: 'file', language: 'go' }, {
    provideHover(document, position) {
      const filePath = document.uri.fsPath;
      const issues = lastIssues.get(filePath) || [];
      const line = position.line + 1;
      const hits = issues.filter(i => i.line === line);
      if (hits.length === 0) return undefined;
      const md = new vscode.MarkdownString();
      for (const iss of hits) {
        md.appendMarkdown(`**${iss.rule}** — ${iss.message}\n\n`);
        const display = `${iss.file}:${iss.line}`;
        try { const uri = vscode.Uri.file(iss.file).toString(); md.appendMarkdown(`[${display}](${uri}#${iss.line})\n\n`); } catch (e) { md.appendMarkdown(`${display}\n\n`); }
      }
      md.isTrusted = true;
      return new vscode.Hover(md);
    }
  });
  context.subscriptions.push(hoverProvider);
}

function scheduleRunLinter(uri?: vscode.Uri) {
  if (require('./context').debounceTimer) clearTimeout(require('./context').debounceTimer);
  require('./context').debounceTimer = setTimeout(() => runLinter(uri), require('./context').DEBOUNCE_MS);
}

export function deactivate() {
  try { require('./context').diagnosticCollection.dispose(); } catch (e) {}
}

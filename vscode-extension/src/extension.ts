import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as path from 'path';

interface CliIssue {
  file: string;
  line: number;
  // column removed in CLI output; extension will map issues to full-line ranges
  rule: string;
  severity: string;
  message: string;
}

let diagnosticCollection: vscode.DiagnosticCollection;
let outputChannel: vscode.OutputChannel;
let debounceTimer: NodeJS.Timeout | undefined;
const DEBOUNCE_MS = 500;
// Keep last-run issues for hover provider
const lastIssues: Map<string, CliIssue[]> = new Map();

export function activate(context: vscode.ExtensionContext) {
  diagnosticCollection = vscode.languages.createDiagnosticCollection('cadence-workflow-linter');
  context.subscriptions.push(diagnosticCollection);
  outputChannel = vscode.window.createOutputChannel('Cadence Workflow Linter');
  context.subscriptions.push(outputChannel);
  outputChannel.appendLine('Cadence Workflow Linter extension activated');

  const runCmd = vscode.commands.registerCommand('cadenceLinter.run', () => runLinter());
  context.subscriptions.push(runCmd);

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

  // Hover provider: show issue details and callstack (if present) when hovering over an affected line
  const hoverProvider = vscode.languages.registerHoverProvider({ scheme: 'file', language: 'go' }, {
    provideHover(document, position) {
      const filePath = document.uri.fsPath;
      const issues = lastIssues.get(filePath) || [];
      const line = position.line + 1; // issues use 1-based lines
      const hits = issues.filter(i => i.line === line);
      if (hits.length === 0) return undefined;

      // Build markdown content with rule/message and optional output/callstack
  const md = new vscode.MarkdownString();
      for (const iss of hits) {
        md.appendMarkdown(`**${iss.rule}** — ${iss.message}\n\n`);
  const display = `${iss.file}:${iss.line}`;
        try {
          const uri = vscode.Uri.file(iss.file).toString();
          md.appendMarkdown(`[${display}](${uri}#${iss.line})\n\n`);
        } catch (e) {
          md.appendMarkdown(`${display}\n\n`);
        }
      }
  md.isTrusted = true;
      return new vscode.Hover(md);
    }
  });
  context.subscriptions.push(hoverProvider);
}

function scheduleRunLinter(uri?: vscode.Uri) {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => runLinter(uri), DEBOUNCE_MS);
}

async function runLinter(targetUri?: vscode.Uri) {
  const cfg = vscode.workspace.getConfiguration('cadenceLinter');
  const cliPath = cfg.get<string>('cliPath', 'cadence-workflow-linter.exe');
  const extraArgs = cfg.get<string[]>('args', []);

  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    vscode.window.showWarningMessage('No workspace folder open');
    return;
  }
  const cwd = workspaceFolders[0].uri.fsPath;

  const args = ['--format', 'json'].concat(extraArgs);
  if (targetUri) {
    args.push(targetUri.fsPath);
  } else {
    args.push(cwd);
  }

  outputChannel.appendLine(`Running: ${cliPath} ${args.map(a => a.includes(' ') ? '"' + a + '"' : a).join(' ')}`);

  await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'Cadence Workflow Linter', cancellable: false }, async (progress) => {
    progress.report({ message: 'Running linter...' });

    return new Promise<void>((resolve) => {
      const spawnAndCollect = (bin: string, a: string[]) => {
        return new Promise<{stdout: string, stderr: string, code: number|null, err?: any}>((res) => {
          const c = spawn(bin, a, { cwd });
          let _stdout = '';
          let _stderr = '';
          c.stdout?.on('data', (chunk) => {
            const s = chunk.toString();
            _stdout += s;
            outputChannel.append(s);
          });
          c.stderr?.on('data', (chunk) => {
            const s = chunk.toString();
            _stderr += s;
            outputChannel.append(s);
          });
          c.on('error', (err) => {
            res({ stdout: _stdout, stderr: _stderr, code: null, err });
          });
          c.on('close', (code) => {
            res({ stdout: _stdout, stderr: _stderr, code });
          });
        });
      };

      // Try configured CLI first; if it's not available, attempt `go run ./cmd/cadence-linter` as a sensible fallback.
      (async () => {
        let result = await spawnAndCollect(cliPath, args);
        if (result.err && (result.err as any).code === 'ENOENT') {
          outputChannel.appendLine(`Configured CLI not found: ${cliPath} — attempting 'go run ./cmd/cadence-linter' fallback`);
          const goArgs = ['run', './cmd/cadence-linter', '--format', 'json'].concat(extraArgs);
          if (targetUri) goArgs.push(targetUri.fsPath); else goArgs.push(cwd);
          result = await spawnAndCollect('go', goArgs);
        }

        if (result.err && (result.err as any).code !== 'ENOENT') {
          const msg = result.stderr || (result.err && (result.err as any).message) || String(result.err);
          vscode.window.showErrorMessage('cadence-workflow-linter failed: ' + msg);
          outputChannel.appendLine('Error: ' + msg);
          resolve();
          return;
        }

        if (result.code !== 0) {
          outputChannel.appendLine(`cadence-workflow-linter exited with code ${result.code}`);
        }

        try {
          const issues: CliIssue[] = JSON.parse(result.stdout || '[]');
          publishDiagnostics(issues);
        } catch (e) {
          const msg = e && typeof e === 'object' && 'message' in e ? (e as any).message : String(e);
          vscode.window.showErrorMessage('Failed to parse cadence-workflow-linter output: ' + msg);
          outputChannel.appendLine('Failed to parse output: ' + msg);
          outputChannel.appendLine('STDOUT: ' + result.stdout);
          outputChannel.appendLine('STDERR: ' + result.stderr);
        }
        resolve();
      })();
    });
  });
}

function publishDiagnostics(issues: CliIssue[]) {
  diagnosticCollection.clear();
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders) return;
  const cwd = workspaceFolders[0].uri.fsPath;

  const diagMap: Map<string, vscode.Diagnostic[]> = new Map();
  for (const issue of issues) {
    const filePath = path.isAbsolute(issue.file) ? issue.file : path.join(cwd, issue.file);
    // Use a full-line range (ignore column): start at column 0, end at a large column index
    const range = new vscode.Range(issue.line - 1, 0, issue.line - 1, 1000);
    const severity = issue.severity === 'error' ? vscode.DiagnosticSeverity.Error : (issue.severity === 'warning' ? vscode.DiagnosticSeverity.Warning : vscode.DiagnosticSeverity.Information);
    const diag = new vscode.Diagnostic(range, `${issue.rule}: ${issue.message}`, severity);
    diag.source = 'cadence-workflow-linter';

    const arr = diagMap.get(filePath) || [];
    arr.push(diag);
    diagMap.set(filePath, arr);
  }

  // store last issues for hover provider
  lastIssues.clear();
  for (const iss of issues) {
    const fp = path.isAbsolute(iss.file) ? iss.file : path.join(cwd, iss.file);
    const arr = lastIssues.get(fp) || [];
    arr.push(iss);
    lastIssues.set(fp, arr);
  }

  diagMap.forEach((diags, file) => {
    const uri = vscode.Uri.file(file);
    diagnosticCollection.set(uri, diags);
  });
}

export function deactivate() {
  if (diagnosticCollection) {
    diagnosticCollection.dispose();
  }
}

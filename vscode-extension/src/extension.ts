import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as path from 'path';

interface CliIssue {
  file: string;
  line: number;
  column: number;
  rule: string;
  severity: string;
  message: string;
}

let diagnosticCollection: vscode.DiagnosticCollection;
let outputChannel: vscode.OutputChannel;
let debounceTimer: NodeJS.Timeout | undefined;
const DEBOUNCE_MS = 500;

export function activate(context: vscode.ExtensionContext) {
  diagnosticCollection = vscode.languages.createDiagnosticCollection('cadence-workflow-linter');
  context.subscriptions.push(diagnosticCollection);
  outputChannel = vscode.window.createOutputChannel('Cadence Workflow Linter');
  context.subscriptions.push(outputChannel);

  const runCmd = vscode.commands.registerCommand('cadenceLinter.run', () => runLinter());
  context.subscriptions.push(runCmd);

  // Run on save if enabled (debounced)
  vscode.workspace.onDidSaveTextDocument((doc) => {
    const cfg = vscode.workspace.getConfiguration('cadenceLinter');
    if (cfg.get('runOnSave', true) && doc.languageId === 'go') {
      scheduleRunLinter(doc.uri);
    }
  });

  // Run on activation
  scheduleRunLinter();
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
      const child = spawn(cliPath, args, { cwd });
      let stdout = '';
      let stderr = '';

      child.stdout?.on('data', (chunk) => {
        const s = chunk.toString();
        stdout += s;
        outputChannel.append(s);
      });
      child.stderr?.on('data', (chunk) => {
        const s = chunk.toString();
        stderr += s;
        outputChannel.append(s);
      });

      child.on('error', (err) => {
        const msg = stderr || (err && (err as any).message) || String(err);
        vscode.window.showErrorMessage('cadence-workflow-linter failed: ' + msg);
        outputChannel.appendLine('Error: ' + msg);
        resolve();
      });

      child.on('close', (code) => {
        if (code !== 0) {
          outputChannel.appendLine(`cadence-workflow-linter exited with code ${code}`);
        }
        try {
          const issues: CliIssue[] = JSON.parse(stdout || '[]');
          publishDiagnostics(issues);
        } catch (e) {
          const msg = e && typeof e === 'object' && 'message' in e ? (e as any).message : String(e);
          vscode.window.showErrorMessage('Failed to parse cadence-workflow-linter output: ' + msg);
          outputChannel.appendLine('Failed to parse output: ' + msg);
          outputChannel.appendLine('STDOUT: ' + stdout);
          outputChannel.appendLine('STDERR: ' + stderr);
        }
        resolve();
      });
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
    const range = new vscode.Range(issue.line - 1, Math.max(0, issue.column-1), issue.line - 1, Math.max(0, issue.column));
    const severity = issue.severity === 'error' ? vscode.DiagnosticSeverity.Error : (issue.severity === 'warning' ? vscode.DiagnosticSeverity.Warning : vscode.DiagnosticSeverity.Information);
    const diag = new vscode.Diagnostic(range, `${issue.rule}: ${issue.message}`, severity);
    diag.source = 'cadence-workflow-linter';

    const arr = diagMap.get(filePath) || [];
    arr.push(diag);
    diagMap.set(filePath, arr);
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

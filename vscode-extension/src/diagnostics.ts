import * as vscode from 'vscode';
import { spawn } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import { diagnosticCollection, lastIssues, outputChannel } from './context';
import { resolveCliPath, resolveRulesPath } from './cli';

interface CliIssue { file: string; line: number; rule: string; severity: string; message: string }

export async function runLinter(targetUri?: vscode.Uri) {
  const cfg = vscode.workspace.getConfiguration('cadenceLinter');
  const extraArgs = cfg.get<string[]>('args', []);

  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders || workspaceFolders.length === 0) {
    vscode.window.showWarningMessage('No workspace folder open');
    return;
  }
  const cwd = workspaceFolders[0].uri.fsPath;

  const args = ['--format', 'json'].concat(extraArgs);
  const cliPath = await resolveCliPath(cfg, cwd);
  const rulesPath = resolveRulesPath(cwd);
  args.push('--rules', rulesPath);
  if (targetUri) args.push(targetUri.fsPath); else args.push(cwd);

  try { outputChannel.appendLine(`Running: ${cliPath} ${args.map(a => a.includes(' ') ? '"' + a + '"' : a).join(' ')}`); } catch (e) {}

  await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'Cadence Workflow Linter', cancellable: false }, async (progress) => {
    progress.report({ message: 'Running linter...' });

    return new Promise<void>((resolve) => {
      const spawnAndCollect = (bin: string, a: string[]) => {
        return new Promise<{stdout: string, stderr: string, code: number|null, err?: any}>((res) => {
          const c = spawn(bin, a, { cwd });
          let _stdout = '';
          let _stderr = '';
          c.stdout?.on('data', (chunk) => { const s = chunk.toString(); _stdout += s; try { outputChannel.append(s); } catch (e) {} });
          c.stderr?.on('data', (chunk) => { const s = chunk.toString(); _stderr += s; try { outputChannel.append(s); } catch (e) {} });
          c.on('error', (err) => { res({ stdout: _stdout, stderr: _stderr, code: null, err }); });
          c.on('close', (code) => { res({ stdout: _stdout, stderr: _stderr, code }); });
        });
      };

      (async () => {
        let result = await spawnAndCollect(cliPath, args);
        if (result.err && (result.err as any).code === 'ENOENT') {
          try { outputChannel.appendLine(`Configured CLI not found: ${cliPath} — attempting 'go run ./cmd/cadence-linter' fallback`); } catch (e) {}
          const goArgs = ['run', './cmd/cadence-linter', '--format', 'json', '--rules', rulesPath].concat(extraArgs);
          if (targetUri) goArgs.push(targetUri.fsPath); else goArgs.push(cwd);
          result = await spawnAndCollect('go', goArgs);
        }

        if (result.err && (result.err as any).code !== 'ENOENT') {
          const msg = result.stderr || (result.err && (result.err as any).message) || String(result.err);
          vscode.window.showErrorMessage('cadence-workflow-linter failed: ' + msg);
          try { outputChannel.appendLine('Error: ' + msg); } catch (e) {}
          resolve();
          return;
        }

        if (result.code !== 0) {
          try { outputChannel.appendLine(`cadence-workflow-linter exited with code ${result.code}`); } catch (e) {}
        }

        try {
          const issues: CliIssue[] = JSON.parse(result.stdout || '[]');
          publishDiagnostics(issues);
        } catch (e) {
          const msg = e && typeof e === 'object' && 'message' in e ? (e as any).message : String(e);
          vscode.window.showErrorMessage('Failed to parse cadence-workflow-linter output: ' + msg);
          try { outputChannel.appendLine('Failed to parse output: ' + msg); outputChannel.appendLine('STDOUT: ' + result.stdout); outputChannel.appendLine('STDERR: ' + result.stderr); } catch (e) {}
        }
        resolve();
      })();
    });
  });
}

export function publishDiagnostics(issues: CliIssue[]) {
  try { diagnosticCollection.clear(); } catch (e) {}
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders) return;
  const cwd = workspaceFolders[0].uri.fsPath;

  const diagMap: Map<string, vscode.Diagnostic[]> = new Map();
  for (const issue of issues) {
    const filePath = path.isAbsolute(issue.file) ? issue.file : path.join(cwd, issue.file);
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

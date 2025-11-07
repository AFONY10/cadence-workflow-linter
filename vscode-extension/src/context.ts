import * as vscode from 'vscode';

// Shared extension context and channels
export let extContext: vscode.ExtensionContext | undefined;
export let outputChannel: vscode.OutputChannel;
export let diagnosticCollection: vscode.DiagnosticCollection;
export let debounceTimer: NodeJS.Timeout | undefined;
export const DEBOUNCE_MS = 500;
// Keep last-run issues for hover provider
export const lastIssues: Map<string, any[]> = new Map();

export function initContext(ctx: vscode.ExtensionContext, out: vscode.OutputChannel, diag: vscode.DiagnosticCollection) {
  extContext = ctx;
  outputChannel = out;
  diagnosticCollection = diag;
}

export function clearContext() {
  extContext = undefined;
}

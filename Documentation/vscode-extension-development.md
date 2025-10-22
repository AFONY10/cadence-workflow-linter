# VS Code Extension Development: Cadence Workflow Linter

This document explains the current state of the VS Code extension for the Cadence Workflow Linter, how it works today (CLI-wrapper), what npm packages are used and why, the contents of `vscode-extension/src/extension.ts`, how to build and test the extension locally, limitations, and recommended next steps (including moving to an LSP-based design).

## Goal
Provide developers with live diagnostics (errors/warnings) inside VS Code for Cadence/Temporal workflows. The final target UX is:
- Diagnostics are shown as errors/warnings in the editor and Problems pane.
- Diagnostics are updated automatically while coding (on open, on save, and ideally on typing).
- Quick fixes (CodeActions) are available to suppress rules or apply fixes.
- The extension is installable from the VS Code Marketplace.

## Current high-level approach
- We currently use the existing Go-based CLI linter (`cadence-workflow-linter`) as the analysis engine.
- The VS Code extension runs that CLI (spawn) and consumes its JSON output to publish Diagnostics.
- This approach is a pragmatic, lower-effort path to get editor integration quickly. It uses the CLI as a source of truth for rule detection.
- Long term we plan to implement a Language Server (LSP) in Go to provide typed, incremental analysis and on-type diagnostics.

## Installed npm packages (devDependencies) and purpose
Located in `vscode-extension/package.json`:
- `typescript` (^4.9.5): TypeScript compiler used to compile `src/` into `out/` JavaScript that the extension uses.
- `@types/node` (^18.0.0): Node.js type definitions used at compile time for `child_process`, `path`, and other Node APIs.
- `vscode` (^1.1.37): VS Code extension type definitions used for the VS Code API during TypeScript compilation. (Note: this is *types only* and not bundled into the extension runtime.)

These packages are development-only; the shipped extension contains compiled JS only.

## File: `vscode-extension/src/extension.ts` — detailed walkthrough
Below is a function-by-function explanation of the code present in `extension.ts`.

### Top-level imports and types
- `import * as vscode from 'vscode';` — VS Code API.
- `import { spawn } from 'child_process';` — used to spawn the CLI process so stdout/stderr can be streamed.
- `import * as path from 'path';` — used to resolve relative file paths in multi-file workspaces.

- `CliIssue` interface: describes the expected JSON output from the CLI: `file`, `line`, `column`, `rule`, `severity`, `message`.

- Globals:
  - `diagnosticCollection: vscode.DiagnosticCollection` — where diagnostics are stored and cleared.
  - `outputChannel: vscode.OutputChannel` — logs linter stdout/stderr and internal errors.
  - `debounceTimer` and `DEBOUNCE_MS` — used to avoid running the linter too frequently.

### `activate(context)`
- Creates the DiagnosticCollection (`cadence-workflow-linter`) and OutputChannel (`Cadence Workflow Linter`) and registers them for cleanup.
- Registers a command `cadenceLinter.run` which users can execute manually to run the linter.
- Listens to `workspace.onDidSaveTextDocument` and, if `cadenceLinter.runOnSave` is true and the saved document is a Go file, schedules a debounced linter run for that file.
- Schedules an initial run on activation.

### `scheduleRunLinter(uri?)`
- Debounces calls so multiple saves/changes in a short time window result in a single linter invocation.

### `runLinter(targetUri?)`
- Reads configuration `cadenceLinter.cliPath`, `cadenceLinter.args` and determines the workspace `cwd` (first workspace folder).
- Prepares args for the linter (`--format json` plus any user-supplied args) and appends either the `targetUri` path or the workspace root.
- Writes a logging line to the OutputChannel with the command being executed.
- Executes the CLI using `spawn` and wraps it in `withProgress` to show a progress notification while the linter runs.
- Streams `stdout` and `stderr` to the OutputChannel in real time.
- On `close`, attempts to `JSON.parse` stdout into `CliIssue[]` and calls `publishDiagnostics`. If parse fails, shows an error and writes STDOUT/STDERR to the OutputChannel.
- On process `error`, shows an error message.

Notes on `spawn` vs `execFile`:
- `spawn` is used to stream output incrementally and avoid output-size limits.

### `publishDiagnostics(issues)`
- Clears the existing diagnostics and maps CLI issues to VS Code `Diagnostic` objects.
- Resolves file paths: if the CLI returned relative paths, they are joined against the first workspace folder path.
- Creates a `vscode.Range` for each issue (1-line range using the provided line/column) and maps severities (`error`/`warning`/else -> `Information`).
- Publishes diagnostics per-file via `diagnosticCollection.set(uri, diags)`.

### `deactivate()`
- Disposes the diagnostic collection if present.

## Limitations & known issues (current state)
- CLI-only approach is not incremental and can be slow for large workspaces; it runs as a full process and returns a batch of results.
- No LSP means no on-type diagnostics; only on-save, on-open, or manual runs (unless you enable change triggers)
- Multi-root handling is basic: the extension uses the first workspace folder as CWD and resolves relative paths against it.
- No CodeActions implemented yet — no quick-fix or suppression insert features.
- JSON parsing strongly couples the extension to the specific CLI output schema; changes to the CLI must be reflected in the extension code.

## How to build and test locally
1. Build the extension (from `vscode-extension` folder):
```powershell
cd c:\Users\antho\Desktop\bachelor-project-swt\cadence-workflow-linter\vscode-extension
npm ci
npm run compile
```
2. Run the extension in the Extension Development Host:
- Open the `vscode-extension` folder in VS Code and press F5, or
- Use Run > Start Debugging. The extension host will open and `cadenceLinter.run` command should be available and the extension should react to saves.
3. Provide the path to your `cadence-workflow-linter` CLI or build it locally and set `cadenceLinter.cliPath` in Settings.

## Next steps (short-term immediate)
1. Add a CodeAction provider so users can quickly suppress a rule or add a file-level ignore comment. This improves workflow and is fairly small change.
2. Add multi-root support and run per-workspace-folder.
3. Add `onDidOpen` and optional `onDidChange` triggers to refresh diagnostics earlier.
4. Add tests for the extension (unit + integration) and configure GitHub Actions to run compile + tests.

## Long-term (recommended)
- Implement an LSP server in Go that imports or wraps the existing detectors so the server can provide precise typed diagnostics and CodeActions on the fly. The VS Code extension should then use `vscode-languageclient` to connect to that server and only fall back to the CLI-wrapper if the server is not available.

---
If you want, I can implement the next immediate step (add a CodeAction provider) and commit the change. I can also generate the requested documentation file in the repo (this file). Which would you like me to do right now? 

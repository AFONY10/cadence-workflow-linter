# Local installation and configuration — Cadence Workflow Linter (VSIX)

This document explains how to install the locally-built VSIX, how to override settings (user or workspace), how the extension auto-download works, and how to test the extension behavior.

## 1) What the VSIX contains
The VSIX file created (`cadence-workflow-linter-vscode-0.1.0.vsix`) contains:
- `out/` — compiled extension JavaScript (the runtime)
- `resources/` — bundled resources the extension uses at runtime:
  - `default-rules.yaml` — fallback rules the extension will use if the workspace doesn't provide `config/rules.yaml`
  - `releases.json` — manifest mapping platform/arch to expected GitHub Release artifact filenames
  - `icon.png` — extension icon placeholder
- `package.json` — extension metadata, activation events, configuration schema, and `contributes` info
- `README.md` — user-facing documentation included in the VSIX

You do NOT need the source repository present for the installed extension to run — the VSIX installation registers the extension with VS Code.

## 2) Install the VSIX locally (quick)
Open a PowerShell or terminal and run:

```powershell
cd .\vscode-extension
code --install-extension .\cadence-workflow-linter-vscode-0.1.0.vsix
```

Alternatively, in VS Code: Extensions view → `...` → "Install from VSIX..." and pick the `.vsix` file.

After installing, open a new (normal) VS Code window and open the project folder you want to lint.

## 3) How to override settings (User vs Workspace)
You can set configuration either at User scope (applies to all workspaces) or Workspace scope (applies to the opened workspace).

- User settings: File → Preferences → Settings, search for `cadenceLinter`, or edit the User settings.json.
- Workspace settings: create or edit `.vscode/settings.json` inside the workspace root.

Example: User-level settings (powershell command to edit or copy into `%APPDATA%/Code/User/settings.json`):

```json
{
  "cadenceLinter.cliPath": "C:/tools/cadence-linter.exe",
  "cadenceLinter.runOnSave": true,
  "cadenceLinter.args": ["--format", "json"]
}
```

Example: Workspace-level settings (`.vscode/settings.json` in project):

```json
{
  "cadenceLinter.cliPath": "./tools/cadence-linter.exe",
  "cadenceLinter.runOnSave": false
}
```

Notes:
- `cadenceLinter.cliPath` accepts absolute or relative paths (relative resolved against the workspace root).
- If `cadenceLinter.cliPath` is set, the extension will use that binary and will not attempt to auto-download.

## 4) How the extension discovers and (optionally) downloads the CLI
When running, the extension follows this order to choose a CLI to run:

1. If user/workspace `cadenceLinter.cliPath` is set and exists, use it.
2. If the extension previously downloaded a CLI (stored in extension global storage), re-use it.
3. Check workspace locations (e.g. `<workspace>/.vscode/.cadence-linter/bin/<artifact>`, `<workspace>/bin/<artifact>`).
4. Probe the system `PATH` for common names (`cadence-linter`, `cadence-workflow-linter`, etc.).
5. If none found, attempt to download the correct binary from GitHub Releases using the bundled `resources/releases.json` manifest and save it to the extension's globalStorage `bin/` directory.

Important: the download step expects a matching artifact to exist in GitHub Releases with the filename exactly as specified in `resources/releases.json`.

## 5) How rules injection works
- The extension always passes an absolute `--rules <path>` when invoking the CLI:
  - If the workspace contains `config/rules.yaml` that path is used.
  - Otherwise the extension copies `resources/default-rules.yaml` to a temp file and passes that absolute path.
- This prevents the "config/rules.yaml not found" error when the CLI is run in an arbitrary workspace.

## 6) Where files are stored (globalStorage and temp)
- Downloaded binary: the extension stores the downloaded binary in the extension global storage. On Windows this is typically:
  `%APPDATA%\Code\User\globalStorage\AFONY10.cadence-workflow-linter-vscode\bin\`
  (replace `AFONY10.cadence-workflow-linter-vscode` with your extension id if you change `publisher` or `name`).
- Copied default rules: a temp file under the OS temp directory (e.g., `%TEMP%` on Windows).

## 7) How to test the extension on a new system (end-to-end)
1. Ensure VSIX is installed: `code --install-extension .\cadence-workflow-linter-vscode-0.1.0.vsix`.
2. Open a new regular VS Code window and open a workspace that contains Go code (or any folder and then open a `.go` file).
3. Open the Output panel: View → Output, then select "Cadence Workflow Linter" in the dropdown. You should see logs describing discovery steps.
4. If the CLI is not on PATH and not configured via `cadenceLinter.cliPath`, the extension will attempt to download the binary from GitHub Releases (see note above). Watch the Output channel for download status.
5. If the CLI runs successfully, the extension will parse JSON and populate Diagnostics (Problems) for files. Hover over the line to see details.

## 8) If auto-download fails (how to debug)
- Check Output channel for errors (download URL, HTTP status, file paths).
- Inspect globalStorage folder to see if a partial file exists.
- Temporarily set `cadenceLinter.cliPath` to a local binary path (pre-built CLI) to skip download.

## 9) Example GitHub Releases artifact names (must match `resources/releases.json`)
Your `resources/releases.json` currently maps artifacts like:
- `cadence-linter-windows-amd64.exe`
- `cadence-linter-darwin-arm64`
- `cadence-linter-darwin-amd64`
- `cadence-linter-linux-amd64`
- `cadence-linter-linux-arm64`

When you create GitHub Releases these files must be uploaded with exactly those names for the extension's auto-download to work.

## 10) Quick note about security and checksums
It's recommended to publish a checksum file (SHA256) for each artifact and modify the extension to verify checksums before running the downloaded binary. We can implement checksum verification in the extension once CI produces checksums.

---

If you want, I can now:
- Scaffold a GitHub Actions workflow that cross-builds these binaries and creates a Release artifacts (so auto-download works), or
- Implement checksum verification in the extension now (if you prefer to handle security first).

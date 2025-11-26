# Cadence Workflow Linter — VS Code Extension

## About
  The Cadence Workflow Linter extension runs the `cadence-workflow-linter` CLI and surfaces issues as VS Code Problems/Diagnostics. The extension is small and ships without binaries: it will prefer a configured CLI binary, then a workspace or PATH binary, and — if none are found — will auto-download the correct platform binary from the repository's GitHub Release.

## Features

  - Problems panel diagnostics for linter findings
  - Output channel with discovery, download, checksum and run logs
  - Hover support for reported issues
  - Commands: run linter and check/download CLI updates

## How to use

  - Logs: View → Output → select "Cadence Workflow Linter" to see discovery, download and run logs.
  - Problems: View → Problems shows all reported issues from the linter.
  - Run on save: toggle `cadenceLinter.runOnSave` (default: true) to run the linter automatically when saving files.
  - Commands (Command Palette):
    - "Cadence Workflow Linter: Run" — run the linter immediately
    - "Cadence Workflow Linter: Check for CLI update" — fetch the release manifest and attempt to download the platform-specific CLI
      - "Cadence Workflow Linter: Export SARIF" — run the linter and save results in SARIF format (for CI/Code Scanning)

Cadence Workflow Linter VS Code Extension

This extension runs the `cadence-workflow-linter` CLI and shows issues as VS Code diagnostics.

Development

1. Install dependencies
   npm install

2. Compile
   npm run compile

3. Launch Extension
   - Press F5 in VS Code to open an Extension Development Host

Configuration
- `cadenceLinter.cliPath`: path to the linter CLI
- `cadenceLinter.runOnSave`: whether to run the linter on save
- `cadenceLinter.args`: extra args passed to the CLI

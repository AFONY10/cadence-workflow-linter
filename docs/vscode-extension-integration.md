VS Code extension integration notes

Goal

Make the VS Code extension able to present linter findings from any adapter (Go, Java, etc.) in the Problems view.

Two supported integration approaches

1) CLI-first (recommended for simplicity)

- The extension executes the top-level CLI (`cmd/cadence-linter`) with a workspace path and `--format sarif` (or JSON). The CLI will:
  - For Go: import the in-process adapter and run it.
  - For other languages: shell out to the adapter CLI and consolidate results.
- The extension parses SARIF and populates VS Code Diagnostics (Problems view).
- Advantages: minimal extension code, single command for users, language adapters can be implemented in any language.

2) Adapter-first (embedded adapter)

- The extension calls language-specific adapters directly (for example, spawn `java -jar adapters/java/analyzer/cli.jar` or run a long-lived adapter process).
- Useful for faster feedback (language server) or when adapters expose LSP.
- More complex to implement and maintain but enables richer UX (incremental analysis, LSP features).

Design recommendations

- Standardize on SARIF or JSON `core.Issue` output. SARIF is preferred because of tool support and richer metadata (rule info, properties).
- The CLI should expose `--format sarif|json` and `--output <path|->` options.
- The extension should parse SARIF into VS Code `Diagnostic` objects and map `severity` to VS Code severities.

Extension workflow (CLI-first)

1. Extension command (e.g., "Run Cadence Linter") invokes the CLI with the current workspace path.
2. CLI returns SARIF JSON to stdout (or to a temp file).
3. Extension parses SARIF and publishes diagnostics for open files.
4. Optionally: Extension stores a cache and refreshes on file save or workspace open.

Mapping fields

- core.Issue.file -> Diagnostic.file
- core.Issue.line/column -> Diagnostic.range
- core.Issue.severity -> DiagnosticSeverity
- core.Issue.message -> Diagnostic.message

Where the CLI lives and Java question

- `cmd/cadence-linter` is the canonical Go CLI entrypoint. It acts as an orchestrator and does not imply `main.java` should live there.
- Language-specific executables (e.g., Java `.jar`) belong to the adapter's folder (e.g., `adapters/java/analyzer/cli/`). The CLI can call them via `exec.Command`.

Quick next steps for the extension

- Implement a small runner command in the extension that calls `cmd/cadence-linter --format sarif <workspace>` and parses SARIF.
- Add a Problems panel consumer that shows issues and supports file navigation.
- Add a command to run the linter on save or via an LSP-backed adapter for incremental analysis.


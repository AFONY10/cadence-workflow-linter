Cadence Workflow Linter — Monorepo

This repository contains a Cadence workflow linter. It has been refactored to
support a multi-language adapter model and a monorepo layout for easier
extension and integration (VS Code extension, server, other language adapters).

This document explains the new layout, migration notes, how to run the CLI and
tests, and how to add new adapters.

Quick summary
- Project now exposes language adapters under `adapters/` (e.g. `adapters/go`).
- A small `core/` package holds the canonical Issue schema, Adapter contract,
  emitters (JSON/YAML/SARIF), and rule catalog. Language-specific analyzers
  implement adapters and detectors.
- The CLI (`cmd/cadence-linter` or top-level `main.go`) imports adapters where
  needed. The current Go adapter is at `adapters/go/analyzer`.

New repository layout (desired final state)

/
- README.md (this file)
- go.mod
- cmd/
  - cadence-linter/ (CLI entrypoint)
    - main.go
- core/                    # language-agnostic interfaces and emitters
  - issue.go
  - adapter.go
  - emit.go
  - sarif.go
  - rules.go
  - configmap.go
- adapters/
  - go/
    - analyzer/            # Go adapter implementation (migrated from analyzer/)
      - scanner.go
      - detectors/
      - registry/
      - modutils/
      - README.md
      - example/
  - typescript/           # (future) TS/JS adapter
  - python/               # (future) Python adapter
- docs/
  - monorepo-structure.md
  - migration.md
- config/
  - rules.yaml
- testdata/
- tests/
- vscode-extension/
  - package.json
  - src/
  - README.md

Why this layout
- core/: central, language-agnostic contract (Issue, Adapter) -> allows
  multiple adapters to produce a consistent output format.
- adapters/: each language gets its own folder. The Go adapter implements
  the current detectors and can be replaced or extended independently.
- cmd/: CLI and optional server/LSP wrappers.
- vscode-extension/: the VS Code client remains separate and consumes SARIF/JSON
  emitted by the CLI or calls adapters directly when running in the workspace.

Migration notes (what changed)
- The original `analyzer/` implementation has been copied into
  `adapters/go/analyzer/` and imports in `main.go` and tests were updated to use
  the adapter path. Tests have been updated accordingly.
- `core/` now contains the canonical `Issue` type and helpers for emitting
  results (JSON/YAML/SARIF). Detectors use `core.Issue` via an alias.
- Detectors now attach `Issue.Callee` where meaningful (function calls,
  goroutine call sites) and `core.ApplyConfigOverrides` prefers `Callee` when
  replacing `%FUNC%` placeholders in configured messages.

How to run (local)

Run the CLI against a directory or file (example):

```powershell
# build
go build -o cadence-linter ./cmd/cadence-linter
# run
./cadence-linter --format json ./testdata
```

Run tests:

```powershell
go test ./...
```

How to add a new adapter (high level)
1. Create `adapters/<lang>/analyzer` and implement the adapter contract from
   `core/adapter.go`.
2. Implement a scanner that produces `core.Issue` outputs or adapter-local
   detectors that alias to `core.Issue`.
3. Add an example under `adapters/<lang>/example` and unit tests under
   `adapters/<lang>/analyzer/tests` or top-level `tests/` referencing the
   adapter.
4. Update the CLI (or add a new cmd) to discover and call the adapter.

VS Code extension guidance
- The existing extension can either call the CLI (recommended for simplicity)
  or call the Go adapter directly when running inside the repository. Prefer
  using SARIF or JSON output so the extension can parse and present results in
  the Problems view.

Next steps I can do for you
- Remove the old `analyzer/` directory to leave a single canonical copy under
  `adapters/go/analyzer` (I can perform the removal and re-run tests).
- Create a migration PR with a succinct commit message and migration notes.
- Wire the VS Code extension to call the CLI and parse SARIF output.

If you'd like, I can now: (A) remove the old `analyzer/` directory, or (B)
produce a migration PR template and commit message. Which should I do next?
# Cadence Static Analyzer (Go)

This is a prototype CLI tool that performs static analysis on Go Cadence workflow code. Its purpose is to detect potentially non-deterministic code that could break workflow replay or versioning.

## Usage
Run following command from root of repository: (format, json)
```bash
go run . --rules config/rules.yaml --format json /path/to/test/folder
```

If you want to get the output in yml-format, you can run this:
```bash
go run . --rules config/rules.yaml --format yml /path/to/test/folder
```

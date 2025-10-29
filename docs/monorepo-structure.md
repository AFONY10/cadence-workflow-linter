Monorepo layout and how adapters work

Overview

This repository has been refactored to a monorepo layout with language-specific adapters. The goals are:
- Keep a small, language-agnostic core contract describing issues and emitters.
- Let language adapters (Go, Java, TypeScript, etc.) implement analysis and produce a canonical output.
- Make it easy to add new adapters without changing core logic or the VS Code extension.

Top-level structure (important folders)

- cmd/
  - cadence-linter/ - CLI entrypoint (Go). Contains `main.go` that wires adapters and outputs results.
- core/
  - Canonical schema and small helpers shared across adapters: `Issue` type, emitters (JSON/YAML/SARIF), and rule catalog.
- adapters/
  - go/analyzer/ - The Go adapter. Scanner, detectors, registry live here.
  - java/analyzer/ - (skeleton) Java adapter lives here.
  - <other> - future adapters can be added under `adapters/<lang>/analyzer`.
- config/ - rules.yaml and loader used by the CLI.
- tests/ - cross-adapter/integration tests (CLI-level). Keep these focused on end-to-end scenarios involving multiple adapters or the CLI.

Adapter contract

- Each adapter should produce core.Issue-compatible outputs. The simplest approach is to emit JSON or SARIF using the same field names as `core.Issue`.
- Adapters should expose a minimal programmatic API (if they are in-process languages like Go) or a thin CLI (if they are out-of-process languages like Java):
  - Go adapter: expose `ScanDirectory` / `ScanFile` functions that return []core.Issue.
  - Java adapter: expose a CLI (jar) that accepts a path and writes JSON/SARIF to stdout.

Why a CLI folder (cmd/) and not a single main at repo root?

- Standard Go projects use `cmd/<appname>` for executables. This keeps the repo root free of binaries and makes it easy to add more CLIs or servers.
- A `cmd/cadence-linter` (Go) is the canonical CLI that can call any adapter. We do not put `main.java` in the same folder. Language-specific executables belong with their adapter (e.g., adapters/java/analyzer/cli or adapters/java/analyzer/build).

How adapters are discovered and invoked

- The top-level Go CLI can be configured to call adapters in two ways:
  1. In-process (Go adapters only): import `adapters/go/analyzer` and call `ScanDirectory`.
  2. Out-of-process (any language): run an adapter CLI (e.g., `java -jar adapters/java/analyzer/cli.jar`) and parse JSON/SARIF output.

- For cross-language consistency, the adapter must produce issues in the `core.Issue` shape (JSON keys matching field names) or SARIF, which the CLI or extension consumes.

File organization for tests and testdata

- Language-specific unit tests and test fixtures should live with the adapter under:
  - adapters/<lang>/tests/unit/...
  - adapters/<lang>/tests/testdata/...

- Keep top-level `tests/` for integration tests that exercise the CLI or multi-adapter scenarios.

Adding a new language adapter (Java example)

1. Create `adapters/java/analyzer/`.
2. Add an example Java project in `adapters/java/analyzer/example/` that contains sample code and tests.
3. Implement an adapter CLI (for example `cli.jar`) that:
   - Accepts a path or file to analyze and a `--format json|sarif` flag.
   - Produces results in the `core.Issue` JSON shape or SARIF.
4. The Go CLI (`cmd/cadence-linter`) can execute the CLI and parse results.

Notes

- Keep the `core` contract stable and language-agnostic. Changes to `core.Issue` should be made carefully and coordinated with adapters and the extension.
- Prefer SARIF or JSON for interop. SARIF has good tooling and integrates easily with VS Code Problem Matchers and other tools.

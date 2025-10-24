Go adapter (adapters/go)

This folder contains the Go language adapter for the linter. It exposes a
stable import path `github.com/afony10/cadence-workflow-linter/adapters/go/analyzer`
so the CLI or other tooling can import a language adapter while the internal
analyzer package is refactored or moved.

Current status: thin wrapper that forwards to the repository `analyzer` package.

Next steps when migrating:
- Move `analyzer/` into this folder (preserve package names where possible).
- Update internal imports to use `github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/...`.
- Keep the `detectors/`, `registry/`, and `modutils/` packages colocated under the adapter.

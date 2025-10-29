How to add a new language adapter (detailed guide)

This guide explains a step-by-step approach to adding a language adapter to the repository.

1. Create adapter skeleton
   - Create `adapters/<lang>/analyzer/`.
   - Add `README.md` describing how the adapter should be built and run.
   - If the language is compiled (Java, C#), add a `cli/` subfolder with build instructions (Maven/Gradle or similar).

2. Implement the analyzer
   - For in-process languages (Go): implement functions like `ScanDirectory(path string, factory Factory) ([]core.Issue, error)` that return `core.Issue` structs directly.
   - For out-of-process languages (Java): implement a CLI that outputs JSON or SARIF with `core.Issue`-like fields.

3. Tests & testdata
   - Add unit tests in `adapters/<lang>/tests/unit/`.
   - Put language-specific fixtures under `adapters/<lang>/tests/testdata/`.
   - Keep integration/e2e tests at top-level under `tests/`.

4. Integration with the CLI
   - For Go adapter: update `cmd/cadence-linter` to import the adapter directly.
   - For non-Go adapters: have the CLI shell out to the adapter (e.g., `java -jar adapters/java/analyzer/cli.jar --format json <path>`) and parse the JSON/SARIF output.

5. Emitting results
   - Adapters must use the `core` contract for issue fields: file, line, column, rule id, severity, message, callee, callstack (where available).
   - Prefer SARIF for rich metadata; JSON is fine for quick integration.

6. Example: Java adapter pattern
   - `adapters/java/analyzer/cli/` contains a small Java CLI that:
     - Accepts: `--format json|sarif`, `--path <dir|file>`
     - Analyzes the given path and writes the results to stdout.
   - The CLI can be run from the Go `cmd/cadence-linter` command using `exec.Command`.

7. Linting & CI
   - Add CI steps to build and test each adapter independently.
   - Ensure `go test ./...` still passes on the repo root.

8. Versioning
   - adapters that are external artifacts (e.g., Java jars) should include a versioning strategy and release notes. Keep adapter artifacts out of the repo `bin/` directory; prefer published artifacts or CI-built artifacts.

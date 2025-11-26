# Cadence Workflow Linter

About
-----
The Cadence Workflow Linter is a lightweight static analysis tool for Cadence
workflows written in Go. It detects patterns that may break workflow replay or
cause non-deterministic behavior and emits findings in machine-friendly
formats (JSON, YAML, SARIF) so results can be consumed by editors, CI, and
code-scanning systems.

Why it is useful
- Helps find common non-deterministic code (e.g. time/rand/io/goroutines) that
   can break Cadence workflow replay.
- Produces SARIF output which integrates with CI and code-scanning tools.
- Includes a small VS Code extension to surface problems directly in the
   editor.

Usage
-----
**CLI (run locally)**

From the repository root you can either run the linter directly with `go run`
or build a native binary first.

Run without building (developer quick run):

```powershell
go run ./cmd/cadence-linter --format json path\to\your\project
```

Build and run the binary (recommended for testing the packaged behavior):

```powershell
go build -o cadence-linter.exe ./cmd/cadence-linter
.\cadence-linter.exe --format json path\to\your\project
```

Produce SARIF for CI / code scanning:

```powershell
.\cadence-linter.exe --format sarif path\to\your\project > results.sarif
```

Run tests for the repository:

```powershell
go test ./...
```

**VS Code extension (install from a GitHub Release VSIX)**

The extension is available under `vscode-extension/`. To install a packaged
VSIX from Releases:

1. Download the `.vsix` file from this repository's Releases page.
2. In VS Code: Extensions view → menu (⋯) → "Install from VSIX..." → select the
   downloaded file.
3. Reload VS Code and open the Output panel -> select "Cadence Workflow Linter"
   to see activation logs.

Annotated steps (screenshots)

1) Open the Release
![](Documentation/vscode-extension/images/image.png)

2) Download the VSIX from Assets
![](Documentation/vscode-extension/images/image-1.png)

3) Install from VSIX in VS Code
![](Documentation/vscode-extension/images/image-2.png)

4) Reload & verify
![](Documentation/vscode-extension/images/image-3.png)

Alternative option:
You can produce and build a .vsix file locally by running this:

  ```powershell
  cd vscode-extension
  npx vsce package --no-dependencies
  # produces a cadence-workflow-linter-vscode-<version>.vsix file
  ```

Repository layout (important folders)
----------
- `cmd/` — CLI entrypoint (`cmd/cadence-linter`).
- `core/` — shared types and emitters (Issue schema, SARIF/JSON/YAML emitters).
- `adapters/go/` — Go analyzer and detectors.
- `vscode-extension/` — VS Code client that runs the CLI and publishes
  diagnostics.

More docs
----------
- Documentation for entire system: 
  `Documentation/`.
- Extension-specific guidance: `vscode-extension/README.md`



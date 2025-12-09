# Cadence Workflow Linter — Overview

Purpose
-------
The Cadence Workflow Linter is a lightweight static-analysis tool that flags
non-deterministic or unsafe operations when they appear in Cadence workflow code.
It aims to help developers keep workflows replay-safe while avoiding false positives
in activity code.

High-level design (short)
-------------------------
- CLI boundary: a small command-line program (or VS Code extension) invokes the analyzer.
- Analyzer pipeline: Scanner → Workflow Registry (call graph) → Detectors → Reporter.
- Detectors: small, focused rules that inspect AST nodes and consult the registry to
  determine whether a call is reachable from a workflow.
- Configuration: rules are defined in `config/rules.yaml` so behaviour can be tuned
  without recompilation.

Why this approach
------------------
- Minimal surface area: a thin CLI keeps integration simple.
- Workflow-aware detection: only report when code is reachable from workflows,
  which reduces false positives.
- Extensible: detectors are independent and can be added or tuned via config.

Quick diagram
-------------
```mermaid
flowchart LR
  CLI[CLI / VS Code] --> Analyzer[Analyzer]
  Analyzer --> Scanner[Scanner / Parser]
  Scanner --> Registry[Workflow Registry / Call Graph]
  Registry --> Detectors[Detectors (rules)]
  Detectors --> Reporter[Reporter (JSON/SARIF)]
  Reporter --> CLI
```

Where to look in the repo
-------------------------
- `cmd/` — CLI entrypoint
- `adapters/go/analyzer/` — scanner, registry, detectors (Go adapter)
- `config/rules.yaml` — rule definitions and messages
- `vscode-extension/` — the extension that uses the CLI and shows diagnostics

If you want more detail
----------------------
Detailed design and implementation notes are archived in `Documentation/archive/`.
Keep the overview short and check the code for precise implementation details.

📑 Architectural Overview of the Cadence Workflow Linter
1. Purpose of the Tool

The Cadence Workflow Linter is a static analysis tool developed to identify nondeterministic or unsafe operations in Uber Cadence workflows.
Its main goal is to ensure that workflow code remains deterministic, reliable, and production-safe, while still allowing activities to use standard Go functions without false positives.

The linter is designed to be:

Extensible: new detectors (rules) can be added without modifying core logic.

Configurable: rules and severities are defined in a rules.yaml file.

Accurate: uses workflow context analysis and call-graph tracking to avoid false positives in activity code.

2. High-Level Architecture

The tool follows a modular architecture inspired by compilers and linters (like ESLint, golangci-lint).

flowchart TD
    A[CLI entrypoint: main.go] --> B[Analyzer]
    B --> C[Scanner]
    C --> D[Workflow Registry & Call Graph]
    C --> E[Detectors]
    E -->|rules.yaml| F[Config Loader]

    D --> E
    E --> G[Issues]
    G --> A

3. Component Descriptions
1. CLI Entrypoint (main.go)

Reads command-line arguments (--rules, --format, <path>).

Calls the Analyzer to run checks on files or directories.

Outputs results in json or yaml format.

Example:

go run . --rules config/rules.yaml ./testdata

2. Analyzer (analyzer/scanner.go)

Coordinates the entire analysis process.

For each Go file:

Parses source into an AST (Abstract Syntax Tree).

Walks the AST to register workflows in the Workflow Registry.

Executes each Detector on the AST, passing the workflow context.

Can scan single files or entire directories.

3. Workflow Registry (analyzer/registry/)

Keeps track of workflow functions and call graphs.

Detects:

Functions explicitly defined as workflows (func(ctx workflow.Context, ...)).

Helper functions reachable from workflows (recursively).

This ensures violations are flagged only in workflow code, avoiding false positives in activities.

4. Detectors (analyzer/detectors/)

Each detector implements a single linting rule, e.g.:

TimeUsageDetector: flags time.Now() or time.Sleep() inside workflows.

RandomnessDetector: flags math/rand calls inside workflows.

IOCallsDetector: flags fmt.Println, os.Open, file/network operations.

ConcurrencyDetectors: flags go statements and make(chan) inside workflows.

All detectors implement a shared interface (Detector) and can be dynamically extended.

5. Config Loader (config/loader.go)

Loads rules and severities from rules.yaml.

Example rule:

rules:
  - name: TimeUsage
    severity: error
    message: "Detected time.Now() in workflow. Use workflow.Now(ctx) instead."


This design decouples rules definition from detection logic, making the tool more maintainable and adaptable.

6. Tests (tests/)

Unit tests verify each detector individually.

End-to-end tests run on testdata/ source files.

Recent additions:

Tests for helper functions (workflow calling external functions).

Multi-file tests ensuring detection works across imports.

7. Testdata (testdata/)

Contains small Go programs designed to trigger specific violations:

time_violation.go → tests time.Now() in workflows.

rand_violation.go → tests math/rand.

io_violation.go → tests fmt.Println and os.Open.

cadence_workshop_test.go → extended test from Cadence workshop project, simulating real workflow code.

4. Design Principles Applied

Single Responsibility: registry, detectors, config, and scanner are independent.

Open/Closed Principle: new detectors can be added without modifying the scanner.

Config-driven: rules can evolve without code changes.

Scalability: designed for future IDE integration (VS Code, Cursor).

5. Current Limitations

Helper detection works for intra-project Go functions but not yet across packages.

Imports (math/rand, time) currently flagged at file level, even if mixed workflow/activity code.

No call stack in issue output (planned).

6. Next Steps

Finalize fixes for activity false positives.

Extend call-graph analysis to catch helpers in different files/packages.

Improve UX with call stack trace in reported issues.

Prepare IDE integration (long-term) for VS Code/Cursor.

Explore Protobuf/gRPC for future multi-language linting.
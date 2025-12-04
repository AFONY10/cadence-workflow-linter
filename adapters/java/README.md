Java adapter (adapters/java)

This is a minimal sample Java adapter for the cadence-workflow-linter project.

It provides a tiny, demonstration `ScanFile` / `ScanDirectory` API that returns
the language-agnostic `core.Issue` items so the CLI or other tooling can
consume results the same way as the Go adapter does.

This adapter is intentionally small: it performs a simple text scan for
calls that look like `Instant.now(` or `System.currentTimeMillis(` and emits
an issue for each occurrence. It is meant to be a starting point for a full
Java analyzer implementation.

Usage (example):

  import "github.com/afony10/cadence-workflow-linter/adapters/java/analyzer"

  issues, err := analyzer.ScanDirectory("./my-java-project")

The returned `[]core.Issue` can be passed to the existing emitters.

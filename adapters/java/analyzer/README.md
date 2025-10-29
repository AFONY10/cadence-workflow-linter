Java adapter (skeleton)

This folder is a placeholder showing how a Java adapter would be organized.
It does not contain a full implementation — just a template and notes.

Structure suggestion:
- adapters/java/analyzer/
  - README.md (this file)
  - example/ (Java sample projects used for tests)
  - src/ (adapter implementation, e.g. a small CLI wrapper or a jar)
  - build.gradle or pom.xml (build script if needed)

Integration notes:
- Java adapters should emit the canonical `core.Issue` format as JSON/SARIF so the CLI and VS Code extension can consume results.
- Prefer a thin CLI wrapper (Java -> JSON) and call it from the top-level CLI or extension.

Java adapter (adapters/java)


This adapter performs light-weight static analysis of Java sources. It:

- Recognises classes and methods and looks for Cadence/Temporal annotations such as
  `@WorkflowMethod` and `@WorkflowInterface`.
- Builds a simple intra-file call graph and computes reachability from methods
  marked as workflow entry points.
- Flags uses of `java.time.Instant.now()` and `System.currentTimeMillis()` when they
  appear in methods reachable from workflow entry points (including helper methods).

Limitations
-----------
- This is a pragmatic, file-scoped analyzer (no cross-file or cross-package call graph).
- It uses regex-based parsing and heuristics; a future improvement is an AST-based
  Java parser (tree-sitter or javaparser) for more accurate detection.

The tests under `adapters/java/tests/testdata/` include a `WorkflowExample.java` which
demonstrates detection in a helper method called from a workflow method.


Usage (example):

  import (
      "github.com/afony10/cadence-workflow-linter/adapters/java/analyzer"
      "github.com/afony10/cadence-workflow-linter/config"
  )

  rules, _ := config.LoadRules("config/rules.yaml")
  issues, err := analyzer.ScanDirectory("./my-java-project", rules)

The returned `[]core.Issue` can be passed to the existing emitters. Note that the
CLI already loads `config/rules.yaml` and passes the shared RuleSet into the Java
adapter, ensuring top-level configuration changes apply to all adapters.

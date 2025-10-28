package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/config"
)

func TestMapIterationDetector(t *testing.T) {
	tempDir := t.TempDir()
	f := filepath.Join(tempDir, "m.go")
	code := `package test

func Foo() {
	m := map[string]int{"a":1, "b":2}
	for k := range m { // nondeterministic
		_, _ = k, m[k]
	}

	for _, v := range someMap { // assume someMap variable
		_ = v
	}

	for k := range map[int]string{1:"a"} { // literal map
		_ = k
	}
}`
	err := os.WriteFile(f, []byte(code), 0644)
	if err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	src, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, f, src, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Build detector
	rules, _ := config.LoadRules("../config/rules.yaml")
	d := detectors.NewMapIterationDetector(rules.FunctionCalls)
	// Provide file context
	d.SetFileContext(detectors.FileContext{File: f, Fset: fset, ImportMap: map[string]string{}})
	// Walk
	ast.Walk(d, node)
	issues := d.Issues()
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 map iteration issues, got %d", len(issues))
	}
}

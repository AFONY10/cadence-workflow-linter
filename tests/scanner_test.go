package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

// --- helpers ---------------------------------------------------------------

func parse(t *testing.T, rel string) (*token.FileSet, *ast.File, string) {
	t.Helper()
	filename := "../testdata/" + rel

	src, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return fset, node, filename
}

func importMapFromFile(node *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			if i := strings.LastIndex(path, "/"); i >= 0 {
				alias = path[i+1:]
			} else {
				alias = path
			}
		}
		m[alias] = path
	}
	return m
}

func walkOnce(t *testing.T, v ast.Visitor, fset *token.FileSet, node *ast.File, filename string) []detectors.Issue {
	t.Helper()

	reg := registry.NewWorkflowRegistry() // keep current (non-config) registry
	ast.Walk(reg, node)

	if wa, ok := v.(detectors.WorkflowAware); ok {
		wa.SetWorkflowRegistry(reg)
	}
	if fca, ok := v.(detectors.FileContextAware); ok {
		fca.SetFileContext(detectors.FileContext{
			File:      filename,
			Fset:      fset,
			ImportMap: importMapFromFile(node),
		})
	}

	ast.Walk(v, node)

	if ip, ok := v.(detectors.IssueProvider); ok {
		return ip.Issues()
	}
	return nil
}

// --- tests -----------------------------------------------------------------

func TestFuncCallDetector_TimeUsage(t *testing.T) {
	rules, err := config.LoadRules("../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	fset, node, file := parse(t, "time_violation.go")
	d := detectors.NewFuncCallDetector(rules.FunctionCalls)
	issues := walkOnce(t, d, fset, node, file)
	if len(issues) == 0 {
		t.Fatalf("expected at least one TimeUsage issue in %s", file)
	}
}

func TestFuncCallDetector_Randomness(t *testing.T) {
	rules, err := config.LoadRules("../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	fset, node, file := parse(t, "rand_violation.go")
	d := detectors.NewFuncCallDetector(rules.FunctionCalls)
	issues := walkOnce(t, d, fset, node, file)
	if len(issues) == 0 {
		t.Fatalf("expected at least one Randomness issue in %s", file)
	}
}

func TestFuncCallDetector_IOCalls(t *testing.T) {
	rules, err := config.LoadRules("../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	fset, node, file := parse(t, "io_violation.go")
	d := detectors.NewFuncCallDetector(rules.FunctionCalls)
	issues := walkOnce(t, d, fset, node, file)
	if len(issues) == 0 {
		t.Fatalf("expected at least one IOCalls issue in %s", file)
	}
}

func TestGoroutineDetector(t *testing.T) {
	fset, node, file := parse(t, "goroutine_violation.go")
	d := detectors.NewGoroutineDetector()
	issues := walkOnce(t, d, fset, node, file)
	if len(issues) == 0 {
		t.Fatalf("expected at least one goroutine issue in %s", file)
	}
}

func TestChannelDetector(t *testing.T) {
	fset, node, file := parse(t, "channel_violation.go")
	d := detectors.NewChannelDetector()
	issues := walkOnce(t, d, fset, node, file)
	if len(issues) == 0 {
		t.Fatalf("expected at least one channel issue in %s", file)
	}
}

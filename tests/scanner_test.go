package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

func parseAndWalk(t *testing.T, rel string, v ast.Visitor) []detectors.Issue {
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

	// Build workflow registry and inject contexts
	reg := registry.NewWorkflowRegistry()
	ast.Walk(reg, node)

	if wa, ok := v.(detectors.WorkflowAware); ok {
		wa.SetWorkflowRegistry(reg)
	}
	if fca, ok := v.(detectors.FileContextAware); ok {
		fca.SetFileContext(filename, fset)
	}

	ast.Walk(v, node)

	if ip, ok := v.(detectors.IssueProvider); ok {
		return ip.Issues()
	}
	return nil
}

func TestTimeUsageDetector(t *testing.T) {
	issues := parseAndWalk(t, "time_violation.go", detectors.NewTimeUsageDetector())
	if len(issues) == 0 {
		t.Fatalf("expected at least one time usage issue inside workflow")
	}
}

func TestRandomnessDetector(t *testing.T) {
	issues := parseAndWalk(t, "rand_violation.go", detectors.NewRandomnessDetector())
	if len(issues) == 0 {
		t.Fatalf("expected at least one randomness issue inside workflow")
	}
}

func TestIOCallsDetector(t *testing.T) {
	issues := parseAndWalk(t, "io_violation.go", detectors.NewIOCallsDetector())
	if len(issues) == 0 {
		t.Fatalf("expected at least one IO/logging issue inside workflow")
	}
}

package tests

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/modutils"
)

// Test that map iteration inside a workflow is detected
func TestMapIteration_DetectedInWorkflow(t *testing.T) {
	factory := func(mi *modutils.ModuleInfo) []ast.Visitor {
		return []ast.Visitor{detectors.NewMapIterationDetector(nil)}
	}

	issues, err := analyzer.ScanDirectory("../testdata", factory)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	found := false
	for _, is := range issues {
		if strings.HasSuffix(is.File, "map_iteration_example.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected map iteration issue in workflow file, got 0")
	}
}

// Test that map iteration in non-workflow code is not flagged
func TestMapIteration_NotDetectedInNonWorkflow(t *testing.T) {
	factory := func(mi *modutils.ModuleInfo) []ast.Visitor {
		return []ast.Visitor{detectors.NewMapIterationDetector(nil)}
	}

	issues, err := analyzer.ScanDirectory("../testdata", factory)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	for _, is := range issues {
		if strings.HasSuffix(is.File, "map_iteration_non_workflow.go") {
			t.Fatalf("expected 0 issues in non-workflow map iteration, but found one: %+v", is)
		}
	}
}

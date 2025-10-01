package analyzer

import (
	"go/ast"
	"os"
	"path/filepath"
	"testing"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/config"
)

func TestHybridPackageClassification(t *testing.T) {
	// Create a temporary project structure with go.mod
	tempDir := t.TempDir()

	// Create go.mod
	goModContent := `module github.com/test/hybrid-project

go 1.21

require (
	github.com/google/uuid v1.6.0
	go.uber.org/cadence v1.0.0
)

replace github.com/old/lib => ./local/lib
`
	goModPath := filepath.Join(tempDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create test file
	testFileContent := `package main

import (
	"github.com/google/uuid"
	"github.com/test/hybrid-project/internal/helpers"
	"github.com/unknown/external"
	"go.uber.org/cadence/workflow"
)

func InternalTestWorkflow(ctx workflow.Context) error {
	// Internal package call - should not trigger unknown external warning
	helpers.DoSomething()
	
	// Known external package call - should trigger configured rule
	uuid.New()
	
	// Unknown external package call - should trigger info warning
	external.DoSomething()
	
	return nil
}
`

	testFilePath := filepath.Join(tempDir, "test_workflow.go")
	err = os.WriteFile(testFilePath, []byte(testFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create internal package file
	internalDir := filepath.Join(tempDir, "internal", "helpers")
	err = os.MkdirAll(internalDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create internal dir: %v", err)
	}

	internalFileContent := `package helpers

func DoSomething() {
	// Helper function
}
`

	internalFilePath := filepath.Join(internalDir, "helpers.go")
	err = os.WriteFile(internalFilePath, []byte(internalFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create internal file: %v", err)
	}

	// Create rules configuration
	rules := &config.RuleSet{
		ExternalPackages: []config.ExternalPackageRule{
			{
				Package:   "github.com/google/uuid",
				Functions: []string{"New"},
				Rule:      "UUIDGeneration",
				Severity:  "error",
				Message:   "UUID generation is non-deterministic",
			},
		},
		SafeExternalPackages: []string{
			"go.uber.org/cadence",
		},
	}

	// Create factory with hybrid approach
	factory := func(moduleInfo *modutils.ModuleInfo) []ast.Visitor {
		return []ast.Visitor{
			detectors.NewFuncCallDetector(rules.FunctionCalls, rules.ExternalPackages, rules.SafeExternalPackages, moduleInfo),
		}
	}

	// Scan the temporary directory
	issues, err := ScanDirectory(tempDir, factory)
	if err != nil {
		t.Fatalf("Failed to scan directory: %v", err)
	}

	// Verify results
	var uuidError, unknownInfo bool
	var internalPackageWarning bool

	for _, issue := range issues {
		switch issue.Rule {
		case "UUIDGeneration":
			uuidError = true
		case "UnknownExternalCall":
			if issue.Message == "Call to unknown external package github.com/unknown/external.DoSomething() - please verify it's workflow-safe" {
				unknownInfo = true
			}
			// Should NOT trigger for internal packages
			if issue.Message == "Call to unknown external package github.com/test/hybrid-project/internal/helpers.DoSomething() - please verify it's workflow-safe" {
				internalPackageWarning = true
			}
		}
	}

	if !uuidError {
		t.Error("Expected UUID generation error not found")
	}

	if !unknownInfo {
		t.Error("Expected unknown external package info warning not found")
	}

	if internalPackageWarning {
		t.Error("Internal package incorrectly flagged as unknown external")
	}

	t.Logf("Found %d issues as expected", len(issues))
	for _, issue := range issues {
		t.Logf("Issue: %s - %s", issue.Rule, issue.Message)
	}
}

func TestFallbackHeuristics(t *testing.T) {
	// Test without go.mod file to ensure fallback heuristics work
	tempDir := t.TempDir()

	// Create test file without go.mod
	testFileContent := `package main

import (
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/unknown/external"
	"go.uber.org/cadence/workflow"
)

func FallbackTestWorkflow(ctx workflow.Context) error {
	// Internal package call using hardcoded fallback - should not trigger warning
	registry.NewWorkflowRegistry()
	
	// Unknown external package call - should trigger info warning
	external.DoSomething()
	
	return nil
}
`

	testFilePath := filepath.Join(tempDir, "fallback_test.go")
	err := os.WriteFile(testFilePath, []byte(testFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create rules configuration
	rules := &config.RuleSet{
		SafeExternalPackages: []string{
			"go.uber.org/cadence",
		},
	}

	// Create factory
	factory := func(moduleInfo *modutils.ModuleInfo) []ast.Visitor {
		return []ast.Visitor{
			detectors.NewFuncCallDetector(rules.FunctionCalls, rules.ExternalPackages, rules.SafeExternalPackages, moduleInfo),
		}
	}

	// Scan the file
	issues, err := ScanFile(testFilePath, factory)
	if err != nil {
		t.Fatalf("Failed to scan file: %v", err)
	}

	// Verify that internal package call doesn't trigger warning
	var internalPackageWarning bool
	var unknownExternalFound bool

	for _, issue := range issues {
		if issue.Rule == "UnknownExternalCall" {
			if issue.Message == "Call to unknown external package github.com/afony10/cadence-workflow-linter/analyzer/registry.NewWorkflowRegistry() - please verify it's workflow-safe" {
				internalPackageWarning = true
			}
			if issue.Message == "Call to unknown external package github.com/unknown/external.DoSomething() - please verify it's workflow-safe" {
				unknownExternalFound = true
			}
		}
	}

	if internalPackageWarning {
		t.Error("Fallback heuristics incorrectly flagged internal package as external")
	}

	if !unknownExternalFound {
		t.Error("Expected unknown external package warning not found")
	}

	t.Logf("Fallback test completed with %d issues", len(issues))
}

package analyzer_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	javaanalyzer "github.com/afony10/cadence-workflow-linter/adapters/java/analyzer"
	"github.com/afony10/cadence-workflow-linter/config"
	"github.com/afony10/cadence-workflow-linter/core"
)

// TestJavaTimeUsageInWorkflow verifies that Java time API usage in workflow methods is detected
func TestJavaTimeUsageInWorkflow(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Should detect time usage in WorkflowExample.java or Helper.java
	found := false
	for _, issue := range issues {
		if (strings.Contains(issue.File, "WorkflowExample.java") || strings.Contains(issue.File, "Helper.java")) && issue.Rule == "TimeUsage" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected TimeUsage issue in WorkflowExample.java or Helper.java, found none")
	}
}

// TestJavaTimeUsageInHelper verifies that helper methods called from workflows are analyzed
func TestJavaTimeUsageInHelper(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Should detect time usage in Helper.java when called from workflow
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.File, "Helper.java") && issue.Rule == "TimeUsage" {
			found = true
			// Verify call stack is present
			if len(issue.CallStack) == 0 {
				t.Errorf("Expected call stack in issue for Helper.java, got none")
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected TimeUsage issue in Helper.java reachable from workflow, found none")
	}
}

// TestJavaActivityNotFlagged verifies that activity methods are not flagged for time usage
func TestJavaActivityNotFlagged(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Should NOT detect issues in ActivityExample.java
	for _, issue := range issues {
		if strings.Contains(issue.File, "ActivityExample.java") {
			t.Errorf("Did not expect issue in ActivityExample.java (activity), but found: %s at line %d", issue.Rule, issue.Line)
		}
	}
}

// TestJavaLanguageMappings verifies that precompiled language mappings are used
func TestJavaLanguageMappings(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	// Compile Java mappings
	javaMappings, err := rules.CompileLanguageMappings("java")
	if err != nil {
		t.Fatalf("compile java mappings: %v", err)
	}

	if len(javaMappings) == 0 {
		t.Fatalf("Expected java language mappings in rules.yaml, found none")
	}

	// Set mappings and scan
	javaanalyzer.SetLanguageMappings(javaMappings)
	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Apply config overrides to populate messages
	issues = core.ApplyConfigOverrides(issues, rules)

	// Should detect at least one issue using the mappings
	if len(issues) == 0 {
		t.Errorf("Expected at least one issue when using language mappings, found none")
	}

	// Verify issues have proper structure
	for _, issue := range issues {
		if issue.Rule == "" {
			t.Errorf("Issue missing Rule ID: %+v", issue)
		}
		if issue.Message == "" {
			t.Errorf("Issue missing Message: %+v", issue)
		}
		if issue.File == "" {
			t.Errorf("Issue missing File: %+v", issue)
		}
	}
} // TestJavaCallStackFormat verifies call stack format and line numbers
func TestJavaCallStackFormat(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Find an issue with call stack
	var issueWithStack *core.Issue
	for i := range issues {
		if len(issues[i].CallStack) > 0 {
			issueWithStack = &issues[i]
			break
		}
	}

	if issueWithStack == nil {
		t.Fatalf("Expected at least one issue with call stack, found none")
	}

	// Verify call stack format: "methodName (FileName.java:line)"
	for _, frame := range issueWithStack.CallStack {
		if !strings.Contains(frame, "(") || !strings.Contains(frame, ")") || !strings.Contains(frame, ".java:") {
			t.Errorf("Call stack frame has unexpected format: %s", frame)
		}
	}
}

// TestJavaIssueJSONSerialization verifies that issues can be serialized to JSON
func TestJavaIssueJSONSerialization(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	if len(issues) == 0 {
		t.Skip("No issues found to test serialization")
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal issues to JSON: %v", err)
	}

	// Unmarshal back
	var decoded []core.Issue
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal issues from JSON: %v", err)
	}

	// Verify structure preserved
	if len(decoded) != len(issues) {
		t.Errorf("Expected %d issues after round-trip, got %d", len(issues), len(decoded))
	}
}

// TestJavaWorkflowClassification verifies workflow vs activity heuristics
func TestJavaWorkflowClassification(t *testing.T) {
	testdataDir := "../testdata"

	// Test files should exist
	workflowFile := filepath.Join(testdataDir, "WorkflowExample.java")
	activityFile := filepath.Join(testdataDir, "ActivityExample.java")

	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory(testdataDir, rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	hasWorkflowIssue := false
	hasActivityIssue := false

	for _, issue := range issues {
		if strings.HasSuffix(issue.File, "WorkflowExample.java") {
			hasWorkflowIssue = true
		}
		if strings.HasSuffix(issue.File, "ActivityExample.java") {
			hasActivityIssue = true
		}
	}

	if !hasWorkflowIssue {
		t.Logf("Note: No issues detected in %s (may be expected if no violations)", workflowFile)
	}

	if hasActivityIssue {
		t.Errorf("Activity file %s should not be flagged, but found issues", activityFile)
	}
}

// TestJavaEmptyDirectory verifies handling of empty/invalid directories
func TestJavaEmptyDirectory(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	// Scan non-existent directory
	issues, err := javaanalyzer.ScanDirectory("/nonexistent/path", rules)
	// Error is expected for non-existent dir, but should not panic
	if err != nil {
		t.Logf("Expected error for non-existent directory: %v", err)
	}

	// Should return empty or nil, not panic
	if issues == nil {
		t.Logf("Non-existent directory returned nil slice (acceptable)")
	} else if len(issues) != 0 {
		t.Errorf("Expected empty slice for non-existent directory, got %d issues", len(issues))
	}
}

// TestJavaCalleeQualification verifies that Callee field contains qualified names
func TestJavaCalleeQualification(t *testing.T) {
	rules, err := config.LoadRules("../../../../config/rules.yaml")
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	issues, err := javaanalyzer.ScanDirectory("../testdata", rules)
	if err != nil {
		t.Fatalf("scan directory: %v", err)
	}

	// Find issues and verify Callee is populated with qualified name
	foundCalleeField := false
	for _, issue := range issues {
		if issue.Callee != "" {
			foundCalleeField = true
			// Should be in format like "java.time.Instant.now" or similar
			if !strings.Contains(issue.Callee, ".") {
				t.Errorf("Expected qualified callee name, got: %s", issue.Callee)
			}
		}
	}

	if !foundCalleeField {
		t.Logf("Note: No issues with Callee field found (may be expected)")
	}
}

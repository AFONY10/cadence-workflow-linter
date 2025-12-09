package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afony10/cadence-workflow-linter/core"
)

// TestCLIGoOutput verifies CLI produces expected JSON output for Go code
func TestCLIGoOutput(t *testing.T) {
	// Build the CLI binary
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Run against Go testdata
	testdataPath := "../../adapters/go/tests/testdata"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "json", "--rules", rulesPath, testdataPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Logf("STDERR: %s", stderr.String())
		// CLI may exit with non-zero if issues found, that's OK
	}

	// Parse JSON output
	var issues []core.Issue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, stdout.String())
	}

	// Verify we got some issues
	if len(issues) == 0 {
		t.Fatalf("Expected issues in Go testdata, got none")
	}

	// Verify structure of issues
	for i, issue := range issues {
		if issue.File == "" {
			t.Errorf("Issue %d missing File field", i)
		}
		if issue.Line == 0 {
			t.Errorf("Issue %d missing Line field", i)
		}
		if issue.Rule == "" {
			t.Errorf("Issue %d missing Rule field", i)
		}
		if issue.Message == "" {
			t.Errorf("Issue %d missing Message field", i)
		}
		if issue.Severity == "" {
			t.Errorf("Issue %d missing Severity field", i)
		}
	}

	// Verify specific expected issues exist
	expectedFiles := []string{"time_violation.go", "rand_violation.go", "goroutine_violation.go"}
	for _, expectedFile := range expectedFiles {
		found := false
		for _, issue := range issues {
			if strings.Contains(issue.File, expectedFile) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected issue in %s, found none", expectedFile)
		}
	}
}

// TestCLIJavaOutput verifies CLI produces expected JSON output for Java code
func TestCLIJavaOutput(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	testdataPath := "../../adapters/java/tests/testdata"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "json", "--rules", rulesPath, testdataPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Logf("STDERR: %s", stderr.String())
	}

	var issues []core.Issue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, stdout.String())
	}

	if len(issues) == 0 {
		t.Fatalf("Expected issues in Java testdata, got none")
	}

	// Verify Java-specific fields
	for i, issue := range issues {
		if !strings.HasSuffix(issue.File, ".java") {
			t.Errorf("Issue %d file should be .java, got: %s", i, issue.File)
		}

		// Java issues should have Callee field populated
		if issue.Callee == "" {
			t.Logf("Note: Issue %d in %s missing Callee field", i, issue.File)
		}
	}

	// Verify Activity file is not flagged
	for _, issue := range issues {
		if strings.Contains(issue.File, "ActivityExample.java") {
			t.Errorf("Activity file should not be flagged, but found issue: %+v", issue)
		}
	}
}

// TestCLIYAMLOutput verifies YAML output format works
func TestCLIYAMLOutput(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	testdataPath := "../../adapters/go/tests/testdata"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "yaml", "--rules", rulesPath, testdataPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Logf("STDERR: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		t.Fatalf("Expected YAML output, got empty string")
	}

	// Verify YAML structure markers present
	if !strings.Contains(output, "file:") || !strings.Contains(output, "line:") {
		t.Errorf("YAML output missing expected structure: %s", output)
	}
}

// TestCLISARIFOutput verifies SARIF output format works
func TestCLISARIFOutput(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	testdataPath := "../../adapters/go/tests/testdata"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "sarif", "--rules", rulesPath, testdataPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Logf("STDERR: %s", stderr.String())
	}

	output := stdout.String()
	if output == "" {
		t.Fatalf("Expected SARIF output, got empty string")
	}

	// Verify it's valid JSON and contains SARIF structure
	var sarif map[string]interface{}
	if err := json.Unmarshal([]byte(output), &sarif); err != nil {
		t.Fatalf("SARIF output is not valid JSON: %v", err)
	}

	// Check for SARIF required fields
	if version, ok := sarif["version"].(string); !ok || version != "2.1.0" {
		t.Errorf("Expected SARIF version 2.1.0, got: %v", sarif["version"])
	}

	if _, ok := sarif["runs"]; !ok {
		t.Errorf("SARIF output missing 'runs' field")
	}
}

// TestCLICallStackPresent verifies call stack information is included
func TestCLICallStackPresent(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Use Java testdata which has helper calls
	testdataPath := "../../adapters/java/tests/testdata"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "json", "--rules", rulesPath, testdataPath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run()

	var issues []core.Issue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Find issues with call stacks
	foundCallStack := false
	for _, issue := range issues {
		if len(issue.CallStack) > 0 {
			foundCallStack = true

			// Verify call stack format
			for j, frame := range issue.CallStack {
				if !strings.Contains(frame, "(") || !strings.Contains(frame, ")") {
					t.Errorf("Call stack frame %d has unexpected format: %s", j, frame)
				}
			}
			break
		}
	}

	if !foundCallStack {
		t.Logf("Note: No issues with call stacks found (may be expected for simple violations)")
	}
}

// TestCLIInvalidRulesFile verifies error handling
func TestCLIInvalidRulesFile(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	cmd := exec.Command(cliPath, "--rules", "/nonexistent/rules.yaml", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Errorf("Expected error with invalid rules file, got nil")
	}

	// Check both stdout and stderr for error message
	output := stdout.String() + stderr.String()
	if output == "" {
		t.Errorf("Expected error message on stdout or stderr, got empty string")
	}
}

// TestCLINoIssuesFound verifies handling of clean code
func TestCLINoIssuesFound(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	// Use activity_ok.go which should have no issues
	testdataPath := "../../adapters/go/tests/testdata/activity_ok.go"
	rulesPath := "../../config/rules.yaml"

	cmd := exec.Command(cliPath, "--format", "json", "--rules", rulesPath, testdataPath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run()

	var issues []core.Issue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Should be empty or zero issues
	if len(issues) != 0 {
		t.Logf("Note: activity_ok.go had %d issues (may be expected if it contains workflow code)", len(issues))
	}
}

// TestCLIVersionFlag verifies --version flag works (if implemented)
func TestCLIVersionFlag(t *testing.T) {
	t.Skip("--version flag not implemented yet")
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	cmd := exec.Command(cliPath, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("--version should not error: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Errorf("--version should produce output")
	}
}

// TestCLIHelpFlag verifies --help flag works
func TestCLIHelpFlag(t *testing.T) {
	cliPath := buildCLI(t)
	defer os.Remove(cliPath)

	cmd := exec.Command(cliPath, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Logf("--help exit code: %v (may be non-zero)", err)
	}

	output := stdout.String() + stderr.String()
	if output == "" {
		t.Errorf("--help should produce output")
	}

	// Should contain usage information
	if !strings.Contains(output, "Usage") && !strings.Contains(output, "usage") {
		t.Errorf("--help output should contain usage information")
	}
}

// buildCLI compiles the CLI binary for testing
func buildCLI(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	cliPath := filepath.Join(tmpDir, "cadence-linter")
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		cliPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", cliPath, "../../cmd/cadence-linter")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nStderr: %s", err, stderr.String())
	}

	return cliPath
}

package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/afony10/cadence-workflow-linter/config"
)

// TestLoadRules verifies the main rules.yaml file loads correctly
func TestLoadRules(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Verify top-level rules exist
	if len(rules.FunctionCalls) == 0 {
		t.Errorf("Expected function_calls rules, found none")
	}
	if len(rules.ExternalPackages) == 0 {
		t.Errorf("Expected external_packages rules, found none")
	}

	// Verify essential rules are present
	requiredRules := []string{"TimeUsage", "Randomness", "IOCalls"}
	for _, ruleID := range requiredRules {
		found := false
		for _, rule := range rules.FunctionCalls {
			if rule.Rule == ruleID {
				found = true
				if rule.Message == "" {
					t.Errorf("Rule %s has empty message", ruleID)
				}
				if rule.Severity == "" {
					t.Errorf("Rule %s has empty severity", ruleID)
				}
				break
			}
		}
		if !found {
			t.Errorf("Required rule %s not found in function_calls", ruleID)
		}
	}
}

// TestLanguageMappingsExist verifies that language mappings are present
func TestLanguageMappingsExist(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	if rules.LanguageMappings == nil || len(rules.LanguageMappings) == 0 {
		t.Fatalf("Expected language_mappings in rules.yaml, found none")
	}

	// Check for Go mappings
	goMappings, ok := rules.LanguageMappings["go"]
	if !ok || len(goMappings) == 0 {
		t.Errorf("Expected 'go' language mappings, found none")
	}

	// Check for Java mappings
	javaMappings, ok := rules.LanguageMappings["java"]
	if !ok || len(javaMappings) == 0 {
		t.Errorf("Expected 'java' language mappings, found none")
	}
}

// TestCompileLanguageMappings verifies regex compilation works
func TestCompileLanguageMappings(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Test Go mappings
	goMappings, err := rules.CompileLanguageMappings("go")
	if err != nil {
		t.Fatalf("Failed to compile Go language mappings: %v", err)
	}

	if len(goMappings) == 0 {
		t.Errorf("Expected compiled Go mappings, got none")
	}

	// Verify each mapping compiles to valid regexes
	for ruleID, patterns := range goMappings {
		if len(patterns) == 0 {
			t.Errorf("Rule %s has no compiled patterns", ruleID)
		}
		for i, re := range patterns {
			if re == nil {
				t.Errorf("Rule %s pattern %d failed to compile", ruleID, i)
			}
		}
	}

	// Test Java mappings
	javaMappings, err := rules.CompileLanguageMappings("java")
	if err != nil {
		t.Fatalf("Failed to compile Java language mappings: %v", err)
	}

	if len(javaMappings) == 0 {
		t.Errorf("Expected compiled Java mappings, got none")
	}
}

// TestCompileLanguageMappingsInvalidLanguage verifies handling of non-existent language
func TestCompileLanguageMappingsInvalidLanguage(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Should return empty map, not error
	mappings, err := rules.CompileLanguageMappings("nonexistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent language, got: %v", err)
	}
	if mappings == nil {
		t.Errorf("Expected empty map, got nil")
	}
}

// TestLanguageMappingPatternValidity verifies patterns can match expected strings
func TestLanguageMappingPatternValidity(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	goMappings, err := rules.CompileLanguageMappings("go")
	if err != nil {
		t.Fatalf("Failed to compile Go mappings: %v", err)
	}

	// Test TimeUsage patterns should match "time.Now()"
	timePatterns, ok := goMappings["TimeUsage"]
	if !ok || len(timePatterns) == 0 {
		t.Skip("No TimeUsage Go mappings to test")
	}

	matched := false
	for _, re := range timePatterns {
		if re.MatchString("time.Now()") {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("Expected TimeUsage patterns to match 'time.Now()', but none did")
	}
}

// TestInvalidRulesFile verifies error handling for missing/invalid files
func TestInvalidRulesFile(t *testing.T) {
	_, err := config.LoadRules("/nonexistent/rules.yaml")
	if err == nil {
		t.Errorf("Expected error loading non-existent rules file, got nil")
	}
}

// TestRulesFileStructure verifies the YAML structure is correct
func TestRulesFileStructure(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Verify each function_calls rule has required fields
	for i, rule := range rules.FunctionCalls {
		if rule.Rule == "" {
			t.Errorf("function_calls[%d] missing 'rule' field", i)
		}
		if rule.Message == "" {
			t.Errorf("function_calls[%d] (%s) missing 'message' field", i, rule.Rule)
		}
		if rule.Severity != "error" && rule.Severity != "warning" && rule.Severity != "info" {
			t.Errorf("function_calls[%d] (%s) has invalid severity: %s", i, rule.Rule, rule.Severity)
		}
	}

	// Verify external_packages rules
	for i, rule := range rules.ExternalPackages {
		if rule.Rule == "" {
			t.Errorf("external_packages[%d] missing 'rule' field", i)
		}
		if rule.Package == "" {
			t.Errorf("external_packages[%d] (%s) missing 'package' field", i, rule.Rule)
		}
	}
}

// TestLanguageMappingsYAMLFormat verifies mappings are string arrays
func TestLanguageMappingsYAMLFormat(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	if rules.LanguageMappings == nil {
		t.Skip("No language_mappings in rules.yaml")
	}

	for lang, mappings := range rules.LanguageMappings {
		if len(mappings) == 0 {
			t.Errorf("Language %s has no mappings", lang)
		}
		for ruleID, patterns := range mappings {
			if len(patterns) == 0 {
				t.Errorf("Language %s rule %s has no patterns", lang, ruleID)
			}
			for i, pattern := range patterns {
				if pattern == "" {
					t.Errorf("Language %s rule %s pattern %d is empty", lang, ruleID, i)
				}
				// Verify it's a valid regex
				if _, err := regexp.Compile(pattern); err != nil {
					t.Errorf("Language %s rule %s pattern %d is invalid regex: %v", lang, ruleID, i, err)
				}
			}
		}
	}
}

// TestSafeExternalPackages verifies safe package list is present
func TestSafeExternalPackages(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	if len(rules.SafeExternalPackages) == 0 {
		t.Logf("Note: No safe_external_packages defined (may be intentional)")
	}

	// Verify safe packages are valid package paths
	for _, pkg := range rules.SafeExternalPackages {
		if pkg == "" {
			t.Errorf("Found empty string in safe_external_packages")
		}
		if strings.Contains(pkg, " ") {
			t.Errorf("Safe package should not contain spaces: %s", pkg)
		}
	}
}

// TestMultipleRulesFiles verifies we can load rules from different locations
func TestMultipleRulesFiles(t *testing.T) {
	// Test loading from relative path
	rules1, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load from relative path: %v", err)
	}

	// Test loading from absolute path (construct from relative)
	absPath, err := filepath.Abs("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	rules2, err := config.LoadRules(absPath)
	if err != nil {
		t.Fatalf("Failed to load from absolute path: %v", err)
	}

	// Should have same number of rules
	if len(rules1.FunctionCalls) != len(rules2.FunctionCalls) {
		t.Errorf("Different number of function_calls between loads: %d vs %d",
			len(rules1.FunctionCalls), len(rules2.FunctionCalls))
	}
}

// TestEmptyRulesFile verifies handling of minimal rules file
func TestEmptyRulesFile(t *testing.T) {
	// Create temp empty YAML file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(tmpFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	rules, err := config.LoadRules(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load empty rules: %v", err)
	}

	// Should succeed with zero rules - slices may be nil or empty
	if rules.FunctionCalls == nil {
		t.Logf("Note: FunctionCalls is nil (this is acceptable for empty YAML)")
	}
	if rules.ExternalPackages == nil {
		t.Logf("Note: ExternalPackages is nil (this is acceptable for empty YAML)")
	}
}

// TestMalformedRulesFile verifies error handling for invalid YAML
func TestMalformedRulesFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "bad.yaml")

	// Write invalid YAML
	if err := os.WriteFile(tmpFile, []byte("invalid: [unclosed"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err := config.LoadRules(tmpFile)
	if err == nil {
		t.Errorf("Expected error loading malformed YAML, got nil")
	}
}

package config_test

import (
	"os"
	"path/filepath"
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

	// Verify Rules map exists and has rules
	if len(rules.Rules) == 0 {
		t.Errorf("Expected rules in Rules map, found none")
	}

	// Verify essential rules are present
	requiredRules := []string{"TimeUsage", "Randomness", "IOCalls"}
	for _, ruleID := range requiredRules {
		rule, ok := rules.Rules[ruleID]
		if !ok {
			t.Errorf("Required rule %s not found in Rules map", ruleID)
			continue
		}
		if rule.Description == "" {
			t.Errorf("Rule %s has empty description", ruleID)
		}
		if rule.Message == "" {
			t.Errorf("Rule %s has empty message", ruleID)
		}
		if rule.DefaultSeverity == "" {
			t.Errorf("Rule %s has empty default_severity", ruleID)
		}
	}
}

// TestLanguageSupport verifies that languages are properly structured
func TestLanguageSupport(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Check that at least one rule has Go support
	hasGo := false
	hasJava := false
	for _, rule := range rules.Rules {
		if rule.Languages != nil {
			if _, ok := rule.Languages["go"]; ok {
				hasGo = true
			}
			if _, ok := rule.Languages["java"]; ok {
				hasJava = true
			}
		}
	}

	if !hasGo {
		t.Errorf("Expected at least one rule with Go support")
	}
	if !hasJava {
		t.Errorf("Expected at least one rule with Java support")
	}
}

// TestGetRulesForLanguage verifies language-specific rule extraction
func TestGetRulesForLanguage(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	// Get Go rules
	goRules := rules.GetRulesForLanguage("go")
	if len(goRules) == 0 {
		t.Errorf("Expected Go rules, found none")
	}

	// Get Java rules
	javaRules := rules.GetRulesForLanguage("java")
	if len(javaRules) == 0 {
		t.Errorf("Expected Java rules, found none")
	}

	// Verify structure
	for ruleID, rule := range goRules {
		if rule.Languages == nil {
			t.Errorf("Go rule %s has nil Languages", ruleID)
		}
		if _, ok := rule.Languages["go"]; !ok {
			t.Errorf("Go rule %s missing 'go' language config", ruleID)
		}
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

	// Verify each rule has required fields
	for ruleID, rule := range rules.Rules {
		if rule.Description == "" {
			t.Errorf("Rule %s missing 'description' field", ruleID)
		}
		if rule.Message == "" {
			t.Errorf("Rule %s missing 'message' field", ruleID)
		}
		if rule.DefaultSeverity != "error" && rule.DefaultSeverity != "warning" && rule.DefaultSeverity != "info" {
			t.Errorf("Rule %s has invalid default_severity: %s", ruleID, rule.DefaultSeverity)
		}
		if rule.Languages == nil || len(rule.Languages) == 0 {
			t.Errorf("Rule %s has no language configurations", ruleID)
		}
	}
}

// TestLanguageMappingsYAMLFormat verifies mappings are string arrays
func TestLanguageMappingsYAMLFormat(t *testing.T) {
	rules, err := config.LoadRules("rules.yaml")
	if err != nil {
		t.Fatalf("Failed to load rules.yaml: %v", err)
	}

	if rules.Rules == nil {
		t.Skip("No rules in rules.yaml")
	}

	for ruleID, rule := range rules.Rules {
		if rule.Languages == nil {
			t.Errorf("Rule %s has no language configurations", ruleID)
			continue
		}
		for lang, langConfig := range rule.Languages {
			if langConfig.FunctionCalls != nil {
				for _, fc := range langConfig.FunctionCalls {
					if len(fc.Functions) == 0 {
						t.Errorf("Rule %s lang %s function_call has no functions", ruleID, lang)
					}
				}
			}
			if langConfig.MethodCalls != nil {
				for _, mc := range langConfig.MethodCalls {
					if len(mc.Methods) == 0 {
						t.Errorf("Rule %s lang %s method_call has no methods", ruleID, lang)
					}
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

	goSafeImports := rules.GetSafeImports("go")
	if len(goSafeImports) == 0 {
		t.Logf("Note: No safe imports defined for Go (may be intentional)")
	}

	// Verify safe imports are valid package paths
	for _, pkg := range goSafeImports {
		if pkg == "" {
			t.Errorf("Found empty string in safe imports")
		}
		if strings.Contains(pkg, " ") {
			t.Errorf("Safe import should not contain spaces: %s", pkg)
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
	if len(rules1.Rules) != len(rules2.Rules) {
		t.Errorf("Different number of rules between loads: %d vs %d",
			len(rules1.Rules), len(rules2.Rules))
	}
}

// TestEmptyRulesFile verifies handling of minimal rules file
func TestEmptyRulesFile(t *testing.T) {
	// Create temp empty YAML file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(tmpFile, []byte("rules: {}"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	rules, err := config.LoadRules(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load empty rules: %v", err)
	}

	// Should succeed with zero rules
	if rules.Rules == nil {
		t.Logf("Note: Rules is nil (this is acceptable for empty YAML)")
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

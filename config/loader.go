package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// FunctionCall represents a Go-style function call pattern
type FunctionCall struct {
	Import    string   `yaml:"import"`             // e.g., "time", "math/rand"
	Functions []string `yaml:"functions"`          // e.g., ["Now", "Since"]
	Severity  string   `yaml:"severity,omitempty"` // optional override
	Message   string   `yaml:"message,omitempty"`  // optional override
}

// MethodCall represents a Java-style method call pattern
type MethodCall struct {
	Package  string   `yaml:"package"`            // e.g., "java.time", "java.util"
	Types    []string `yaml:"types"`              // e.g., ["Instant", "System"], empty means any
	Methods  []string `yaml:"methods"`            // e.g., ["now", "currentTimeMillis"]
	Severity string   `yaml:"severity,omitempty"` // optional override
	Message  string   `yaml:"message,omitempty"`  // optional override
}

// ASTPattern represents language-specific AST patterns
type ASTPattern struct {
	Kind string `yaml:"kind"` // e.g., "range_over_map"
}

// ImportsConfig represents import restrictions
type ImportsConfig struct {
	Disallowed []struct {
		Path string `yaml:"path"`
	} `yaml:"disallowed"`
}

// LanguageConfig holds all detection patterns for a specific language
type LanguageConfig struct {
	FunctionCalls []FunctionCall `yaml:"function_calls,omitempty"`
	MethodCalls   []MethodCall   `yaml:"method_calls,omitempty"`
	ASTPatterns   []ASTPattern   `yaml:"ast_patterns,omitempty"`
	Imports       *ImportsConfig `yaml:"imports,omitempty"`
}

// Rule represents a single rule definition with multi-language support
type Rule struct {
	Description     string                    `yaml:"description"`
	DefaultSeverity string                    `yaml:"default_severity"`
	Message         string                    `yaml:"message"`
	Languages       map[string]LanguageConfig `yaml:"languages"`
}

// RuleSet is the top-level configuration structure
type RuleSet struct {
	Rules       map[string]Rule     `yaml:"rules"`
	SafeImports map[string][]string `yaml:"safe_imports"`
}

func LoadRules(path string) (*RuleSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rs RuleSet
	if err := yaml.Unmarshal(b, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

// GetRulesForLanguage returns all rules that apply to a specific language
func (rs *RuleSet) GetRulesForLanguage(lang string) map[string]Rule {
	result := make(map[string]Rule)
	for ruleID, rule := range rs.Rules {
		if _, hasLang := rule.Languages[lang]; hasLang {
			result[ruleID] = rule
		}
	}
	return result
}

// GetSafeImports returns the list of safe imports for a language
func (rs *RuleSet) GetSafeImports(lang string) []string {
	if rs.SafeImports == nil {
		return nil
	}
	return rs.SafeImports[lang]
}

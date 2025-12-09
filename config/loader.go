package config

import (
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type FunctionRule struct {
	Rule      string   `yaml:"rule"`
	Package   string   `yaml:"package"`   // import path (e.g., "time", "math/rand", "fmt", "os")
	Functions []string `yaml:"functions"` // selector names
	Severity  string   `yaml:"severity"`  // e.g., "error", "warning"
	Message   string   `yaml:"message"`
}

type ImportRule struct {
	Rule     string `yaml:"rule"`
	Severity string `yaml:"severity"` // e.g., "error", "warning"
	Path     string `yaml:"path"`     // import path
	Message  string `yaml:"message"`  // message if path is present in file with workflows
}

type ExternalPackageRule struct {
	Rule      string   `yaml:"rule"`
	Package   string   `yaml:"package"`   // full import path (e.g., "github.com/google/uuid")
	Functions []string `yaml:"functions"` // function names to flag
	Severity  string   `yaml:"severity"`  // e.g., "error", "warning"
	Message   string   `yaml:"message"`   // message when violation is detected
}

type RuleSet struct {
	FunctionCalls        []FunctionRule        `yaml:"function_calls"`
	DisallowedImports    []ImportRule          `yaml:"disallowed_imports"`
	ExternalPackages     []ExternalPackageRule `yaml:"external_packages"`
	SafeExternalPackages []string              `yaml:"safe_external_packages"`
	// LanguageMappings allows adapters to provide language-specific patterns
	// for top-level rule IDs. Structure: language -> ruleID -> []patterns
	LanguageMappings map[string]map[string][]string `yaml:"language_mappings"`
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

// CompileLanguageMappings compiles regex patterns under LanguageMappings for
// the given language (e.g. "go", "java"). It returns a map from ruleID to
// compiled regexps. If no mappings exist for the language an empty map is
// returned.
func (rs *RuleSet) CompileLanguageMappings(lang string) (map[string][]*regexp.Regexp, error) {
	out := make(map[string][]*regexp.Regexp)
	if rs == nil || rs.LanguageMappings == nil {
		return out, nil
	}
	lm, ok := rs.LanguageMappings[lang]
	if !ok {
		return out, nil
	}
	for ruleID, pats := range lm {
		for _, p := range pats {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, err
			}
			out[ruleID] = append(out[ruleID], re)
		}
	}
	return out, nil
}

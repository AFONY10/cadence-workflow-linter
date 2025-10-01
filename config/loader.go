package config

import (
	"os"

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

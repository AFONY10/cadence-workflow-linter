package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/config"
	"github.com/afony10/cadence-workflow-linter/core"

	"go/ast"
)

func main() {
	// Command-line flags
	var format string
	var rulesPath string
	var enableMapIteration bool
	flag.StringVar(&format, "format", "json", "output format: json|yaml")
	flag.StringVar(&rulesPath, "rules", "config/rules.yaml", "path to rules yaml")
	flag.BoolVar(&enableMapIteration, "map-iteration", true, "enable detection of nondeterministic map iteration")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: cadence-workflow-linter [--format json|yaml] [--rules path] <file_or_directory>")
		os.Exit(1)
	}

	target := flag.Arg(0)

	rules, err := config.LoadRules(rulesPath)
	if err != nil {
		fmt.Println("Error loading rules:", err)
		os.Exit(1)
	}

	// Factory returns fresh visitors per file using config and module info
	factory := func(moduleInfo *modutils.ModuleInfo) []ast.Visitor {
		visitors := []ast.Visitor{
			detectors.NewFuncCallDetector(rules.FunctionCalls, rules.ExternalPackages, rules.SafeExternalPackages, moduleInfo),
			detectors.NewImportDetector(rules.DisallowedImports),
			detectors.NewGoroutineDetector(),
			detectors.NewChannelDetector(),
		}
		if enableMapIteration {
			visitors = append(visitors, detectors.NewMapIterationDetector(rules.FunctionCalls))
		}
		return visitors
	}

	var issues []detectors.Issue
	info, statErr := os.Stat(target)
	if statErr != nil {
		fmt.Println("Error:", statErr)
		os.Exit(1)
	}

	if info.IsDir() {
		issues, err = analyzer.ScanDirectory(target, factory)
	} else {
		issues, err = analyzer.ScanFile(target, factory)
	}
	if err != nil {
		fmt.Println("Scan error:", err)
		os.Exit(1)
	}

	// Apply any overrides from the rules configuration so severities/messages
	// are consistent with user config (maps config rule names -> core IDs)
	issues = core.ApplyConfigOverrides(issues, rules)

	switch format {
	case "yaml", "yml":
		out, mErr := core.EmitYAML(issues)
		if mErr != nil {
			fmt.Println("Marshal error:", mErr)
			os.Exit(1)
		}
		fmt.Print(string(out))
	default:
		out, mErr := core.EmitJSON(issues)
		if mErr != nil {
			fmt.Println("Marshal error:", mErr)
			os.Exit(1)
		}
		fmt.Print(string(out))
	}
}

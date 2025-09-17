package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/afony10/cadence-workflow-linter/analyzer"
	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/config"

	"go/ast"
)

func main() {
	// Command-line flags
	var format string
	var rulesPath string
	flag.StringVar(&format, "format", "json", "output format: json|yaml")
	flag.StringVar(&rulesPath, "rules", "config/rules.yaml", "path to rules yaml")
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

	// Factory returns fresh visitors per file using config
	factory := func() []ast.Visitor {
		return []ast.Visitor{
			detectors.NewFuncCallDetector(rules.FunctionCalls),
			detectors.NewImportDetector(rules.DisallowedImports),
			detectors.NewGoroutineDetector(),
			detectors.NewChannelDetector(),
		}
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

	switch format {
	case "yaml", "yml":
		out, mErr := yaml.Marshal(issues)
		if mErr != nil {
			fmt.Println("Marshal error:", mErr)
			os.Exit(1)
		}
		fmt.Print(string(out))
	default:
		out, mErr := json.MarshalIndent(issues, "", "  ")
		if mErr != nil {
			fmt.Println("Marshal error:", mErr)
			os.Exit(1)
		}
		fmt.Print(string(out))
	}
}

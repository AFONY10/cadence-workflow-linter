package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/afony10/cadence-workflow-linter/analyzer"
	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"

	"go/ast"
)

func main() {
	var format string
	flag.StringVar(&format, "format", "json", "output format: json|yaml")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: cadence-workflow-linter [--format json|yaml] <file_or_directory>")
		os.Exit(1)
	}

	target := flag.Arg(0)

	// Factory creates fresh detectors per file
	factory := func() []ast.Visitor {
		return []ast.Visitor{
			detectors.NewTimeUsageDetector(),
			detectors.NewRandomnessDetector(),
			detectors.NewIOCallsDetector(),
		}
	}

	var issues []detectors.Issue
	var err error

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
	default: // json
		out, mErr := json.MarshalIndent(issues, "", "  ")
		if mErr != nil {
			fmt.Println("Marshal error:", mErr)
			os.Exit(1)
		}
		fmt.Print(string(out))
	}
}

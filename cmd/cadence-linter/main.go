package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/modutils"
	javaanalyzer "github.com/afony10/cadence-workflow-linter/adapters/java/analyzer"
	"github.com/afony10/cadence-workflow-linter/config"
	"github.com/afony10/cadence-workflow-linter/core"

	"go/ast"
)

func main() {
	// Command-line flags
	var format string
	var rulesPath string
	var enableMapIteration bool
	var lang string
	flag.StringVar(&format, "format", "json", "output format: json|yaml|sarif")
	flag.StringVar(&rulesPath, "rules", "config/rules.yaml", "path to rules yaml")
	flag.BoolVar(&enableMapIteration, "map-iteration", true, "enable detection of nondeterministic map iteration")
	flag.StringVar(&lang, "lang", "auto", "language to scan: auto|go|java")
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

	var issues []core.Issue
	info, statErr := os.Stat(target)
	if statErr != nil {
		fmt.Println("Error:", statErr)
		os.Exit(1)
	}

	// Determine language to use
	detectedLang := lang
	if detectedLang == "auto" {
		if !info.IsDir() {
			if filepath.Ext(target) == ".java" {
				detectedLang = "java"
			} else {
				detectedLang = "go"
			}
		} else {
			// look for any .java files under the directory as a heuristic
			found := false
			stopErr := errors.New("_found_java_")
			_ = filepath.Walk(target, func(path string, fi os.FileInfo, werr error) error {
				if werr != nil {
					return werr
				}
				if fi == nil || fi.IsDir() {
					return nil
				}
				if filepath.Ext(path) == ".java" {
					found = true
					return stopErr
				}
				return nil
			})
			if found {
				detectedLang = "java"
			} else {
				detectedLang = "go"
			}
		}
	}

	switch detectedLang {
	case "go":
		if info.IsDir() {
			temp, gerr := analyzer.ScanDirectory(target, factory)
			err = gerr
			if err == nil {
				// "detectors.Issue" is an alias to core.Issue; append explicitly
				for _, it := range temp {
					issues = append(issues, it)
				}
			}
		} else {
			temp, gerr := analyzer.ScanFile(target, factory)
			err = gerr
			if err == nil {
				for _, it := range temp {
					issues = append(issues, it)
				}
			}
		}
	case "java":
		if info.IsDir() {
			issues, err = javaanalyzer.ScanDirectory(target)
		} else {
			issues, err = javaanalyzer.ScanFile(target)
		}
	default:
		fmt.Println("Unsupported language:", detectedLang)
		os.Exit(1)
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
	case "sarif":
		out, mErr := core.EmitSARIF(issues)
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

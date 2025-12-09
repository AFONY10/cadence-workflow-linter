package main

import (
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
	flag.StringVar(&lang, "lang", "auto", "language to scan: auto|go|java|all")
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

	// Compile language mappings for Go and Java (if present). These are used by
	// adapters to prefer centralized patterns over heuristics.
	goMappings, _ := rules.CompileLanguageMappings("go")
	javaMappings, _ := rules.CompileLanguageMappings("java")

	// Factory returns fresh visitors per file using config and module info
	factory := func(moduleInfo *modutils.ModuleInfo) []ast.Visitor {
		// create func call detector and set compiled go mappings
		fc := detectors.NewFuncCallDetector(rules.FunctionCalls, rules.ExternalPackages, rules.SafeExternalPackages, moduleInfo)
		if len(goMappings) > 0 {
			fc.SetLanguageMappings(goMappings)
		}

		visitors := []ast.Visitor{
			fc,
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
		// For directories: scan ALL languages (multi-language support)
		if info.IsDir() {
			detectedLang = "all"
		} else {
			// For single files: detect by extension
			if filepath.Ext(target) == ".java" {
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
		// provide precompiled java mappings to the java analyzer for efficiency
		if len(javaMappings) > 0 {
			javaanalyzer.SetLanguageMappings(javaMappings)
		}
		if info.IsDir() {
			issues, err = javaanalyzer.ScanDirectory(target, rules)
		} else {
			issues, err = javaanalyzer.ScanFile(target, rules)
		}
	case "all":
		// Scan all supported languages in the directory
		if len(javaMappings) > 0 {
			javaanalyzer.SetLanguageMappings(javaMappings)
		}

		// Scan Go code
		goIssues, goErr := analyzer.ScanDirectory(target, factory)
		if goErr != nil {
			fmt.Println("Go scan error:", goErr)
		} else {
			for _, it := range goIssues {
				issues = append(issues, it)
			}
		}

		// Scan Java code
		javaIssues, javaErr := javaanalyzer.ScanDirectory(target, rules)
		if javaErr != nil {
			fmt.Println("Java scan error:", javaErr)
		} else {
			issues = append(issues, javaIssues...)
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

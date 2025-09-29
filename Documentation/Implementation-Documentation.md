# Cadence Workflow Linter - Implementation Documentation

## Table of Contents
1. [Project Structure](#project-structure)
2. [Entry Point Implementation](#entry-point-implementation)
3. [Core Analysis Engine](#core-analysis-engine)
4. [Workflow Registry Implementation](#workflow-registry-implementation)
5. [Detector Implementation](#detector-implementation)
6. [Configuration System](#configuration-system)
7. [Testing Strategy](#testing-strategy)
8. [Implementation Details](#implementation-details)

## Project Structure

The project follows a clean, modular Go structure:

```
cadence-workflow-linter/
├── main.go                          # CLI entry point
├── go.mod                           # Go module definition
├── go.sum                           # Dependency checksums
├── analyzer/
│   ├── scanner.go                   # Core analysis engine
│   ├── detectors/
│   │   ├── detector.go              # Base interfaces and types
│   │   ├── func_call_detector.go    # Function call violation detector
│   │   ├── import_detector.go       # Import violation detector
│   │   ├── goroutine_detector.go    # Goroutine usage detector
│   │   └── channel_detector.go      # Channel usage detector
│   └── registry/
│       └── workflow_registry.go     # Workflow/activity classification
├── config/
│   ├── loader.go                    # Configuration loading logic
│   └── rules.yaml                   # Default linting rules
└── testdata/                        # Test files for validation
    ├── activity_ok.go
    ├── cadence_workshop_test.go
    ├── channel_violation.go
    ├── goroutine_violation.go
    ├── io_violation.go
    ├── rand_violation.go
    ├── time_violation.go
    └── workflow_violation.go
```

## Entry Point Implementation

### main.go - Command Line Interface

The main entry point implements a clean CLI interface with flag parsing and output formatting:

```go
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

	// Load configuration
	rules, err := config.LoadRules(rulesPath)
	if err != nil {
		fmt.Println("Error loading rules:", err)
		os.Exit(1)
	}

	// Factory pattern for creating detectors with configuration
	factory := func() []ast.Visitor {
		return []ast.Visitor{
			detectors.NewFuncCallDetector(rules.FunctionCalls),
			detectors.NewImportDetector(rules.DisallowedImports),
			detectors.NewGoroutineDetector(),
			detectors.NewChannelDetector(),
		}
	}

	// Determine if target is file or directory and scan accordingly
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

	// Format and output results
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
```

**Key Implementation Details:**

1. **Flag Parsing**: Uses Go's standard `flag` package for CLI argument handling
2. **Factory Pattern**: Creates fresh detector instances for each analysis run
3. **Flexible Output**: Supports both JSON and YAML output formats
4. **Error Handling**: Comprehensive error handling with appropriate exit codes
5. **Configuration Loading**: Loads rules from configurable YAML file

## Core Analysis Engine

### analyzer/scanner.go - Two-Pass Analysis

The scanner implements a sophisticated two-pass analysis approach:

```go
package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

// Internal structure to keep parsed files with metadata
type parsedFile struct {
	filename  string
	fset      *token.FileSet
	node      *ast.File
	importMap map[string]string
}

// Build an alias->import map for the file (e.g., r -> math/rand)
func buildImportMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			// Extract package name from path
			if i := strings.LastIndex(path, "/"); i >= 0 {
				alias = path[i+1:]
			} else {
				alias = path
			}
		}
		m[alias] = path
	}
	return m
}

// First pass: parse files and build the global registry
func parseAllAndBuildRegistry(target string) ([]parsedFile, *registry.WorkflowRegistry, error) {
	var files []parsedFile
	wr := registry.NewWorkflowRegistry()

	addFile := func(path string) error {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}

		files = append(files, parsedFile{
			filename:  path,
			fset:      fset,
			node:      node,
			importMap: buildImportMap(node),
		})

		// Build registry from this file
		ast.Walk(wr, node)
		return nil
	}

	// Handle both single files and directories
	info, err := os.Stat(target)
	if err != nil {
		return nil, nil, err
	}

	if info.IsDir() {
		err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				return addFile(path)
			}
			return nil
		})
	} else if strings.HasSuffix(target, ".go") {
		err = addFile(target)
	}

	return files, wr, err
}

// Second pass: run detectors on all files with full context
func runDetectorsOnFiles(files []parsedFile, wr *registry.WorkflowRegistry, factory func() []ast.Visitor) []detectors.Issue {
	var allIssues []detectors.Issue

	for _, pf := range files {
		visitors := factory()

		// Configure each detector with context
		for _, visitor := range visitors {
			if wa, ok := visitor.(detectors.WorkflowAware); ok {
				wa.SetWorkflowRegistry(wr)
			}
			if fca, ok := visitor.(detectors.FileContextAware); ok {
				fca.SetFileContext(detectors.FileContext{
					File:      pf.filename,
					Fset:      pf.fset,
					ImportMap: pf.importMap,
				})
			}
		}

		// Run each detector on the file
		for _, visitor := range visitors {
			ast.Walk(visitor, pf.node)
		}

		// Collect issues from detectors
		for _, visitor := range visitors {
			if ip, ok := visitor.(detectors.IssueProvider); ok {
				allIssues = append(allIssues, ip.Issues()...)
			}
		}
	}

	return allIssues
}

// Public interface for scanning a single file
func ScanFile(filename string, factory func() []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, err := parseAllAndBuildRegistry(filename)
	if err != nil {
		return nil, err
	}
	return runDetectorsOnFiles(files, wr, factory), nil
}

// Public interface for scanning a directory
func ScanDirectory(dirname string, factory func() []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, err := parseAllAndBuildRegistry(dirname)
	if err != nil {
		return nil, err
	}
	return runDetectorsOnFiles(files, wr, factory), nil
}
```

**Key Implementation Features:**

1. **Two-Pass Analysis**: 
   - **Pass 1**: Parse all files and build workflow registry
   - **Pass 2**: Run detectors with full context
2. **Import Map Building**: Tracks import aliases for accurate package resolution
3. **Context Injection**: Provides detectors with file context and workflow registry
4. **Flexible File Handling**: Supports both single files and directory scanning

## Workflow Registry Implementation

### analyzer/registry/workflow_registry.go - Context Classification

The workflow registry is the core component for distinguishing workflow from activity code:

```go
package registry

import "go/ast"

// WorkflowRegistry tracks function classifications and call relationships
type WorkflowRegistry struct {
	WorkflowFuncs map[string]bool     // Functions with workflow.Context parameter
	ActivityFuncs map[string]bool     // Functions with context.Context parameter
	CallGraph     map[string][]string // Function call relationships
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		WorkflowFuncs: make(map[string]bool),
		ActivityFuncs: make(map[string]bool),
		CallGraph:     make(map[string][]string),
	}
}

// Visit implements ast.Visitor interface for registry building
func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
	// Classify functions by their parameter types
	if fn, ok := node.(*ast.FuncDecl); ok {
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				// Look for workflow.Context or context.Context parameters
				if sel, ok := param.Type.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && sel.Sel.Name == "Context" {
						switch ident.Name {
						case "workflow":
							wr.WorkflowFuncs[fn.Name.Name] = true
						case "context":
							wr.ActivityFuncs[fn.Name.Name] = true
						}
					}
				}
			}
		}

		// Build call graph from function body
		if fn.Body != nil {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					// Handle direct function calls: foo()
					if ident, ok := call.Fun.(*ast.Ident); ok {
						wr.CallGraph[fn.Name.Name] = append(wr.CallGraph[fn.Name.Name], ident.Name)
					}
					// Handle method calls: obj.method()
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							methodCall := ident.Name + "." + sel.Sel.Name
							wr.CallGraph[fn.Name.Name] = append(wr.CallGraph[fn.Name.Name], methodCall)
						}
					}
				}
				return true
			})
		}
	}
	return wr
}

// IsWorkflowReachable determines if a function is reachable from workflow code
func (wr *WorkflowRegistry) IsWorkflowReachable(funcName string) bool {
	// Direct workflow function
	if wr.WorkflowFuncs[funcName] {
		return true
	}

	// Check if reachable from any workflow function via call graph
	visited := make(map[string]bool)
	return wr.isReachableFrom(funcName, wr.WorkflowFuncs, visited)
}

// Recursive helper for reachability analysis
func (wr *WorkflowRegistry) isReachableFrom(target string, sources map[string]bool, visited map[string]bool) bool {
	if visited[target] {
		return false // Avoid infinite loops
	}
	visited[target] = true

	// Check if any source directly calls the target
	for source := range sources {
		for _, callee := range wr.CallGraph[source] {
			if callee == target {
				return true
			}
		}
	}

	// Recursively check indirect calls
	nextLevel := make(map[string]bool)
	for source := range sources {
		for _, callee := range wr.CallGraph[source] {
			nextLevel[callee] = true
		}
	}

	if len(nextLevel) > 0 {
		return wr.isReachableFrom(target, nextLevel, visited)
	}

	return false
}

// GetCallStack provides debugging information for call paths
func (wr *WorkflowRegistry) GetCallStack(from, to string) []string {
	visited := make(map[string]bool)
	path := []string{}
	if wr.findPath(from, to, visited, &path) {
		return path
	}
	return nil
}

// Recursive path finding for call stack construction
func (wr *WorkflowRegistry) findPath(from, to string, visited map[string]bool, path *[]string) bool {
	if visited[from] {
		return false
	}
	visited[from] = true
	*path = append(*path, from)

	if from == to {
		return true
	}

	for _, callee := range wr.CallGraph[from] {
		if wr.findPath(callee, to, visited, path) {
			return true
		}
	}

	*path = (*path)[:len(*path)-1] // Backtrack
	return false
}
```

**Key Implementation Features:**

1. **Function Classification**: Automatically identifies workflow/activity functions by parameter types
2. **Call Graph Construction**: Builds complete call relationship map
3. **Reachability Analysis**: Determines if functions are callable from workflow contexts
4. **Path Finding**: Provides call stack information for debugging

## Detector Implementation

### Base Detector Interfaces

```go
// analyzer/detectors/detector.go
package detectors

import (
	"go/token"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

// Issue represents a linting violation
type Issue struct {
	File      string   `json:"file" yaml:"file"`
	Line      int      `json:"line" yaml:"line"`
	Column    int      `json:"column" yaml:"column"`
	Rule      string   `json:"rule" yaml:"rule"`
	Severity  string   `json:"severity" yaml:"severity"`
	Message   string   `json:"message" yaml:"message"`
	Func      string   `json:"func,omitempty" yaml:"func,omitempty"`
	CallStack []string `json:"callstack,omitempty" yaml:"callstack,omitempty"`
}

// WorkflowAware interface for detectors that need workflow context
type WorkflowAware interface {
	SetWorkflowRegistry(reg *registry.WorkflowRegistry)
}

// FileContext provides file-specific information to detectors
type FileContext struct {
	File      string
	Fset      *token.FileSet
	ImportMap map[string]string // alias -> import path
}

// FileContextAware interface for detectors that need file context
type FileContextAware interface {
	SetFileContext(ctx FileContext)
}

// IssueProvider interface for collecting issues from detectors
type IssueProvider interface {
	Issues() []Issue
}
```

### Function Call Detector Implementation

```go
// analyzer/detectors/func_call_detector.go
package detectors

import (
	"go/ast"
	"strings"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type FuncCallDetector struct {
	rules       []config.FunctionCallRule
	registry    *registry.WorkflowRegistry
	issues      []Issue
	fileContext FileContext
	currentFunc string
}

func NewFuncCallDetector(rules []config.FunctionCallRule) *FuncCallDetector {
	return &FuncCallDetector{
		rules:  rules,
		issues: make([]Issue, 0),
	}
}

// Implement WorkflowAware interface
func (d *FuncCallDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.registry = reg
}

// Implement FileContextAware interface
func (d *FuncCallDetector) SetFileContext(ctx FileContext) {
	d.fileContext = ctx
}

// Implement IssueProvider interface
func (d *FuncCallDetector) Issues() []Issue {
	return d.issues
}

// Visit implements ast.Visitor interface
func (d *FuncCallDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		// Track current function for context
		if n.Name != nil {
			d.currentFunc = n.Name.Name
		}
		return d

	case *ast.CallExpr:
		d.checkFunctionCall(n)
		return d

	default:
		return d
	}
}

// Check if a function call violates any rules
func (d *FuncCallDetector) checkFunctionCall(call *ast.CallExpr) {
	// Skip if not in workflow context
	if d.registry != nil && !d.registry.IsWorkflowReachable(d.currentFunc) {
		return
	}

	pkg, fn := d.extractPackageAndFunction(call)
	if pkg == "" || fn == "" {
		return
	}

	// Check against all rules
	for _, rule := range d.rules {
		if pkg == rule.Package {
			for _, disallowedFunc := range rule.Functions {
				if fn == disallowedFunc {
					d.createIssue(call, rule, fn)
					return
				}
			}
		}
	}
}

// Extract package and function name from call expression
func (d *FuncCallDetector) extractPackageAndFunction(call *ast.CallExpr) (string, string) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			pkg := ident.Name
			fn := sel.Sel.Name

			// Resolve package alias to full import path
			if fullPath, exists := d.fileContext.ImportMap[pkg]; exists {
				return fullPath, fn
			}
			return pkg, fn
		}
	}
	return "", ""
}

// Create an issue for a rule violation
func (d *FuncCallDetector) createIssue(call *ast.CallExpr, rule config.FunctionCallRule, funcName string) {
	pos := d.fileContext.Fset.Position(call.Pos())
	message := strings.ReplaceAll(rule.Message, "%FUNC%", funcName)
	
	issue := Issue{
		File:     d.fileContext.File,
		Line:     pos.Line,
		Column:   pos.Column,
		Rule:     rule.Rule,
		Severity: rule.Severity,
		Message:  message,
		Func:     d.currentFunc,
	}

	d.issues = append(d.issues, issue)
}
```

### Import Detector Implementation

```go
// analyzer/detectors/import_detector.go
package detectors

import (
	"go/ast"
	"strings"
	"github.com/afony10/cadence-workflow-linter/config"
)

type ImportDetector struct {
	rules       []config.ImportRule
	issues      []Issue
	fileContext FileContext
}

func NewImportDetector(rules []config.ImportRule) *ImportDetector {
	return &ImportDetector{
		rules:  rules,
		issues: make([]Issue, 0),
	}
}

// Implement FileContextAware interface
func (d *ImportDetector) SetFileContext(ctx FileContext) {
	d.fileContext = ctx
}

// Implement IssueProvider interface
func (d *ImportDetector) Issues() []Issue {
	return d.issues
}

// Visit implements ast.Visitor interface
func (d *ImportDetector) Visit(node ast.Node) ast.Visitor {
	if imp, ok := node.(*ast.ImportSpec); ok {
		d.checkImport(imp)
	}
	return d
}

// Check if an import violates any rules
func (d *ImportDetector) checkImport(imp *ast.ImportSpec) {
	importPath := strings.Trim(imp.Path.Value, `"`)
	
	for _, rule := range d.rules {
		if importPath == rule.Path {
			pos := d.fileContext.Fset.Position(imp.Pos())
			
			issue := Issue{
				File:     d.fileContext.File,
				Line:     pos.Line,
				Column:   pos.Column,
				Rule:     rule.Rule,
				Severity: rule.Severity,
				Message:  rule.Message,
			}
			
			d.issues = append(d.issues, issue)
			return
		}
	}
}
```

### Specialized Detectors

#### Goroutine Detector
```go
// analyzer/detectors/goroutine_detector.go
package detectors

import (
	"go/ast"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type GoroutineDetector struct {
	registry    *registry.WorkflowRegistry
	issues      []Issue
	fileContext FileContext
	currentFunc string
}

func NewGoroutineDetector() *GoroutineDetector {
	return &GoroutineDetector{
		issues: make([]Issue, 0),
	}
}

func (d *GoroutineDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.registry = reg
}

func (d *GoroutineDetector) SetFileContext(ctx FileContext) {
	d.fileContext = ctx
}

func (d *GoroutineDetector) Issues() []Issue {
	return d.issues
}

func (d *GoroutineDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Name != nil {
			d.currentFunc = n.Name.Name
		}
		return d

	case *ast.GoStmt:
		// Only flag in workflow context
		if d.registry != nil && d.registry.IsWorkflowReachable(d.currentFunc) {
			pos := d.fileContext.Fset.Position(n.Pos())
			
			issue := Issue{
				File:     d.fileContext.File,
				Line:     pos.Line,
				Column:   pos.Column,
				Rule:     "Goroutine",
				Severity: "error",
				Message:  "Detected 'go' statement in workflow. Use workflow.Go() instead.",
				Func:     d.currentFunc,
			}
			
			d.issues = append(d.issues, issue)
		}
		return d

	default:
		return d
	}
}
```

#### Channel Detector
```go
// analyzer/detectors/channel_detector.go
package detectors

import (
	"go/ast"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type ChannelDetector struct {
	registry    *registry.WorkflowRegistry
	issues      []Issue
	fileContext FileContext
	currentFunc string
}

func NewChannelDetector() *ChannelDetector {
	return &ChannelDetector{
		issues: make([]Issue, 0),
	}
}

func (d *ChannelDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.registry = reg
}

func (d *ChannelDetector) SetFileContext(ctx FileContext) {
	d.fileContext = ctx
}

func (d *ChannelDetector) Issues() []Issue {
	return d.issues
}

func (d *ChannelDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Name != nil {
			d.currentFunc = n.Name.Name
		}
		return d

	case *ast.CallExpr:
		// Check for make(chan ...) calls
		if d.isMakeChannelCall(n) && d.registry != nil && d.registry.IsWorkflowReachable(d.currentFunc) {
			pos := d.fileContext.Fset.Position(n.Pos())
			
			issue := Issue{
				File:     d.fileContext.File,
				Line:     pos.Line,
				Column:   pos.Column,
				Rule:     "Channel",
				Severity: "error",
				Message:  "Detected channel creation in workflow. Use workflow.Channel() instead.",
				Func:     d.currentFunc,
			}
			
			d.issues = append(d.issues, issue)
		}
		return d

	default:
		return d
	}
}

func (d *ChannelDetector) isMakeChannelCall(call *ast.CallExpr) bool {
	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "make" {
		if len(call.Args) > 0 {
			if chanType, ok := call.Args[0].(*ast.ChanType); ok {
				return chanType != nil
			}
		}
	}
	return false
}
```

## Configuration System

### config/loader.go - YAML Configuration Loading

```go
package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

// Rules represents the complete configuration structure
type Rules struct {
	FunctionCalls      []FunctionCallRule `yaml:"function_calls"`
	DisallowedImports  []ImportRule       `yaml:"disallowed_imports"`
}

// FunctionCallRule defines rules for function call violations
type FunctionCallRule struct {
	Rule      string   `yaml:"rule"`
	Package   string   `yaml:"package"`
	Functions []string `yaml:"functions"`
	Severity  string   `yaml:"severity"`
	Message   string   `yaml:"message"`
}

// ImportRule defines rules for import violations
type ImportRule struct {
	Rule     string `yaml:"rule"`
	Path     string `yaml:"path"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

// LoadRules reads and parses the rules configuration file
func LoadRules(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rules Rules
	err = yaml.Unmarshal(data, &rules)
	if err != nil {
		return nil, err
	}

	return &rules, nil
}
```

### config/rules.yaml - Default Configuration

```yaml
function_calls:
  - rule: TimeUsage
    package: time
    functions: [Now, Since, Sleep]
    severity: error
    message: "Detected time.%FUNC%() in workflow. Use workflow.Now(ctx)/workflow.Sleep(ctx) instead."

  - rule: Randomness  
    package: math/rand
    functions: [Intn, Int, Float32, Float64, Read]
    severity: error
    message: "Detected rand.%FUNC%() in workflow. Avoid nondeterminism; use workflow.SideEffect if needed."

  - rule: IOCalls
    package: os
    functions: [Open, OpenFile, ReadFile, WriteFile, Mkdir, Remove]
    severity: error
    message: "Detected os.%FUNC%() in workflow. Avoid file I/O inside workflows."

  - rule: IOCalls
    package: fmt
    functions: [Println, Printf, Print]
    severity: warning
    message: "Detected fmt.%FUNC%() in workflow. Use workflow.GetLogger(ctx) instead."

  - rule: Network
    package: net/http
    functions: [Get, Post, Do, Head]
    severity: error
    message: "Detected HTTP call in workflow. Use workflow activities for network calls."

disallowed_imports:
  - rule: ImportRandom
    path: math/rand
    severity: warning
    message: "Importing math/rand in files with workflows is discouraged; consider deterministic alternatives."
```

## Testing Strategy

The project implements comprehensive testing with various violation scenarios:

### Test Files Structure

1. **activity_ok.go**: Valid activity code that should not trigger violations
2. **workflow_violation.go**: Basic workflow violations
3. **time_violation.go**: Time-related violations in workflows
4. **rand_violation.go**: Randomness violations
5. **io_violation.go**: I/O operation violations
6. **goroutine_violation.go**: Goroutine usage violations
7. **channel_violation.go**: Channel creation violations
8. **cadence_workshop_test.go**: Real-world Cadence workshop example

### Example Test File - time_violation.go

```go
package testdata

import (
	"context"
	"time"
	"go.uber.org/cadence/workflow"
)

// This should trigger a violation
func MyWorkflow(ctx workflow.Context) error {
	now := time.Now() // VIOLATION: time.Now() in workflow
	time.Sleep(5 * time.Second) // VIOLATION: time.Sleep() in workflow
	return nil
}

// This should NOT trigger a violation (activity)
func MyActivity(ctx context.Context) error {
	now := time.Now() // OK: activity can use time.Now()
	time.Sleep(1 * time.Second) // OK: activity can use time.Sleep()
	return nil
}

// Helper function called from workflow - should trigger violation
func helperFunction() time.Time {
	return time.Now() // VIOLATION: reachable from workflow
}

func MyWorkflowWithHelper(ctx workflow.Context) error {
	t := helperFunction() // This call makes helperFunction workflow-reachable
	return nil
}
```

## Implementation Details

### Key Design Decisions

1. **Two-Pass Analysis**: 
   - Ensures accurate cross-file analysis
   - Builds complete call graph before detection
   - Handles complex helper function scenarios

2. **Interface-Based Design**:
   - Pluggable detector architecture
   - Easy to add new detection rules
   - Clean separation of concerns

3. **Context-Aware Detection**:
   - Distinguishes workflow from activity code
   - Avoids false positives in activity functions
   - Handles indirect calls through helper functions

4. **Configuration-Driven Rules**:
   - YAML-based rule definitions
   - Easy customization without code changes
   - Flexible severity levels and messages

### Advanced Features

1. **Import Alias Resolution**: Correctly handles import aliases like `import r "math/rand"`
2. **Call Graph Analysis**: Tracks function call relationships across files
3. **Reachability Analysis**: Determines if functions are callable from workflow contexts
4. **Method Call Detection**: Handles both function calls and method calls
5. **Template Message System**: Supports dynamic message generation with placeholders

### Performance Considerations

1. **Efficient AST Walking**: Uses Go's built-in AST walker for optimal performance
2. **Lazy Evaluation**: Only performs expensive operations when necessary
3. **Memory Management**: Reuses detector instances across files when possible
4. **Parallel Processing**: Could be extended for concurrent file processing

### Error Handling

1. **Graceful Degradation**: Continues analysis even if some files fail to parse
2. **Comprehensive Error Messages**: Provides detailed error information
3. **Validation**: Validates configuration files and command-line arguments
4. **Exit Codes**: Uses appropriate exit codes for different error conditions

This implementation provides a robust, extensible foundation for static analysis of Cadence workflows while maintaining high accuracy and performance.
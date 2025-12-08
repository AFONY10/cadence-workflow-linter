_This file has been archived. The project documentation has been simplified —
see `Overview.md` for the current high-level documentation. The original,
more detailed implementation notes have been moved to `Documentation/archive/`._


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
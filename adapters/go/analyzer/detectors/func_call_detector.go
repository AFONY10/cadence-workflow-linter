package detectors

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

// ruleInfo holds information extracted from the unified schema for a specific import+function combination
type ruleInfo struct {
	ruleID   string
	severity string
	message  string
}

type FuncCallDetector struct {
	safeImports []string
	moduleInfo  *modutils.ModuleInfo // For hybrid package classification
	ctx         FileContext
	wr          *registry.WorkflowRegistry
	currFunc    string
	pkgPath     string // package path for the current file
	issues      []Issue
	// importPath -> funcName -> ruleInfo
	functionSet map[string]map[string]ruleInfo
}

func NewFuncCallDetector(ruleSet *config.RuleSet, moduleInfo *modutils.ModuleInfo) *FuncCallDetector {
	// Extract Go-specific rules from the unified schema
	fnSet := make(map[string]map[string]ruleInfo)

	for ruleID, rule := range ruleSet.Rules {
		goLang, hasGo := rule.Languages["go"]
		if !hasGo {
			continue
		}

		// Process function_calls
		for _, fc := range goLang.FunctionCalls {
			if _, ok := fnSet[fc.Import]; !ok {
				fnSet[fc.Import] = make(map[string]ruleInfo)
			}

			// Determine severity and message (function-level overrides, or fall back to rule defaults)
			severity := fc.Severity
			if severity == "" {
				severity = rule.DefaultSeverity
			}
			message := fc.Message
			if message == "" {
				message = rule.Message
			}

			for _, fn := range fc.Functions {
				fnSet[fc.Import][fn] = ruleInfo{
					ruleID:   ruleID,
					severity: severity,
					message:  message,
				}
			}
		}
	}

	safeImports := ruleSet.GetSafeImports("go")

	return &FuncCallDetector{
		safeImports: safeImports,
		moduleInfo:  moduleInfo,
		issues:      []Issue{},
		functionSet: fnSet,
	}
}

func (d *FuncCallDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) { d.wr = reg }
func (d *FuncCallDetector) SetFileContext(ctx FileContext)                     { d.ctx = ctx }
func (d *FuncCallDetector) Issues() []Issue                                    { return d.issues }

// SetPackagePath sets the package path for canonical function naming
func (d *FuncCallDetector) SetPackagePath(pkgPath string) {
	d.pkgPath = pkgPath
}

func (d *FuncCallDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Name != nil {
			d.currFunc = n.Name.Name
		}

	case *ast.SelectorExpr:
		// pkg.Func(...)
		ident, ok := n.X.(*ast.Ident)
		if !ok {
			return d
		}
		pkgAlias := ident.Name
		importPath := d.ctx.ImportMap[pkgAlias]
		if importPath == "" {
			importPath = pkgAlias // best-effort for stdlib aliases like "time"
		}
		funcName := n.Sel.Name
		qualified := importPath + "." + funcName

		// Check function call rules
		if ruleMap, ok := d.functionSet[importPath]; ok {
			if rule, ok := ruleMap[funcName]; ok {
				d.createIssueIfInWorkflow(n, rule.ruleID, rule.severity, strings.ReplaceAll(rule.message, "%FUNC%", funcName), qualified)
				return d
			}
		}

		// Check if it's a safe external package (no issue needed)
		if d.isSafeImport(importPath) {
			return d
		}

		// Check if it's an unknown external package (not stdlib, not project internal)
		if d.isUnknownExternalPackage(importPath) {
			canonicalCurrentFunc := d.pkgPath + "." + d.currFunc
			if d.wr != nil && d.wr.IsWorkflowReachable(canonicalCurrentFunc) {
				pos := d.ctx.Fset.Position(n.Sel.Pos())
				d.issues = append(d.issues, Issue{
					File:     d.ctx.File,
					Line:     pos.Line,
					Rule:     "UnknownExternalCall",
					Severity: "info",
					Message:  fmt.Sprintf("Call to unknown external package %s.%s() - please verify it's workflow-safe", importPath, funcName),
					Func:     d.currFunc,
					Callee:   qualified,
				})
			}
		}
	}
	return d
}

// Helper method to create issue if in workflow context
func (d *FuncCallDetector) createIssueIfInWorkflow(node *ast.SelectorExpr, rule, severity, message, callee string) {
	// Check if we're in a workflow context using canonical function name
	canonicalCurrentFunc := d.pkgPath + "." + d.currFunc
	if d.wr != nil && d.wr.IsWorkflowReachable(canonicalCurrentFunc) {
		pos := d.ctx.Fset.Position(node.Sel.Pos())

		// Try to get call stack for better debugging
		callStack := d.wr.CallPathTo(canonicalCurrentFunc)

		d.issues = append(d.issues, Issue{
			File:      d.ctx.File,
			Line:      pos.Line,
			Rule:      rule,
			Severity:  severity,
			Message:   message,
			Func:      d.currFunc,
			Callee:    callee,
			CallStack: callStack,
		})
	}
}

// Helper method to check if a package is in the safe imports list
func (d *FuncCallDetector) isSafeImport(importPath string) bool {
	for _, safePkg := range d.safeImports {
		if importPath == safePkg || strings.HasPrefix(importPath, safePkg+"/") {
			return true
		}
	}
	return false
}

// Helper method to check if a package is an unknown external package
func (d *FuncCallDetector) isUnknownExternalPackage(importPath string) bool {
	// Skip standard library packages (no dots, or golang.org/x/)
	if !strings.Contains(importPath, ".") || strings.HasPrefix(importPath, "golang.org/x/") {
		return false
	}

	// Skip Cadence framework packages (these are expected and safe)
	if strings.HasPrefix(importPath, "go.uber.org/cadence") {
		return false
	}

	// Skip if it's in our known rules
	if _, exists := d.functionSet[importPath]; exists {
		return false
	}

	// Skip if it's a safe import
	if d.isSafeImport(importPath) {
		return false
	}

	// Skip if it appears to be project-internal using hybrid approach
	if d.isInternalPackage(importPath) {
		return false
	}

	// Skip testdata packages
	if strings.HasPrefix(importPath, "testdata/") || strings.HasPrefix(importPath, "example.com/linttest/") {
		return false
	}

	// If we get here, it's likely an external third-party package we don't know about
	return true
}

// isInternalPackage determines if a package is internal using hybrid approach
func (d *FuncCallDetector) isInternalPackage(importPath string) bool {
	if d.moduleInfo != nil {
		if d.moduleInfo.IsInternalPackage(importPath) {
			return true
		}
		if isReplaced, newPath := d.moduleInfo.IsReplacedPackage(importPath); isReplaced {
			if !strings.Contains(newPath, "/") || strings.HasPrefix(newPath, "./") || strings.HasPrefix(newPath, "../") {
				return true
			}
		}
	}
	if strings.HasPrefix(importPath, "github.com/afony10/cadence-workflow-linter") {
		return true
	}
	if strings.HasPrefix(importPath, "testdata/") || strings.HasPrefix(importPath, "example.com/linttest/") {
		return true
	}
	return false
}

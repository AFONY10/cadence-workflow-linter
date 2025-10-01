package detectors

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/afony10/cadence-workflow-linter/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type FuncCallDetector struct {
	rules            []config.FunctionRule
	externalRules    []config.ExternalPackageRule
	safeExternalPkgs []string
	moduleInfo       *modutils.ModuleInfo // For hybrid package classification
	ctx              FileContext
	wr               *registry.WorkflowRegistry
	currFunc         string
	pkgPath          string // package path for the current file
	issues           []Issue
	functionSet      map[string]map[string]config.FunctionRule        // importPath -> funcName -> rule
	externalFuncSet  map[string]map[string]config.ExternalPackageRule // external importPath -> funcName -> rule
}

func NewFuncCallDetector(rules []config.FunctionRule, externalRules []config.ExternalPackageRule, safeExternalPkgs []string, moduleInfo *modutils.ModuleInfo) *FuncCallDetector {
	// Build regular function rules map
	fnSet := map[string]map[string]config.FunctionRule{}
	for _, r := range rules {
		p := r.Package
		if _, ok := fnSet[p]; !ok {
			fnSet[p] = map[string]config.FunctionRule{}
		}
		for _, f := range r.Functions {
			fnSet[p][f] = r
		}
	}

	// Build external package rules map
	extFnSet := map[string]map[string]config.ExternalPackageRule{}
	for _, r := range externalRules {
		p := r.Package
		if _, ok := extFnSet[p]; !ok {
			extFnSet[p] = map[string]config.ExternalPackageRule{}
		}
		for _, f := range r.Functions {
			extFnSet[p][f] = r
		}
	}

	return &FuncCallDetector{
		rules:            rules,
		externalRules:    externalRules,
		safeExternalPkgs: safeExternalPkgs,
		moduleInfo:       moduleInfo,
		issues:           []Issue{},
		functionSet:      fnSet,
		externalFuncSet:  extFnSet,
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

		// Check regular function call rules first
		if ruleMap, ok := d.functionSet[importPath]; ok {
			if rule, ok := ruleMap[funcName]; ok {
				d.createIssueIfInWorkflow(n, rule.Rule, rule.Severity, strings.ReplaceAll(rule.Message, "%FUNC%", funcName))
				return d
			}
		}

		// Check external package rules
		if extRuleMap, ok := d.externalFuncSet[importPath]; ok {
			if extRule, ok := extRuleMap[funcName]; ok {
				d.createIssueIfInWorkflow(n, extRule.Rule, extRule.Severity, strings.ReplaceAll(extRule.Message, "%FUNC%", funcName))
				return d
			}
		}

		// Check if it's a safe external package (no issue needed)
		if d.isSafeExternalPackage(importPath) {
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
					Column:   pos.Column,
					Rule:     "UnknownExternalCall",
					Severity: "info",
					Message:  fmt.Sprintf("Call to unknown external package %s.%s() - please verify it's workflow-safe", importPath, funcName),
					Func:     d.currFunc,
				})
			}
		}
	}
	return d
}

// Helper method to create issue if in workflow context
func (d *FuncCallDetector) createIssueIfInWorkflow(node *ast.SelectorExpr, rule, severity, message string) {
	// Check if we're in a workflow context using canonical function name
	canonicalCurrentFunc := d.pkgPath + "." + d.currFunc
	if d.wr != nil && d.wr.IsWorkflowReachable(canonicalCurrentFunc) {
		pos := d.ctx.Fset.Position(node.Sel.Pos())

		// Try to get call stack for better debugging
		callStack := d.wr.CallPathTo(canonicalCurrentFunc)

		d.issues = append(d.issues, Issue{
			File:      d.ctx.File,
			Line:      pos.Line,
			Column:    pos.Column,
			Rule:      rule,
			Severity:  severity,
			Message:   message,
			Func:      d.currFunc,
			CallStack: callStack,
		})
	}
}

// Helper method to check if a package is in the safe external packages list
func (d *FuncCallDetector) isSafeExternalPackage(importPath string) bool {
	for _, safePkg := range d.safeExternalPkgs {
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

	// Skip if it's in our known external rules
	if _, exists := d.externalFuncSet[importPath]; exists {
		return false
	}

	// Skip if it's a safe external package
	if d.isSafeExternalPackage(importPath) {
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
	// Solution 1: Use go.mod information if available
	if d.moduleInfo != nil {
		// Check if it's an internal package according to go.mod
		if d.moduleInfo.IsInternalPackage(importPath) {
			return true
		}

		// Check if it's replaced by a local path (also considered internal)
		if isReplaced, newPath := d.moduleInfo.IsReplacedPackage(importPath); isReplaced {
			// If replaced with local path, consider it internal
			if !strings.Contains(newPath, "/") || strings.HasPrefix(newPath, "./") || strings.HasPrefix(newPath, "../") {
				return true
			}
		}
	}

	// Solution 3: Enhanced heuristics as fallback
	// Hardcoded project path as fallback when go.mod is not available
	if strings.HasPrefix(importPath, "github.com/afony10/cadence-workflow-linter") {
		return true
	}

	// Testdata packages are considered internal for testing purposes
	if strings.HasPrefix(importPath, "testdata/") || strings.HasPrefix(importPath, "example.com/linttest/") {
		return true
	}

	return false
}

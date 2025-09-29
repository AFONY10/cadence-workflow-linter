package detectors

import (
	"go/ast"
	"strings"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type FuncCallDetector struct {
	rules       []config.FunctionRule
	ctx         FileContext
	wr          *registry.WorkflowRegistry
	currFunc    string
	pkgPath     string // package path for the current file
	issues      []Issue
	functionSet map[string]map[string]config.FunctionRule // importPath -> funcName -> rule
}

func NewFuncCallDetector(rules []config.FunctionRule) *FuncCallDetector {
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
	return &FuncCallDetector{
		rules:       rules,
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

		if ruleMap, ok := d.functionSet[importPath]; ok {
			if rule, ok := ruleMap[funcName]; ok {
				// Check if we're in a workflow context using canonical function name
				canonicalCurrentFunc := d.pkgPath + "." + d.currFunc
				if d.wr != nil && d.wr.IsWorkflowReachable(canonicalCurrentFunc) {
					pos := d.ctx.Fset.Position(n.Sel.Pos())
					msg := strings.ReplaceAll(rule.Message, "%FUNC%", funcName)

					// Try to get call stack for better debugging
					callStack := d.wr.CallPathTo(canonicalCurrentFunc)

					d.issues = append(d.issues, Issue{
						File:      d.ctx.File,
						Line:      pos.Line,
						Column:    pos.Column,
						Rule:      rule.Rule,
						Severity:  rule.Severity,
						Message:   msg,
						Func:      d.currFunc,
						CallStack: callStack,
					})
				}
			}
		}
	}
	return d
}

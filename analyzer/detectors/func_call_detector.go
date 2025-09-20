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
	issues      []Issue
	functionSet map[string]map[string]config.FunctionRule // importPath -> funcName -> rule
}

func NewFuncCallDetector(rules []config.FunctionRule) *FuncCallDetector {
	// index rules for faster matching
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

func (d *FuncCallDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.wr = reg
}

func (d *FuncCallDetector) SetFileContext(ctx FileContext) {
	d.ctx = ctx
}

func (d *FuncCallDetector) Issues() []Issue { return d.issues }

func (d *FuncCallDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.SelectorExpr:
		// Only flag if inside known workflow function
		if d.wr != nil {
			if !d.wr.WorkflowFuncs[d.currFunc] {
				return d // skip if not a workflow function
			}

			if d.wr.ActivityFuncs[d.currFunc] {
				return d // skip if activity function
			}
		}
		ident, ok := n.X.(*ast.Ident)
		if !ok {
			return d
		}
		pkgAlias := ident.Name
		importPath := d.ctx.ImportMap[pkgAlias]
		if importPath == "" {
			// Best-effort: fall back to alias as if it were a path (for std pkgs like "time", "fmt", "os")
			importPath = pkgAlias
		}
		funcName := n.Sel.Name

		if ruleMap, ok := d.functionSet[importPath]; ok {
			if rule, ok := ruleMap[funcName]; ok {
				pos := d.ctx.Fset.Position(n.Sel.Pos())
				msg := strings.ReplaceAll(rule.Message, "%FUNC%", funcName)
				d.issues = append(d.issues, Issue{
					File:     d.ctx.File,
					Line:     pos.Line,
					Column:   pos.Column,
					Rule:     rule.Rule,
					Severity: rule.Severity,
					Message:  msg,
				})
			}
		}
	}
	return d
}

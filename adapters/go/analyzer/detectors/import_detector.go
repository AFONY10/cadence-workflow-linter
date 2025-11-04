package detectors

import (
	"go/ast"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type ImportDetector struct {
	rules  []config.ImportRule
	ctx    FileContext
	wr     *registry.WorkflowRegistry
	issues []Issue
}

func NewImportDetector(rules []config.ImportRule) *ImportDetector {
	return &ImportDetector{rules: rules, issues: []Issue{}}
}

func (d *ImportDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) { d.wr = reg }
func (d *ImportDetector) SetFileContext(ctx FileContext)                     { d.ctx = ctx }
func (d *ImportDetector) Issues() []Issue                                    { return d.issues }

func (d *ImportDetector) Visit(node ast.Node) ast.Visitor {
	if len(d.wr.WorkflowFuncs) == 0 {
		return d
	}
	switch n := node.(type) {
	case *ast.ImportSpec:
		path := ""
		if n.Path != nil && len(n.Path.Value) >= 2 {
			path = n.Path.Value[1 : len(n.Path.Value)-1]
		}
		for _, r := range d.rules {
			if r.Path == path {
				pos := d.ctx.Fset.Position(n.Pos())
				d.issues = append(d.issues, Issue{
					File:     d.ctx.File,
					Line:     pos.Line,
					Rule:     r.Rule,
					Severity: r.Severity,
					Message:  r.Message,
					Func:     "",
				})
			}
		}
	}
	return d
}

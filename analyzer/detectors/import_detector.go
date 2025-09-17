package detectors

import (
	"go/ast"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type ImportDetector struct {
	rules  []config.ImportRule
	ctx    FileContext
	wr     *registry.WorkflowRegistry
	issues []Issue
}

func NewImportDetector(rules []config.ImportRule) *ImportDetector {
	return &ImportDetector{
		rules:  rules,
		issues: []Issue{},
	}
}

func (d *ImportDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.wr = reg
}

func (d *ImportDetector) SetFileContext(ctx FileContext) {
	d.ctx = ctx
}

func (d *ImportDetector) Issues() []Issue { return d.issues }

// We flag disallowed imports only if the file contains at least one workflow.
func (d *ImportDetector) Visit(node ast.Node) ast.Visitor {
	if len(d.wr.WorkflowFuncs) == 0 {
		return d
	}
	switch n := node.(type) {
	case *ast.ImportSpec:
		pathLit := n.Path.Value // quoted, e.g. "\"math/rand\""
		path := pathLit
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}
		for _, r := range d.rules {
			if r.Path == path {
				pos := d.ctx.Fset.Position(n.Pos())
				d.issues = append(d.issues, Issue{
					File:     d.ctx.File,
					Line:     pos.Line,
					Column:   pos.Column,
					Rule:     r.Rule,
					Severity: r.Severity,
					Message:  r.Message,
				})
			}
		}
	}
	return d
}

package detectors

import (
	"go/ast"
	"strings"

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

func (d *ImportDetector) Visit(node ast.Node) ast.Visitor {
	imp, ok := node.(*ast.ImportSpec)
	if !ok {
		return d
	}

	path := strings.Trim(imp.Path.Value, `"`)
	for _, rule := range d.rules {
		if path == rule.Path {
			pos := d.ctx.Fset.Position(imp.Pos())

			// If this file only contains activities, mark as warning instead of error
			severity := rule.Severity
			if len(d.wr.WorkflowFuncs) == 0 && len(d.wr.ActivityFuncs) > 0 {
				severity = "warning"
			}

			d.issues = append(d.issues, Issue{
				File:     d.ctx.File,
				Line:     pos.Line,
				Column:   pos.Column,
				Rule:     rule.Rule,
				Severity: severity,
				Message:  rule.Message,
			})
		}
	}
	return d
}

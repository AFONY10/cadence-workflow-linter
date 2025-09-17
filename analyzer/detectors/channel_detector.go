package detectors

import (
	"go/ast"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type ChannelDetector struct {
	ctx      FileContext
	wr       *registry.WorkflowRegistry
	currFunc string
	issues   []Issue
}

func NewChannelDetector() *ChannelDetector {
	return &ChannelDetector{issues: []Issue{}}
}

func (d *ChannelDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.wr = reg
}

func (d *ChannelDetector) SetFileContext(ctx FileContext) {
	d.ctx = ctx
}

func (d *ChannelDetector) Issues() []Issue { return d.issues }

func (d *ChannelDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.CallExpr:
		// Look for make(chan ...)
		if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "make" {
			if len(n.Args) > 0 {
				if _, ok := n.Args[0].(*ast.ChanType); ok {
					if d.wr != nil && !d.wr.WorkflowFuncs[d.currFunc] {
						return d
					}
					pos := d.ctx.Fset.Position(n.Lparen)
					d.issues = append(d.issues, Issue{
						File:     d.ctx.File,
						Line:     pos.Line,
						Column:   pos.Column,
						Rule:     "Concurrency",
						Severity: "error",
						Message:  "Detected channel creation in workflow. Use workflow.Channel(ctx) instead.",
					})
				}
			}
		}
	}
	return d
}

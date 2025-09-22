package detectors

import (
	"go/ast"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type GoroutineDetector struct {
	ctx      FileContext
	wr       *registry.WorkflowRegistry
	currFunc string
	issues   []Issue
}

func NewGoroutineDetector() *GoroutineDetector {
	return &GoroutineDetector{issues: []Issue{}}
}

func (d *GoroutineDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) { d.wr = reg }
func (d *GoroutineDetector) SetFileContext(ctx FileContext)                     { d.ctx = ctx }
func (d *GoroutineDetector) Issues() []Issue                                    { return d.issues }

// Visit implements ast.Visitor
// We look for "go func()" statements inside workflow functions.
func (d *GoroutineDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.GoStmt:
		pos := d.ctx.Fset.Position(n.Go)
		d.issues = append(d.issues, Issue{
			File:     d.ctx.File,
			Line:     pos.Line,
			Column:   pos.Column,
			Rule:     "Concurrency",
			Severity: "error",
			Message:  "Detected goroutine. Use workflow.Go(ctx) inside workflows.",
			Func:     d.currFunc,
		})
	}
	return d
}

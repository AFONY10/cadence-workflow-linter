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
		// Try to extract the callee when the go statement is a call expression
		callee := ""
		if n.Call != nil {
			callExpr := n.Call
			switch fn := callExpr.Fun.(type) {
			case *ast.Ident:
				callee = fn.Name
			case *ast.SelectorExpr:
				// pkg.Func or receiver.Method
				if id, ok := fn.X.(*ast.Ident); ok {
					callee = id.Name + "." + fn.Sel.Name
				} else {
					callee = fn.Sel.Name
				}
			}
		}

		pos := d.ctx.Fset.Position(n.Go)
		iss := Issue{
			File:     d.ctx.File,
			Line:     pos.Line,
			Column:   pos.Column,
			Rule:     "Concurrency",
			Severity: "error",
			Message:  "Detected goroutine. Use workflow.Go(ctx) inside workflows.",
			Func:     d.currFunc,
		}
		if callee != "" {
			iss.Callee = callee
		}
		d.issues = append(d.issues, iss)
	}
	return d
}

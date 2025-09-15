package detectors

import (
	"go/ast"
	"go/token"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type TimeUsageDetector struct {
	file        string
	fset        *token.FileSet
	workflowReg *registry.WorkflowRegistry
	currFunc    string
	issues      []Issue
}

func NewTimeUsageDetector() *TimeUsageDetector {
	return &TimeUsageDetector{issues: []Issue{}}
}

func (d *TimeUsageDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.workflowReg = reg
}

func (d *TimeUsageDetector) SetFileContext(file string, fset *token.FileSet) {
	d.file, d.fset = file, fset
}

func (d *TimeUsageDetector) Issues() []Issue { return d.issues }

func (d *TimeUsageDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.SelectorExpr:
		// Only flag if we're inside a workflow function.
		if d.workflowReg != nil && !d.workflowReg.WorkflowFuncs[d.currFunc] {
			return d
		}

		// Match: time.Now() or time.Since(...)
		if ident, ok := n.X.(*ast.Ident); ok && ident.Name == "time" &&
			(n.Sel.Name == "Now" || n.Sel.Name == "Since") {
			pos := d.fset.Position(n.Sel.Pos())
			d.issues = append(d.issues, Issue{
				File:    d.file,
				Line:    pos.Line,
				Column:  pos.Column,
				Rule:    "TimeUsage",
				Message: "Detected time." + n.Sel.Name + "() in workflow. Use workflow.Now(ctx)/workflow.Sleep(ctx) instead.",
			})
		}
	}
	return d
}

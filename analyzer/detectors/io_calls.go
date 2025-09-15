package detectors

import (
	"go/ast"
	"go/token"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type IOCallsDetector struct {
	file        string
	fset        *token.FileSet
	workflowReg *registry.WorkflowRegistry
	currFunc    string
	issues      []Issue
}

func NewIOCallsDetector() *IOCallsDetector {
	return &IOCallsDetector{issues: []Issue{}}
}

func (d *IOCallsDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.workflowReg = reg
}

func (d *IOCallsDetector) SetFileContext(file string, fset *token.FileSet) {
	d.file, d.fset = file, fset
}

func (d *IOCallsDetector) Issues() []Issue { return d.issues }

func (d *IOCallsDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.SelectorExpr:
		if d.workflowReg != nil && !d.workflowReg.WorkflowFuncs[d.currFunc] {
			return d
		}

		// Disallow file I/O (os.Open/OpenFile/ReadFile/WriteFile etc.)
		if ident, ok := n.X.(*ast.Ident); ok && ident.Name == "os" {
			switch n.Sel.Name {
			case "Open", "OpenFile", "ReadFile", "WriteFile", "Mkdir", "Remove":
				pos := d.fset.Position(n.Sel.Pos())
				d.issues = append(d.issues, Issue{
					File:    d.file,
					Line:    pos.Line,
					Column:  pos.Column,
					Rule:    "IOCalls",
					Message: "Detected os." + n.Sel.Name + "() in workflow. Avoid file I/O inside workflows.",
				})
			}
		}

		// Also disallow stdout logging via fmt.Println inside workflows
		if ident, ok := n.X.(*ast.Ident); ok && ident.Name == "fmt" && n.Sel.Name == "Println" {
			pos := d.fset.Position(n.Sel.Pos())
			d.issues = append(d.issues, Issue{
				File:    d.file,
				Line:    pos.Line,
				Column:  pos.Column,
				Rule:    "IOCalls",
				Message: "Detected fmt.Println() in workflow. Use workflow.GetLogger(ctx) instead.",
			})
		}
	}
	return d
}

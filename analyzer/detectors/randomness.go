package detectors

import (
	"go/ast"
	"go/token"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type RandomnessDetector struct {
	file        string
	fset        *token.FileSet
	workflowReg *registry.WorkflowRegistry
	currFunc    string
	issues      []Issue
}

func NewRandomnessDetector() *RandomnessDetector {
	return &RandomnessDetector{issues: []Issue{}}
}

func (d *RandomnessDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) {
	d.workflowReg = reg
}

func (d *RandomnessDetector) SetFileContext(file string, fset *token.FileSet) {
	d.file, d.fset = file, fset
}

func (d *RandomnessDetector) Issues() []Issue { return d.issues }

func (d *RandomnessDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		d.currFunc = n.Name.Name

	case *ast.SelectorExpr:
		if d.workflowReg != nil && !d.workflowReg.WorkflowFuncs[d.currFunc] {
			return d
		}

		// Match: rand.Intn / rand.Int / rand.Float32 / rand.Float64
		if ident, ok := n.X.(*ast.Ident); ok && ident.Name == "rand" {
			switch n.Sel.Name {
			case "Intn", "Int", "Float32", "Float64", "Read":
				pos := d.fset.Position(n.Sel.Pos())
				d.issues = append(d.issues, Issue{
					File:    d.file,
					Line:    pos.Line,
					Column:  pos.Column,
					Rule:    "Randomness",
					Message: "Detected rand." + n.Sel.Name + "() in workflow. Avoid nondeterminism; use workflow.SideEffect if needed.",
				})
			}
		}
	}
	return d
}

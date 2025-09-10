package detectors

import (
	"go/ast"
	"go/token"
)

type RandomnessDetector struct{}

func (d RandomnessDetector) Name() string {
	return "Randomness"
}

func (d RandomnessDetector) Detect(node ast.Node, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.SelectorExpr)
		if ok {
			ident, ok := callExpr.X.(*ast.Ident)
			if ok && ident.Name == "rand" {
				pos := fset.Position(callExpr.Pos())
				issues = append(issues, Issue{
					Message: "[" + d.Name() + "] Detected rand." + callExpr.Sel.Name + "() - avoid randomness in workflows.",
					Line:    pos.Line,
					Column:  pos.Column,
				})
			}
		}
		return true
	})

	return issues
}

package detectors

import (
	"go/ast"
	"go/token"
)

type IOCallsDetector struct{}

func (d IOCallsDetector) Name() string {
	return "IOCalls"
}

func (d IOCallsDetector) Detect(node ast.Node, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.SelectorExpr)
		if ok {
			ident, ok := callExpr.X.(*ast.Ident)
			if ok && ident.Name == "os" {
				if callExpr.Sel.Name == "Open" || callExpr.Sel.Name == "ReadFile" {
					pos := fset.Position(callExpr.Pos())
					issues = append(issues, Issue{
						Message: "[" + d.Name() + "] Detected os." + callExpr.Sel.Name + "() - file I/O not allowed in workflows.",
						Line:    pos.Line,
						Column:  pos.Column,
					})
				}
			}
		}
		return true
	})

	return issues
}

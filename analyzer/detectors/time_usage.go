// TODO: Implement analyzer feature to detect time usage issues

package detectors

import (
	"go/ast"
	"go/token"
)

type TimeUsageDetector struct{}

func (d TimeUsageDetector) Name() string {
	return "TimeUsage"
}

func (d TimeUsageDetector) Detect(node ast.Node, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.SelectorExpr)
		if ok {
			ident, ok := callExpr.X.(*ast.Ident)
			if ok && ident.Name == "time" {
				if callExpr.Sel.Name == "Now" || callExpr.Sel.Name == "Since" {
					pos := fset.Position(callExpr.Pos())
					issues = append(issues, Issue{
						Message: "[" + d.Name() + "] Detected time." + callExpr.Sel.Name + "() - use workflow.Now(ctx) instead.",
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

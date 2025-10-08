package detectors

import (
	"go/ast"
	"strings"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

// MapIterationDetector flags range loops over maps inside workflow context
type MapIterationDetector struct {
	rules   []config.FunctionRule // reuse function rule structure for config if desired
	ctx     FileContext
	wr      *registry.WorkflowRegistry // keep registry reference if needed
	issues  []Issue
	pkgPath string
}

func NewMapIterationDetector(rules []config.FunctionRule) *MapIterationDetector {
	return &MapIterationDetector{rules: rules, issues: []Issue{}}
}

func (d *MapIterationDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) { d.wr = reg }

func (d *MapIterationDetector) SetFileContext(ctx FileContext) { d.ctx = ctx }
func (d *MapIterationDetector) SetPackagePath(pkgPath string)  { d.pkgPath = pkgPath }
func (d *MapIterationDetector) Issues() []Issue                { return d.issues }

func (d *MapIterationDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.RangeStmt:
		// We only care about `for k := range m {}` style or `for k, v := range m {}`
		// where m is a map. The AST doesn't give us static types here, so use
		// heuristics: if the expression is an Ident whose name is a map literal or
		// a composite literal keyed by map type, or a SelectorExpr/Ident that is a
		// known import that returns a map (best effort). We're conservative and
		// only flag simple cases like map literals and obvious identifiers named with "Map" suffix.

		// Case: range over a composite literal: `for k := range map[string]int{"a":1} {}`
		switch expr := n.X.(type) {
		case *ast.CompositeLit:
			if _, ok := expr.Type.(*ast.MapType); ok {
				d.addIssue(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.")
			}
		case *ast.Ident:
			// Heuristic: variable name ends with "Map" or "map"
			if strings.HasSuffix(expr.Name, "Map") || strings.HasSuffix(expr.Name, "map") {
				d.addIssue(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.")
			}
		case *ast.SelectorExpr:
			// Heuristic: pkg.MapVar or function returning map - best-effort based on name
			if expr.Sel != nil {
				if strings.HasSuffix(expr.Sel.Name, "Map") || strings.HasSuffix(expr.Sel.Name, "map") {
					d.addIssue(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.")
				}
			}
		}
	}
	return d
}

func (d *MapIterationDetector) addIssue(n ast.Node, rule, severity, message string) {
	if d.ctx.Fset == nil {
		return
	}
	pos := d.ctx.Fset.Position(n.Pos())
	d.issues = append(d.issues, Issue{
		File:     d.ctx.File,
		Line:     pos.Line,
		Column:   pos.Column,
		Rule:     rule,
		Severity: severity,
		Message:  message,
	})
}

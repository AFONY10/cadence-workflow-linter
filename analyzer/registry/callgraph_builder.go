// analyzer/registry/callgraph_builder.go (new)
package registry

import (
	"go/ast"
	"strings"
)

type Edge struct{ Caller, Callee string }

// BuildEdges inspects one file and returns call edges (canonicalized).
// callerName should be the canonical "pkgPath.Func", passed per function.
func BuildEdges(file *ast.File, pkgPath string, importMap map[string]string) []Edge {
	var edges []Edge

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}

		caller := canonical(pkgPath, fn.Name.Name)

		ast.Inspect(fn.Body, func(m ast.Node) bool {
			call, ok := m.(*ast.CallExpr)
			if !ok {
				return true
			}
			// foo()
			if ident, ok := call.Fun.(*ast.Ident); ok {
				edges = append(edges, Edge{
					Caller: caller,
					Callee: canonical(pkgPath, ident.Name),
				})
				return true
			}
			// alias.Func()
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if recv, ok := sel.X.(*ast.Ident); ok {
					alias := recv.Name
					imp := importMap[alias]
					if imp == "" {
						// best-effort: if no import mapping, fall back to alias
						imp = alias
					}
					edges = append(edges, Edge{
						Caller: caller,
						Callee: canonical(imp, sel.Sel.Name),
					})
				}
			}
			return true
		})
		return true
	})

	return edges
}

func canonical(pkgOrImportPath, funcName string) string {
	// ensure pkg path is something like "github.com/me/proj/pkg" or "time"
	p := strings.TrimSpace(pkgOrImportPath)
	if p == "" {
		p = "local"
	}
	return p + "." + funcName
}

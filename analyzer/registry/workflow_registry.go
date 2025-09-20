package registry

import (
	"go/ast"
)

type WorkflowRegistry struct {
	WorkflowFuncs map[string]bool
	ActivityFuncs map[string]bool
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		WorkflowFuncs: make(map[string]bool),
		ActivityFuncs: make(map[string]bool),
	}
}

func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
	if fn, ok := node.(*ast.FuncDecl); ok {
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				switch t := param.Type.(type) {
				case *ast.SelectorExpr:
					// workflow.Context
					if t.Sel.Name == "Context" {
						if pkgIdent, ok := t.X.(*ast.Ident); ok {
							if pkgIdent.Name == "workflow" {
								wr.WorkflowFuncs[fn.Name.Name] = true
							} else if pkgIdent.Name == "context" {
								wr.ActivityFuncs[fn.Name.Name] = true
							}
						}
					}
				}
			}
		}
	}
	// Workflow registration calls
	if call, ok := node.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" {
				switch sel.Sel.Name {
				case "Register", "RegisterWithOptions":
					if len(call.Args) == 2 {
						if fnIdent, ok := call.Args[1].(*ast.Ident); ok {
							wr.WorkflowFuncs[fnIdent.Name] = true
						}
					}
				case "RegisterActivity", "RegisterActivityWithOptions":
					if len(call.Args) >= 1 {
						if fnIdent, ok := call.Args[0].(*ast.Ident); ok {
							wr.ActivityFuncs[fnIdent.Name] = true
						}
					}
				}
			}
		}
	}
	return wr
}

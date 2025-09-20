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
	// Workflows: func with workflow.Context param
	if fn, ok := node.(*ast.FuncDecl); ok {
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				if sel, ok := param.Type.(*ast.SelectorExpr); ok {
					if sel.Sel.Name == "Context" {
						wr.WorkflowFuncs[fn.Name.Name] = true
					}
				}
			}
		}
	}

	// Workflows: workflow.Register(...)
	if call, ok := node.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" {
				if sel.Sel.Name == "Register" || sel.Sel.Name == "RegisterWithOptions" {
					if len(call.Args) == 2 {
						if fnIdent, ok := call.Args[1].(*ast.Ident); ok {
							wr.WorkflowFuncs[fnIdent.Name] = true
						}
					}
				}
				// Activities: workflow.RegisterActivity(...)
				if sel.Sel.Name == "RegisterActivity" || sel.Sel.Name == "RegisterActivityWithOptions" {
					if len(call.Args) == 1 {
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

package registry

import "go/ast"

type WorkflowRegistry struct {
	WorkflowFuncs map[string]bool
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		WorkflowFuncs: make(map[string]bool),
	}
}

// Walk the AST and register functions with workflow.Context params or registered via workflow.Register
func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
	// Function declarations with workflow.Context
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

	// workflow.Register("MyWorkflow", MyWorkflow)
	if call, ok := node.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" && sel.Sel.Name == "Register" {
				if len(call.Args) == 2 {
					if fnIdent, ok := call.Args[1].(*ast.Ident); ok {
						wr.WorkflowFuncs[fnIdent.Name] = true
					}
				}
			}
		}
	}

	return wr
}

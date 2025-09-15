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

// Walk the AST and register functions with workflow.Context params
func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
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
	return wr
}

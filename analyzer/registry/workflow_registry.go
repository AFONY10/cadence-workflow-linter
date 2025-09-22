package registry

import (
	"go/ast"
)

// WorkflowRegistry tracks which functions are workflows, which are activities,
// and a call graph (who calls who). It also provides reachability and call-stack helpers.
type WorkflowRegistry struct {
	WorkflowFuncs map[string]bool     // functions that take workflow.Context or registered as workflow
	ActivityFuncs map[string]bool     // functions that take context.Context or registered as activity
	CallGraph     map[string][]string // caller -> []callees
}

// NewWorkflowRegistry creates a fresh registry instance.
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		WorkflowFuncs: make(map[string]bool),
		ActivityFuncs: make(map[string]bool),
		CallGraph:     make(map[string][]string),
	}
}

// Visit is called by ast.Walk for each node in the AST.
// We classify functions (workflow/activity) and record call graph edges.
func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
	// Classify functions by signature (workflow.Context vs context.Context)
	if fn, ok := node.(*ast.FuncDecl); ok {
		// 1) Classify by parameters (workflow.Context vs context.Context)
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				// Expect SelectorExpr like: workflow.Context or context.Context
				if sel, ok := param.Type.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && sel.Sel.Name == "Context" {
						switch ident.Name {
						case "workflow":
							wr.WorkflowFuncs[fn.Name.Name] = true
						case "context":
							wr.ActivityFuncs[fn.Name.Name] = true
						}
					}
				}
			}
		}

		// 2) Build call graph from inside the function body
		if fn.Body != nil {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// Normal direct function calls: foo()
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok {
						wr.CallGraph[fn.Name.Name] = append(wr.CallGraph[fn.Name.Name], ident.Name)
					}
					// NOTE: calls like pkg.Func() show up as *ast.SelectorExpr; for helper recursion,
					// we care primarily about Ident (user funcs). You can extend this later if needed.
				}
				return true
			})
		}
		return wr
	}

	// Classify by registration calls (workflow.Register / RegisterActivity)
	if call, ok := node.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "workflow" {
				switch sel.Sel.Name {
				case "Register", "RegisterWithOptions":
					// workflow.Register(name, MyWorkflow)
					if len(call.Args) == 2 {
						if wfIdent, ok := call.Args[1].(*ast.Ident); ok {
							wr.WorkflowFuncs[wfIdent.Name] = true
						}
					}
				case "RegisterActivity", "RegisterActivityWithOptions":
					// workflow.RegisterActivity(MyActivity)
					if len(call.Args) >= 1 {
						if actIdent, ok := call.Args[0].(*ast.Ident); ok {
							wr.ActivityFuncs[actIdent.Name] = true
						}
					}
				}
			}
		}
	}
	return wr
}

// ReachableFromWorkflows returns a set of functions that are reachable
// from any workflow function by following call graph edges (excludes activities).
func (wr *WorkflowRegistry) ReachableFromWorkflows() map[string]bool {
	reach := make(map[string]bool)
	visited := make(map[string]bool)
	for wf := range wr.WorkflowFuncs {
		wr.collectReachable(wf, reach, visited)
	}
	return reach
}

func (wr *WorkflowRegistry) collectReachable(fn string, reach, visited map[string]bool) {
	if visited[fn] {
		return
	}
	visited[fn] = true
	reach[fn] = true

	for _, callee := range wr.CallGraph[fn] {
		// Skip activities in reachability.
		if wr.ActivityFuncs[callee] {
			continue
		}
		wr.collectReachable(callee, reach, visited)
	}
}

// CallPathTo returns one simple call path (as a slice of function names)
// from any workflow function to the target function, if one exists.
// Used to attach a "call stack" for explanation.
func (wr *WorkflowRegistry) CallPathTo(target string) []string {
	// BFS from all workflow funcs
	type qitem struct {
		name string
		path []string
	}
	seen := make(map[string]bool)
	var q []qitem

	for wf := range wr.WorkflowFuncs {
		q = append(q, qitem{name: wf, path: []string{wf}})
		seen[wf] = true
	}

	for len(q) > 0 {
		cur := q[0]
		q = q[1:]

		if cur.name == target {
			return cur.path
		}

		for _, callee := range wr.CallGraph[cur.name] {
			// Skip activities in call path
			if wr.ActivityFuncs[callee] {
				continue
			}
			if !seen[callee] {
				seen[callee] = true
				next := append(append([]string{}, cur.path...), callee)
				q = append(q, qitem{name: callee, path: next})
			}
		}
	}
	return nil
}

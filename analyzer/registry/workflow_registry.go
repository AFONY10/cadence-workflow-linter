package registry

import (
	"go/ast"
)

// WorkflowRegistry tracks which functions are workflows, which are activities,
// and a call graph (who calls who). It also provides reachability and call-stack helpers.
type WorkflowRegistry struct {
	WorkflowFuncs map[string]bool     // functions that take workflow.Context (canonical: "pkgPath.Func")
	ActivityFuncs map[string]bool     // functions that take context.Context (canonical: "pkgPath.Func")
	CallGraph     map[string][]string // caller -> []callees (canonical names)
}

// MarkWorkflow marks a function as a workflow using canonical naming
func (wr *WorkflowRegistry) MarkWorkflow(pkgPath, funcName string) {
	wr.WorkflowFuncs[canonical(pkgPath, funcName)] = true
}

// MarkActivity marks a function as an activity using canonical naming
func (wr *WorkflowRegistry) MarkActivity(pkgPath, funcName string) {
	wr.ActivityFuncs[canonical(pkgPath, funcName)] = true
}

// AddEdges adds call graph edges to the registry
func (wr *WorkflowRegistry) AddEdges(edges []Edge) {
	for _, e := range edges {
		wr.CallGraph[e.Caller] = append(wr.CallGraph[e.Caller], e.Callee)
	}
}

// IsWorkflowReachable determines if a function (in canonical form) is reachable from workflow code
func (wr *WorkflowRegistry) IsWorkflowReachable(canonicalFuncName string) bool {
	// Direct workflow function
	if wr.WorkflowFuncs[canonicalFuncName] {
		return true
	}

	// Check if reachable from any workflow function via call graph
	visited := make(map[string]bool)
	return wr.isReachableFrom(canonicalFuncName, wr.WorkflowFuncs, visited)
}

// isReachableFrom performs recursive reachability analysis
func (wr *WorkflowRegistry) isReachableFrom(target string, sources map[string]bool, visited map[string]bool) bool {
	if visited[target] {
		return false // Avoid infinite loops
	}
	visited[target] = true

	// Check if any source directly calls the target
	for source := range sources {
		for _, callee := range wr.CallGraph[source] {
			if callee == target {
				return true
			}
		}
	}

	// Recursively check indirect calls
	nextLevel := make(map[string]bool)
	for source := range sources {
		for _, callee := range wr.CallGraph[source] {
			nextLevel[callee] = true
		}
	}

	if len(nextLevel) > 0 {
		return wr.isReachableFrom(target, nextLevel, visited)
	}

	return false
}

// NewWorkflowRegistry creates a fresh registry instance.
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		WorkflowFuncs: make(map[string]bool),
		ActivityFuncs: make(map[string]bool),
		CallGraph:     make(map[string][]string),
	}
}

// ProcessFile analyzes a single file to classify functions and build call graph edges
// This replaces the old Visit method with a more structured approach
func (wr *WorkflowRegistry) ProcessFile(file *ast.File, pkgPath string, importMap map[string]string) {
	// 1) Classify functions by signature (workflow.Context vs context.Context)
	ast.Inspect(file, func(node ast.Node) bool {
		if fn, ok := node.(*ast.FuncDecl); ok && fn.Name != nil {
			if fn.Type.Params != nil {
				for _, param := range fn.Type.Params.List {
					// Expect SelectorExpr like: workflow.Context or context.Context
					if sel, ok := param.Type.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok && sel.Sel.Name == "Context" {
							switch ident.Name {
							case "workflow":
								wr.MarkWorkflow(pkgPath, fn.Name.Name)
							case "context":
								wr.MarkActivity(pkgPath, fn.Name.Name)
							}
						}
					}
				}
			}
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
								wr.MarkWorkflow(pkgPath, wfIdent.Name)
							}
						}
					case "RegisterActivity", "RegisterActivityWithOptions":
						// workflow.RegisterActivity(MyActivity)
						if len(call.Args) >= 1 {
							if actIdent, ok := call.Args[0].(*ast.Ident); ok {
								wr.MarkActivity(pkgPath, actIdent.Name)
							}
						}
					}
				}
			}
		}
		return true
	})

	// 2) Build call graph edges using the new builder
	edges := BuildEdges(file, pkgPath, importMap)
	wr.AddEdges(edges)
}

// Visit is kept for backward compatibility but should be replaced with ProcessFile
// Deprecated: Use ProcessFile instead for better package-aware analysis
func (wr *WorkflowRegistry) Visit(node ast.Node) ast.Visitor {
	// Classify functions by signature (workflow.Context vs context.Context)
	if fn, ok := node.(*ast.FuncDecl); ok && fn.Name != nil {
		// 1) Classify by parameters (workflow.Context vs context.Context)
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				// Expect SelectorExpr like: workflow.Context or context.Context
				if sel, ok := param.Type.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && sel.Sel.Name == "Context" {
						switch ident.Name {
						case "workflow":
							// Use "local" as fallback package path for backward compatibility
							wr.MarkWorkflow("local", fn.Name.Name)
						case "context":
							wr.MarkActivity("local", fn.Name.Name)
						}
					}
				}
			}
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
							wr.MarkWorkflow("local", wfIdent.Name)
						}
					}
				case "RegisterActivity", "RegisterActivityWithOptions":
					// workflow.RegisterActivity(MyActivity)
					if len(call.Args) >= 1 {
						if actIdent, ok := call.Args[0].(*ast.Ident); ok {
							wr.MarkActivity("local", actIdent.Name)
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

// GetCallStack provides debugging information for call paths from workflow to target
func (wr *WorkflowRegistry) GetCallStack(from, to string) []string {
	visited := make(map[string]bool)
	path := []string{}
	if wr.findPath(from, to, visited, &path) {
		return path
	}
	return nil
}

// findPath performs recursive path finding for call stack construction
func (wr *WorkflowRegistry) findPath(from, to string, visited map[string]bool, path *[]string) bool {
	if visited[from] {
		return false
	}
	visited[from] = true
	*path = append(*path, from)

	if from == to {
		return true
	}

	for _, callee := range wr.CallGraph[from] {
		if wr.findPath(callee, to, visited, path) {
			return true
		}
	}

	*path = (*path)[:len(*path)-1] // Backtrack
	return false
}

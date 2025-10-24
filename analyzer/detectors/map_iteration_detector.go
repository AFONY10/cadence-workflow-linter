package detectors

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

// MapIterationDetector flags range loops over maps inside workflow context.
// It uses type information (go/types) when available to detect map ranges
// precisely, and falls back to conservative heuristics otherwise. It also
// checks workflow reachability via the WorkflowRegistry before emitting issues.
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

// findEnclosingFunc finds the name of the function that contains the given
// position by scanning top-level declarations in the file. Returns empty string
// if not inside a function.
func findEnclosingFunc(f *ast.File, pos token.Pos) string {
	if f == nil {
		return ""
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil {
			if fn.Pos() <= pos && pos <= fn.End() {
				return fn.Name.Name
			}
		}
	}
	return ""
}

// findEnclosingFuncDecl returns the *ast.FuncDecl that contains pos, or nil.
func findEnclosingFuncDecl(f *ast.File, pos token.Pos) *ast.FuncDecl {
	if f == nil {
		return nil
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			if fn.Pos() <= pos && pos <= fn.End() {
				return fn
			}
		}
	}
	return nil
}

func isMapCompositeLit(e ast.Expr) bool {
	if cl, ok := e.(*ast.CompositeLit); ok {
		if _, ok := cl.Type.(*ast.MapType); ok {
			return true
		}
	}
	return false
}

// varIsMapInFunc heuristically checks if a variable with the given name is
// initialized as a map inside the provided function declaration (covers `:=`
// assignments and value specs).
func varIsMapInFunc(fd *ast.FuncDecl, varName string) bool {
	if fd == nil || fd.Body == nil {
		return false
	}
	for _, stmt := range fd.Body.List {
		switch s := stmt.(type) {
		case *ast.DeclStmt:
			if gd, ok := s.Decl.(*ast.GenDecl); ok {
				for _, spec := range gd.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range vs.Names {
							if name.Name == varName {
								if vs.Type != nil {
									if _, ok := vs.Type.(*ast.MapType); ok {
										return true
									}
								}
								if i < len(vs.Values) {
									if isMapCompositeLit(vs.Values[i]) {
										return true
									}
								}
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == varName {
					if i < len(s.Rhs) {
						if isMapCompositeLit(s.Rhs[i]) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func (d *MapIterationDetector) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.RangeStmt:
		// Attempt to use precise type information when available.
		if d.ctx.TypesInfo != nil {
			// Try common cases: identifier (variable) or expression type from Types map
			var typ types.Type
			switch e := n.X.(type) {
			case *ast.Ident:
				// Look up the object in Uses or Defs
				var obj types.Object
				if d.ctx.TypesInfo.Uses != nil {
					if o, ok := d.ctx.TypesInfo.Uses[e]; ok {
						obj = o
					}
				}
				if obj == nil && d.ctx.TypesInfo.Defs != nil {
					if o, ok := d.ctx.TypesInfo.Defs[e]; ok {
						obj = o
					}
				}
				if obj != nil {
					typ = obj.Type()
				}
			default:
				// For non-identifier expressions we currently don't extract the type
				// from TypesInfo to avoid Go version differences; rely on composite
				// literal handling below as a safe fallback.
			}
			if typ != nil {
				if _, ok := typ.Underlying().(*types.Map); ok {
					// Determine enclosing function and canonical name
					funcName := findEnclosingFunc(d.ctx.Node, n.Pos())
					canonical := d.pkgPath + "." + funcName
					if d.wr != nil {
						if reachable, path := d.wr.IsReachableWithPath(canonical); reachable {
							d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", path)
						}
					} else {
						// No registry: conservatively add issue
						d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", nil)
					}
				}
			}
		} else {
			// Fallback heuristic when types not available: leave existing heuristics
			switch expr := n.X.(type) {
			case *ast.CompositeLit:
				if _, ok := expr.Type.(*ast.MapType); ok {
					funcName := findEnclosingFunc(d.ctx.Node, n.Pos())
					canonical := d.pkgPath + "." + funcName
					if d.wr != nil {
						if reachable, path := d.wr.IsReachableWithPath(canonical); reachable {
							d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", path)
						}
					} else {
						d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", nil)
					}
				}
			case *ast.Ident:
				// Heuristic: variable name suffix OR local declaration initialization
				if len(expr.Name) >= 3 && (expr.Name[len(expr.Name)-3:] == "Map" || expr.Name[len(expr.Name)-3:] == "map") {
					funcName := findEnclosingFunc(d.ctx.Node, n.Pos())
					canonical := d.pkgPath + "." + funcName
					if d.wr != nil {
						if reachable, path := d.wr.IsReachableWithPath(canonical); reachable {
							d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", path)
						}
					} else {
						d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", nil)
					}
				} else {
					// Check if the variable was initialized with a map literal in the enclosing function
					fd := findEnclosingFuncDecl(d.ctx.Node, n.Pos())
					if fd != nil && varIsMapInFunc(fd, expr.Name) {
						funcName := findEnclosingFunc(d.ctx.Node, n.Pos())
						canonical := d.pkgPath + "." + funcName
						if d.wr != nil {
							if reachable, path := d.wr.IsReachableWithPath(canonical); reachable {
								d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", path)
							}
						} else {
							d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", nil)
						}
					}
				}
			case *ast.SelectorExpr:
				if expr.Sel != nil {
					name := expr.Sel.Name
					if len(name) >= 3 && (name[len(name)-3:] == "Map" || name[len(name)-3:] == "map") {
						funcName := findEnclosingFunc(d.ctx.Node, n.Pos())
						canonical := d.pkgPath + "." + funcName
						if d.wr != nil {
							if reachable, path := d.wr.IsReachableWithPath(canonical); reachable {
								d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", path)
							}
						} else {
							d.addIssueWithStack(n, "MapIteration", "warning", "Iteration over map keys is nondeterministic; avoid range over maps in workflows or make the order deterministic.", nil)
						}
					}
				}
			}
		}
	}
	return d
}

func (d *MapIterationDetector) addIssueWithStack(n ast.Node, rule, severity, message string, stack []string) {
	if d.ctx.Fset == nil {
		return
	}
	pos := d.ctx.Fset.Position(n.Pos())
	iss := Issue{
		File:     d.ctx.File,
		Line:     pos.Line,
		Column:   pos.Column,
		Rule:     rule,
		Severity: severity,
		Message:  message,
	}
	if len(stack) > 0 {
		iss.CallStack = stack
		// Optionally set Func to the last element (the target)
		iss.Func = stack[len(stack)-1]
	}
	d.issues = append(d.issues, iss)
}

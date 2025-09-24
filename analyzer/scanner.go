package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

// internal structure of a parsed file, with import map
type parsedFile struct {
	filename  string
	fset      *token.FileSet
	node      *ast.File
	importMap map[string]string
}

// Build an alias->import map for the file (e.g., r -> math/rand)
func buildImportMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			if i := strings.LastIndex(path, "/"); i >= 0 {
				alias = path[i+1:]
			} else {
				alias = path
			}
		}
		m[alias] = path
	}
	return m
}

// First pass: parse files and build the global registry (workflows, activities, call graph)
func parseAllAndBuildRegistry(target string) ([]parsedFile, *registry.WorkflowRegistry, error) {
	var files []parsedFile
	wr := registry.NewWorkflowRegistry()

	addFile := func(path string) error {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, src, parser.AllErrors)
		if err != nil {
			return err
		}
		files = append(files, parsedFile{
			filename:  path,
			fset:      fset,
			node:      node,
			importMap: buildImportMap(node),
		})
		// walk registry on this file
		ast.Walk(wr, node)
		return nil
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		err = filepath.Walk(target, func(path string, fi os.FileInfo, _ error) error {
			if fi == nil || fi.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			return addFile(path)
		})
	} else {
		err = addFile(target)
	}
	if err != nil {
		return nil, nil, err
	}

	return files, wr, nil
}

// Second pass: run detectors on each file with global registry, then filter/enrich issues.
func runDetectors(files []parsedFile, wr *registry.WorkflowRegistry, factory func() []ast.Visitor) ([]detectors.Issue, error) {
	var all []detectors.Issue

	// Build reachability set once
	reachable := wr.ReachableFromWorkflows()

	// 1) Run detectors over all files, collect raw issues
	for _, pf := range files {
		visitors := factory()
		ctx := detectors.FileContext{File: pf.filename, Fset: pf.fset, ImportMap: pf.importMap}
		for _, v := range visitors {
			if wa, ok := v.(detectors.WorkflowAware); ok {
				wa.SetWorkflowRegistry(wr)
			}
			if fca, ok := v.(detectors.FileContextAware); ok {
				fca.SetFileContext(ctx)
			}
			ast.Walk(v, pf.node)
			if ip, ok := v.(detectors.IssueProvider); ok {
				all = append(all, ip.Issues()...)
			}
		}
	}

	// 2) Filter issues to only those in workflow-reachable functions (and not activities)
	filtered := make([]detectors.Issue, 0, len(all))
	for _, is := range all {
		// file-level issues (imports) have empty Func => only keep if file contains workflows
		if is.Func == "" {
			// keep as-is; ImportDetector already gated on workflows present in file
			filtered = append(filtered, is)
			continue
		}

		// If function is an activity, drop (no false positives there)
		if wr.ActivityFuncs[is.Func] {
			continue
		}

		// Keep only if this function is reachable from a workflow
		if reachable[is.Func] {
			// attach a simple call stack (path)
			if path := wr.CallPathTo(is.Func); len(path) > 0 {
				is.CallStack = path
			}
			filtered = append(filtered, is)
		}
	}

	return filtered, nil
}

// Public API: ScanFile or ScanDirectory using two-pass analysis
func ScanFile(path string, factory func() []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, err := parseAllAndBuildRegistry(path)
	if err != nil {
		return nil, err
	}
	return runDetectors(files, wr, factory)
}

func ScanDirectory(root string, factory func() []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, err := parseAllAndBuildRegistry(root)
	if err != nil {
		return nil, err
	}
	return runDetectors(files, wr, factory)
}
